package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

func TestBootstrapConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")
	peers := []raft.Peer{{ID: "b", GRPCAddress: "127.0.0.1:9102", HTTPAddress: "http://127.0.0.1:8102"}, {ID: "a", GRPCAddress: "127.0.0.1:9101", HTTPAddress: "http://127.0.0.1:8101"}}
	data, _ := json.Marshal(clusterFile{Peers: peers})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readPeers(path, "a")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "a" {
		t.Fatal("not canonicalized")
	}
	if err := bindBootstrap(dir, "a", got); err != nil {
		t.Fatal(err)
	}
	if err := bindBootstrap(dir, "a", got); err != nil {
		t.Fatal(err)
	}
	if err := bindBootstrap(dir, "b", got); err == nil {
		t.Fatal("allowed identity change")
	}
	got[0].Learner = true
	if err := bindBootstrap(dir, "a", got); err == nil {
		t.Fatal("allowed voter change")
	}
	if _, err := readPeers(path, "missing"); err == nil {
		t.Fatal("accepted absent self")
	}
	if _, err := parseConfig([]string{"--demo", "--cluster-config", path}); err == nil {
		t.Fatal("mixed standalone and cluster mode")
	}
}
