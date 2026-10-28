// Package raft implements fixed-configuration Raft elections and replication.
// It supports snapshot transfer/recovery; live membership remains Stage 6.
// It may consume storage and state contracts, but must not import transport,
// api, or membership implementations.
package raft
