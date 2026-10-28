# ADR-0007: Applied-state checkpoints and chunked snapshot transfer

Status: implemented; required Raft review remains pending. 29 October 2026.

## Scope and contracts

Stage 5.3 builds on the existing Storage and InstallSnapshot interfaces without
changing protobuf messages. Membership remains fixed. Snapshot Configuration is
version-1 JSON containing peers sorted by ID, with addresses and learner flags.
Recovery and incoming transfers must match the configured peer set exactly.
Stage 6 must deliberately evolve this rule with its membership protocol.

The node serializes snapshot capture with Apply. The boundary is lastApplied,
never lastLogIndex. Recovery restores that state and sets applied/commit to the
snapshot boundary; the remaining log is replayed only when commitment is known.
The retained log starts with a boundary sentinel carrying the included index
and term. Election freshness and replication use absolute Raft indices.

## Durability and reclamation

A snapshot is one checksummed WAL record containing metadata, state payload and
the retain-suffix decision. Installation syncs it before success. A torn final
record recovers the old generation. Complete corrupted records fail closed.
Storage verifies a matching boundary term before retaining a suffix. It rejects
regressing snapshots and appends at or below the installed boundary.

Compact writes a temporary WAL with current term/vote, snapshot and retained
entries, syncs it, renames it over raft.wal, and syncs the containing directory.
The node never appends to the retired inode. Errors poison storage and halt the
node. Crash recovery uses only raft.wal; orphan checkpoint files are ignored.
Process-kill tests cover before rename, after rename and after directory sync.
These tests do not establish behavior on hardware power loss beyond OS/filesystem
sync guarantees. Old binaries do not understand snapshot records: do not downgrade
a data directory after it has installed a snapshot.

## Transfer and bounds

The leader sends 256 KiB chunks when a follower's next index is covered by the
snapshot. One in-flight replication operation per peer prevents overlapping
append/snapshot requests. Each chunk has its own RPC deadline. Assembly is bounded
to 32 MiB and held in memory, identified by leader, term, boundary and configuration.
Offset zero restarts assembly; subsequent chunks must have the exact next offset.
The sender restarts at zero after failure. No resumable on-disk transfer is claimed.

Partial transfers cannot change state. The final chunk validates/restores the
state machine, durably installs the snapshot and compacts before acknowledgement.
If persistence fails after Restore, the node halts; restart restores durable state.
Already-applied snapshots are acknowledged without rolling state backwards,
making lost final acknowledgements safe to retry. Matching boundary terms retain
the suffix; mismatches discard it. Stale-term responses cannot advance replication.

## Operation and limits

Durable CLI nodes default to --snapshot-entries 1000; zero disables automatic
capture. Node.TakeSnapshot is also available to callers. No new public admin API
is introduced. Snapshots and compaction run synchronously under the node lock.
This favors a small correctness-first implementation; large checkpoints can
pause heartbeats and may provoke elections. Performance work is Stage 7/8.

The current snapshot payload limit is 32 MiB (configuration limit 1 MiB). Local
capture beyond that bound halts the node explicitly; it is not a silent success.
This bounds transfer memory, not total KV size or the uncommitted log. Admission
limits and larger/streaming checkpoints remain reliability work. Compaction uses
temporary disk space and does not clean orphan files from an interrupted rewrite.

## Evidence

Tests cover retained/discarded suffixes, owned buffers, torn records at every byte,
corruption, failed sync, compaction process-crash boundaries, invalid metadata
and payloads, out-of-order chunks, final retries, uncommitted suffix exclusion,
restart/election after snapshots, >4 MiB catch-up over real gRPC and WAL, and
three-process leader kill/full restart with automatic snapshots enabled.
