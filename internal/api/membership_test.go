package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

type adminNode struct {
	apiNode
	operation, id string
	peer          *raft.Peer
}

func (n *adminNode) ChangeMembership(_ context.Context, op, id string, p *raft.Peer) error {
	n.calls++
	n.operation, n.id, n.peer = op, id, p
	return n.err
}

func TestMembershipHTTPContracts(t *testing.T) {
	for _, tc := range []struct {
		path, body string
		err        error
		code       int
	}{
		{"join", `{"id":"n4","grpc_address":"localhost:9104","http_address":"http://localhost:8104"}`, nil, 202},
		{"join", `{"id":"n4","grpc_address":"bad","http_address":"http://localhost"}`, nil, 400},
		{"promote", `{"id":"n4"}`, nil, 202},
		{"remove", `{"id":"n4"}`, nil, 202},
		{"promote", `{"id":"n4","extra":true}`, nil, 400},
		{"remove", `{"id":"n4"}`, raft.ErrNotLeader, 307},
		{"promote", `{"id":"n4"}`, errors.New("not caught up"), 409},
		{"remove", `{"id":"n4"}`, context.DeadlineExceeded, 504},
	} {
		n := &adminNode{apiNode: apiNode{err: tc.err, status: raft.NodeStatus{LeaderHTTPAddress: "http://leader:8101"}}}
		w := httptest.NewRecorder()
		NewRaftHandler(n).ServeHTTP(w, httptest.NewRequest("POST", "/cluster/"+tc.path, strings.NewReader(tc.body)))
		if w.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
		if tc.code == 400 && n.calls != 0 {
			t.Fatal("invalid request reached consensus")
		}
		if tc.path == "join" && tc.code == 202 && (n.peer == nil || !n.peer.Learner || n.operation != "add_learner") {
			t.Fatal("join did not admit learner")
		}
		if tc.path != "join" && tc.code == 202 && (n.id != "n4" || n.operation != tc.path) {
			t.Fatal("wrong member operation")
		}
		if tc.code == 307 && w.Header().Get("Location") != "http://leader:8101/cluster/remove" {
			t.Fatal("wrong redirect")
		}
	}
}
