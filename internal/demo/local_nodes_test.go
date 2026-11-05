package demo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

func TestLocalNodeChild(t *testing.T) {
	if os.Getenv("RAFTKV_SUPERVISOR_CHILD") != "1" {
		return
	}
	var id, address string
	for i, a := range os.Args {
		if i+1 < len(os.Args) {
			if a == "--node-id" {
				id = os.Args[i+1]
			}
			if a == "--http-addr" {
				address = os.Args[i+1]
			}
		}
	}
	_ = http.ListenAndServe(address, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(raft.NodeStatus{ID: id}) }))
}

func TestLocalNodeOwnershipPortsAndReopen(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed.json")
	if err := os.WriteFile(seed, []byte(`{"peers":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	wrapper := filepath.Join(root, "child.sh")
	if err = os.WriteFile(wrapper, []byte("#!/bin/sh\nRAFTKV_SUPERVISOR_CHILD=1 exec "+quote(exe)+" -test.run=^TestLocalNodeChild$ -- \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "managed")
	m, err := OpenLocalNodes(wrapper, seed, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if duplicate, e := OpenLocalNodes(wrapper, seed, dir); e == nil {
		duplicate.Close()
		t.Fatal("two supervisors acquired directory")
	}
	a, err := m.Create()
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Create()
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID || a.HTTPAddress == b.HTTPAddress || a.GRPCAddress == b.GRPCAddress || strings.TrimPrefix(a.HTTPAddress, "http://") == a.GRPCAddress {
		t.Fatal("duplicate identity or ports")
	}
	if err = m.Action("external", "crash"); err == nil {
		t.Fatal("unowned process control accepted")
	}
	if err = m.Action(a.ID, "crash"); err != nil {
		t.Fatal(err)
	}
	if err = m.Action(a.ID, "restart"); err != nil {
		t.Fatal(err)
	}
	if err = m.Action(b.ID, "crash"); err != nil {
		t.Fatal(err)
	}
	m.Close()
	reopened, err := OpenLocalNodes(wrapper, seed, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	peers := reopened.Peers()
	if len(peers) != 2 || peers[0] != a || peers[1] != b {
		t.Fatal("identity/ports changed on reopen")
	}
	if reopened.children[a.ID] == nil || reopened.children[b.ID] != nil {
		t.Fatal("running/stopped intent not recovered")
	}
}

func TestLocalProcessRequestBoundary(t *testing.T) {
	m := &LocalNodes{}
	mux := http.NewServeMux()
	m.routes(mux)
	for _, tc := range []struct {
		host, body string
		want       int
	}{
		{"evil.example:8080", `{"action":"create"}`, 403},
		{"127.0.0.1:8080", `{"action":"create"} {}`, 400},
		{"127.0.0.1:8080", `{"action":"create","command":"anything"}`, 400},
		{"127.0.0.1:8080", `{"action":"crash","id":"external"}`, 409},
	} {
		r := httptest.NewRequest("POST", "/demo/api/cluster/process", strings.NewReader(tc.body))
		r.Host = tc.host
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s: got %d want %d", tc.body, w.Code, tc.want)
		}
	}
}
