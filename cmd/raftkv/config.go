package main

import (
	"errors"
	"flag"
	"os"
)

type config struct {
	id, peers, dataDir, httpAddr, grpcAddr string
	clusterConfig                          string
	demo                                   bool
	joining                                bool
	demoClusterConfig                      string
	snapshotEntries                        uint64
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseConfig(args []string) (config, error) {
	var c config
	f := flag.NewFlagSet("raftkv", flag.ContinueOnError)
	f.BoolVar(&c.demo, "demo", false, "enable standalone KV presentation UI at /demo/ (no Raft)")
	f.StringVar(&c.demoClusterConfig, "demo-cluster-config", "", "allowlisted cluster endpoints for the local demo console")
	f.BoolVar(&c.joining, "join", false, "join as a learner using a seed configuration that excludes this node")
	f.Uint64Var(&c.snapshotEntries, "snapshot-entries", 1000, "checkpoint after this many applied entries (0 disables periodic snapshots; membership checkpoints remain)")
	f.StringVar(&c.clusterConfig, "cluster-config", env("RAFTKV_CLUSTER_CONFIG", ""), "JSON peer configuration; enables durable Raft mode")
	f.StringVar(&c.id, "node-id", env("RAFTKV_NODE_ID", "node1"), "unique node identity")
	f.StringVar(&c.peers, "peers", env("RAFTKV_PEERS", ""), "bootstrap peers (reserved; not consumed in Stage 1)")
	f.StringVar(&c.dataDir, "data-dir", env("RAFTKV_DATA_DIR", "./data"), "exclusive node data directory in durable Raft mode")
	f.StringVar(&c.httpAddr, "http-addr", env("RAFTKV_HTTP_ADDR", ":8001"), "HTTP listen address")
	f.StringVar(&c.grpcAddr, "grpc-addr", env("RAFTKV_GRPC_ADDR", ":9001"), "gRPC listen address")
	if err := f.Parse(args); err != nil {
		return c, err
	}
	if f.NArg() != 0 {
		return c, errors.New("unexpected positional arguments")
	}
	if c.id == "" || c.dataDir == "" || c.httpAddr == "" || c.grpcAddr == "" {
		return c, errors.New("node-id, data-dir and listen addresses must be nonempty")
	}
	if c.demo && c.clusterConfig != "" {
		return c, errors.New("--demo is standalone; do not combine it with --cluster-config")
	}
	if c.joining && c.clusterConfig == "" {
		return c, errors.New("--join requires --cluster-config")
	}
	if c.demoClusterConfig != "" && !c.demo {
		return c, errors.New("--demo-cluster-config requires --demo")
	}
	return c, nil
}
