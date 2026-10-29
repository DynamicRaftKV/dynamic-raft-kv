package raft

import (
	"context"
	"errors"
	"sync"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

// Campaign starts a new election; normal callers let Tick choose its deadline.
func (n *Node) Campaign(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n.mu.Lock()
	if n.err != nil {
		err := n.err
		n.mu.Unlock()
		return err
	}
	if !n.voter(n.c.ID) || n.role == Leader {
		n.mu.Unlock()
		return nil
	}
	req := n.startCampaignLocked()
	n.mu.Unlock()
	if req.Term != 0 {
		n.sendVotes(ctx, req)
	}
	return n.Err()
}

func (n *Node) startCampaignLocked() RequestVoteRequest {
	if n.hard.Term == ^uint64(0) {
		n.haltLocked(errors.New("raft: term exhausted"))
		return RequestVoteRequest{}
	}
	if err := n.saveLocked(storage.HardState{Term: n.hard.Term + 1, VotedFor: n.c.ID}); err != nil {
		return RequestVoteRequest{}
	}
	n.role = Candidate
	n.leader = ""
	n.resetElectionLocked()
	n.incoming = nil
	last := n.entryLocked(n.lastLocked())
	req := RequestVoteRequest{Term: n.hard.Term, CandidateID: n.c.ID, LastLogIndex: last.Index, LastLogTerm: last.Term}
	if n.hasQuorumLocked(func(id string) bool { return id == n.c.ID }) {
		n.leadLocked()
	}
	return req
}

func (n *Node) sendVotes(ctx context.Context, req RequestVoteRequest) {
	votes := map[string]bool{n.c.ID: true} // accessed only with n.mu held
	var wg sync.WaitGroup
	for _, p := range n.peerList() {
		if p.ID == n.c.ID || p.Learner {
			continue
		}
		wg.Add(1)
		go func(p Peer) {
			defer wg.Done()
			rpc, cancel := n.rpcContext(ctx)
			defer cancel()
			r, err := n.c.Transport.SendRequestVote(rpc, p, req)
			n.mu.Lock()
			defer n.mu.Unlock()
			if err != nil || n.err != nil {
				return
			}
			if r.Term > n.hard.Term {
				n.followLocked(r.Term, "")
				return
			}
			if n.role != Candidate || n.hard.Term != req.Term || r.Term != req.Term || !r.VoteGranted {
				return
			}
			votes[p.ID] = true
			if n.hasQuorumLocked(func(id string) bool { return votes[id] }) {
				n.leadLocked()
			}
		}(p)
	}
	wg.Wait()
	n.replicate(ctx)
}

func (n *Node) leadLocked() {
	n.role = Leader
	n.leader = n.c.ID
	n.heartbeat = 0
	last := n.lastLocked()
	for id := range n.peers {
		n.next[id] = last + 1
		n.match[id] = 0
	}
	if n.appendLocked([]storage.Entry{{Index: last + 1, Term: n.hard.Term, Kind: storage.EntryNoop}}) != nil {
		return
	}
	n.match[n.c.ID] = n.lastLocked()
	n.advanceLocked()
}

func (n *Node) RequestVote(ctx context.Context, r RequestVoteRequest) (RequestVoteResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	resp := func(ok bool, err error) (RequestVoteResponse, error) {
		return RequestVoteResponse{Term: n.hard.Term, VoteGranted: ok}, err
	}
	if err := ctx.Err(); err != nil {
		return resp(false, err)
	}
	if n.err != nil {
		return resp(false, n.err)
	}
	if !n.voter(r.CandidateID) || r.Term < n.hard.Term || r.Term == 0 {
		return resp(false, nil)
	}
	if r.Term > n.hard.Term {
		if err := n.followLocked(r.Term, ""); err != nil {
			return resp(false, err)
		}
	}
	last := n.entryLocked(n.lastLocked())
	fresh := r.LastLogTerm > last.Term || (r.LastLogTerm == last.Term && r.LastLogIndex >= last.Index)
	if !n.voter(n.c.ID) || !fresh || (n.hard.VotedFor != "" && n.hard.VotedFor != r.CandidateID) {
		return resp(false, nil)
	}
	if err := n.saveLocked(storage.HardState{Term: r.Term, VotedFor: r.CandidateID}); err != nil {
		return resp(false, err)
	}
	n.resetElectionLocked()
	return resp(true, nil)
}
