package transport

import "github.com/DynamicRaftKV/dynamic-raft-kv/proto/raftkvpb"

// Server inherits generated Unimplemented responses for every RPC. Real
// handler dispatch belongs to Stage 4; this server has no Raft node attached.
type Server struct {
	raftkvpb.UnimplementedRaftServiceServer
}

var _ raftkvpb.RaftServiceServer = (*Server)(nil)
