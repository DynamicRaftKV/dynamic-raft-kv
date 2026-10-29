package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

type registerCall struct {
	start, end uint64
	write      bool
	value      string
	found      bool
	unknown    bool
}

// A bounded, exhaustive single-register checker. Real-time predecessors must
// appear first. Failed writes may have taken effect or may be omitted; failed
// reads are not observations. This is a test oracle, not production code.
func linearizable(h []registerCall) bool {
	if len(h) > 16 {
		panic("history checker bound exceeded")
	}
	seen := map[string]bool{}
	var search func(uint64, string, bool) bool
	search = func(done uint64, value string, exists bool) bool {
		if done == (uint64(1)<<len(h))-1 {
			return true
		}
		key := fmt.Sprintf("%d/%t/%q", done, exists, value)
		if seen[key] {
			return false
		}
		seen[key] = true
		for i, op := range h {
			bit := uint64(1) << i
			if done&bit != 0 {
				continue
			}
			eligible := true
			for j, prior := range h {
				if prior.end < op.start && done&(uint64(1)<<j) == 0 {
					eligible = false
					break
				}
			}
			if !eligible {
				continue
			}
			if op.unknown && search(done|bit, value, exists) {
				return true
			}
			if op.write {
				if search(done|bit, op.value, true) {
					return true
				}
			} else if op.found == exists && (!exists || op.value == value) && search(done|bit, value, exists) {
				return true
			}
		}
		return false
	}
	return search(0, "", false)
}

func TestHistoryOracleRejectsImpossibleReads(t *testing.T) {
	if linearizable([]registerCall{{start: 1, end: 2, write: true, value: "new"}, {start: 3, end: 4, found: true, value: "old"}}) {
		t.Fatal("accepted stale read")
	}
	if !linearizable([]registerCall{{start: 1, end: 2, write: true, value: "maybe", unknown: true}, {start: 3, end: 4, found: false}}) {
		t.Fatal("unknown write forced to apply")
	}
	if !linearizable([]registerCall{{start: 1, end: 2, write: true, value: "maybe", unknown: true}, {start: 3, end: 4, found: true, value: "maybe"}}) {
		t.Fatal("unknown write forced to disappear")
	}
}

func TestConcurrentHistoriesAcrossPartitionAndRestart(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			c := newCluster(t, 3, false)
			c.campaign(0)
			c.flush(0)
			c.isolate(0)
			var clock atomic.Uint64
			history := []registerCall{}
			unknown := registerCall{start: clock.Add(1), write: true, value: "uncertain", unknown: true}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			_, err := c.nodes[0].Propose(ctx, []byte(`{"op":"PUT","key":"register","value":"uncertain"}`))
			cancel()
			if err == nil {
				t.Fatal("minority committed")
			}
			unknown.end = clock.Add(1)
			history = append(history, unknown)
			c.campaign(1)
			c.flush(1)
			runCtx, stop := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				ticker := time.NewTicker(5 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-runCtx.Done():
						return
					case <-ticker.C:
						c.nodes[1].Tick(runCtx)
					}
				}
			}()
			defer func() { stop(); <-done }()
			var mu sync.Mutex
			var wg sync.WaitGroup
			order := rand.New(rand.NewSource(seed)).Perm(6)
			for _, id := range order {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					call := registerCall{start: clock.Add(1), write: id%2 == 0, value: fmt.Sprint(id)}
					cmd := state.Command{Op: state.OpGet, Key: "register"}
					if call.write {
						cmd.Op = state.OpPut
						cmd.Value = call.value
					}
					payload, _ := state.EncodeCommand(cmd)
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					out, err := c.nodes[1].Propose(ctx, payload)
					call.end = clock.Add(1)
					if err != nil {
						t.Errorf("healthy majority request failed: %v", err)
						return
					}
					if !call.write {
						var result state.Result
						if err := json.Unmarshal(out, &result); err != nil {
							t.Error(err)
							return
						}
						call.value, call.found = result.Value, result.Found
					}
					mu.Lock()
					history = append(history, call)
					mu.Unlock()
				}(id)
			}
			wg.Wait()
			if !linearizable(history) {
				t.Fatalf("nonlinearizable history: %+v", history)
			}
			c.restart(2)
			c.heal()
			c.flush(1)
			c.equalStores()
		})
	}
}
