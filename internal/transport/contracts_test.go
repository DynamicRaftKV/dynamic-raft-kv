package transport_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/transport"
	"github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// Verify generated service registration and all four stubs over in-memory gRPC.
func TestServerReturnsUnimplemented(t *testing.T) {
	l := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	raftkvpb.RegisterRaftServiceServer(s, &transport.Server{})
	go func() { _ = s.Serve(l) }()
	t.Cleanup(func() { s.Stop(); _ = l.Close() })
	conn, err := grpc.NewClient("passthrough:///stub", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return l.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	c := raftkvpb.NewRaftServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, ae := c.AppendEntries(ctx, &raftkvpb.AppendEntriesRequest{})
	_, rv := c.RequestVote(ctx, &raftkvpb.RequestVoteRequest{})
	_, ss := c.InstallSnapshot(ctx, &raftkvpb.InstallSnapshotRequest{})
	_, jc := c.JoinCluster(ctx, &raftkvpb.JoinClusterRequest{})
	for name, err := range map[string]error{"append": ae, "vote": rv, "snapshot": ss, "join": jc} {
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("%s: %v", name, err)
		}
	}
}
