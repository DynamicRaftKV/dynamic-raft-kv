package transport

import (
	"context"
	"errors"
	"sync"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client owns reusable connections keyed by peer address. The zero value is
// ready to use with plaintext gRPC for the project's trusted-network deployment.
// RPC deadlines are supplied by the caller; Close cancels outstanding RPCs.
// Options must be set before use, typically to inject a dialer in tests.
type Client struct {
	Options     []grpc.DialOption
	mu          sync.Mutex
	connections map[string]*grpc.ClientConn
	closed      bool
}

var _ raft.Transport = (*Client)(nil)

func (c *Client) peer(ctx context.Context, p raft.Peer) (raftkvpb.RaftServiceClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.GRPCAddress == "" {
		return nil, errors.New("transport: peer gRPC address required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, errors.New("transport: client closed")
	}
	if c.connections == nil {
		c.connections = make(map[string]*grpc.ClientConn)
	}
	conn := c.connections[p.GRPCAddress]
	if conn == nil {
		options := append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, c.Options...)
		var err error
		conn, err = grpc.NewClient("passthrough:///"+p.GRPCAddress, options...)
		if err != nil {
			return nil, err
		}
		c.connections[p.GRPCAddress] = conn
	}
	return raftkvpb.NewRaftServiceClient(conn), nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	var errs []error
	for address, conn := range c.connections {
		errs = append(errs, conn.Close())
		delete(c.connections, address)
	}
	return errors.Join(errs...)
}

func (c *Client) SendAppendEntries(ctx context.Context, p raft.Peer, r raft.AppendEntriesRequest) (raft.AppendEntriesResponse, error) {
	client, err := c.peer(ctx, p)
	if err != nil {
		return raft.AppendEntriesResponse{}, err
	}
	entries := make([]*raftkvpb.Entry, len(r.Entries))
	for i, e := range r.Entries {
		entries[i] = &raftkvpb.Entry{Index: e.Index, Term: e.Term, Kind: raftkvpb.EntryKind(e.Kind), Data: e.Data}
	}
	reply, err := client.AppendEntries(ctx, &raftkvpb.AppendEntriesRequest{Term: r.Term, LeaderId: r.LeaderID, PrevLogIndex: r.PrevLogIndex, PrevLogTerm: r.PrevLogTerm, Entries: entries, LeaderCommit: r.LeaderCommit})
	if err != nil {
		return raft.AppendEntriesResponse{}, err
	}
	return raft.AppendEntriesResponse{Term: reply.Term, Success: reply.Success}, nil
}
func (c *Client) SendRequestVote(ctx context.Context, p raft.Peer, r raft.RequestVoteRequest) (raft.RequestVoteResponse, error) {
	client, err := c.peer(ctx, p)
	if err != nil {
		return raft.RequestVoteResponse{}, err
	}
	reply, err := client.RequestVote(ctx, &raftkvpb.RequestVoteRequest{Term: r.Term, CandidateId: r.CandidateID, LastLogIndex: r.LastLogIndex, LastLogTerm: r.LastLogTerm})
	if err != nil {
		return raft.RequestVoteResponse{}, err
	}
	return raft.RequestVoteResponse{Term: reply.Term, VoteGranted: reply.VoteGranted}, nil
}
func (c *Client) SendInstallSnapshot(ctx context.Context, p raft.Peer, r raft.InstallSnapshotRequest) (raft.InstallSnapshotResponse, error) {
	client, err := c.peer(ctx, p)
	if err != nil {
		return raft.InstallSnapshotResponse{}, err
	}
	reply, err := client.InstallSnapshot(ctx, &raftkvpb.InstallSnapshotRequest{Term: r.Term, LeaderId: r.LeaderID, LastIncludedIndex: r.Snapshot.LastIncludedIndex, LastIncludedTerm: r.Snapshot.LastIncludedTerm, Configuration: r.Snapshot.Configuration, Data: r.Snapshot.Data, Offset: r.Offset, Done: r.Done})
	if err != nil {
		return raft.InstallSnapshotResponse{}, err
	}
	return raft.InstallSnapshotResponse{Term: reply.Term, Accepted: reply.Accepted, NextOffset: reply.NextOffset}, nil
}
func (c *Client) JoinCluster(ctx context.Context, p raft.Peer, r raft.JoinClusterRequest) (raft.JoinClusterResponse, error) {
	client, err := c.peer(ctx, p)
	if err != nil {
		return raft.JoinClusterResponse{}, err
	}
	reply, err := client.JoinCluster(ctx, &raftkvpb.JoinClusterRequest{Peer: &raftkvpb.Peer{Id: r.Peer.ID, GrpcAddress: r.Peer.GRPCAddress, HttpAddress: r.Peer.HTTPAddress, Learner: r.Peer.Learner}})
	if err != nil {
		return raft.JoinClusterResponse{}, err
	}
	return raft.JoinClusterResponse{Accepted: reply.Accepted, LeaderHTTPAddress: reply.LeaderHttpAddress}, nil
}
