package transport_test

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/transport"
	"github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type recordingHandler struct {
	mu       sync.Mutex
	append   raft.AppendEntriesRequest
	vote     raft.RequestVoteRequest
	snapshot raft.InstallSnapshotRequest
	join     raft.JoinClusterRequest
	err      error
	wait     bool
}

func (h *recordingHandler) AppendEntries(ctx context.Context, r raft.AppendEntriesRequest) (raft.AppendEntriesResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.append = r
	return raft.AppendEntriesResponse{Term: 9, Success: false}, h.err
}
func (h *recordingHandler) RequestVote(ctx context.Context, r raft.RequestVoteRequest) (raft.RequestVoteResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vote = r
	if h.wait {
		<-ctx.Done()
		return raft.RequestVoteResponse{}, ctx.Err()
	}
	return raft.RequestVoteResponse{Term: 9, VoteGranted: true}, h.err
}
func (h *recordingHandler) InstallSnapshot(ctx context.Context, r raft.InstallSnapshotRequest) (raft.InstallSnapshotResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snapshot = r
	return raft.InstallSnapshotResponse{Term: 9, Accepted: true, NextOffset: 88}, h.err
}
func (h *recordingHandler) JoinCluster(ctx context.Context, r raft.JoinClusterRequest) (raft.JoinClusterResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.join = r
	return raft.JoinClusterResponse{Accepted: true, LeaderHTTPAddress: "http://leader:8001"}, h.err
}
func adapter(t *testing.T, h raft.RPCHandler) *transport.Client {
	t.Helper()
	l := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	raftkvpb.RegisterRaftServiceServer(s, &transport.Server{Handler: h})
	go s.Serve(l)
	c := &transport.Client{Options: []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return l.DialContext(ctx) })}}
	t.Cleanup(func() { c.Close(); s.Stop(); l.Close() })
	return c
}
func TestAdaptersPreserveFieldsAndRejections(t *testing.T) {
	h := &recordingHandler{}
	c := adapter(t, h)
	p := raft.Peer{ID: "remote", GRPCAddress: "memory"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a := raft.AppendEntriesRequest{Term: 8, LeaderID: "a", PrevLogIndex: 2, PrevLogTerm: 7, LeaderCommit: 2, Entries: []storage.Entry{{Index: 3, Term: 8, Kind: storage.EntryCommand, Data: []byte{0, 255, 1}}}}
	ar, err := c.SendAppendEntries(ctx, p, a)
	if err != nil || ar.Term != 9 || ar.Success {
		t.Fatal(ar, err)
	}
	v := raft.RequestVoteRequest{Term: 8, CandidateID: "a", LastLogIndex: 3, LastLogTerm: 7}
	vr, err := c.SendRequestVote(ctx, p, v)
	if err != nil || !vr.VoteGranted || vr.Term != 9 {
		t.Fatal(vr, err)
	}
	s := raft.InstallSnapshotRequest{Term: 8, LeaderID: "a", Snapshot: storage.Snapshot{LastIncludedIndex: 4, LastIncludedTerm: 7, Configuration: []byte("config"), Data: []byte("chunk")}, Offset: 83, Done: true}
	sr, err := c.SendInstallSnapshot(ctx, p, s)
	if err != nil || sr.NextOffset != 88 || !sr.Accepted {
		t.Fatal(sr, err)
	}
	j := raft.JoinClusterRequest{Peer: raft.Peer{ID: "new", GRPCAddress: "new:9", HTTPAddress: "http://new:8", Learner: true}}
	jr, err := c.JoinCluster(ctx, p, j)
	if err != nil || !jr.Accepted || jr.LeaderHTTPAddress != "http://leader:8001" {
		t.Fatal(jr, err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !reflect.DeepEqual(a, h.append) || !reflect.DeepEqual(v, h.vote) || !reflect.DeepEqual(s, h.snapshot) || !reflect.DeepEqual(j, h.join) {
		t.Fatalf("wire mapping lost fields: %+v", h)
	}
}
func TestAdapterCancellationAndClose(t *testing.T) {
	h := &recordingHandler{wait: true}
	c := adapter(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := c.SendRequestVote(ctx, raft.Peer{GRPCAddress: "memory"}, raft.RequestVoteRequest{})
	if status.Code(err) != codes.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = c.SendAppendEntries(context.Background(), raft.Peer{GRPCAddress: "memory"}, raft.AppendEntriesRequest{})
	if err == nil {
		t.Fatal("closed client reused")
	}
}
func TestServerRejectsUnknownEnumsAndUnsupportedOperations(t *testing.T) {
	h := &recordingHandler{err: raft.ErrUnsupported}
	s := &transport.Server{Handler: h}
	for _, kind := range []raftkvpb.EntryKind{-1, 256, 999} {
		_, err := s.AppendEntries(context.Background(), &raftkvpb.AppendEntriesRequest{Entries: []*raftkvpb.Entry{{Kind: kind}}})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatal(err)
		}
	}
	_, err := s.InstallSnapshot(context.Background(), &raftkvpb.InstallSnapshotRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatal(err)
	}
	_, err = s.JoinCluster(context.Background(), &raftkvpb.JoinClusterRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
}
