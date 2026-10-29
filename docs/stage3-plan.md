# Stage 3 implementation plan

The owner authorized planning and implementing the next phases on 27 October
2026. This branch implements Stage 3.1–3.4 against the existing contracts. It
does not claim the outstanding synchronous team freeze/reviews have occurred.

## Order and acceptance checks

1. **3.1 — node state and timers (critical):** validate fixed membership, recover
   term/vote/log through Storage, expose status, implement logical ticks and a
   cancellable real-time runner. Test startup, shutdown and independent election
   deadlines. A learner must not campaign or vote.
2. **3.4 — in-process harness (critical, built alongside the node):** independent
   nodes and state machines, test-only storage, routed RPCs, partitions, drops,
   delayed responses and restart fixtures. No network or wall-clock sleeps in
   algorithm scenarios. No pretend durable production storage.
3. **3.2 — elections (critical):** persist term/self-vote before campaigning;
   grant at most one vote per term to an up-to-date voter; use voter-majority
   quorum; step down on higher terms. Test split votes and leader replacement.
4. **3.3 — log replication and application (critical):** durable append before
   acknowledgement, consistency checks, suffix repair, next/match indices,
   current-term quorum commit, leader no-op and ordered application. Propose
   returns only after application. Test partitions, stale replies, recovery,
   immutable committed prefixes, persistence failures and command results.

## Boundary decisions

- Preserve Storage, Transport, RaftNode and StateMachine signatures. Add a
  concrete Node and Config, Tick/Run/Stop lifecycle methods and explicit errors.
- Serialize node transitions, storage and Apply under a node lock. Never hold
  that lock while calling Transport. Bound outbound calls with contexts, allow
  at most one AppendEntries per peer in flight and discard stale replies.
- Fixed voter/learner configuration only. Join, promotion, configuration entries,
  snapshot transfer/compaction and disk WAL are deferred to their planned stages.
- Recover a full uncompacted log only; reject snapshots/configuration entries
  explicitly rather than pretending to support unimplemented recovery paths.
- Recover commit/applied at zero with a fresh state machine. A current-term
  leader no-op re-establishes commitment of the preceding log before replay.
  Persisted entries are never treated as committed solely because they exist.
- KV validation failures are deterministic operation results (errors) and do not
  stop application. Unexpected state-machine/storage errors halt participation.
- Stage 3 does not serve local reads or connect the existing demo/HTTP API to
  consensus. The client integration/read policy remains Stage 4 work.

## References and exclusions

Use the Raft paper Figure 2 and section 5 for algorithm rules, and etcd-io/raft's
raft.go/log.go to cross-check vote freshness, stale-response handling, committed
prefix protection and current-term commit. Implement independently; no etcd
dependency or copied algorithm source. Do not add pre-vote, leases, pipelining,
membership changes or performance extensions.

## Verification and remaining work

Run focused deterministic failure tests, seeded schedules, full build/vet/test,
race checks and import-boundary checks. Record results in stage3-status.md.
Stages 4–6 are subsequent functional work; Stage 7 grows failure testing and
observability. Stage 8 is full-system evaluation. Do not count these as complete
because the core test harness exists. Two Raft reviewers are still required.
