package transport

import (
	"context"
	"errors"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/raft"
	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"
	"github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server maps wire messages to the core without implementing consensus policy.
// Handler is immutable after registration. A nil handler retains scaffold mode.
type Server struct {
	raftkvpb.UnimplementedRaftServiceServer
	Handler raft.RPCHandler
}

var _ raftkvpb.RaftServiceServer = (*Server)(nil)

func rpcError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, raft.ErrUnsupported):
		return status.Error(codes.Unimplemented, err.Error())
	case errors.Is(err, raft.ErrStopped):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, "raft operation failed")
	}
}
func (s *Server) AppendEntries(ctx context.Context, r *raftkvpb.AppendEntriesRequest) (*raftkvpb.AppendEntriesResponse, error) {
	if s.Handler == nil {
		return nil, status.Error(codes.Unimplemented, "Raft node not attached")
	}
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	entries := make([]storage.Entry, len(r.Entries))
	for i, e := range r.Entries {
		if e == nil || e.Kind < raftkvpb.EntryKind_ENTRY_COMMAND || e.Kind > raftkvpb.EntryKind_ENTRY_CONFIGURATION {
			return nil, status.Error(codes.InvalidArgument, "invalid log entry kind")
		}
		entries[i] = storage.Entry{Index: e.Index, Term: e.Term, Kind: storage.EntryKind(e.Kind), Data: append([]byte(nil), e.Data...)}
	}
	reply, err := s.Handler.AppendEntries(ctx, raft.AppendEntriesRequest{Term: r.Term, LeaderID: r.LeaderId, PrevLogIndex: r.PrevLogIndex, PrevLogTerm: r.PrevLogTerm, Entries: entries, LeaderCommit: r.LeaderCommit})
	if err != nil {
		return nil, rpcError(err)
	}
	return &raftkvpb.AppendEntriesResponse{Term: reply.Term, Success: reply.Success}, nil
}
func (s *Server) RequestVote(ctx context.Context, r *raftkvpb.RequestVoteRequest) (*raftkvpb.RequestVoteResponse, error) {
	if s.Handler == nil {
		return nil, status.Error(codes.Unimplemented, "Raft node not attached")
	}
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	reply, err := s.Handler.RequestVote(ctx, raft.RequestVoteRequest{Term: r.Term, CandidateID: r.CandidateId, LastLogIndex: r.LastLogIndex, LastLogTerm: r.LastLogTerm})
	if err != nil {
		return nil, rpcError(err)
	}
	return &raftkvpb.RequestVoteResponse{Term: reply.Term, VoteGranted: reply.VoteGranted}, nil
}
func (s *Server) InstallSnapshot(ctx context.Context, r *raftkvpb.InstallSnapshotRequest) (*raftkvpb.InstallSnapshotResponse, error) {
	if s.Handler == nil {
		return nil, status.Error(codes.Unimplemented, "Raft node not attached")
	}
	if r == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	reply, err := s.Handler.InstallSnapshot(ctx, raft.InstallSnapshotRequest{Term: r.Term, LeaderID: r.LeaderId, Snapshot: storage.Snapshot{LastIncludedIndex: r.LastIncludedIndex, LastIncludedTerm: r.LastIncludedTerm, Configuration: append([]byte(nil), r.Configuration...), Data: append([]byte(nil), r.Data...)}, Offset: r.Offset, Done: r.Done})
	if err != nil {
		return nil, rpcError(err)
	}
	return &raftkvpb.InstallSnapshotResponse{Term: reply.Term, Accepted: reply.Accepted, NextOffset: reply.NextOffset}, nil
}
func (s *Server) JoinCluster(ctx context.Context, r *raftkvpb.JoinClusterRequest) (*raftkvpb.JoinClusterResponse, error) {
	if s.Handler == nil {
		return nil, status.Error(codes.Unimplemented, "Raft node not attached")
	}
	if r == nil || r.Peer == nil || r.Peer.Id == "" || r.Peer.GrpcAddress == "" || r.Peer.HttpAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "complete peer required")
	}
	reply, err := s.Handler.JoinCluster(ctx, raft.JoinClusterRequest{Peer: raft.Peer{ID: r.Peer.Id, GRPCAddress: r.Peer.GrpcAddress, HTTPAddress: r.Peer.HttpAddress, Learner: r.Peer.Learner}})
	if err != nil {
		return nil, rpcError(err)
	}
	return &raftkvpb.JoinClusterResponse{Accepted: reply.Accepted, LeaderHttpAddress: reply.LeaderHTTPAddress}, nil
}
