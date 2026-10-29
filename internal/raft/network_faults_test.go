package raft

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type lossyDuplicateTransport struct {
	Transport
	calls, duplicates atomic.Uint64
}

func (f *lossyDuplicateTransport) SendAppendEntries(ctx context.Context, p Peer, r AppendEntriesRequest) (AppendEntriesResponse, error) {
	count := f.calls.Add(1)
	if count%3 == 0 {
		return AppendEntriesResponse{}, context.DeadlineExceeded
	}
	reply, err := f.Transport.SendAppendEntries(ctx, p, r)
	if err == nil && count%5 == 0 {
		f.duplicates.Add(1)
		_, _ = f.Transport.SendAppendEntries(ctx, p, r)
	}
	return reply, err
}
func TestLostDuplicateMessagesWithCheckpointCatchup(t *testing.T) {
	c := newCluster(t, 5, false)
	c.campaign(0)
	c.flush(0)
	c.isolate(4)
	n := c.nodes[0]
	wire := &lossyDuplicateTransport{Transport: n.c.Transport}
	n.mu.Lock()
	n.c.Transport = wire
	n.c.SnapshotEntries = 3
	n.mu.Unlock()
	for i := 0; i < 12; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := make(chan error, 1)
		go func() {
			_, err := n.Propose(ctx, []byte(`{"op":"PUT","key":"faults","value":"survived"}`))
			done <- err
		}()
		finished := false
		for !finished {
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
				finished = true
			default:
				n.Tick(ctx)
			}
		}
		cancel()
	}
	c.heal()
	c.flush(0)
	c.equalStores()
	if wire.duplicates.Load() == 0 || wire.calls.Load() < 3 {
		t.Fatal("faults not exercised")
	}
	if c.nodes[4].Diagnostics().SnapshotIndex == 0 {
		t.Fatal("lagging node did not install checkpoint")
	}
}
