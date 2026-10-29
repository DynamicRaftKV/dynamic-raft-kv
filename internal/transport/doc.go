// Package transport adapts the raft-owned transport contract to gRPC.
// Adapters preserve core request/response semantics and propagate cancellation.
// Snapshot and membership support depends on the attached core implementation.
package transport
