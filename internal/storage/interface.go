package storage

// Storage is a DRAFT synchronous durability contract, not an implementation.
// Success means the operation survives a process crash. Calls are serialized
// by the owner; no concurrent-use guarantee is required. Input buffers may be
// reused after return and recovery results are caller-owned.
// Failure is fatal to the current node operation: do not acknowledge a peer
// or continue issuing dependent operations after an uncertain storage failure.
type Storage interface {
	// Recover returns persisted state, not a claim that the suffix is committed.
	Recover() (Recovered, error)
	SaveHardState(HardState) error
	// Append replaces the suffix beginning at entries[0].Index, then appends
	// the contiguous batch. It must never truncate a committed entry. The
	// caller ensures this and rejects gaps or indices covered by a snapshot.
	// An empty batch does nothing. Success durably records the entire change.
	Append(entries []Entry) error
	// InstallSnapshot publishes metadata, payload, and suffix-retention policy
	// atomically for recovery. When retainSuffix is false, recovery discards
	// the entire old log; when true, it returns only entries above the snapshot
	// boundary. The caller verifies the boundary term matches before retaining.
	InstallSnapshot(snapshot Snapshot, retainSuffix bool) error
	// Compact discards entries at or below through, only after a snapshot
	// covering through is durable. It does not repair conflicting suffixes.
	Compact(through uint64) error
}
