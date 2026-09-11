package raft

import "context"

// RaftNode is the draft client-facing surface. Propose succeeds only after
// commit and application, returning opaque StateMachine result bytes. Context
// cancellation stops waiting, not an already accepted operation. IsLeader is
// advisory and cannot authorize a subsequent write. Status is a local view.
// GET routing and membership administration are unresolved; see contracts.md.
type RaftNode interface {
	Propose(context.Context, []byte) ([]byte, error)
	Status() NodeStatus
	IsLeader() bool
}
