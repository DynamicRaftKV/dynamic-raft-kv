package raft

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
)

// membershipConfig is the Stage 6 configuration model. Current contains the
// desired peers; Outgoing preserves the old voters during a joint transition.
// A removed voter remains a voting/replication target until joint consensus
// ends. The latest durable entry determines the active configuration.
type membershipConfig struct {
	Version  int      `json:"version"`
	Current  []Peer   `json:"current"`
	Outgoing []Peer   `json:"outgoing,omitempty"`
	Removed  []string `json:"removed,omitempty"`
}

func stableMembership(peers []Peer) membershipConfig {
	return membershipConfig{Version: 2, Current: append([]Peer(nil), peers...)}
}

func (c membershipConfig) clone() membershipConfig {
	c.Current = append([]Peer(nil), c.Current...)
	c.Outgoing = append([]Peer(nil), c.Outgoing...)
	c.Removed = append([]string(nil), c.Removed...)
	return c
}

func voterIDs(peers []Peer) []string {
	var out []string
	for _, p := range peers {
		if !p.Learner {
			out = append(out, p.ID)
		}
	}
	return out
}

// hasQuorum counts distinct configured voter IDs only. Joint decisions need
// a majority of each set, never a majority of their combined union.
func (c membershipConfig) hasQuorum(yes func(string) bool) bool {
	majority := func(peers []Peer) bool {
		votes, total := 0, 0
		for _, p := range peers {
			if !p.Learner {
				total++
				if yes(p.ID) {
					votes++
				}
			}
		}
		return total > 0 && votes >= total/2+1
	}
	return majority(c.Current) && (len(c.Outgoing) == 0 || majority(c.Outgoing))
}

func (c membershipConfig) validate() error {
	if c.Version != 2 {
		return errors.New("raft: unsupported membership format")
	}
	removed := map[string]bool{}
	for _, id := range c.Removed {
		if id == "" || removed[id] {
			return errors.New("raft: invalid retired identity")
		}
		removed[id] = true
	}
	seenPeers := map[string]Peer{}
	addresses := map[string]string{}
	for setIndex, peers := range [][]Peer{c.Current, c.Outgoing} {
		if setIndex == 1 && len(peers) == 0 {
			continue
		}
		seen := map[string]bool{}
		voters := 0
		for _, p := range peers {
			if p.ID == "" || seen[p.ID] || removed[p.ID] {
				return errors.New("raft: duplicate, empty or retired peer")
			}
			seen[p.ID] = true
			if setIndex == 1 && p.Learner {
				return errors.New("raft: outgoing set must contain voters only")
			}
			if !p.Learner {
				voters++
			}
			if old, ok := seenPeers[p.ID]; ok && (old.GRPCAddress != p.GRPCAddress || old.HTTPAddress != p.HTTPAddress || old.Learner) {
				return errors.New("raft: inconsistent peer across voter sets")
			}
			seenPeers[p.ID] = p
			if p.GRPCAddress != "" {
				if id, ok := addresses[p.GRPCAddress]; ok && id != p.ID {
					return errors.New("raft: duplicate peer address")
				}
				addresses[p.GRPCAddress] = p.ID
			}
		}
		if voters == 0 {
			return errors.New("raft: empty voter set")
		}
	}
	return nil
}

// encodeMembership is canonical and owns its buffers. Unchanged version 1
// snapshots remain readable through snapshotMembership.
func encodeMembership(c membershipConfig) ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	c = c.clone()
	sort.Slice(c.Current, func(i, j int) bool { return c.Current[i].ID < c.Current[j].ID })
	sort.Slice(c.Outgoing, func(i, j int) bool { return c.Outgoing[i].ID < c.Outgoing[j].ID })
	sort.Strings(c.Removed)
	data, err := json.Marshal(c)
	if len(data) > MaxCommandBytes {
		return nil, ErrProposalTooLarge
	}
	return data, err
}

func decodeMembership(data []byte) (membershipConfig, error) {
	var c membershipConfig
	if len(data) > MaxCommandBytes {
		return c, ErrProposalTooLarge
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if d.Decode(new(any)) != io.EOF {
		return c, errors.New("raft: trailing membership data")
	}
	return c, c.validate()
}

// addLearner changes replication membership without changing either quorum.
func (c membershipConfig) addLearner(p Peer) (membershipConfig, error) {
	if err := c.validate(); err != nil {
		return c, err
	}
	if len(c.Outgoing) != 0 {
		return c, errors.New("raft: membership change already in progress")
	}
	if !p.Learner {
		return c, errors.New("raft: new members must join as learners")
	}
	for _, old := range c.Current {
		if old.ID == p.ID {
			return c, errors.New("raft: peer already exists")
		}
	}
	next := c.clone()
	next.Current = append(next.Current, p)
	return next, next.validate()
}

// changeVoter produces one joint transition. Promotion requires replication
// through the leader's current last index, supplied under the node lock by the
// administrative proposer. Removal cannot empty the voting set.
func (c membershipConfig) changeVoter(id string, promote bool, matched, last uint64) (membershipConfig, error) {
	if err := c.validate(); err != nil {
		return c, err
	}
	if len(c.Outgoing) != 0 {
		return c, errors.New("raft: membership change already in progress")
	}
	next := c.clone()
	found := false
	for _, p := range c.Current {
		if !p.Learner {
			next.Outgoing = append(next.Outgoing, p)
		}
	}
	for i, p := range next.Current {
		if p.ID == id {
			found = true
			if promote {
				if !p.Learner || matched < last {
					return c, errors.New("raft: promotion requires a caught-up learner")
				}
				next.Current[i].Learner = false
			} else {
				if p.Learner {
					return c, errors.New("raft: learner removal is not a voter transition")
				}
				next.Current = append(next.Current[:i], next.Current[i+1:]...)
			}
			break
		}
	}
	if !found {
		return c, errors.New("raft: unknown peer")
	}
	return next, next.validate()
}

func (c membershipConfig) removeLearner(id string) (membershipConfig, error) {
	if err := c.validate(); err != nil {
		return c, err
	}
	if len(c.Outgoing) != 0 {
		return c, errors.New("raft: membership change already in progress")
	}
	next := c.clone()
	for i, p := range next.Current {
		if p.ID == id {
			if !p.Learner {
				return c, errors.New("raft: voter removal requires joint consensus")
			}
			next.Current = append(next.Current[:i], next.Current[i+1:]...)
			next.Removed = append(next.Removed, id)
			return next, next.validate()
		}
	}
	return c, errors.New("raft: unknown peer")
}

func (c membershipConfig) leaveJoint() (membershipConfig, error) {
	if err := c.validate(); err != nil {
		return c, err
	}
	if len(c.Outgoing) == 0 {
		return c, errors.New("raft: not in joint consensus")
	}
	next := c.clone()
	present := map[string]bool{}
	for _, p := range next.Current {
		present[p.ID] = true
	}
	for _, p := range next.Outgoing {
		if !present[p.ID] {
			next.Removed = append(next.Removed, p.ID)
		}
	}
	next.Outgoing = nil
	return next, next.validate()
}
