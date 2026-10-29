package transport_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/api"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/state"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/transport"
	"github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"
	"google.golang.org/grpc"
)

// Test storage only: production crash durability is Stage 5. Calls are serialized
// by the node. Recovery is used once for these fresh network-cluster fixtures.
type testDisk struct {
	hard    storage.HardState
	entries []storage.Entry
}

func (d *testDisk) Recover() (storage.Recovered, error) {
	return storage.Recovered{HardState: d.hard, Entries: d.entries}, nil
}
func (d *testDisk) SaveHardState(h storage.HardState) error { d.hard = h; return nil }
func (d *testDisk) Append(es []storage.Entry) error {
	if len(es) > 0 {
		for _, e := range es {
			e.Data = bytes.Clone(e.Data)
			if e.Index <= uint64(len(d.entries)) {
				d.entries = d.entries[:e.Index-1]
			}
			d.entries = append(d.entries, e)
		}
	}
	return nil
}
func (*testDisk) InstallSnapshot(storage.Snapshot, bool) error { return raft.ErrUnsupported }
func (*testDisk) Compact(uint64) error                         { return raft.ErrUnsupported }

func TestTCPCluster(t *testing.T) {
	for _, count := range []int{1, 3, 5, 7} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			peers := make([]raft.Peer, count)
			listeners := make([]net.Listener, count)
			httpListeners := make([]net.Listener, count)
			for i := range peers {
				var err error
				listeners[i], err = net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listeners[i].Close()
				httpListeners[i], err = net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer httpListeners[i].Close()
				peers[i] = raft.Peer{ID: fmt.Sprint(i), GRPCAddress: listeners[i].Addr().String(), HTTPAddress: "http://" + httpListeners[i].Addr().String()}
			}
			nodes := make([]*raft.Node, count)
			stores := make([]*state.Store, count)
			servers := make([]*grpc.Server, count)
			for i := range nodes {
				client := &transport.Client{}
				defer client.Close()
				stores[i] = state.NewStore()
				var err error
				nodes[i], err = raft.NewNode(raft.Config{ID: peers[i].ID, Peers: peers, Transport: client, Storage: &testDisk{}, StateMachine: stores[i], Seed: int64(i + 1), RPCTimeout: 200 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				defer nodes[i].Stop()
				servers[i] = grpc.NewServer()
				raftkvpb.RegisterRaftServiceServer(servers[i], &transport.Server{Handler: nodes[i]})
				go servers[i].Serve(listeners[i])
				defer servers[i].Stop()
				hs := httptest.NewUnstartedServer(api.NewRaftHandler(nodes[i]))
				hs.Listener.Close()
				hs.Listener = httpListeners[i]
				hs.Start()
				defer hs.Close()
			}
			if err := nodes[0].Campaign(ctx); err != nil {
				t.Fatal(err)
			}
			if !nodes[0].IsLeader() {
				t.Fatal("no leader over TCP")
			}
			// Notify every follower of the elected leader before using follower routing.
			if err := nodes[0].Tick(ctx); err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Timeout: 5 * time.Second}
			request, err := http.NewRequest(http.MethodPut, peers[count-1].HTTPAddress+"/kv/network", bytes.NewBufferString(`{"value":"works"}`))
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatalf("put: %s", response.Status)
			}
			response, err = client.Get(peers[count-1].HTTPAddress + "/kv/network")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatalf("get: %s", response.Status)
			}
			for j := 0; j < 3; j++ {
				if err := nodes[0].Tick(ctx); err != nil {
					t.Fatal(err)
				}
			}
			expected, _ := stores[0].Snapshot()
			for i, store := range stores {
				got, _ := store.Snapshot()
				if !bytes.Equal(got, expected) {
					t.Fatalf("node %d: %s != %s", i, got, expected)
				}
			}
			if count == 3 {
				// A disconnected follower must catch up when its missing suffix
				// exceeds the gRPC receive limit; requests must be bounded batches.
				servers[2].Stop()
				for j := 0; j < 9; j++ {
					command, _ := state.EncodeCommand(state.Command{Op: state.OpPut, Key: fmt.Sprintf("large-%d", j), Value: strings.Repeat("x", 600000)})
					if _, err := nodes[0].Propose(ctx, command); err != nil {
						t.Fatal(err)
					}
				}
				listener, err := net.Listen("tcp", peers[2].GRPCAddress)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				restarted := grpc.NewServer()
				raftkvpb.RegisterRaftServiceServer(restarted, &transport.Server{Handler: nodes[2]})
				go restarted.Serve(listener)
				defer restarted.Stop()
				for nodes[2].Status().LastApplied < nodes[0].Status().CommitIndex {
					if err := nodes[0].Tick(ctx); err != nil {
						t.Fatal(err)
					}
				}
				want, _ := stores[0].Snapshot()
				got, _ := stores[2].Snapshot()
				if !bytes.Equal(want, got) {
					t.Fatal("large suffix catch-up diverged")
				}
			}
			if count > 1 {
				nodes[0].Stop()
				servers[0].Stop()
				if err := nodes[1].Campaign(ctx); err != nil {
					t.Fatal(err)
				}
				if !nodes[1].IsLeader() {
					t.Fatal("no failover after leader stop")
				}
				if _, err := nodes[1].Propose(ctx, []byte(`{"op":"PUT","key":"after-failover","value":"ok"}`)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
