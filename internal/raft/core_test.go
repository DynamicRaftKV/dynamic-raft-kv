package raft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

func TestElectionReplicationAndResults(t *testing.T) {
	for _, size := range []int{1, 3, 5} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			c := newCluster(t, size, false)
			c.campaign(0)
			if !c.nodes[0].IsLeader() {
				t.Fatal("no leader")
			}
			c.propose(0, `{"op":"PUT","key":"fruit","value":"apple"}`)
			result := c.propose(0, `{"op":"GET","key":"fruit"}`)
			if !bytes.Contains(result, []byte(`"apple"`)) {
				t.Fatalf("result %s", result)
			}
			c.propose(0, `{"op":"DELETE","key":"fruit"}`)
			c.equalStores()
			for _, n := range c.nodes {
				if s := n.Status(); s.CommitIndex != 4 || s.LastApplied != 4 {
					t.Fatalf("status %+v", s)
				}
			}
		})
	}
}

func TestVotePersistenceFreshnessAndLearners(t *testing.T) {
	c := newCluster(t, 3, true)
	n := c.nodes[1]
	r, err := n.RequestVote(context.Background(), RequestVoteRequest{Term: 2, CandidateID: "0"})
	if err != nil || !r.VoteGranted || c.disks[1].hard.VotedFor != "0" {
		t.Fatalf("vote %+v %v", r, err)
	}
	c.restart(1)
	n = c.nodes[1]
	r, _ = n.RequestVote(context.Background(), RequestVoteRequest{Term: 2, CandidateID: "1"})
	if r.VoteGranted {
		t.Fatal("double vote after restart")
	}
	r, _ = n.RequestVote(context.Background(), RequestVoteRequest{Term: 3, CandidateID: "2"})
	if r.VoteGranted || r.Term != 2 {
		t.Fatal("learner changed election")
	}
	c.campaign(2)
	if c.nodes[2].Status().Role != Follower {
		t.Fatal("learner campaigned")
	}
	r, _ = c.nodes[2].RequestVote(context.Background(), RequestVoteRequest{Term: 3, CandidateID: "0"})
	if r.VoteGranted {
		t.Fatal("learner voted")
	}
	c.campaign(0)
	c.campaign(0)
	if !c.nodes[0].IsLeader() {
		t.Fatal("voters did not elect")
	}
	c.propose(0, `{"op":"PUT","key":"x","value":"y"}`)
	r, _ = c.nodes[1].RequestVote(context.Background(), RequestVoteRequest{Term: 10, CandidateID: "0", LastLogIndex: 100, LastLogTerm: 0})
	if r.VoteGranted {
		t.Fatal("stale term wins by length")
	}
	r, _ = c.nodes[1].RequestVote(context.Background(), RequestVoteRequest{Term: 10, CandidateID: "0", LastLogIndex: 1, LastLogTerm: 4})
	if !r.VoteGranted {
		t.Fatal("newer last term must beat length")
	}
}

func TestNoQuorumAndConflictRepair(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.propose(0, `{"op":"PUT","key":"safe","value":"yes"}`)
	c.isolate(0)
	ctx, cancel := context.WithCancel(context.Background())
	// Append a proposal with all links down, then cancel waiting deterministically.
	ch := make(chan error, 1)
	go func() { _, err := c.nodes[0].Propose(ctx, []byte(`{"op":"PUT","key":"lost","value":"no"}`)); ch <- err }()
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return c.nodes[0].lastLocked() == 3 })
	cancel()
	if err := <-ch; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if c.nodes[0].Status().CommitIndex != 2 {
		t.Fatal("minority committed")
	}
	c.campaign(1)
	if !c.nodes[1].IsLeader() {
		t.Fatal("majority failed to elect")
	}
	c.propose(1, `{"op":"PUT","key":"new","value":"yes"}`)
	c.heal()
	c.flush(1)
	c.equalStores()
	for _, n := range c.nodes {
		if n.Status().CommitIndex != 4 {
			t.Fatal("failed to catch up")
		}
		n.mu.Lock()
		if n.log[2].Term != 1 || n.log[3].Kind != storage.EntryNoop {
			t.Error("committed prefix changed or divergent suffix retained")
		}
		n.mu.Unlock()
	}
	if c.nodes[0].IsLeader() {
		t.Fatal("old leader did not step down")
	}
}

// Scheduling helper waits for an observable event, never advances Raft time.
func await(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !predicate() {
		select {
		case <-deadline:
			t.Fatal("event did not happen")
		default:
		}
		runtime.Gosched()
	}
}

func TestCurrentTermCommitAndRecoveredReplay(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.propose(0, `{"op":"PUT","key":"durable-model","value":"yes"}`)
	for i := range c.nodes {
		c.restart(i)
		if c.nodes[i].Status().CommitIndex != 0 {
			t.Fatal("assumed recovered log was committed")
		}
	}
	c.campaign(1)
	c.flush(1)
	c.equalStores()
	got, _ := c.stores[0].Snapshot()
	if !bytes.Contains(got, []byte("durable-model")) {
		t.Fatal("committed prefix not replayed")
	}
	// A majority match for an older term alone must not advance commit.
	n := c.nodes[1]
	n.mu.Lock()
	n.commit = 0
	n.applied = 0
	n.hard.Term++
	for id := range n.peers {
		n.match[id] = 2
	}
	n.advanceLocked()
	commit := n.commit
	n.mu.Unlock()
	if commit != 0 {
		t.Fatal("committed earlier-term entry by counting replicas")
	}
}

func TestAppendConsistencyAndCommitBounds(t *testing.T) {
	c := newCluster(t, 3, false)
	n := c.nodes[1]
	r := AppendEntriesRequest{Term: 2, LeaderID: "0", Entries: []storage.Entry{{Index: 1, Term: 1, Kind: storage.EntryNoop}, {Index: 2, Term: 2, Kind: storage.EntryNoop}}}
	resp, err := n.AppendEntries(context.Background(), r)
	if err != nil || !resp.Success {
		t.Fatal(resp, err)
	}
	resp, _ = n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 2, LeaderID: "0", PrevLogIndex: 2, PrevLogTerm: 1})
	if resp.Success {
		t.Fatal("accepted wrong prev term")
	}
	resp, _ = n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 2, LeaderID: "0", PrevLogIndex: 1, PrevLogTerm: 1, LeaderCommit: 2})
	if !resp.Success || n.Status().CommitIndex != 1 {
		t.Fatal("committed unmatched tail")
	}
	n.mu.Lock()
	last := n.lastLocked()
	n.mu.Unlock()
	if last != 2 {
		t.Fatal("heartbeat truncated suffix")
	}
	resp, err = n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 3, LeaderID: "0", Entries: []storage.Entry{{Index: 1, Term: 3, Kind: storage.EntryNoop}}})
	if resp.Success || err == nil {
		t.Fatal("overwrote committed prefix")
	}
	resp, _ = n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 1, LeaderID: "0"})
	if resp.Success || resp.Term != 3 {
		t.Fatal("accepted stale leader")
	}
}

func TestPersistenceFailuresHaltWithoutAcknowledgement(t *testing.T) {
	for _, mode := range []string{"vote", "append", "proposal"} {
		t.Run(mode, func(t *testing.T) {
			c := newCluster(t, 3, false)
			if mode == "vote" {
				c.disks[1].failHard = true
				r, err := c.nodes[1].RequestVote(context.Background(), RequestVoteRequest{Term: 1, CandidateID: "0"})
				if err == nil || r.VoteGranted {
					t.Fatal("acknowledged failed vote")
				}
			}
			if mode == "append" {
				c.disks[1].failAppend = true
				r, err := c.nodes[1].AppendEntries(context.Background(), AppendEntriesRequest{Term: 1, LeaderID: "0", Entries: []storage.Entry{{Index: 1, Term: 1, Kind: storage.EntryNoop}}})
				if err == nil || r.Success {
					t.Fatal("acknowledged failed append")
				}
			}
			if mode == "proposal" {
				c.campaign(1)
				c.disks[1].failAppend = true
				if _, err := c.nodes[1].Propose(context.Background(), []byte(`{}`)); err == nil {
					t.Fatal("acknowledged failed proposal")
				}
			}
			if c.nodes[1].Err() == nil || c.nodes[1].IsLeader() {
				t.Fatal("node continued after persistence failure")
			}
		})
	}
}

func TestInvalidCommandDoesNotPoisonLog(t *testing.T) {
	c := newCluster(t, 1, false)
	c.campaign(0)
	if _, err := c.nodes[0].Propose(context.Background(), []byte(`{}`)); !errors.Is(err, state.ErrInvalidCommand) {
		t.Fatal(err)
	}
	c.propose(0, `{"op":"PUT","key":"x","value":"ok"}`)
	if c.nodes[0].Err() != nil || c.nodes[0].Status().LastApplied != 3 {
		t.Fatal("invalid command stopped application")
	}
}

func TestDelayedReplyCannotCommitAfterStepDown(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.isolate(2)
	arrived, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	c.net.mu.Lock()
	c.net.appendHook = func(from, to string, r AppendEntriesRequest, _ AppendEntriesResponse) {
		if from == "0" && to == "1" && len(r.Entries) > 0 {
			once.Do(func() { close(arrived); <-release })
		}
	}
	c.net.mu.Unlock()
	result := make(chan error, 1)
	go func() {
		_, err := c.nodes[0].Propose(context.Background(), []byte(`{"op":"PUT","key":"x"}`))
		result <- err
	}()
	select {
	case <-arrived:
	case <-time.After(3 * time.Second):
		t.Fatal("no delayed append")
	}
	_, err := c.nodes[0].RequestVote(context.Background(), RequestVoteRequest{Term: 5, CandidateID: "1", LastLogIndex: 2, LastLogTerm: 1})
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; !errors.Is(err, ErrNotLeader) {
		t.Fatal(err)
	}
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return !c.nodes[0].inflight["1"] })
	if c.nodes[0].Status().CommitIndex != 1 {
		t.Fatal("stale response committed")
	}
}

func TestLogicalTimersAndSplitVoteRecovery(t *testing.T) {
	c := newCluster(t, 3, false)
	for i := range c.nodes {
		c.isolate(i)
		c.campaign(i)
	}
	for _, n := range c.nodes {
		if n.IsLeader() {
			t.Fatal("partition elected leader")
		}
	}
	c.heal()
	leaders := map[uint64]string{}
	for tick := 0; tick < 80; tick++ {
		for _, n := range c.nodes {
			if err := n.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			s := n.Status()
			if s.Role == Leader {
				if old, ok := leaders[s.Term]; ok && old != s.ID {
					t.Fatal("two leaders in a term")
				}
				leaders[s.Term] = s.ID
			}
		}
	}
	if len(leaders) == 0 {
		t.Fatal("split votes never resolved")
	}
}

func TestSeededPartitionsPreserveCommittedHistory(t *testing.T) {
	for seed := int64(1); seed <= 20; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			c := newCluster(t, 3, false)
			rng := rand.New(rand.NewSource(seed))
			c.campaign(0)
			leader := 0
			var committed []storage.Entry
			for step := 0; step < 15; step++ {
				if rng.Intn(2) == 0 {
					c.isolate(leader)
					next := (leader + 1 + rng.Intn(2)) % 3
					c.campaign(next)
					if !c.nodes[next].IsLeader() {
						t.Fatal("majority election failed")
					}
					c.heal()
					leader = next
					c.flush(leader)
				}
				c.propose(leader, fmt.Sprintf(`{"op":"PUT","key":"k%d","value":"%d"}`, step, seed))
				for _, n := range c.nodes {
					n.mu.Lock()
					for i, e := range committed {
						actual := n.log[i+1]
						if actual.Term != e.Term || !bytes.Equal(actual.Data, e.Data) {
							t.Error("committed history changed")
						}
					}
					n.mu.Unlock()
				}
				n := c.nodes[leader]
				n.mu.Lock()
				committed = copyEntries(n.log[1 : n.commit+1])
				n.mu.Unlock()
				c.equalStores()
			}
		})
	}
}

func TestStopAndRunCancellation(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.isolate(0)
	result := make(chan error, 1)
	go func() { _, err := c.nodes[0].Propose(context.Background(), []byte(`{}`)); result <- err }()
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return len(c.nodes[0].pending) == 1 })
	c.nodes[0].Stop()
	if err := <-result; !errors.Is(err, ErrStopped) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.nodes[1].Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !errors.Is(c.nodes[1].Err(), ErrStopped) {
		t.Fatal("Run did not stop")
	}
}

func TestRecoveryRejectsUnsupportedOrBrokenState(t *testing.T) {
	for _, d := range []*memoryDisk{
		{snapshot: &storage.Snapshot{}},
		{hard: storage.HardState{Term: 1}, entries: []storage.Entry{{Index: 2, Term: 1}}},
		{hard: storage.HardState{Term: 1}, entries: []storage.Entry{{Index: 1, Term: 2}}},
		{hard: storage.HardState{Term: 1}, entries: []storage.Entry{{Index: 1, Term: 1, Kind: storage.EntryConfiguration}}},
	} {
		_, err := NewNode(Config{ID: "a", Peers: []Peer{{ID: "a"}}, Storage: d, StateMachine: state.NewStore(), Transport: testTransport{}})
		if err == nil {
			t.Fatal("accepted invalid recovery")
		}
	}
}

func TestLearnerCannotSupplyQuorum(t *testing.T) {
	c := newCluster(t, 4, true)
	c.isolate(1)
	c.isolate(2)
	c.campaign(0)
	if c.nodes[0].IsLeader() {
		t.Fatal("one voter plus learner elected a leader")
	}
	c.heal()
	c.campaign(0)
	c.isolate(1)
	c.isolate(2)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := c.nodes[0].Propose(ctx, []byte(`{"op":"PUT","key":"x"}`)); result <- err }()
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return c.nodes[0].match["3"] == 2 })
	if c.nodes[0].Status().CommitIndex != 1 {
		t.Fatal("learner acknowledgement counted in quorum")
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

type brokenMachine struct{ state.Store }

func (*brokenMachine) Apply([]byte) ([]byte, error) {
	return nil, errors.New("unexpected application fault")
}

func TestApplicationFailureHaltsNode(t *testing.T) {
	c := newCluster(t, 1, false)
	c.nodes[0].c.StateMachine = &brokenMachine{}
	c.campaign(0)
	if _, err := c.nodes[0].Propose(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("application failure acknowledged")
	}
	if c.nodes[0].Err() == nil || c.nodes[0].Status().LastApplied != 1 {
		t.Fatal("failed command marked applied")
	}
}

func TestCancelledProposalMayCommitLater(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.isolate(0)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := c.nodes[0].Propose(ctx, []byte(`{"op":"PUT","key":"after-cancel","value":"yes"}`))
		result <- err
	}()
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return c.nodes[0].lastLocked() == 2 })
	cancel()
	<-result
	c.heal()
	c.flush(0)
	c.equalStores()
	got, _ := c.stores[0].Snapshot()
	if !bytes.Contains(got, []byte("after-cancel")) {
		t.Fatal("cancellation undid accepted proposal")
	}
}

func TestAsynchronousClockKeepsHealthyPeerHeartbeats(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	blocked, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	heartbeats := make(chan struct{}, 10)
	c.net.mu.Lock()
	c.net.appendHook = func(from, to string, _ AppendEntriesRequest, _ AppendEntriesResponse) {
		if from != "0" {
			return
		}
		if to == "2" {
			once.Do(func() { close(blocked); <-release })
		}
		if to == "1" {
			heartbeats <- struct{}{}
		}
	}
	c.net.mu.Unlock()
	defer close(release)
	if err := c.nodes[0].tick(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("no blocked peer")
	}
	select {
	case <-heartbeats:
	case <-time.After(3 * time.Second):
		t.Fatal("healthy peer not contacted")
	}
	await(t, func() bool { c.nodes[0].mu.Lock(); defer c.nodes[0].mu.Unlock(); return !c.nodes[0].inflight["1"] })
	if err := c.nodes[0].tick(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-heartbeats:
	case <-time.After(3 * time.Second):
		t.Fatal("slow peer suspended heartbeats")
	}
}

func TestConcurrentProposals(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errs := make(chan error, 50)
	for i := 0; i < 50; i++ {
		go func(i int) {
			_, err := c.nodes[0].Propose(ctx, []byte(fmt.Sprintf(`{"op":"PUT","key":"%d","value":"ok"}`, i)))
			errs <- err
		}(i)
	}
	// Logical heartbeats drive proposals that arrived during an existing append.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			if err := <-errs; err != nil {
				t.Error(err)
			}
		}
	}()
loop:
	for {
		select {
		case <-done:
			break loop
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		default:
			c.nodes[0].replicate(ctx)
			runtime.Gosched()
		}
	}
	c.flush(0)
	c.equalStores()
	if c.nodes[0].Status().LastApplied != 51 {
		t.Fatal("lost proposal")
	}
}

func TestElectionTimerResetRules(t *testing.T) {
	c := newCluster(t, 3, false)
	n := c.nodes[1]
	_, _ = n.RequestVote(context.Background(), RequestVoteRequest{Term: 2, CandidateID: "0"})
	n.mu.Lock()
	n.elapsed = 3
	n.mu.Unlock()
	_, _ = n.RequestVote(context.Background(), RequestVoteRequest{Term: 2, CandidateID: "1"})
	n.mu.Lock()
	elapsed := n.elapsed
	n.mu.Unlock()
	if elapsed != 3 {
		t.Fatal("denied vote reset timer")
	}
	_, _ = n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 1, LeaderID: "0"})
	n.mu.Lock()
	elapsed = n.elapsed
	n.mu.Unlock()
	if elapsed != 3 {
		t.Fatal("stale leader reset timer")
	}
	_, _ = n.AppendEntries(context.Background(), AppendEntriesRequest{Term: 2, LeaderID: "0", PrevLogIndex: 99})
	n.mu.Lock()
	elapsed = n.elapsed
	n.mu.Unlock()
	if elapsed != 0 {
		t.Fatal("current leader failed to reset timer on log mismatch")
	}
}

func TestRunElectsAndStops(t *testing.T) {
	c := newCluster(t, 1, false)
	c.nodes[0].c.TickInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.nodes[0].Run(ctx) }()
	await(t, func() bool { return c.nodes[0].IsLeader() })
	if err := c.nodes[0].Run(ctx); err == nil {
		t.Fatal("allowed two runners")
	}
	c.propose(0, `{"op":"PUT","key":"run","value":"ok"}`)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit")
	}
}

func TestFiveNodesTolerateTwoUnavailablePeers(t *testing.T) {
	c := newCluster(t, 5, false)
	c.isolate(3)
	c.isolate(4)
	c.campaign(0)
	if !c.nodes[0].IsLeader() {
		t.Fatal("three voters did not elect")
	}
	c.propose(0, `{"op":"PUT","key":"majority","value":"three"}`)
	c.heal()
	c.flush(0)
	c.equalStores()
}

func TestUnsupportedOperationsAndOwnership(t *testing.T) {
	c := newCluster(t, 1, false)
	n := c.nodes[0]
	if r, err := n.InstallSnapshot(context.Background(), InstallSnapshotRequest{}); err != nil || r.Accepted {
		t.Fatal("accepted invalid snapshot", r, err)
	}
	if _, err := n.JoinCluster(context.Background(), JoinClusterRequest{}); !errors.Is(err, ErrNotLeader) {
		t.Fatal(err)
	}
	s := n.Status()
	s.Peers[0].ID = "modified"
	if n.Status().Peers[0].ID != "0" {
		t.Fatal("status leaked peer slice")
	}
	if _, err := n.Propose(context.Background(), nil); !errors.Is(err, ErrNotLeader) {
		t.Fatal(err)
	}
	r, _ := n.RequestVote(context.Background(), RequestVoteRequest{CandidateID: "0"})
	if r.VoteGranted {
		t.Fatal("term zero vote accepted")
	}
	c.campaign(0)
	command := []byte(`{"op":"PUT","key":"owned","value":"yes"}`)
	if _, err := n.Propose(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	command[0] = '!'
	n.mu.Lock()
	if n.log[2].Data[0] != '{' || c.disks[0].entries[1].Data[0] != '{' {
		t.Error("retained caller buffer")
	}
	n.mu.Unlock()
}

func TestOversizedProposalIsRejectedBeforeAppend(t *testing.T) {
	c := newCluster(t, 1, false)
	c.campaign(0)
	if _, err := c.nodes[0].Propose(context.Background(), make([]byte, MaxCommandBytes+1)); !errors.Is(err, ErrProposalTooLarge) {
		t.Fatal(err)
	}
	if c.nodes[0].Status().CommitIndex != 1 {
		t.Fatal("oversized proposal appended")
	}
}
