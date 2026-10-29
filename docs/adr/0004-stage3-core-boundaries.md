# ADR-0004: Stage 3 fixed-configuration consensus core

Status: implemented under owner authorization, 27 October 2026; team review pending.

## Context

The owner requested planning and implementation of the next critical phases.
The existing roadmap defines Stage 3.1–3.4 as state/timers, elections,
replication/commit and an in-process cluster. Existing cross-package signatures
can support those phases without adding networking, WAL or dynamic membership.

## Decision

Implement Node against the existing Storage, StateMachine and Transport
contracts. Every node receives the same immutable peer configuration including
itself. Only voters campaign, grant votes or count toward quorum; learners
receive the log. Reject unknown peer identities. Membership changes and snapshot
RPCs return ErrUnsupported; recovered snapshots and configuration entries are
rejected until the relevant stages are implemented.

Serialize transitions, persistence and application under one node lock. Never
hold it during outbound RPCs. Storage and StateMachine callbacks must not
reenter the node. Transport must be concurrent-safe and honor contexts. Run's
clock progresses independently of outbound RPC waits; manual Tick waits for
its outgoing batch to make deterministic tests practical. Do not mix Run and
manual ticks. At most one append per peer is in flight, including across terms.

Persist before acknowledging. Halt on storage errors or unexpected application
errors. ErrInvalidCommand represents a deterministic unsuccessful operation:
mark it applied and return its error without poisoning subsequent commands.
Propose may carry GET through the log, but no HTTP read policy or direct-read
surface is added. ErrNotLeader and cancellation mean the outcome may be unknown.

Recovery uses a fresh state machine and full uncompacted log. Commit/applied
start at zero. A newly elected leader appends a current-term no-op and commits
only through a voter quorum for a current-term entry. This safely re-establishes
the preceding committed prefix before replay; persistence alone is not proof of
commitment. A failed append must never be acknowledged as successful.

## Consequences

Stage 3 is independently testable without coupling to network implementations.
The memory-backed storage in tests models retained state, not actual durable
storage. There is no crash-safe deployable cluster until Stage 5, and no network
client path until Stage 4. The demo continues to exercise the standalone engine.
The log remains unbounded in memory; there is no compaction. Stage 4 subsequently
added command and append-batch limits for gRPC interoperability (ADR-0005).
Backtracking retries on subsequent heartbeats. No pre-vote, check-quorum,
leases, pipelining or exactly-once retry mechanism is introduced.

The system-wide interface freeze, SDD reconciliation, and two Raft reviewer
approvals are not implied by owner implementation authorization or passing tests.

## References

- [Raft paper, Figure 2 and section 5](https://raft.github.io/raft.pdf)
- [etcd Raft state transitions](https://github.com/etcd-io/raft/blob/main/raft.go)
- [etcd Raft log rules](https://github.com/etcd-io/raft/blob/main/log.go)

Used as conceptual cross-checks. No etcd dependency or source code was copied.
