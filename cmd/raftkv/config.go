package main

import (
	"errors"
	"flag"
	"os"
)

type config struct {
	id, peers, dataDir, httpAddr, grpcAddr string
	demo                                   bool
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
	f.StringVar(&c.id, "node-id", env("RAFTKV_NODE_ID", "node1"), "unique node identity")
	f.StringVar(&c.peers, "peers", env("RAFTKV_PEERS", ""), "bootstrap peers (reserved; not consumed in Stage 1)")
	f.StringVar(&c.dataDir, "data-dir", env("RAFTKV_DATA_DIR", "./data"), "data directory (reserved; no file I/O)")
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
	return c, nil
}
