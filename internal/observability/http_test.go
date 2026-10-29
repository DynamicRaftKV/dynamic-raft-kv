package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
)

type unusedTransport struct{ raft.Transport }

func TestDiagnosticsAndHealth(t *testing.T) {
	dir := t.TempDir()
	disk, err := storage.OpenWAL(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	n, err := raft.NewNode(raft.Config{ID: "one", Peers: []raft.Peer{{ID: "one"}}, Storage: disk, StateMachine: state.NewStore(), Transport: unusedTransport{}})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Stop()
	if err := n.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := Handler(n, dir, http.NotFoundHandler())
	for _, tc := range []struct{ path, want string }{{"/metrics", "raftkv_is_leader 1"}, {"/metrics", "raftkv_wal_bytes "}, {"/metrics", "raftkv_leaderships_total 1"}, {"/debug/raft", `"leader":true`}, {"/healthz", "ok"}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatal(tc.path, w.Code, w.Body.String())
		}
	}
	n.Stop()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 503 {
		t.Fatal("stopped node healthy")
	}
}
