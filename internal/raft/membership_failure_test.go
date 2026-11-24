package raft

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

// The joiner retains only its test disk across replacements, never its state
// machine. This models recovery, not a physical disk or operating-system crash.
func joiningMember(t *testing.T, c *cluster) (func(), func() *Node, func() *state.Store) {
	t.Helper()
	disk := &memoryDisk{}
	var node *Node
	var store *state.Store
	restart := func() {
		if node != nil {
			node.Stop()
		}
		store = state.NewStore()
		var err error
		node, err = NewNode(Config{ID: "joining", Peers: c.peers, Joining: true, Storage: disk, StateMachine: store, Transport: testTransport{net: c.net, from: "joining"}})
		if err != nil {
			t.Fatal(err)
		}
		c.net.mu.Lock()
		c.net.nodes["joining"] = node
		c.net.mu.Unlock()
	}
	restart()
	t.Cleanup(func() { node.Stop() })
	return restart, func() *Node { return node }, func() *state.Store { return store }
}

func assertJoinedState(t *testing.T, c *cluster, leader int, node *Node, store *state.Store) {
	t.Helper()
	c.flush(leader)
	await(t, func() bool { return node.Status().LastApplied >= c.nodes[leader].Status().CommitIndex })
	want, err := c.stores[leader].Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("learner state mismatch: %s != %s", got, want)
	}
	c.equalStores()
}

func TestMembershipJoinDuringConcurrentWrites(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.flush(0)
	_, node, store := joiningMember(t, c)
	// Hold voter responses so a client write is definitely in flight when the
	// learner configuration is appended. Channels make the overlap reproducible.
	entered, release := make(chan struct{}), make(chan struct{})
	var once, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	c.net.mu.Lock()
	c.net.appendHook = func(from, to string, r AppendEntriesRequest, _ AppendEntriesResponse) {
		if from == "0" && (to == "1" || to == "2") && len(r.Entries) > 0 {
			once.Do(func() { close(entered) })
			<-release
		}
	}
	c.net.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	writes := make(chan error, 2)
	for worker := 0; worker < 2; worker++ {
		go func(worker int) {
			for i := worker; i < 24; i += 2 {
				_, err := c.nodes[0].Propose(ctx, []byte(fmt.Sprintf(`{"op":"PUT","key":"write-%d","value":"acknowledged"}`, i)))
				if err != nil {
					writes <- err
					return
				}
			}
			writes <- nil
		}(worker)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("write never reached voters")
	}
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return len(c.nodes[0].pending) >= 2 })
	joined := make(chan error, 1)
	go func() {
		joined <- c.nodes[0].ChangeMembership(ctx, "add_learner", "", &Peer{ID: "joining", Learner: true})
	}()
	await(t, func() bool {
		for _, p := range c.nodes[0].Status().Peers {
			if p.ID == "joining" {
				return true
			}
		}
		return false
	})
	unblock()
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-tick.C:
				c.nodes[0].Tick(heartbeatCtx)
			}
		}
	}()
	defer func() { stopHeartbeat(); <-heartbeatDone }()
	if err := <-joined; err != nil {
		t.Fatal(err)
	}
	for worker := 0; worker < 2; worker++ {
		if err := <-writes; err != nil {
			t.Fatal(err)
		}
	}
	assertJoinedState(t, c, 0, node(), store())
	change(t, c.nodes[0], "promote", "joining", nil)
	assertJoinedState(t, c, 0, node(), store())
	if !node().voterForTest("joining") || c.nodes[0].Status().MembershipPending {
		t.Fatal("promotion not finalized")
	}
	// Each successful write must exist, not merely match an equally empty replica.
	for i := 0; i < 24; i++ {
		out := c.propose(0, fmt.Sprintf(`{"op":"GET","key":"write-%d"}`, i))
		if !bytes.Contains(out, []byte(`"value":"acknowledged"`)) {
			t.Fatalf("acknowledged write %d lost: %s", i, out)
		}
	}
}

func TestMembershipRestartDuringCatchup(t *testing.T) {
	for _, failure := range []string{"learner", "leader"} {
		t.Run(failure, func(t *testing.T) {
			c := newCluster(t, 3, false)
			c.campaign(0)
			c.flush(0)
			restart, node, store := joiningMember(t, c)
			c.net.mu.Lock()
			for _, p := range c.peers {
				c.net.blocked[p.ID+":joining"] = true
			}
			c.net.mu.Unlock()
			change(t, c.nodes[0], "add_learner", "", &Peer{ID: "joining", Learner: true})
			for i := 0; i < 8; i++ {
				c.propose(0, fmt.Sprintf(`{"op":"PUT","key":"saved-%d","value":"durable"}`, i))
			}
			if node().Status().LastApplied >= c.nodes[0].Status().CommitIndex {
				t.Fatal("test failed to leave learner behind")
			}
			if err := c.nodes[0].ChangeMembership(context.Background(), "promote", "joining", nil); err == nil {
				t.Fatal("lagging learner promoted")
			}
			leader := 0
			if failure == "learner" {
				restart()
			} else {
				c.restart(0)
				c.campaign(1)
				leader = 1
				c.flush(leader)
			}
			c.heal()
			assertJoinedState(t, c, leader, node(), store())
			change(t, c.nodes[leader], "promote", "joining", nil)
			assertJoinedState(t, c, leader, node(), store())
			restart()
			assertJoinedState(t, c, leader, node(), store())
			if !node().voterForTest("joining") || c.nodes[leader].Status().MembershipPending {
				t.Fatal("membership lost during recovery")
			}
		})
	}
}
