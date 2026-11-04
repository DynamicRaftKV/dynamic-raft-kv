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

// Exercise a membership transition with an unavailable voter and unreliable
// replication, followed by leader replacement and recovery of the old leader.
func TestPromotionUnderLossThenLeaderFailure(t *testing.T) {
	c := newCluster(t, 5, true)
	c.campaign(0)
	c.propose(0, `{"op":"PUT","key":"membership-fault","value":"retained"}`)
	c.isolate(1)
	n := c.nodes[0]
	wire := &lossyDuplicateTransport{Transport: n.c.Transport}
	n.mu.Lock()
	n.c.Transport = wire
	n.c.TickInterval = 5 * time.Millisecond
	n.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = n.Run(ctx) }()
	defer func() { cancel(); <-done }()
	change(t, n, "promote", "4", nil)
	await(t, func() bool { return !n.Status().MembershipPending })
	cancel()
	<-done
	c.flush(0)
	if !c.nodes[4].voterForTest("4") || wire.calls.Load() < 3 {
		t.Fatal("promotion or fault schedule not exercised")
	}
	c.isolate(0)
	// Dropped finalization messages can leave node 2 behind. Campaign the
	// freshest survivor; an outdated candidate must not be forced to win.
	successor := 2
	for _, i := range []int{3, 4} {
		if c.nodes[i].Diagnostics().LastIndex > c.nodes[successor].Diagnostics().LastIndex {
			successor = i
		}
	}
	c.campaign(successor)
	c.flush(successor)
	if !c.nodes[successor].IsLeader() {
		t.Fatal("remaining three voters could not elect a leader")
	}
	c.propose(successor, `{"op":"PUT","key":"after-promotion-failure","value":"safe"}`)
	c.restart(0)
	c.heal()
	c.flush(successor)
	c.equalStores()
	for _, node := range c.nodes {
		if node.Status().MembershipPending || !node.voterForTest("4") {
			t.Fatal("finalized promotion lost during recovery")
		}
	}
}
