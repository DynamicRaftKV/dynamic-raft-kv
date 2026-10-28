package raft

import (
	"bytes"
	"context"
	"errors"
	"syscall"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

type failingSnapshotDisk struct {
	storage.Storage
	install, compact bool
}

func (d failingSnapshotDisk) InstallSnapshot(s storage.Snapshot, retain bool) error {
	if d.install {
		return syscall.ENOSPC
	}
	return d.Storage.InstallSnapshot(s, retain)
}
func (d failingSnapshotDisk) Compact(i uint64) error {
	if d.compact {
		return syscall.EIO
	}
	return d.Storage.Compact(i)
}

func TestSnapshotStorageFailureHaltsWithoutAcknowledgement(t *testing.T) {
	for _, operation := range []string{"install", "compact"} {
		t.Run(operation, func(t *testing.T) {
			c := newCluster(t, 3, false)
			n := c.nodes[1]
			n.c.Storage = failingSnapshotDisk{Storage: c.disks[1], install: operation == "install", compact: operation == "compact"}
			sm := state.NewStore()
			sm.Apply([]byte(`{"op":"PUT","key":"saved","value":"yes"}`))
			data, _ := sm.Snapshot()
			r := InstallSnapshotRequest{Term: 2, LeaderID: "0", Done: true, Snapshot: storage.Snapshot{LastIncludedIndex: 5, LastIncludedTerm: 2, Configuration: configuration(c.peers), Data: data}}
			reply, err := n.InstallSnapshot(context.Background(), r)
			if err == nil || reply.Accepted || n.Err() == nil {
				t.Fatal("failed snapshot acknowledged or node remained active", reply, err)
			}
			if reply, err := n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 2, LeaderID: "0"}); err == nil || reply.Success {
				t.Fatal("failed node accepted append")
			}
			if reply, err := n.RequestVote(context.Background(), RequestVoteRequest{Term: 3, CandidateID: "0"}); err == nil || reply.VoteGranted {
				t.Fatal("failed node voted")
			}
			c.restart(1)
			got, _ := c.stores[1].Snapshot()
			if operation == "compact" && !bytes.Equal(got, data) {
				t.Fatal("durable install lost after compaction failure")
			}
			if operation == "install" && bytes.Contains(got, []byte("saved")) {
				t.Fatal("undurable restore survived restart")
			}
			if reply, err := c.nodes[1].InstallSnapshot(context.Background(), r); err != nil || !reply.Accepted {
				t.Fatal("retry after recovery failed", reply, err)
			}
		})
	}
}

func TestSnapshotInterruptedTransferRestartsFromZero(t *testing.T) {
	for _, interrupt := range []string{"restart", "higher_term"} {
		t.Run(interrupt, func(t *testing.T) {
			c := newCluster(t, 3, false)
			n := c.nodes[1]
			data, _ := state.NewStore().Snapshot()
			r := InstallSnapshotRequest{Term: 2, LeaderID: "0", Snapshot: storage.Snapshot{LastIncludedIndex: 5, LastIncludedTerm: 2, Configuration: configuration(c.peers), Data: data[:5]}}
			if reply, err := n.InstallSnapshot(context.Background(), r); err != nil || !reply.Accepted {
				t.Fatal(reply, err)
			}
			if interrupt == "restart" {
				c.restart(1)
				n = c.nodes[1]
			} else {
				if _, err := n.RequestVote(context.Background(), RequestVoteRequest{Term: 3, CandidateID: "2"}); err != nil {
					t.Fatal(err)
				}
			}
			r.Offset = 5
			r.Done = true
			r.Snapshot.Data = data[5:]
			if reply, err := n.InstallSnapshot(context.Background(), r); err != nil || reply.Accepted {
				t.Fatal("accepted interrupted transfer tail", reply, err)
			}
			if c.disks[1].snapshot != nil {
				t.Fatal("partial snapshot became durable")
			}
			r.Offset = 0
			r.Term = 4
			r.Snapshot.Data = data
			if reply, err := n.InstallSnapshot(context.Background(), r); err != nil || !reply.Accepted {
				t.Fatal("fresh transfer failed", reply, err)
			}
		})
	}
}

func TestLocalSnapshotFailureReleasesPendingProposal(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.propose(0, `{"op":"PUT","key":"safe","value":"yes"}`)
	c.isolate(0)
	n := c.nodes[0]
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, err := n.Propose(ctx, []byte(`{"op":"PUT","key":"pending","value":"no"}`)); done <- err }()
	await(t, func() bool { n.mu.Lock(); defer n.mu.Unlock(); return len(n.pending) == 1 })
	n.mu.Lock()
	n.c.Storage = failingSnapshotDisk{Storage: c.disks[0], install: true}
	n.mu.Unlock()
	if err := n.TakeSnapshot(); !errors.Is(err, syscall.ENOSPC) {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, syscall.ENOSPC) {
		t.Fatal("pending client not released with storage error", err)
	}
	if n.IsLeader() {
		t.Fatal("failed node remained leader")
	}
}
