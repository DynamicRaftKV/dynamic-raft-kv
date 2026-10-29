package raft

import (
	"bytes"
	"math/bits"
	"reflect"
	"testing"
)

func membershipPeers(ids ...string) []Peer {
	var out []Peer
	for _, id := range ids {
		out = append(out, Peer{ID: id, GRPCAddress: id + ":9001", HTTPAddress: "http://" + id + ":8001"})
	}
	return out
}
func mustMembership(t *testing.T, c membershipConfig, err error) membershipConfig {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestJointQuorumExhaustive(t *testing.T) {
	// Enumerate every nonempty old/new voter subset and every response subset
	// in a five-node universe. Learner/unknown acknowledgements never help.
	peers := membershipPeers("a", "b", "c", "d", "e")
	for old := 1; old < 32; old++ {
		for next := 1; next < 32; next++ {
			c := membershipConfig{Version: 2}
			for i, p := range peers {
				if old&(1<<i) != 0 {
					c.Outgoing = append(c.Outgoing, p)
				}
				if next&(1<<i) != 0 {
					c.Current = append(c.Current, p)
				}
			}
			c.Current = append(c.Current, Peer{ID: "learner", Learner: true})
			if err := c.validate(); err != nil {
				t.Fatal(err)
			}
			for replies := 0; replies < 32; replies++ {
				yes := map[string]bool{"learner": true, "unknown": true}
				for i, p := range peers {
					yes[p.ID] = replies&(1<<i) != 0
				}
				want := bits.OnesCount(uint(old&replies)) > bits.OnesCount(uint(old))/2 && bits.OnesCount(uint(next&replies)) > bits.OnesCount(uint(next))/2
				if got := c.hasQuorum(func(id string) bool { return yes[id] }); got != want {
					t.Fatalf("old=%05b new=%05b replies=%05b got=%v want=%v", old, next, replies, got, want)
				}
			}
		}
	}
	if (membershipConfig{}).hasQuorum(func(string) bool { return true }) {
		t.Fatal("empty configuration elected/committed")
	}
}

func TestMembershipTransitionsAndIdentityRetirement(t *testing.T) {
	initial := stableMembership(membershipPeers("a", "b", "c"))
	original, _ := encodeMembership(initial)
	learner := membershipPeers("d")[0]
	learner.Learner = true
	c, err := initial.addLearner(learner)
	c = mustMembership(t, c, err)
	if _, err := c.changeVoter("d", true, 9, 10); err == nil {
		t.Fatal("promoted lagging learner")
	}
	joint, err := c.changeVoter("d", true, 10, 10)
	joint = mustMembership(t, joint, err)
	if len(joint.Outgoing) != 3 || len(voterIDs(joint.Current)) != 4 {
		t.Fatal(joint)
	}
	if _, err := joint.addLearner(Peer{ID: "e", Learner: true}); err == nil {
		t.Fatal("overlapping change accepted")
	}
	if _, err := joint.changeVoter("a", false, 0, 0); err == nil {
		t.Fatal("overlapping voter change accepted")
	}
	c, err = joint.leaveJoint()
	c = mustMembership(t, c, err)
	if len(c.Outgoing) != 0 {
		t.Fatal("joint state retained")
	}
	joint, err = c.changeVoter("a", false, 0, 0)
	joint = mustMembership(t, joint, err)
	// Removed voter a remains in the outgoing quorum until finalization.
	if !joint.hasQuorum(func(id string) bool { return id == "a" || id == "b" || id == "c" }) {
		t.Fatal("outgoing voter removed prematurely")
	}
	c, err = joint.leaveJoint()
	c = mustMembership(t, c, err)
	if !reflect.DeepEqual(c.Removed, []string{"a"}) {
		t.Fatal("retired identity missing")
	}
	retired := membershipPeers("a")[0]
	retired.Learner = true
	if _, err := c.addLearner(retired); err == nil {
		t.Fatal("reused retired ID")
	}
	after, _ := encodeMembership(initial)
	if !bytes.Equal(original, after) {
		t.Fatal("transition mutated source")
	}
	encoded, err := encodeMembership(c)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := decodeMembership(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.addLearner(retired); err == nil {
		t.Fatal("serialization lost retired identity")
	}
}

func TestMembershipRejectsUnsafeOperations(t *testing.T) {
	c := stableMembership(membershipPeers("only"))
	if _, err := c.changeVoter("only", false, 0, 0); err == nil {
		t.Fatal("removed last voter")
	}
	if _, err := c.changeVoter("missing", false, 0, 0); err == nil {
		t.Fatal("removed unknown voter")
	}
	if _, err := c.changeVoter("only", true, 100, 100); err == nil {
		t.Fatal("promoted voter")
	}
	if _, err := c.removeLearner("only"); err == nil {
		t.Fatal("removed voter without joint transition")
	}
	if _, err := c.leaveJoint(); err == nil {
		t.Fatal("left nonexistent transition")
	}
	if _, err := c.addLearner(Peer{ID: "new"}); err == nil {
		t.Fatal("added voter directly")
	}
	withLearner, err := c.addLearner(Peer{ID: "learner", Learner: true})
	withLearner = mustMembership(t, withLearner, err)
	if _, err := withLearner.changeVoter("learner", false, 0, 0); err == nil {
		t.Fatal("learner used as outgoing voter")
	}
	removed, err := withLearner.removeLearner("learner")
	removed = mustMembership(t, removed, err)
	if len(removed.Outgoing) != 0 || !reflect.DeepEqual(removed.Removed, []string{"learner"}) {
		t.Fatal(removed)
	}
	if !removed.hasQuorum(func(id string) bool { return id == "only" }) {
		t.Fatal("learner removal changed quorum")
	}
}

func TestMembershipEncodingAndValidation(t *testing.T) {
	c := stableMembership(membershipPeers("b", "a", "c"))
	c.Removed = []string{"z", "y"}
	before := c.clone()
	first, err := encodeMembership(c)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, before) {
		t.Fatal("encoder mutated source")
	}
	c.Current[0], c.Current[1] = c.Current[1], c.Current[0]
	c.Removed[0], c.Removed[1] = c.Removed[1], c.Removed[0]
	second, err := encodeMembership(c)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("noncanonical encoding", err)
	}
	for _, data := range [][]byte{[]byte(`{}`), []byte(`{"version":1,"peers":[]}`), append(bytes.Clone(first), []byte(` {}`)...), []byte(`{"version":2,"current":[],"unknown":true}`), []byte(`{"version":2,"current":[{"id":"a"},{"id":"a"}]}`)} {
		if _, err := decodeMembership(data); err == nil {
			t.Fatalf("accepted invalid encoding %s", data)
		}
	}
	for _, bad := range []membershipConfig{
		{Version: 2, Current: []Peer{{ID: "l", Learner: true}}},
		{Version: 2, Current: membershipPeers("a"), Outgoing: []Peer{{ID: "a", Learner: true}}},
		{Version: 2, Current: membershipPeers("a"), Removed: []string{"a"}},
		{Version: 2, Current: membershipPeers("a"), Removed: []string{"b", "b"}},
		{Version: 2, Current: []Peer{{ID: "a", Learner: true}, {ID: "b"}}, Outgoing: []Peer{{ID: "a"}}},
		{Version: 2, Current: []Peer{{ID: "a", GRPCAddress: "same"}, {ID: "b", GRPCAddress: "same"}}},
	} {
		if err := bad.validate(); err == nil {
			t.Fatalf("accepted invalid configuration %+v", bad)
		}
	}
}

func FuzzMembershipDecoder(f *testing.F) {
	data, _ := encodeMembership(stableMembership(membershipPeers("a", "b", "c")))
	f.Add(data)
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, input []byte) {
		c, err := decodeMembership(input)
		if err != nil {
			return
		}
		encoded, err := encodeMembership(c)
		if err != nil {
			t.Fatal(err)
		}
		roundtrip, err := decodeMembership(encoded)
		if err != nil {
			t.Fatal(err)
		}
		again, err := encodeMembership(roundtrip)
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatal("unstable roundtrip", err)
		}
	})
}
