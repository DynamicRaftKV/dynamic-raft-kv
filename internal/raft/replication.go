package raft

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

func (n *Node) AppendEntries(ctx context.Context, r AppendEntriesRequest) (AppendEntriesResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	resp := func(ok bool, err error) (AppendEntriesResponse, error) {
		return AppendEntriesResponse{Term: n.hard.Term, Success: ok}, err
	}
	if err := ctx.Err(); err != nil {
		return resp(false, err)
	}
	if n.err != nil {
		return resp(false, n.err)
	}
	if !n.voter(r.LeaderID) || r.Term < n.hard.Term || r.Term == 0 {
		return resp(false, nil)
	}
	if err := n.followLocked(r.Term, r.LeaderID); err != nil {
		return resp(false, err)
	}
	n.resetElectionLocked()
	if r.PrevLogIndex > n.lastLocked() || n.log[r.PrevLogIndex].Term != r.PrevLogTerm {
		return resp(false, nil)
	}
	prev := r.PrevLogTerm
	for i, e := range r.Entries {
		if len(e.Data) > MaxCommandBytes {
			return resp(false, ErrProposalTooLarge)
		}
		if e.Index != r.PrevLogIndex+uint64(i)+1 || e.Term == 0 || e.Term < prev || e.Term > r.Term {
			return resp(false, errors.New("raft: invalid append batch"))
		}
		if e.Kind != storage.EntryCommand && e.Kind != storage.EntryNoop {
			return resp(false, ErrUnsupported)
		}
		prev = e.Term
	}
	for i, e := range r.Entries {
		if e.Index > n.lastLocked() || n.log[e.Index].Term != e.Term {
			if e.Index <= n.commit {
				return resp(false, errors.New("raft: conflicting committed entry"))
			}
			if err := n.appendLocked(r.Entries[i:]); err != nil {
				return resp(false, err)
			}
			break
		}
	}
	// Only this request's matched prefix is known to agree with the leader.
	bound := r.PrevLogIndex + uint64(len(r.Entries))
	if r.LeaderCommit < bound {
		bound = r.LeaderCommit
	}
	if bound > n.commit {
		n.commit = bound
		if err := n.applyLocked(); err != nil {
			return resp(false, err)
		}
	}
	return resp(true, nil)
}

func (n *Node) replicate(ctx context.Context) {
	var wg sync.WaitGroup
	for _, p := range n.c.Peers {
		if p.ID == n.c.ID {
			continue
		}
		wg.Add(1)
		go func(p Peer) {
			defer wg.Done()
			n.mu.Lock()
			if n.err != nil || n.role != Leader || n.inflight[p.ID] {
				n.mu.Unlock()
				return
			}
			n.inflight[p.ID] = true
			next := n.next[p.ID]
			prev := n.log[next-1]
			// Leave wire-encoding headroom below gRPC's default 4 MiB receive
			// limit. Catch-up proceeds over successive heartbeat batches.
			end, size := next, 0
			for end <= n.lastLocked() && end-next < 128 {
				cost := len(n.log[end].Data) + 64
				if size+cost > 2*MaxCommandBytes {
					break
				}
				size += cost
				end++
			}
			r := AppendEntriesRequest{Term: n.hard.Term, LeaderID: n.c.ID, PrevLogIndex: prev.Index, PrevLogTerm: prev.Term, Entries: copyEntries(n.log[next:end]), LeaderCommit: n.commit}
			n.mu.Unlock()
			rpc, cancel := n.rpcContext(ctx)
			defer cancel()
			resp, err := n.c.Transport.SendAppendEntries(rpc, p, r)
			n.mu.Lock()
			defer n.mu.Unlock()
			n.inflight[p.ID] = false
			if err != nil || n.err != nil {
				return
			}
			if resp.Term > n.hard.Term {
				n.followLocked(resp.Term, "")
				return
			}
			if n.role != Leader || n.hard.Term != r.Term || resp.Term != r.Term {
				return
			}
			if resp.Success {
				matched := r.PrevLogIndex + uint64(len(r.Entries))
				if matched > n.match[p.ID] {
					n.match[p.ID] = matched
					n.next[p.ID] = matched + 1
				}
				n.advanceLocked()
			} else if n.next[p.ID] > n.match[p.ID]+1 {
				n.next[p.ID]--
			}
		}(p)
	}
	wg.Wait()
}

func (n *Node) advanceLocked() {
	for i := n.lastLocked(); i > n.commit; i-- {
		if n.log[i].Term != n.hard.Term {
			continue
		}
		votes := 0
		for id, p := range n.peers {
			if !p.Learner && n.match[id] >= i {
				votes++
			}
		}
		if votes >= n.quorumLocked() {
			n.commit = i
			n.applyLocked()
			return
		}
	}
}
func (n *Node) applyLocked() error {
	for n.applied < n.commit {
		e := n.log[n.applied+1]
		var data []byte
		var err error
		if e.Kind == storage.EntryCommand {
			data, err = n.c.StateMachine.Apply(append([]byte(nil), e.Data...))
		}
		if err != nil && !errors.Is(err, state.ErrInvalidCommand) {
			return n.haltLocked(fmt.Errorf("raft apply: %w", err))
		}
		n.applied = e.Index
		if ch, ok := n.pending[e.Index]; ok {
			ch <- proposalResult{data: append([]byte(nil), data...), err: err}
			delete(n.pending, e.Index)
		}
	}
	return nil
}

func (n *Node) Propose(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) > MaxCommandBytes {
		return nil, ErrProposalTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n.mu.Lock()
	if n.err != nil {
		err := n.err
		n.mu.Unlock()
		return nil, err
	}
	if n.role != Leader {
		n.mu.Unlock()
		return nil, ErrNotLeader
	}
	index := n.lastLocked() + 1
	if err := n.appendLocked([]storage.Entry{{Index: index, Term: n.hard.Term, Kind: storage.EntryCommand, Data: data}}); err != nil {
		n.mu.Unlock()
		return nil, err
	}
	ch := make(chan proposalResult, 1)
	n.pending[index] = ch
	n.match[n.c.ID] = index
	n.advanceLocked()
	n.mu.Unlock()
	// Replication is bounded by the caller and node lifetime. A slow minority
	// must not delay a result already committed by the majority.
	go n.replicate(ctx)
	defer func() {
		n.mu.Lock()
		if n.pending[index] == ch {
			delete(n.pending, index)
		}
		n.mu.Unlock()
	}()
	select {
	case r := <-ch:
		return r.data, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (n *Node) InstallSnapshot(context.Context, InstallSnapshotRequest) (InstallSnapshotResponse, error) {
	return InstallSnapshotResponse{Term: n.Status().Term}, ErrUnsupported
}
func (n *Node) JoinCluster(context.Context, JoinClusterRequest) (JoinClusterResponse, error) {
	return JoinClusterResponse{LeaderHTTPAddress: n.Status().LeaderHTTPAddress}, ErrUnsupported
}
