package raft

import "context"

// Transport is owned by its consumer and has no gRPC dependency. Methods are
// safe for concurrent callers. Context bounds waiting; it cannot undo remote
// work. Protocol rejection is a response, network failure is an error.
// Requests are immutable until the call returns; responses are caller-owned.
type Transport interface {
	SendAppendEntries(context.Context, Peer, AppendEntriesRequest) (AppendEntriesResponse, error)
	SendRequestVote(context.Context, Peer, RequestVoteRequest) (RequestVoteResponse, error)
	SendInstallSnapshot(context.Context, Peer, InstallSnapshotRequest) (InstallSnapshotResponse, error)
	JoinCluster(context.Context, Peer, JoinClusterRequest) (JoinClusterResponse, error)
}

// RPCHandler documents the inbound adapter boundary. Membership dispatch and
// snapshot chunk ownership remain draft until the SDD is available.
type RPCHandler interface {
	AppendEntries(context.Context, AppendEntriesRequest) (AppendEntriesResponse, error)
	RequestVote(context.Context, RequestVoteRequest) (RequestVoteResponse, error)
	InstallSnapshot(context.Context, InstallSnapshotRequest) (InstallSnapshotResponse, error)
	JoinCluster(context.Context, JoinClusterRequest) (JoinClusterResponse, error)
}
