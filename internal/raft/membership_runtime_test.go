package raft

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

func change(t *testing.T, n *Node, op, id string, p *Peer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := n.ChangeMembership(ctx, op, id, p); err != nil {
		t.Fatal(err)
	}
}

func TestMembershipJoinPromoteRemoveRestart(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.propose(0, `{"op":"PUT","key":"before","value":"join"}`)
	disk := &memoryDisk{}
	store := state.NewStore()
	makeJoiner := func() *Node {
		n, err := NewNode(Config{ID: "new", Peers: c.peers, Joining: true, Storage: disk, StateMachine: store, Transport: testTransport{net: c.net, from: "new"}})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	joiner := makeJoiner()
	defer func() { joiner.Stop() }()
	c.net.mu.Lock()
	c.net.nodes["new"] = joiner
	c.net.mu.Unlock()
	if err := joiner.Campaign(context.Background()); err != nil || joiner.Status().Term != 0 {
		t.Fatal("joiner campaigned", err)
	}
	p := Peer{ID: "new", Learner: true}
	change(t, c.nodes[0], "add_learner", "", &p)
	c.flush(0)
	if joiner.Status().LastApplied == 0 {
		t.Fatal("joiner did not receive admission snapshot")
	}
	change(t, c.nodes[0], "promote", "new", nil)
	c.flush(0)
	if c.nodes[0].Status().MembershipPending {
		t.Fatal("promotion did not finalize")
	}
	if !joiner.voterForTest("new") {
		t.Fatal("joiner still learner")
	}
	joiner.Stop()
	store = state.NewStore()
	joiner = makeJoiner()
	c.net.mu.Lock()
	c.net.nodes["new"] = joiner
	c.net.mu.Unlock()
	c.flush(0)
	if !joiner.voterForTest("new") {
		t.Fatal("promotion lost on restart")
	}
	change(t, c.nodes[0], "remove", "1", nil)
	c.flush(0)
	if c.nodes[0].voterForTest("1") || c.nodes[0].Status().MembershipPending {
		t.Fatal("removal did not finalize")
	}
	c.restart(0)
	c.campaign(0)
	c.flush(0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	retired := Peer{ID: "1", Learner: true}
	if err := c.nodes[0].ChangeMembership(ctx, "add_learner", "", &retired); err == nil {
		t.Fatal("retired identity reused")
	}
}

func (n *Node) voterForTest(id string) bool { n.mu.Lock(); defer n.mu.Unlock(); return n.voter(id) }

func TestMembershipRejectsUncaughtLearnerAndOverlappingChange(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	p := Peer{ID: "offline", Learner: true}
	change(t, c.nodes[0], "add_learner", "", &p)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.nodes[0].ChangeMembership(ctx, "promote", "offline", nil); err == nil {
		t.Fatal("promoted offline learner")
	}
	c.isolate(0)
	short, stop := context.WithTimeout(ctx, 10*time.Millisecond)
	defer stop()
	if err := c.nodes[0].ChangeMembership(short, "remove", "1", nil); err == nil {
		t.Fatal("committed without quorum")
	}
	if err := c.nodes[0].ChangeMembership(ctx, "remove", "2", nil); err == nil {
		t.Fatal("overlapping change accepted")
	}
}

func TestMembershipRecoveredJointAndTruncation(t *testing.T) {
	c := newCluster(t, 3, false)
	n := c.nodes[0]
	data, _ := json.Marshal(membershipEntry{Version: 1, Operation: "remove_voter", ID: "2"})
	n.mu.Lock()
	if err := n.saveLocked(storage.HardState{Term: 1}); err != nil {
		t.Fatal(err)
	}
	if err := n.appendLocked([]storage.Entry{{Index: 1, Term: 1, Kind: storage.EntryConfiguration, Data: data}}); err != nil {
		t.Fatal(err)
	}
	n.mu.Unlock()
	c.restart(0)
	if len(c.nodes[0].Status().OutgoingVoters) != 3 {
		t.Fatal("joint state lost on restart")
	}
	r, err := c.nodes[0].AppendEntries(context.Background(), AppendEntriesRequest{Term: 2, LeaderID: "1", Entries: []storage.Entry{{Index: 1, Term: 2, Kind: storage.EntryNoop}}})
	if err != nil || !r.Success {
		t.Fatal(r, err)
	}
	if len(c.nodes[0].Status().OutgoingVoters) != 0 || !c.nodes[0].voterForTest("2") {
		t.Fatal("truncation did not restore configuration")
	}
}

func TestMembershipLeaderFailureAfterJointCommit(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	c.flush(0)
	n := c.nodes[0]
	data, _ := json.Marshal(membershipEntry{Version: 1, Operation: "remove_voter", ID: "0"})
	n.mu.Lock()
	index := n.lastLocked() + 1
	if err := n.appendLocked([]storage.Entry{{Index: index, Term: n.hard.Term, Kind: storage.EntryConfiguration, Data: data}}); err != nil {
		t.Fatal(err)
	}
	n.match["0"] = index
	// Joint entry can commit, but the automatic final entry cannot persist.
	c.disks[0].failAppend = true
	n.mu.Unlock()
	c.flush(0)
	if n.Err() == nil {
		t.Fatal("finalization failure did not halt leader")
	}
	c.campaign(1)
	c.flush(1)
	if !c.nodes[1].IsLeader() || c.nodes[1].Status().MembershipPending || c.nodes[1].voterForTest("0") {
		t.Fatal("new leader did not finish joint transition")
	}
	c.propose(1, `{"op":"PUT","key":"after-change","value":"safe"}`)
	// The old leader's last durable state was joint consensus. Recovery must
	// install final configuration from the new leader without resurrecting it.
	c.disks[0].failAppend = false
	c.restart(0)
	if len(c.nodes[0].Status().OutgoingVoters) == 0 {
		t.Fatal("missing durable joint checkpoint")
	}
}

func TestMembershipRemoveLeaderAndLastVoterGuard(t *testing.T) {
	c := newCluster(t, 3, false)
	c.campaign(0)
	change(t, c.nodes[0], "remove", "0", nil)
	c.flush(0)
	if c.nodes[0].IsLeader() {
		t.Fatal("removed leader did not step down")
	}
	c.campaign(1)
	c.flush(1)
	c.propose(1, `{"op":"PUT","key":"remaining","value":"healthy"}`)
	single := newCluster(t, 1, false)
	single.campaign(0)
	if err := single.nodes[0].ChangeMembership(context.Background(), "remove", "0", nil); err == nil {
		t.Fatal("removed last voter")
	}
}
