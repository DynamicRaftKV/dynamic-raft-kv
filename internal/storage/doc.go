// Package storage defines durable log/snapshot contracts and implements a synced,
// checksummed WAL with snapshot installation and atomic compaction. Storage must
// not import raft.
package storage
