package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

// Membership operations are serialized log entries. Voter changes enter joint
// consensus; only the leader may append finalize after the joint entry commits.
type membershipEntry struct {
	Version   int    `json:"version"`
	Operation string `json:"operation"`
	Peer      *Peer  `json:"peer,omitempty"`
	ID        string `json:"id,omitempty"`
}

func reduceMembership(c membershipConfig, data []byte) (membershipConfig, error) {
	if len(data) > MaxCommandBytes {
		return c, ErrProposalTooLarge
	}
	var e membershipEntry
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return c, err
	}
	if d.Decode(new(any)) != io.EOF || e.Version != 1 {
		return c, errors.New("raft: invalid membership entry")
	}
	if e.Operation == "add_learner" {
		if e.Peer == nil || e.ID != "" {
			return c, errors.New("raft: invalid learner entry")
		}
		return c.addLearner(*e.Peer)
	}
	if e.Peer != nil {
		return c, errors.New("raft: unexpected peer")
	}
	switch e.Operation {
	case "promote":
		return c.changeVoter(e.ID, true, 0, 0)
	case "remove_learner":
		return c.removeLearner(e.ID)
	case "remove_voter":
		return c.changeVoter(e.ID, false, 0, 0)
	case "finalize":
		if e.ID != "" {
			return c, errors.New("raft: unexpected member ID")
		}
		return c.leaveJoint()
	default:
		return c, errors.New("raft: unknown membership operation")
	}
}

func replayMembership(base membershipConfig, entries []storage.Entry) (membershipConfig, uint64, error) {
	c, index := base.clone(), uint64(0)
	for _, e := range entries {
		if e.Kind == storage.EntryConfiguration {
			var err error
			c, err = reduceMembership(c, e.Data)
			if err != nil {
				return c, index, err
			}
			if _, err := encodeMembership(c); err != nil {
				return c, index, err
			}
			index = e.Index
		}
	}
	return c, index, nil
}

func snapshotMembership(data []byte, bootstrap []Peer) (membershipConfig, error) {
	if sameConfiguration(data, bootstrap) {
		return stableMembership(bootstrap), nil
	}
	return decodeMembership(data)
}

func (n *Node) setMembershipLocked(c membershipConfig, index uint64) {
	n.members, n.configIndex = c, index
	peers := make(map[string]Peer)
	for _, p := range c.Current {
		peers[p.ID] = p
	}
	for _, p := range c.Outgoing {
		peers[p.ID] = p
	}
	for id := range peers {
		if _, exists := n.peers[id]; !exists {
			n.next[id] = n.lastLocked() + 1
			n.match[id] = 0
		}
	}
	n.peers = peers
}

func (n *Node) peerList() []Peer {
	n.mu.Lock()
	defer n.mu.Unlock()
	var peers []Peer
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	return peers
}

// ChangeMembership waits for the requested entry to commit. Promotion/removal
// return after entry into joint consensus; Status exposes finalization progress.
func (n *Node) ChangeMembership(ctx context.Context, operation, id string, peer *Peer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n.mu.Lock()
	if n.err != nil {
		err := n.err
		n.mu.Unlock()
		return err
	}
	if n.role != Leader {
		n.mu.Unlock()
		return ErrNotLeader
	}
	if n.configIndex > n.applied || len(n.members.Outgoing) != 0 || n.commit == 0 || n.entryLocked(n.commit).Term != n.hard.Term {
		n.mu.Unlock()
		return errors.New("raft: membership change pending or leader not ready")
	}
	if operation == "promote" && n.match[id] < n.lastLocked() {
		n.mu.Unlock()
		return errors.New("raft: learner has not caught up")
	}
	if operation != "add_learner" && operation != "promote" && operation != "remove" {
		n.mu.Unlock()
		return errors.New("raft: unsupported membership request")
	}
	if operation == "remove" {
		operation = "remove_voter"
		if n.peers[id].Learner {
			operation = "remove_learner"
		}
	}
	e := membershipEntry{Version: 1, Operation: operation, ID: id, Peer: peer}
	data, err := json.Marshal(e)
	if err == nil {
		_, err = reduceMembership(n.members, data)
	}
	if err != nil {
		n.mu.Unlock()
		return err
	}
	index := n.lastLocked() + 1
	if err := n.appendLocked([]storage.Entry{{Index: index, Term: n.hard.Term, Kind: storage.EntryConfiguration, Data: data}}); err != nil {
		n.mu.Unlock()
		return err
	}
	ch := make(chan proposalResult, 1)
	n.pending[index] = ch
	n.match[n.c.ID] = index
	n.advanceLocked()
	n.mu.Unlock()
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
		return r.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *Node) JoinCluster(ctx context.Context, r JoinClusterRequest) (JoinClusterResponse, error) {
	p := r.Peer
	p.Learner = true
	err := n.ChangeMembership(ctx, "add_learner", "", &p)
	return JoinClusterResponse{Accepted: err == nil, LeaderHTTPAddress: n.Status().LeaderHTTPAddress}, err
}

func (n *Node) finalizeMembershipLocked() error {
	if n.role != Leader || n.configIndex > n.applied || len(n.members.Outgoing) == 0 {
		return nil
	}
	data, _ := json.Marshal(membershipEntry{Version: 1, Operation: "finalize"})
	index := n.lastLocked() + 1
	if err := n.appendLocked([]storage.Entry{{Index: index, Term: n.hard.Term, Kind: storage.EntryConfiguration, Data: data}}); err != nil {
		return err
	}
	n.match[n.c.ID] = index
	n.advanceLocked()
	return n.err
}
