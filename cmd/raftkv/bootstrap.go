package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

type clusterFile struct {
	Peers []raft.Peer `json:"peers"`
}
type bootstrapIdentity struct {
	Version int         `json:"version"`
	ID      string      `json:"id"`
	Peers   []raft.Peer `json:"peers"`
}

func readPeers(path, id string) ([]raft.Peer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var config clusterFile
	if err := dec.Decode(&config); err != nil {
		return nil, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing cluster configuration")
	}
	seen := map[string]bool{}
	addresses := map[string]bool{}
	voters := 0
	for _, p := range config.Peers {
		if p.ID == "" || seen[p.ID] {
			return nil, errors.New("peer IDs must be nonempty and unique")
		}
		seen[p.ID] = true
		if addresses[p.GRPCAddress] {
			return nil, errors.New("peer gRPC addresses must be unique")
		}
		addresses[p.GRPCAddress] = true
		host, port, err := net.SplitHostPort(p.GRPCAddress)
		if err != nil || host == "" || port == "" || port == "0" {
			return nil, fmt.Errorf("invalid gRPC address for %s", p.ID)
		}
		u, err := url.Parse(p.HTTPAddress)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return nil, fmt.Errorf("invalid HTTP base URL for %s", p.ID)
		}
		if !p.Learner {
			voters++
		}
	}
	if !seen[id] || voters == 0 {
		return nil, errors.New("cluster must include this node and at least one voter")
	}
	sort.Slice(config.Peers, func(i, j int) bool { return config.Peers[i].ID < config.Peers[j].ID })
	return config.Peers, nil
}

// Called while holding the WAL directory lock, before constructing the node.
// Fixed-membership restarts may not silently change voter identity/configuration.
func bindBootstrap(dir, id string, peers []raft.Peer) error {
	data, err := json.Marshal(bootstrapIdentity{Version: 1, ID: id, Peers: peers})
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "bootstrap.json")
	existing, err := os.ReadFile(path)
	if err == nil {
		if !bytes.Equal(existing, data) {
			return errors.New("bootstrap identity/configuration differs from persisted configuration; dynamic changes are not implemented")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	parent, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}
