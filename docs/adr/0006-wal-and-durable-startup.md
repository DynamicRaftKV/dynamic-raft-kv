# ADR-0006: Synced WAL and explicit durable node startup

Status: implemented under owner authorization; team review pending.

## Context and scope

The owner requested the complete remaining-stage list, the next coding phase,
restoration of the frontend and use of Eternal-Spirit for GitHub pushes.
Implement Stage 5.1 WAL and 5.2 storage/process integration. Snapshots and live
membership remain separate work. Docker deployment stays deferred.

## WAL format and recovery

- `raft.wal` begins with the eight-byte version magic `RAFTKV01`.
- Each record has a 12-byte little-endian header: payload length (uint32),
  CRC32C of the payload (uint32), CRC32C of the preceding eight header bytes
  (uint32). Payload is one JSON hard_state or append record, at most 64 MiB.
- A complete append record replaces the suffix at its first entry index. The
  entire batch is one recoverable record. HardState records contain term/vote.
- Each operation validates, writes all bytes, fsyncs, then changes the in-memory
  view. Any write/sync error poisons the handle; no further operation succeeds.
  A failed operation may have reached disk, so its outcome remains uncertain.
- A partial final header/payload is truncated back to the last complete record
  and the truncation is synced. Complete records with invalid checksums, unknown
  record types or invalid term/log transitions fail closed, without rewriting.
  Arbitrary file truncation cannot be distinguished from an unacknowledged torn
  tail; filesystem/storage integrity is still an underlying assumption.
- New directory entries and the new WAL file are synced before successful open.
  The lock file is held exclusively with OS advisory locking on Linux/macOS.
  Other platforms fail explicitly. Recovery returns owned copies, not slices
  into the active log.
- No committed-index claim is stored. The existing current-term leader no-op
  re-establishes commitment before the fresh state machine replays the log.

The sync contract and tests cover process crashes. Hardware/power-loss behavior
is bounded by the filesystem and OS sync guarantees, not established by these
process-kill tests. Checksums detect corruption; they do not repair it.

## Startup and configuration

`--cluster-config FILE` explicitly enables durable Raft mode. FILE contains a
`peers` array with identities, advertised gRPC/HTTP addresses and learner flags.
All nodes start with the same fixed configuration; each needs its own data dir.

A canonical `bootstrap.json` records node identity/configuration while the WAL
lock is held. Restart rejects a changed identity/configuration or a populated WAL
without a bootstrap identity. Never reinterpret changing a JSON file as dynamic
membership. Stage 6 must replace this fixed-configuration restriction using a
reviewed durable configuration-change protocol.

The process wires WAL, fresh StateMachine, Node, gRPC client/server and HTTP
handler; it drives Node.Run and stops participation before closing storage.
`--demo` remains a separate memory-only UI and cannot combine with cluster mode.
No cluster flag retains the existing scaffold behavior for compatibility with
unfinished Compose/CI smoke scaffolding.

## Deferred

InstallSnapshot/Compact explicitly return unsupported; no snapshot/WAL rewrite
or log reclamation is implemented. The WAL and active log grow without bounds.
Dynamic membership, frontend cluster integration and Docker acceptance remain
on the completion plan. Existing interfaces and protobuf schema are unchanged.
