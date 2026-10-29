package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

const snapshotChunkBytes = 256 << 10

type snapshotTransfer struct {
	leader   string
	term     uint64
	snapshot storage.Snapshot
}

func configuration(peers []Peer) []byte {
	ordered := append([]Peer(nil), peers...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	data, _ := json.Marshal(struct {
		Version int    `json:"version"`
		Peers   []Peer `json:"peers"`
	}{1, ordered})
	return data
}
func sameConfiguration(data []byte, peers []Peer) bool {
	return bytes.Equal(data, configuration(peers))
}
func copySnapshot(s *storage.Snapshot) *storage.Snapshot {
	if s == nil {
		return nil
	}
	r := *s
	r.Data, r.Configuration = bytes.Clone(s.Data), bytes.Clone(s.Configuration)
	return &r
}

// TakeSnapshot checkpoints only applied (therefore committed) entries.
func (n *Node) TakeSnapshot() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.err != nil {
		return n.err
	}
	return n.takeSnapshotLocked()
}
func (n *Node) takeSnapshotLocked() error {
	if n.applied == n.baseLocked() {
		return nil
	}
	data, err := n.c.StateMachine.Snapshot()
	if err != nil {
		return n.haltLocked(fmt.Errorf("raft snapshot: %w", err))
	}
	if len(data) > storage.MaxSnapshotBytes {
		return n.haltLocked(errors.New("raft: snapshot exceeds 32 MiB limit"))
	}
	config, err := encodeMembership(n.appliedMembers)
	if err != nil {
		return n.haltLocked(err)
	}
	// Keep unchanged Stage 5 snapshots byte-compatible.
	legacy, _ := encodeMembership(stableMembership(n.c.Peers))
	if bytes.Equal(config, legacy) {
		config = configuration(n.c.Peers)
	}
	s := storage.Snapshot{LastIncludedIndex: n.applied, LastIncludedTerm: n.entryLocked(n.applied).Term, Configuration: config, Data: data}
	if err = n.c.Storage.InstallSnapshot(s, true); err != nil {
		return n.haltLocked(err)
	}
	n.publishSnapshotLocked(s, true)
	if err = n.c.Storage.Compact(s.LastIncludedIndex); err != nil {
		return n.haltLocked(err)
	}
	return nil
}
func (n *Node) publishSnapshotLocked(s storage.Snapshot, retain bool) {
	var suffix []storage.Entry
	if retain {
		suffix = copyEntries(n.log[s.LastIncludedIndex-n.baseLocked()+1:])
	}
	n.log = append([]storage.Entry{{Index: s.LastIncludedIndex, Term: s.LastIncludedTerm}}, suffix...)
	n.snapshot = copySnapshot(&s)
	n.baseMembers, _ = snapshotMembership(s.Configuration, n.c.Peers)
	n.appliedMembers = n.baseMembers.clone()
	members, index, err := replayMembership(n.baseMembers, suffix)
	if err != nil {
		n.haltLocked(err)
		return
	}
	if index == 0 {
		index = s.LastIncludedIndex
	}
	n.setMembershipLocked(members, index)
}

// InstallSnapshot accepts ordered, bounded chunks. An interrupted transfer is
// memory-only; the sender restarts at zero. Only the final reply means durable.
func (n *Node) InstallSnapshot(ctx context.Context, r InstallSnapshotRequest) (InstallSnapshotResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	resp := func(ok bool, next uint64, err error) (InstallSnapshotResponse, error) {
		return InstallSnapshotResponse{Term: n.hard.Term, Accepted: ok, NextOffset: next}, err
	}
	if err := ctx.Err(); err != nil {
		return resp(false, 0, err)
	}
	if n.err != nil {
		return resp(false, 0, n.err)
	}
	if r.Term == 0 || r.Term < n.hard.Term {
		return resp(false, 0, nil)
	}
	// A lagging node may not yet know a newly promoted leader. The snapshot
	// must identify that leader as a voter. Peer transport is trusted (no TLS
	// authentication in the MVP); this is not an authorization mechanism.
	claimed, claimedErr := snapshotMembership(r.Snapshot.Configuration, n.c.Peers)
	known := false
	for _, p := range append(append([]Peer(nil), claimed.Current...), claimed.Outgoing...) {
		if p.ID == r.LeaderID && !p.Learner {
			known = true
		}
	}
	if claimedErr != nil || !known {
		return resp(false, 0, errors.New("raft: snapshot leader is not a voter"))
	}
	if err := n.followLocked(r.Term, r.LeaderID); err != nil {
		return resp(false, 0, err)
	}
	n.resetElectionLocked()
	s := r.Snapshot
	members, membershipErr := snapshotMembership(s.Configuration, n.c.Peers)
	if s.LastIncludedIndex == 0 || s.LastIncludedIndex == ^uint64(0) || s.LastIncludedTerm == 0 || s.LastIncludedTerm > r.Term || len(s.Data) > snapshotChunkBytes || membershipErr != nil {
		return resp(false, 0, errors.New("raft: invalid snapshot metadata or chunk"))
	}
	if r.Offset > storage.MaxSnapshotBytes || uint64(len(s.Data)) > storage.MaxSnapshotBytes-r.Offset {
		return resp(false, 0, errors.New("raft: snapshot exceeds limit"))
	}
	// Already applied prefixes cannot be replaced. This also makes a lost
	// final acknowledgement safe to retry after installation or restart.
	if s.LastIncludedIndex <= n.applied {
		return resp(true, r.Offset+uint64(len(s.Data)), nil)
	}
	if r.Offset == 0 {
		n.incoming = &snapshotTransfer{leader: r.LeaderID, term: r.Term, snapshot: storage.Snapshot{LastIncludedIndex: s.LastIncludedIndex, LastIncludedTerm: s.LastIncludedTerm, Configuration: bytes.Clone(s.Configuration)}}
	}
	t := n.incoming
	if t == nil || t.leader != r.LeaderID || t.term != r.Term || t.snapshot.LastIncludedIndex != s.LastIncludedIndex || t.snapshot.LastIncludedTerm != s.LastIncludedTerm || !bytes.Equal(t.snapshot.Configuration, s.Configuration) || r.Offset != uint64(len(t.snapshot.Data)) {
		return resp(false, 0, nil)
	}
	t.snapshot.Data = append(t.snapshot.Data, s.Data...)
	next := uint64(len(t.snapshot.Data))
	if !r.Done {
		return resp(true, next, nil)
	}
	s = t.snapshot
	n.incoming = nil
	retain := s.LastIncludedIndex <= n.lastLocked() && n.entryLocked(s.LastIncludedIndex).Term == s.LastIncludedTerm
	if retain {
		if _, _, err := replayMembership(members, n.log[s.LastIncludedIndex-n.baseLocked()+1:]); err != nil {
			return resp(false, 0, err)
		}
	}
	// Restore validates atomically. A subsequent persistence failure halts the
	// node before acknowledgement; restart recovers only the durable generation.
	if err := n.c.StateMachine.Restore(s.Data); err != nil {
		return resp(false, 0, err)
	}
	if err := n.c.Storage.InstallSnapshot(s, retain); err != nil {
		return resp(false, 0, n.haltLocked(err))
	}
	n.publishSnapshotLocked(s, retain)
	n.commit, n.applied = s.LastIncludedIndex, s.LastIncludedIndex
	if err := n.c.Storage.Compact(s.LastIncludedIndex); err != nil {
		return resp(false, 0, n.haltLocked(err))
	}
	return resp(true, next, nil)
}

func (n *Node) sendSnapshot(ctx context.Context, p Peer, term uint64, snapshot *storage.Snapshot) {
	defer func() { n.mu.Lock(); n.inflight[p.ID] = false; n.mu.Unlock() }()
	if snapshot == nil {
		return
	}
	for offset := 0; ; {
		end := offset + snapshotChunkBytes
		if end > len(snapshot.Data) {
			end = len(snapshot.Data)
		}
		chunk := *snapshot
		chunk.Data = snapshot.Data[offset:end]
		rpc, cancel := n.rpcContext(ctx)
		r, err := n.c.Transport.SendInstallSnapshot(rpc, p, InstallSnapshotRequest{Term: term, LeaderID: n.c.ID, Snapshot: chunk, Offset: uint64(offset), Done: end == len(snapshot.Data)})
		cancel()
		n.mu.Lock()
		if err == nil && r.Term > n.hard.Term {
			n.followLocked(r.Term, "")
		}
		valid := err == nil && n.err == nil && n.role == Leader && n.hard.Term == term && r.Term == term && r.Accepted && r.NextOffset == uint64(end)
		if valid && end == len(snapshot.Data) {
			if snapshot.LastIncludedIndex > n.match[p.ID] {
				n.match[p.ID], n.next[p.ID] = snapshot.LastIncludedIndex, snapshot.LastIncludedIndex+1
			}
			n.advanceLocked()
		}
		n.mu.Unlock()
		if !valid || end == len(snapshot.Data) {
			return
		}
		offset = end
	}
}
