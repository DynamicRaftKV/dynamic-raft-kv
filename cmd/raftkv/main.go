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
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
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

// serve runs only listeners and stub handlers, with bounded shutdown.
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
	if c.demo {
		mux := http.NewServeMux()
		mux.Handle("/demo/", demo.NewHandler(state.NewStore()))
		mux.Handle("/", handler)
		handler = mux
		log.Printf("Standalone KV demo enabled: http://%s/demo/ (memory only, no Raft)", c.httpAddr)
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	grpcServer := grpc.NewServer()
	raftkvpb.RegisterRaftServiceServer(grpcServer, &transport.Server{})
	errs := make(chan error, 2)
	go func() { errs <- httpServer.Serve(httpListener) }()
	go func() { errs <- grpcServer.Serve(grpcListener) }()
	log.Printf("Stage 1 skeleton node=%s http=%s grpc=%s; peers/data-dir reserved; no consensus or persistence", c.id, httpListener.Addr(), grpcListener.Addr())
	select {
	case <-ctx.Done():
	case err = <-errs:
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
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}
