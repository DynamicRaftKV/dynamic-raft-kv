package raft

import "github.com/DynamicRaftKV/dynamic-raft-kv/internal/storage"

// Role contains only the standard three Raft roles.
type Role string

const (
	Follower  Role = "follower"
	Candidate Role = "candidate"
	Leader    Role = "leader"
)

type Peer struct {
	ID          string `json:"id"`
	GRPCAddress string `json:"grpc_address"`
	HTTPAddress string `json:"http_address"`
	Learner     bool   `json:"learner"`
}

type NodeStatus struct {
	ID                string `json:"id"`
	Role              Role   `json:"role"`
	Term              uint64 `json:"term"`
	LeaderID          string `json:"leader_id"`
	LeaderHTTPAddress string `json:"leader_http_address"`
	CommitIndex       uint64 `json:"commit_index"`
	LastApplied       uint64 `json:"last_applied"`
	Peers             []Peer `json:"peers"`
}

type AppendEntriesRequest struct {
	Term         uint64
	LeaderID     string
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []storage.Entry
	LeaderCommit uint64
}
type AppendEntriesResponse struct {
	Term    uint64
	Success bool
}
type RequestVoteRequest struct {
	Term         uint64
	CandidateID  string
	LastLogIndex uint64
	LastLogTerm  uint64
}
type RequestVoteResponse struct {
	Term        uint64
	VoteGranted bool
}

// InstallSnapshotRequest is a DRAFT chunked transfer. Offset zero starts a
// transfer identified by leader, term, and included index/term. Done marks the
// final chunk. Snapshot.Data holds this chunk, not the complete payload.
type InstallSnapshotRequest struct {
	Term     uint64
	LeaderID string
	Snapshot storage.Snapshot
	Offset   uint64
	Done     bool
}

// Accepted acknowledges a chunk; the final acknowledgement means installed.
type InstallSnapshotResponse struct {
	Term       uint64
	Accepted   bool
	NextOffset uint64
}
type JoinClusterRequest struct{ Peer Peer }
type JoinClusterResponse struct {
	Accepted          bool
	LeaderHTTPAddress string
}
