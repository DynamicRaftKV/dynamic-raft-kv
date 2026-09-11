// Package state implements a deterministic, standalone in-memory KV engine.
// It must not depend on Raft, transport, API, membership, or networking.
// Apply consumes JSON commands; Snapshot and Restore use a versioned KV-only
// JSON payload. There is no persistence or distributed read guarantee here.
package state
