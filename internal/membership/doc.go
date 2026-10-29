// Package membership is reserved for orchestration beyond the Raft core.
// Durable membership transitions live in internal/raft so they share its lock,
// persistence ordering and quorum rules. HTTP adaptation lives in internal/api.
// Learner is a membership attribute, not a fourth Raft role.
package membership
