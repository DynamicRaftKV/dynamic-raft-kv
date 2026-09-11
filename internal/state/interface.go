package state

// StateMachine is a DRAFT contract pending SDD reconciliation and review.
// Apply consumes an opaque encoded Command and returns an encoded Result.
// The caller serializes committed commands in log order; concurrent safety
// alone is not sufficient to establish that order. Invalid commands must not
// mutate state. Returned byte slices are owned by the caller.
type StateMachine interface {
	Apply(command []byte) (result []byte, err error)
	// Snapshot returns a consistent KV-only payload. The caller coordinates
	// application and pairs it with the corresponding Raft index and term.
	Snapshot() ([]byte, error)
	// Restore replaces, never merges, state. Failure leaves state unchanged.
	// The caller serializes restoration with committed application.
	Restore([]byte) error
}
