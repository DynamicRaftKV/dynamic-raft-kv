package storage

// EntryKind distinguishes KV payloads from Raft-owned records.
type EntryKind uint8

const (
	EntryCommand EntryKind = iota
	EntryNoop
	EntryConfiguration
)

// Entry payload is opaque to storage. Indices begin at 1; 0 is the initial
// boundary before any entries. The encoding of configuration data is pending.
type Entry struct {
	Index uint64
	Term  uint64
	Kind  EntryKind
	Data  []byte
}

// HardState is persisted before a protocol response relying on its new values.
// Empty VotedFor means no vote in Term.
type HardState struct {
	Term     uint64
	VotedFor string
}

// Snapshot wraps the KV payload; Configuration is Raft/membership-owned.
// Its encoding and configuration-change algorithm require SDD confirmation.
type Snapshot struct {
	LastIncludedIndex uint64
	LastIncludedTerm  uint64
	Configuration     []byte
	Data              []byte
}

// Recovered contains a consistent snapshot and the contiguous log suffix after
// it. A nil Snapshot means no stored snapshot. Entries need not be committed.
type Recovered struct {
	HardState HardState
	Snapshot  *Snapshot
	Entries   []Entry
}
