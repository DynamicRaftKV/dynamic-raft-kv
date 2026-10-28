package raft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

// memoryDisk is exclusively a test double. It models retained state across a
// Node replacement, not fsync, process crashes or disk corruption.
type memoryDisk struct {
	hard                 storage.HardState
	entries              []storage.Entry
	snapshot             *storage.Snapshot
	failHard, failAppend bool
}

func (d *memoryDisk) Recover() (storage.Recovered, error) {
	return storage.Recovered{HardState: d.hard, Entries: copyEntries(d.entries), Snapshot: d.snapshot}, nil
}
func (d *memoryDisk) SaveHardState(h storage.HardState) error {
	if d.failHard {
		return errors.New("disk hard-state failure")
	}
	d.hard = h
	return nil
}
func (d *memoryDisk) Append(es []storage.Entry) error {
	if d.failAppend {
		return errors.New("disk append failure")
	}
	if len(es) > 0 {
		base := uint64(0)
		if d.snapshot != nil {
			base = d.snapshot.LastIncludedIndex
		}
		d.entries = append(d.entries[:es[0].Index-base-1], copyEntries(es)...)
	}
	return nil
}
func (d *memoryDisk) InstallSnapshot(s storage.Snapshot, retain bool) error {
	base := uint64(0)
	if d.snapshot != nil {
		base = d.snapshot.LastIncludedIndex
	}
	if retain {
		d.entries = copyEntries(d.entries[s.LastIncludedIndex-base:])
	} else {
		d.entries = nil
	}
	d.snapshot = copySnapshot(&s)
	return nil
}
func (*memoryDisk) Compact(uint64) error { return nil }

type testNetwork struct {
	mu         sync.Mutex
	nodes      map[string]*Node
	blocked    map[string]bool
	appendHook func(string, string, AppendEntriesRequest, AppendEntriesResponse)
}
type testTransport struct {
	net  *testNetwork
	from string
}

func (t testTransport) target(ctx context.Context, id string) (*Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t.net.mu.Lock()
	defer t.net.mu.Unlock()
	if t.net.blocked[t.from+":"+id] || t.net.nodes[id] == nil {
		return nil, errors.New("link unavailable")
	}
	return t.net.nodes[id], nil
}
func (t testTransport) SendRequestVote(ctx context.Context, p Peer, r RequestVoteRequest) (RequestVoteResponse, error) {
	n, err := t.target(ctx, p.ID)
	if err != nil {
		return RequestVoteResponse{}, err
	}
	return n.RequestVote(ctx, r)
}
func (t testTransport) SendAppendEntries(ctx context.Context, p Peer, r AppendEntriesRequest) (AppendEntriesResponse, error) {
	n, err := t.target(ctx, p.ID)
	if err != nil {
		return AppendEntriesResponse{}, err
	}
	resp, err := n.AppendEntries(ctx, r)
	t.net.mu.Lock()
	hook := t.net.appendHook
	t.net.mu.Unlock()
	if hook != nil {
		hook(t.from, p.ID, r, resp)
	}
	return resp, err
}
func (t testTransport) SendInstallSnapshot(ctx context.Context, p Peer, r InstallSnapshotRequest) (InstallSnapshotResponse, error) {
	n, err := t.target(ctx, p.ID)
	if err != nil {
		return InstallSnapshotResponse{}, err
	}
	return n.InstallSnapshot(ctx, r)
}
func (testTransport) JoinCluster(context.Context, Peer, JoinClusterRequest) (JoinClusterResponse, error) {
	return JoinClusterResponse{}, ErrUnsupported
}

type cluster struct {
	t      *testing.T
	net    *testNetwork
	nodes  []*Node
	disks  []*memoryDisk
	stores []*state.Store
	peers  []Peer
}

func newCluster(t *testing.T, count int, learner bool) *cluster {
	t.Helper()
	c := &cluster{t: t, net: &testNetwork{nodes: map[string]*Node{}, blocked: map[string]bool{}}}
	for i := 0; i < count; i++ {
		c.peers = append(c.peers, Peer{ID: fmt.Sprint(i), Learner: learner && i == count-1})
	}
	for i := 0; i < count; i++ {
		c.disks = append(c.disks, &memoryDisk{})
		c.stores = append(c.stores, state.NewStore())
		n := c.makeNode(i)
		c.nodes = append(c.nodes, n)
		c.net.nodes[n.c.ID] = n
	}
	t.Cleanup(func() {
		for _, n := range c.nodes {
			n.Stop()
		}
	})
	return c
}
func (c *cluster) makeNode(i int) *Node {
	c.t.Helper()
	n, err := NewNode(Config{ID: c.peers[i].ID, Peers: c.peers, Storage: c.disks[i], StateMachine: c.stores[i], Transport: testTransport{net: c.net, from: c.peers[i].ID}, Seed: int64(i + 1), ElectionMinTicks: 5, ElectionMaxTicks: 10, RPCTimeout: time.Second})
	if err != nil {
		c.t.Fatal(err)
	}
	return n
}
func (c *cluster) isolate(i int) {
	c.net.mu.Lock()
	defer c.net.mu.Unlock()
	for _, p := range c.peers {
		if p.ID != c.peers[i].ID {
			c.net.blocked[c.peers[i].ID+":"+p.ID] = true
			c.net.blocked[p.ID+":"+c.peers[i].ID] = true
		}
	}
}
func (c *cluster) heal() { c.net.mu.Lock(); c.net.blocked = map[string]bool{}; c.net.mu.Unlock() }
func (c *cluster) campaign(i int) {
	c.t.Helper()
	if err := c.nodes[i].Campaign(context.Background()); err != nil {
		c.t.Fatal(err)
	}
}
func (c *cluster) flush(i int) {
	c.t.Helper()
	for j := 0; j < 30; j++ {
		await(c.t, func() bool {
			c.nodes[i].mu.Lock()
			defer c.nodes[i].mu.Unlock()
			for _, busy := range c.nodes[i].inflight {
				if busy {
					return false
				}
			}
			return true
		})
		c.nodes[i].replicate(context.Background())
	}
}
func (c *cluster) propose(i int, command string) []byte {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := c.nodes[i].Propose(ctx, []byte(command))
	if err != nil {
		c.t.Fatal(err)
	}
	c.flush(i)
	return out
}
func (c *cluster) restart(i int) {
	c.nodes[i].Stop()
	c.stores[i] = state.NewStore()
	n := c.makeNode(i)
	c.nodes[i] = n
	c.net.mu.Lock()
	c.net.nodes[n.c.ID] = n
	c.net.mu.Unlock()
}
func (c *cluster) equalStores() {
	c.t.Helper()
	first, _ := c.stores[0].Snapshot()
	for i, s := range c.stores {
		got, _ := s.Snapshot()
		if !bytes.Equal(got, first) {
			c.t.Fatalf("node %d state %s != %s", i, got, first)
		}
	}
}
