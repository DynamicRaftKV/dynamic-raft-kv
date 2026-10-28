package raft

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

func TestSnapshotRejectsInvalidPayloadAndConfiguration(t *testing.T) {
	c := newCluster(t, 3, false)
	n := c.nodes[1]
	r := InstallSnapshotRequest{Term: 1, LeaderID: "0", Done: true, Snapshot: storage.Snapshot{LastIncludedIndex: 4, LastIncludedTerm: 1, Configuration: configuration(c.peers), Data: []byte("broken JSON")}}
	before, _ := c.stores[1].Snapshot()
	if reply, err := n.InstallSnapshot(context.Background(), r); err == nil || reply.Accepted {
		t.Fatal("invalid payload installed")
	}
	r.Snapshot.Data = before
	r.Snapshot.Configuration = []byte("wrong peers")
	if reply, err := n.InstallSnapshot(context.Background(), r); err == nil || reply.Accepted {
		t.Fatal("wrong configuration installed")
	}
	after, _ := c.stores[1].Snapshot()
	if !bytes.Equal(before, after) || n.Status().LastApplied != 0 || c.disks[1].snapshot != nil {
		t.Fatal("invalid snapshot changed state")
	}
}

func TestSnapshotDoesNotIncludeUncommittedSuffix(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.propose(0, `{"op":"PUT","key":"committed","value":"yes"}`)
	c.isolate(0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := c.nodes[0].Propose(ctx, []byte(`{"op":"PUT","key":"uncommitted","value":"no"}`))
		done <- err
	}()
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return c.nodes[0].lastLocked() == 3 })
	cancel()
	<-done
	if err := c.nodes[0].TakeSnapshot(); err != nil {
		t.Fatal(err)
	}
	if c.disks[0].snapshot.LastIncludedIndex != 2 || bytes.Contains(c.disks[0].snapshot.Data, []byte("uncommitted")) || len(c.disks[0].entries) != 1 {
		t.Fatal("snapshot included uncommitted state")
	}
	c.restart(0)
	got, _ := c.stores[0].Snapshot()
	if bytes.Contains(got, []byte("uncommitted")) {
		t.Fatal("replayed uncommitted suffix on recovery")
	}
}

func TestSnapshotLaggingFollowerCatchupAndRestart(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.isolate(2)
	for i := 0; i < 5; i++ {
		c.propose(0, fmt.Sprintf(`{"op":"PUT","key":"k%d","value":"%s"}`, i, strings.Repeat("v", 100000)))
	}
	if err := c.nodes[0].TakeSnapshot(); err != nil {
		t.Fatal(err)
	}
	c.propose(0, `{"op":"PUT","key":"suffix","value":"retained"}`)
	c.heal()
	c.flush(0)
	c.equalStores()
	if c.disks[2].snapshot == nil {
		t.Fatal("follower did not receive chunked snapshot")
	}
	c.restart(2)
	c.flush(0)
	c.equalStores()
	c.campaign(2)
	if !c.nodes[2].IsLeader() {
		t.Fatal("snapshot follower could not become leader")
	}
	c.propose(2, `{"op":"PUT","key":"after","value":"election"}`)
	c.equalStores()
}

func TestSnapshotChunksAndSuffixPolicy(t *testing.T) {
	for _, retain := range []bool{false, true} {
		c := newCluster(t, 3, false)
		n := c.nodes[1]
		_, err := n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 2, LeaderID: "0", Entries: []storage.Entry{{Index: 1, Term: 1, Kind: storage.EntryNoop}, {Index: 2, Term: 2, Kind: storage.EntryNoop}}})
		if err != nil {
			t.Fatal(err)
		}
		sm := state.NewStore()
		sm.Apply([]byte(`{"op":"PUT","key":"x","value":"snapshot"}`))
		data, _ := sm.Snapshot()
		term := uint64(1)
		if !retain {
			term = 2
		}
		r := InstallSnapshotRequest{Term: 2, LeaderID: "0", Snapshot: storage.Snapshot{LastIncludedIndex: 1, LastIncludedTerm: term, Configuration: configuration(c.peers), Data: data[:10]}}
		first, err := n.InstallSnapshot(context.Background(), r)
		if err != nil || !first.Accepted {
			t.Fatal(first, err)
		}
		if n.Status().LastApplied != 0 {
			t.Fatal("partial snapshot applied")
		}
		r.Offset = 11
		r.Done = true
		r.Snapshot.Data = data[10:]
		if reply, _ := n.InstallSnapshot(context.Background(), r); reply.Accepted {
			t.Fatal("accepted out of order chunk")
		}
		r.Offset = 10
		reply, err := n.InstallSnapshot(context.Background(), r)
		if err != nil || !reply.Accepted {
			t.Fatal(reply, err)
		}
		if n.Status().LastApplied != 1 {
			t.Fatal("snapshot not applied")
		}
		if retain && len(c.disks[1].entries) != 1 || !retain && len(c.disks[1].entries) != 0 {
			t.Fatal("wrong suffix retention")
		}
		// Retrying the final chunk after a lost acknowledgement is harmless.
		if reply, err = n.InstallSnapshot(context.Background(), r); err != nil || !reply.Accepted {
			t.Fatal(reply, err)
		}
		c.restart(1)
		got, _ := c.stores[1].Snapshot()
		if !bytes.Equal(got, data) {
			t.Fatal("snapshot not recovered")
		}
	}
}

func TestSnapshotDurableAutomaticRecovery(t *testing.T) {
	dir := t.TempDir()
	disk, err := storage.OpenWAL(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := newCluster(t, 1, false)
	c.nodes[0].Stop()
	config := c.nodes[0].c
	config.Storage = disk
	config.StateMachine = state.NewStore()
	config.SnapshotEntries = 2
	n, err := NewNode(config)
	if err != nil {
		t.Fatal(err)
	}
	if err = n.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = n.Propose(context.Background(), []byte(`{"op":"PUT","key":"disk","value":"safe"}`)); err != nil {
		t.Fatal(err)
	}
	n.Stop()
	disk.Close()
	disk, err = storage.OpenWAL(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	r, err := disk.Recover()
	if err != nil || r.Snapshot == nil || len(r.Entries) != 0 {
		t.Fatal(r, err)
	}
	config.Storage = disk
	sm := state.NewStore()
	config.StateMachine = sm
	n, err = NewNode(config)
	if err != nil {
		t.Fatal(err)
	}
	defer n.Stop()
	got, _ := sm.Snapshot()
	if !bytes.Contains(got, []byte(`"safe"`)) || n.Status().LastApplied != 2 {
		t.Fatal(string(got), n.Status())
	}
	if err = n.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = n.Propose(context.Background(), []byte(`{"op":"PUT","key":"next","value":"works"}`)); err != nil {
		t.Fatal(err)
	}
}
