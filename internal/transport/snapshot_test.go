package transport_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/transport"
	"github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"
	"google.golang.org/grpc"
)

// Force one lost command replication to the only available follower. The
// leader must retry via its clock even while the caller waits for Propose.
type dropFirstCommand struct {
	raft.Transport
	dropped atomic.Bool
}

func (d *dropFirstCommand) SendAppendEntries(ctx context.Context, peer raft.Peer, r raft.AppendEntriesRequest) (raft.AppendEntriesResponse, error) {
	if peer.ID == "1" {
		for _, e := range r.Entries {
			if e.Kind == storage.EntryCommand && d.dropped.CompareAndSwap(false, true) {
				return raft.AppendEntriesResponse{}, context.DeadlineExceeded
			}
		}
	}
	return d.Transport.SendAppendEntries(ctx, peer, r)
}

func TestSnapshotCatchupOverTCPWithWAL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	peers := make([]raft.Peer, 3)
	listeners := make([]net.Listener, 3)
	for i := range peers {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		listeners[i] = l
		peers[i] = raft.Peer{ID: fmt.Sprint(i), GRPCAddress: l.Addr().String()}
	}
	nodes := make([]*raft.Node, 3)
	stores := make([]*state.Store, 3)
	servers := make([]*grpc.Server, 3)
	disks := make([]*storage.WAL, 3)
	var leaderTransport *dropFirstCommand
	for i := range nodes {
		d, err := storage.OpenWAL(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		disks[i] = d
		client := &transport.Client{}
		defer client.Close()
		stores[i] = state.NewStore()
		var wire raft.Transport = client
		if i == 0 {
			leaderTransport = &dropFirstCommand{Transport: client}
			wire = leaderTransport
		}
		n, err := raft.NewNode(raft.Config{ID: peers[i].ID, Peers: peers, Storage: d, StateMachine: stores[i], Transport: wire, RPCTimeout: 100 * time.Millisecond, TickInterval: 10 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		nodes[i] = n
		defer n.Stop()
		servers[i] = grpc.NewServer()
		defer servers[i].Stop()
		raftkvpb.RegisterRaftServiceServer(servers[i], &transport.Server{Handler: n})
		if i < 2 {
			go servers[i].Serve(listeners[i])
		}
	}
	if err := nodes[0].Campaign(ctx); err != nil {
		t.Fatal(err)
	}
	// A real node keeps heartbeating during proposals. Drive the leader's
	// production clock throughout the test, including the initial large writes.
	// Followers use RPCs only here so elections do not obscure snapshot catch-up.
	runCtx, stopClock := context.WithCancel(ctx)
	clockDone := make(chan struct{})
	go func() { defer close(clockDone); _ = nodes[0].Run(runCtx) }()
	defer func() { stopClock(); <-clockDone }()
	// Total snapshot exceeds the normal 4 MiB unary receive limit.
	for i := 0; i < 6; i++ {
		command := []byte(fmt.Sprintf(`{"op":"PUT","key":"large%d","value":"%s"}`, i, strings.Repeat("x", 800000)))
		if _, err := nodes[0].Propose(ctx, command); err != nil {
			t.Fatal(err)
		}
	}
	if err := nodes[0].TakeSnapshot(); err != nil {
		t.Fatal(err)
	}
	go servers[2].Serve(listeners[2])
	if _, err := nodes[0].Propose(ctx, []byte(`{"op":"PUT","key":"suffix","value":"present"}`)); err != nil {
		t.Fatal(err)
	}
	want, _ := stores[0].Snapshot()
	for {
		got, _ := stores[2].Snapshot()
		if bytes.Equal(got, want) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("snapshot catchup timeout")
		case <-time.After(10 * time.Millisecond):
		}
	}
	r, err := disks[2].Recover()
	if err != nil || r.Snapshot == nil || len(r.Snapshot.Data) < 4<<20 {
		t.Fatal("no large durable snapshot", err)
	}
	if !leaderTransport.dropped.Load() {
		t.Fatal("lost-replication regression was not exercised")
	}
}
