// Package raft implements fixed-configuration Raft elections and replication.
// Snapshot transfer and membership changes remain deferred to Stages 5 and 6.
// It may consume storage and state contracts, but must not import transport,
// api, or membership implementations.
package raft
