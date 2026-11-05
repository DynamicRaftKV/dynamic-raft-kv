package raft

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQuorumLossAdmissionAndRecovery(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.flush(0)
	n := c.nodes[0]
	n.mu.Lock()
	n.c.MaxUncommittedEntries = 3
	n.c.MaxUncommittedBytes = 2048
	n.mu.Unlock()
	c.isolate(0)
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		_, err := n.Propose(ctx, []byte(`{"op":"PUT","key":"bounded","value":"retained"}`))
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
	before := n.Diagnostics()
	for i := 0; i < 20; i++ {
		if _, err := n.Propose(context.Background(), []byte(`{"op":"GET","key":"bounded"}`)); !errors.Is(err, ErrBusy) {
			t.Fatal(err)
		}
	}
	after := n.Diagnostics()
	if after.LastIndex != before.LastIndex || after.UncommittedEntries != 3 || after.Rejected != 20 || after.Pending != 0 {
		t.Fatal(after)
	}
	c.heal()
	c.flush(0)
	c.propose(0, `{"op":"GET","key":"bounded"}`)
	if n.Diagnostics().UncommittedEntries != 0 {
		t.Fatal("capacity did not recover")
	}
}

func TestProposalByteAndPendingLimits(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.flush(0)
	n := c.nodes[0]
	n.mu.Lock()
	n.c.MaxUncommittedBytes = 70
	n.mu.Unlock()
	if _, err := n.Propose(context.Background(), make([]byte, 7)); !errors.Is(err, ErrBusy) {
		t.Fatal("byte limit", err)
	}
	n.mu.Lock()
	n.c.MaxUncommittedBytes = 1024
	n.c.MaxPendingProposals = 1
	n.pending[999] = make(chan proposalResult, 1)
	n.mu.Unlock()
	if _, err := n.Propose(context.Background(), []byte("a")); !errors.Is(err, ErrBusy) {
		t.Fatal("pending limit", err)
	}
	n.mu.Lock()
	delete(n.pending, 999)
	n.mu.Unlock()
}
