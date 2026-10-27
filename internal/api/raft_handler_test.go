package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
)

type apiNode struct {
	store  state.Store
	err    error
	status raft.NodeStatus
	calls  int
}

func (n *apiNode) Propose(ctx context.Context, data []byte) ([]byte, error) {
	n.calls++
	if n.err != nil {
		return nil, n.err
	}
	return n.store.Apply(data)
}
func (n *apiNode) Status() raft.NodeStatus { return n.status }
func (n *apiNode) IsLeader() bool          { return n.err == nil }
func TestConsensusHTTPCommands(t *testing.T) {
	n := &apiNode{}
	h := NewRaftHandler(n)
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/kv/missing", "", 404}, {"PUT", "/kv/a", `{"value":""}`, 200}, {"GET", "/kv/a", "", 200}, {"DELETE", "/kv/a", "", 200}, {"GET", "/kv/a", "", 404},
		{"PUT", "/kv/a", `{"value":null}`, 400}, {"PUT", "/kv/a", `{"value":"x","ttl":1}`, 400}, {"GET", "/cluster/status", "", 200},
		{"POST", "/cluster/join", `{"id":"new","grpc_address":"new:9","http_address":"new:8"}`, 501},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != tc.code {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if n.calls != 5 {
		t.Fatalf("expected all five valid KV operations through Raft; got %d", n.calls)
	}
}
func TestHTTPFailureAndRedirectSemantics(t *testing.T) {
	for _, tc := range []struct {
		err      error
		leader   string
		code     int
		location string
	}{
		{raft.ErrNotLeader, "", 503, ""}, {raft.ErrNotLeader, "http://leader:8001", 307, "http://leader:8001/kv/a%2Fb"},
		{raft.ErrNotLeader, "javascript:bad", 503, ""}, {context.DeadlineExceeded, "", 504, ""}, {raft.ErrStopped, "", 503, ""}, {raft.ErrProposalTooLarge, "", 413, ""}, {errors.New("disk details"), "", 503, ""},
	} {
		n := &apiNode{err: tc.err, status: raft.NodeStatus{LeaderHTTPAddress: tc.leader}}
		w := httptest.NewRecorder()
		NewRaftHandler(n).ServeHTTP(w, httptest.NewRequest("PUT", "/kv/a%2Fb", strings.NewReader(`{"value":"x"}`)))
		if w.Code != tc.code || w.Header().Get("Location") != tc.location {
			t.Fatalf("%v: %d %s", tc.err, w.Code, w.Header())
		}
	}
}
