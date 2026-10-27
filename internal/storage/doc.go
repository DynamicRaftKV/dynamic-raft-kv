// Package storage defines durable log/snapshot contracts and implements a synced,
// checksummed WAL. Snapshot installation/compaction remain pending. Storage must
// not import raft.
package storage
