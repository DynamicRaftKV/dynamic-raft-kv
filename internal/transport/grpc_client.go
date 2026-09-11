package transport

import (
	"context"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client is the Stage 1 adapter placeholder. It does not dial or send RPCs.
type Client struct{}

var _ raft.Transport = (*Client)(nil)

func (*Client) SendAppendEntries(context.Context, raft.Peer, raft.AppendEntriesRequest) (raft.AppendEntriesResponse, error) {
	return raft.AppendEntriesResponse{}, status.Error(codes.Unimplemented, "TODO: Stage 4")
}
func (*Client) SendRequestVote(context.Context, raft.Peer, raft.RequestVoteRequest) (raft.RequestVoteResponse, error) {
	return raft.RequestVoteResponse{}, status.Error(codes.Unimplemented, "TODO: Stage 4")
}
func (*Client) SendInstallSnapshot(context.Context, raft.Peer, raft.InstallSnapshotRequest) (raft.InstallSnapshotResponse, error) {
	return raft.InstallSnapshotResponse{}, status.Error(codes.Unimplemented, "TODO: Stage 4")
}
func (*Client) JoinCluster(context.Context, raft.Peer, raft.JoinClusterRequest) (raft.JoinClusterResponse, error) {
	return raft.JoinClusterResponse{}, status.Error(codes.Unimplemented, "TODO: Stage 6")
}
