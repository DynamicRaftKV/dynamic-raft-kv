package raft

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

var (
	ErrStopped          = errors.New("raft: stopped")
	ErrNotLeader        = errors.New("raft: not leader; proposal outcome may be unknown")
	ErrUnsupported      = errors.New("raft: membership changes are not implemented")
	ErrProposalTooLarge = errors.New("raft: encoded command exceeds 1 MiB")
)

// MaxCommandBytes bounds one encoded proposal for the unary gRPC transport.
const MaxCommandBytes = 1 << 20

// Config describes one node in an identical, fixed configuration on every peer.
// StateMachine must be fresh: recovery restores a snapshot and replays the
// suffix only after commitment is known. Storage must honor durability.
type Config struct {
	ID                                                 string
	Peers                                              []Peer
	Storage                                            storage.Storage
	StateMachine                                       state.StateMachine
	Transport                                          Transport
	ElectionMinTicks, ElectionMaxTicks, HeartbeatTicks int
	TickInterval, RPCTimeout                           time.Duration
	// Seed makes logical-tick tests reproducible. Zero chooses a random seed.
	Seed int64
	// SnapshotEntries triggers a checkpoint after this many applied entries.
	// Zero disables automatic snapshots; TakeSnapshot remains available.
	SnapshotEntries uint64
}

type proposalResult struct {
	data []byte
	err  error
}

// Node serializes transitions, persistence and application. It never calls the
// transport while holding mu. Callbacks from storage/application must not reenter.
type Node struct {
	mu                           sync.Mutex
	c                            Config
	peers                        map[string]Peer
	rng                          *rand.Rand
	hard                         storage.HardState
	log                          []storage.Entry // slot zero is the snapshot boundary sentinel
	snapshot                     *storage.Snapshot
	incoming                     *snapshotTransfer
	role                         Role
	leader                       string
	commit, applied              uint64
	elapsed, deadline, heartbeat int
	next, match                  map[string]uint64
	inflight                     map[string]bool
	pending                      map[uint64]chan proposalResult
	err                          error
	running                      bool
	life                         context.Context
	cancel                       context.CancelFunc
}

var _ RaftNode = (*Node)(nil)
var _ RPCHandler = (*Node)(nil)

func NewNode(c Config) (*Node, error) {
	if c.ID == "" || c.Storage == nil || c.StateMachine == nil || c.Transport == nil {
		return nil, errors.New("raft: ID, storage, state machine and transport required")
	}
	if c.HeartbeatTicks == 0 {
		c.HeartbeatTicks = 1
	}
	if c.ElectionMinTicks == 0 {
		c.ElectionMinTicks = 10
	}
	if c.ElectionMaxTicks == 0 {
		c.ElectionMaxTicks = 20
	}
	if c.TickInterval == 0 {
		c.TickInterval = 100 * time.Millisecond
	}
	if c.RPCTimeout == 0 {
		c.RPCTimeout = time.Second
	}
	if c.HeartbeatTicks < 1 || c.ElectionMinTicks <= c.HeartbeatTicks || c.ElectionMaxTicks < c.ElectionMinTicks || c.TickInterval < 0 || c.RPCTimeout < 0 {
		return nil, errors.New("raft: invalid timer configuration")
	}
	c.Peers = append([]Peer(nil), c.Peers...)
	peers := make(map[string]Peer)
	voters := 0
	for _, p := range c.Peers {
		if p.ID == "" {
			return nil, errors.New("raft: empty peer ID")
		}
		if _, ok := peers[p.ID]; ok {
			return nil, errors.New("raft: duplicate peer ID")
		}
		peers[p.ID] = p
		if !p.Learner {
			voters++
		}
	}
	if _, ok := peers[c.ID]; !ok || voters == 0 {
		return nil, errors.New("raft: self and at least one voter required")
	}
	r, err := c.Storage.Recover()
	if err != nil {
		return nil, fmt.Errorf("raft recover: %w", err)
	}
	base, lastTerm := uint64(0), uint64(0)
	if r.Snapshot != nil {
		base, lastTerm = r.Snapshot.LastIncludedIndex, r.Snapshot.LastIncludedTerm
		if base == 0 || base == ^uint64(0) || lastTerm == 0 || lastTerm > r.HardState.Term || len(r.Snapshot.Data) > storage.MaxSnapshotBytes || !sameConfiguration(r.Snapshot.Configuration, c.Peers) {
			return nil, errors.New("raft: invalid recovered snapshot")
		}
	}
	if r.HardState.VotedFor != "" {
		p, ok := peers[r.HardState.VotedFor]
		if !ok || p.Learner || r.HardState.Term == 0 {
			return nil, errors.New("raft: invalid recovered vote")
		}
	}
	for i, e := range r.Entries {
		if len(e.Data) > MaxCommandBytes {
			return nil, ErrProposalTooLarge
		}
		if e.Index != base+uint64(i+1) || e.Term == 0 || e.Term < lastTerm || e.Term > r.HardState.Term {
			return nil, errors.New("raft: invalid recovered log")
		}
		if e.Kind != storage.EntryCommand && e.Kind != storage.EntryNoop {
			return nil, ErrUnsupported
		}
		lastTerm = e.Term
	}
	if r.Snapshot != nil {
		if err := c.StateMachine.Restore(r.Snapshot.Data); err != nil {
			return nil, fmt.Errorf("raft restore: %w", err)
		}
	}
	if c.Seed == 0 {
		c.Seed = rand.Int63()
	}
	life, cancel := context.WithCancel(context.Background())
	n := &Node{c: c, peers: peers, rng: rand.New(rand.NewSource(c.Seed)), hard: r.HardState, log: append([]storage.Entry{{}}, copyEntries(r.Entries)...), role: Follower, next: map[string]uint64{}, match: map[string]uint64{}, inflight: map[string]bool{}, pending: map[uint64]chan proposalResult{}, life: life, cancel: cancel}
	n.snapshot = copySnapshot(r.Snapshot)
	if r.Snapshot != nil {
		n.log[0] = storage.Entry{Index: base, Term: r.Snapshot.LastIncludedTerm}
		n.commit, n.applied = base, base
	}
	n.resetElectionLocked()
	return n, nil
}

func copyEntries(es []storage.Entry) []storage.Entry {
	out := append([]storage.Entry(nil), es...)
	for i := range out {
		out[i].Data = append([]byte(nil), out[i].Data...)
	}
	return out
}
func (n *Node) baseLocked() uint64                     { return n.log[0].Index }
func (n *Node) entryLocked(index uint64) storage.Entry { return n.log[index-n.baseLocked()] }
func (n *Node) lastLocked() uint64                     { return n.baseLocked() + uint64(len(n.log)-1) }
func (n *Node) resetElectionLocked() {
	n.elapsed = 0
	n.deadline = n.c.ElectionMinTicks + n.rng.Intn(n.c.ElectionMaxTicks-n.c.ElectionMinTicks+1)
}
func (n *Node) failPendingLocked(err error) {
	for i, ch := range n.pending {
		ch <- proposalResult{err: err}
		delete(n.pending, i)
	}
}
func (n *Node) haltLocked(err error) error {
	if n.err == nil {
		n.err = err
		n.role = Follower
		n.leader = ""
		n.cancel()
		n.failPendingLocked(err)
	}
	return n.err
}
func (n *Node) saveLocked(h storage.HardState) error {
	if err := n.c.Storage.SaveHardState(h); err != nil {
		return n.haltLocked(fmt.Errorf("raft hard state: %w", err))
	}
	n.hard = h
	return nil
}
func (n *Node) followLocked(term uint64, leader string) error {
	if term > n.hard.Term {
		n.incoming = nil
		if err := n.saveLocked(storage.HardState{Term: term}); err != nil {
			return err
		}
	}
	n.role = Follower
	n.leader = leader
	n.failPendingLocked(ErrNotLeader)
	return nil
}
func (n *Node) appendLocked(es []storage.Entry) error {
	if len(es) == 0 {
		return nil
	}
	if err := n.c.Storage.Append(copyEntries(es)); err != nil {
		return n.haltLocked(fmt.Errorf("raft append: %w", err))
	}
	n.log = append(n.log[:es[0].Index-n.baseLocked()], copyEntries(es)...)
	return nil
}
func (n *Node) quorumLocked() int {
	count := 0
	for _, p := range n.peers {
		if !p.Learner {
			count++
		}
	}
	return count/2 + 1
}
func (n *Node) voter(id string) bool { p, ok := n.peers[id]; return ok && !p.Learner }

func (n *Node) Status() NodeStatus {
	n.mu.Lock()
	defer n.mu.Unlock()
	return NodeStatus{ID: n.c.ID, Role: n.role, Term: n.hard.Term, LeaderID: n.leader, LeaderHTTPAddress: n.peers[n.leader].HTTPAddress, CommitIndex: n.commit, LastApplied: n.applied, Peers: append([]Peer(nil), n.c.Peers...)}
}
func (n *Node) IsLeader() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.err == nil && n.role == Leader
}

// Err reports a terminal persistence/application failure or explicit shutdown.
func (n *Node) Err() error { n.mu.Lock(); defer n.mu.Unlock(); return n.err }
func (n *Node) Stop()      { n.mu.Lock(); defer n.mu.Unlock(); n.haltLocked(ErrStopped) }

// Tick advances a logical clock. Use either manual ticks or Run, not both.
// RPC completion can take up to RPCTimeout; Transport must honor cancellation.
func (n *Node) Tick(ctx context.Context) error {
	return n.tick(ctx, false)
}

func (n *Node) tick(ctx context.Context, asynchronous bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n.mu.Lock()
	if n.err != nil {
		err := n.err
		n.mu.Unlock()
		return err
	}
	n.elapsed++
	n.heartbeat++
	campaign := n.role != Leader && n.voter(n.c.ID) && n.elapsed >= n.deadline
	replicate := n.role == Leader && n.heartbeat >= n.c.HeartbeatTicks
	if replicate {
		n.heartbeat = 0
	}
	var req RequestVoteRequest
	if campaign {
		req = n.startCampaignLocked()
	}
	n.mu.Unlock()
	work := func() {
		if campaign && req.Term != 0 {
			n.sendVotes(ctx, req)
		}
		if replicate {
			n.replicate(ctx)
		}
	}
	if asynchronous {
		go work()
	} else {
		work()
	}
	return n.Err()
}

// Run drives ticks until cancellation, then permanently stops this node.
func (n *Node) Run(ctx context.Context) error {
	n.mu.Lock()
	if n.running {
		n.mu.Unlock()
		return errors.New("raft: Run already active")
	}
	if n.err != nil {
		err := n.err
		n.mu.Unlock()
		return err
	}
	n.running = true
	n.mu.Unlock()
	defer n.Stop()
	t := time.NewTicker(n.c.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-n.life.Done():
			return n.Err()
		case <-t.C:
			// Slow peers must not suspend the clock or healthy-peer heartbeats.
			if err := n.tick(ctx, true); err != nil {
				return err
			}
		}
	}
}

func (n *Node) rpcContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, n.c.RPCTimeout)
	stop := context.AfterFunc(n.life, cancel)
	return ctx, func() { stop(); cancel() }
}
