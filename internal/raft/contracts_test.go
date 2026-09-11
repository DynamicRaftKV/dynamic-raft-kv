package raft_test

import (
	"context"
	"errors"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
)

var errStub = errors.New("test stub")

type mockNode struct{}

func (*mockNode) Propose(context.Context, []byte) ([]byte, error) { return nil, errStub }
func (*mockNode) Status() raft.NodeStatus                         { return raft.NodeStatus{} }
func (*mockNode) IsLeader() bool                                  { return false }

var _ raft.RaftNode = (*mockNode)(nil)

type mockTransport struct{}

func (*mockTransport) SendAppendEntries(context.Context, raft.Peer, raft.AppendEntriesRequest) (raft.AppendEntriesResponse, error) {
	return raft.AppendEntriesResponse{}, errStub
}
func (*mockTransport) SendRequestVote(context.Context, raft.Peer, raft.RequestVoteRequest) (raft.RequestVoteResponse, error) {
	return raft.RequestVoteResponse{}, errStub
}
func (*mockTransport) SendInstallSnapshot(context.Context, raft.Peer, raft.InstallSnapshotRequest) (raft.InstallSnapshotResponse, error) {
	return raft.InstallSnapshotResponse{}, errStub
}
func (*mockTransport) JoinCluster(context.Context, raft.Peer, raft.JoinClusterRequest) (raft.JoinClusterResponse, error) {
	return raft.JoinClusterResponse{}, errStub
}

var _ raft.Transport = (*mockTransport)(nil)

type mockHandler struct{}

func (*mockHandler) AppendEntries(context.Context, raft.AppendEntriesRequest) (raft.AppendEntriesResponse, error) {
	return raft.AppendEntriesResponse{}, errStub
}
func (*mockHandler) RequestVote(context.Context, raft.RequestVoteRequest) (raft.RequestVoteResponse, error) {
	return raft.RequestVoteResponse{}, errStub
}
func (*mockHandler) InstallSnapshot(context.Context, raft.InstallSnapshotRequest) (raft.InstallSnapshotResponse, error) {
	return raft.InstallSnapshotResponse{}, errStub
}
func (*mockHandler) JoinCluster(context.Context, raft.JoinClusterRequest) (raft.JoinClusterResponse, error) {
	return raft.JoinClusterResponse{}, errStub
}

var _ raft.RPCHandler = (*mockHandler)(nil)
