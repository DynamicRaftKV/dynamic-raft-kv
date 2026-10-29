package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/api"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/demo"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/transport"
	"github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"
	"google.golang.org/grpc"
)

func main() {
	c, err := parseConfig(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, c); err != nil {
		log.Fatal(err)
	}
}

// serve runs either the standalone/scaffold mode or an explicitly configured
// durable Raft node, with bounded shutdown.
func serve(ctx context.Context, c config) error {
	httpListener, err := net.Listen("tcp", c.httpAddr)
	if err != nil {
		return err
	}
	defer httpListener.Close()
	grpcListener, err := net.Listen("tcp", c.grpcAddr)
	if err != nil {
		return err
	}
	defer grpcListener.Close()
	handler := api.NewHandler()
	var node *raft.Node
	adapter := &transport.Server{}
	if c.clusterConfig != "" {
		peers, err := readSeedPeers(c.clusterConfig, c.id, c.joining)
		if err != nil {
			return err
		}
		disk, err := storage.OpenWAL(c.dataDir)
		if err != nil {
			return err
		}
		defer disk.Close()
		recovered, err := disk.Recover()
		if err != nil {
			return err
		}
		// Never assign a missing bootstrap identity to an already populated WAL.
		if _, err := os.Stat(c.dataDir + "/bootstrap.json"); errors.Is(err, os.ErrNotExist) && (recovered.HardState.Term != 0 || len(recovered.Entries) > 0) {
			return errors.New("nonempty WAL has no bootstrap identity")
		}
		if err := bindBootstrap(c.dataDir, c.id, peers); err != nil {
			return err
		}
		client := &transport.Client{}
		defer client.Close()
		node, err = raft.NewNode(raft.Config{ID: c.id, Peers: peers, Storage: disk, StateMachine: state.NewStore(), Transport: client, SnapshotEntries: c.snapshotEntries, Joining: c.joining})
		if err != nil {
			return err
		}
		defer node.Stop()
		adapter.Handler = node
		handler = api.NewRaftHandler(node)
	}
	if c.demo {
		mux := http.NewServeMux()
		mux.Handle("/demo/", demo.NewHandler(state.NewStore()))
		mux.Handle("/", handler)
		handler = mux
		log.Printf("Standalone KV demo enabled: http://%s/demo/ (memory only, no Raft)", c.httpAddr)
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	grpcServer := grpc.NewServer()
	raftkvpb.RegisterRaftServiceServer(grpcServer, adapter)
	errs := make(chan error, 3)
	go func() { errs <- httpServer.Serve(httpListener) }()
	go func() { errs <- grpcServer.Serve(grpcListener) }()
	if node != nil {
		go func() { errs <- node.Run(ctx) }()
		log.Printf("Durable Raft node=%s http=%s grpc=%s data=%s", c.id, httpListener.Addr(), grpcListener.Addr(), c.dataDir)
	} else {
		log.Printf("Scaffold node=%s http=%s grpc=%s; use --cluster-config for durable Raft", c.id, httpListener.Addr(), grpcListener.Addr())
	}
	select {
	case <-ctx.Done():
	case err = <-errs:
	}
	if node != nil {
		node.Stop()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	grpcDone := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcDone) }()
	if shutdownErr := httpServer.Shutdown(shutdownCtx); shutdownErr != nil {
		_ = httpServer.Close()
	}
	select {
	case <-grpcDone:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, grpc.ErrServerStopped) || (ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, raft.ErrStopped))) {
		return nil
	}
	return err
}
