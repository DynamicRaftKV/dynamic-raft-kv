# RaftKV readiness review and etcd comparison

Reviewed 27 October 2026 against local source/tests and fetched GitHub main
`6e6840a`. Local Stage 3–5.2 changes are not published merely because this report
exists. Required Raft reviews remain open.

## What “full Raft KV” means here

The target is an independently implemented, durable Raft-backed KV service with
safe live membership changes. It is not yet complete, and it is not an etcd
product clone. The current code implements a fixed-peer cluster with consensus
reads/writes, HTTP/gRPC, WAL persistence and process restart recovery.

etcd-io/raft supplies consensus; applications supply network and disk I/O. Its
feature set also includes snapshots/compaction, membership changes, efficient
reads and optional replication enhancements. The project's layered structure
is compatible with that separation, without adopting its library or API design.
[Source: etcd Raft](https://github.com/etcd-io/raft).

The official raftexample demonstrates a REST KV application with Raft and live
reconfiguration. It is a useful integration reference, not proof our independent
implementation is correct. [Source: raftexample](https://github.com/etcd-io/etcd/tree/release-3.7/contrib/raftexample).

The etcd database has additional APIs such as transactions, watches and leases.
Those are outside this project's agreed MVP. [Source: etcd API](https://etcd.io/docs/v3.6/learning/api/).

## Implementation assessment

| Area | Local evidence | Gap before completion |
|---|---|---|
| Elections/replication | Fixed voter quorum, no-op, freshness, suffix repair, current-term commit | Team review, larger adversarial schedules |
| Client consistency | GET/PUT/DELETE through Propose; success after application | History-based linearizability validation during partitions/restarts |
| Storage | Synced framed WAL, checksums, fail-stop errors and retained-state recovery | Atomic snapshots, compaction, bounded recovery/log retention |
| Networking | Real TCP tests for 1/3/5/7 nodes; bounded append requests | Docker/public-address acceptance and broader network-fault tests |
| Recovery | Actual leader kill and full three-process restart tests | Snapshot boundary/follower recovery and more crash points |
| Membership | Static learner attribute; immutable bootstrap configuration | Durable live configuration changes, join/promotion/removal |
| UI | Standalone commands/snapshots and updated project report | Real-cluster status and operational controls |

## Add explicit acceptance criteria to the existing plan

These refine the current stages rather than adding a new architecture:

1. **5.3 snapshot publication:** publish state, last-included index/term and
   membership consistently; test crashes before/after publication and WAL
   reclamation. Transfer retries must never install partial state. Test suffix
   retention, a stale snapshot and a follower too far behind for log catch-up.
2. **6.1 membership protocol:** choose and review one precise algorithm before
   coding; define when a configuration takes effect and which quorum commits
   it. Do not mix joint-consensus and single-change rules. Serialize changes,
   persist membership, prohibit identity reuse and recover it from storage.
3. **6.2–6.4 lifecycle:** joining does not mean promotion; verify catch-up before
   voting, learner exclusion from quorum, leader removal, restart mid-change,
   concurrent admin requests and operations that would destroy quorum.
4. **7.3 client histories:** check completed reads/writes under partitions and
   restarts. Treat timeout/cancellation as unknown outcome; do not claim exactly
   once or silently retry arbitrary failures. Deduplication would be a separate
   scope decision if later command types require it.
5. **7.1/7.4 resource handling:** measure pending requests, uncommitted log growth,
   WAL size and RPC pressure under minority isolation. Add explicit admission
   limits/backpressure as a proposed reliability task; bounded RPC batches alone
   do not bound total memory/disk growth. Define disk-full fail-stop behavior.
6. **4.4/8.1 deployment:** test advertised addresses from host and containers,
   separate persistent volumes, actual majority loss, restart and an accurate
   cluster UI. Compose currently remains scaffold-only; HTTP 501 smoke checks
   must become real-operation acceptance when Compose is migrated.

## Optional improvements, not hidden MVP requirements

Evaluate pre-vote, check-quorum, ReadIndex, leadership transfer and pipelining
only after the core MVP is stable. Keep current logged GET semantics until an
alternative has its own correctness tests. TLS/authentication and etcd-specific
MVCC/watch/transaction/lease features need explicit scope approval.

## Repository review findings

- Main's README mixes target capabilities with implemented behavior. Correct the
  unsupported fixed failover-time claim, “all nodes always agree” wording and
  old quick-start/API paths; make runnable cluster mode explicit.
- The frontend still said consensus was unimplemented. Replace that with actual
  Stage 3–5.2 status, separate standalone/demo data from durable cluster data,
  and expose the next work and runnable instructions without fake telemetry.
- New implementation files are local changes. Publish only under the requested
  authenticated account and use review before merging consensus changes.
- Passing tests do not replace required reviewer approval or prove all possible
  interleavings. The current project is a working fixed-membership Raft KV,
  not a feature-complete or production-equivalent replacement for etcd.
