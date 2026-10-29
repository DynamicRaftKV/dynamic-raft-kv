# Stage 1 contract review

Status: system-wide DRAFT. The owner explicitly authorized Stage 2 using the
existing state interface; see ADR-0003, and Stage 3 core on 27 October 2026;
see ADR-0004. Compile-time conformance is not a system-wide freeze. State and
Raft semantics are implemented as documented in stage2-status.md and
stage3-status.md. Stage 4 gRPC/HTTP/CLI adapters are implemented; see ADR-0005.
Stage 5.1–5.2 now implements WAL storage and durable process wiring; see ADR-0006.
Stage 5.3 implements snapshot assembly, recovery and compaction; ADR-0007 defines
the fixed configuration envelope, ordering, retry and size constraints.

## Established architecture

Stage 6 evolves membership via [ADR-0008](adr/0008-dynamic-membership.md):
configuration entries, version-2 membership snapshots, an optional
`ChangeMembership(context.Context, operation, id, peer)` administration method,
`POST /cluster/promote`, and status fields for joint consensus/finalization.
Join/remove are live on durable nodes. Existing RaftNode and protobuf interfaces
remain compatible. Earlier fixed-membership descriptions below are superseded
by ADR-0008 for dynamic nodes; the interface review remains required.

- state owns StateMachine; it has no Raft/network/API/membership dependencies.
- raft owns Transport and consumes storage/state contracts.
- transport implements the raft-owned adapter and owns generated-wire mapping.
- storage owns log and snapshot records, avoiding a storage-to-raft cycle.
- api exposes the specified six HTTP method/path combinations.
- cmd/raftkv is the composition root; raftctl is a separate HTTP client binary.
- Learner is separate from the three standard roles; ADR-0001 awaits sign-off.

## Draft choices made visible for review

| Topic | Draft | Implementation deferred |
|---|---|---|
| Command boundary | Apply([]byte) returns opaque []byte plus error | Implemented Stage 2 |
| KV encoding | JSON Command/Result, string key/value, PUT/GET/DELETE | Implemented Stage 2 |
| Results | PUT/DELETE acknowledge; GET found flag + value; absent DELETE succeeds | Implemented Stage 2 |
| Keys | Nonempty UTF-8 string; empty values allowed | Implemented Stage 2 |
| Limits | State engine remains unrestricted; encoded Raft command at most 1 MiB, append batches at most 128 entries and approximately 2 MiB | Stage 4 transport safety; ADR-0005 |
| Errors | Invalid commands wrap ErrInvalidCommand; invalid restore wraps ErrInvalidSnapshot; neither mutates state | Implemented Stage 2; HTTP mapping remains pending |
| Proposal | Success after application; cancellation stops waiting, not committed work | Core and HTTP adapter implemented |
| RPC types | Plain Go structs in raft, protobuf types in proto/raftkvpb | gRPC mapping implemented Stage 4 |
| Snapshot RPC | Ordered unary chunks ≤256 KiB; assembly ≤32 MiB; retries start at zero | Implemented; final reply means durable; review ADR-0007 |
| Storage | Synchronous durable success; owner serializes calls; snapshot and active suffix recovered into memory | WAL, snapshot installation and atomic compaction implemented |
| Log append | Contiguous batch can replace an uncommitted suffix, never a committed one | Stage 5 |
| Snapshot install | Atomically publish metadata, payload and suffix-retention decision | Implemented Stage 5.3 |
| Compaction | Reclaim covered prefix after durable snapshot; recovery filters covered entries | Implemented Stage 5.3 |

Store methods must not rely on time, randomness, or map iteration order. The
caller applies committed commands in log order. Mutex protection alone does not
establish that order. Results and output byte buffers belong to the caller;
implementations must not retain mutable caller input after return.

Snapshot returns a consistent KV payload. The Raft caller coordinates Apply and
Snapshot so lastIncludedIndex/Term describe exactly the captured state. Restore
replaces state atomically and leaves state unchanged on invalid input. The caller
serializes restore with committed application. KV format version 1 is documented
in snapshot-format.md; Raft metadata and membership configuration stay outside it.
The implementation also emits repeatable JSON bytes by sorting map keys.

Storage recovery returns term/vote, a snapshot, and a contiguous suffix; it does
not declare that all recovered entries are committed. A failed durability call
must stop dependent protocol work. Tail-corruption policy, supported recovery
states, and configuration payload encoding require SDD review.

## Decisions required before freeze

1. Supply actual SDD and reconcile its coupling, storage, read-consistency,
   membership, deployment, and exclusions sections.
2. Confirm module path, Go baseline, license and actual review participants.
3. Approve command/result representation, missing-key behavior, validation and
   error types. Specify whether binary values are required instead of strings.
4. Review the Stage 4 decision to send GET through Propose (ADR-0005).
   No separate read surface, lease/read-index algorithm or bypass to state exists.
5. Specify membership administration boundary for HTTP join/remove, the admission
   versus promotion completion result, and configuration-change algorithm.
   RaftNode currently contains only the plan's Propose/Status/IsLeader surface.
6. Review the Stage 4 ErrNotLeader to HTTP 307/503 mapping and externally
   reachable leader address configuration. IsLeader cannot authorize a proposal.
7. Review implemented inbound RPC dispatch and ADR-0007 snapshot chunk assembly.
   Server accepts RPCHandler; durable startup wiring is implemented in Stage 5.2.
8. Approve Storage atomicity/recovery guarantees and memory-backed active-log
   assumption; define WAL constants only after the actual format is known.
9. Review ADR-0007 fixed membership metadata and transfer bounds/identity, including
   whether the SDD requires gRPC streaming instead of chunked unary calls.
10. Specify fault-hook location/build constraint; only a placeholder is in scope.
11. Confirm priority amendments: API docs, local build target and interface
    harness count toward completion despite conflicting SHOULD labels.

No exactly-once/retry deduplication guarantee is made. An ambiguous proposal
timeout must not be documented as proof a write did not occur.

## Freeze procedure

Resolve these decisions in ADRs, review Go/protobuf/HTTP shapes together, record
the plan's synchronous approvals (or an owner-approved replacement process),
then mark contracts frozen. Later changes require an ADR and review. No review,
merge, ratification, or external branch protection is claimed by this scaffold.
