# Stage 1 contract review

Status: system-wide DRAFT. The owner explicitly authorized Stage 2 using the
existing state interface; see ADR-0003. Stage 3 remains gated. Compile-time
conformance is not a system-wide freeze. State semantics are implemented and
documented in stage2-status.md; storage/transport/node remain declarations/stubs.

## Established architecture

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
| Limits | No domain key/value maximum selected | Confirm against SDD/API before freeze |
| Errors | Invalid commands wrap ErrInvalidCommand; invalid restore wraps ErrInvalidSnapshot; neither mutates state | Implemented Stage 2; HTTP mapping remains pending |
| Proposal | Success after application; cancellation stops waiting, not committed work | Stage 3/4 |
| RPC types | Plain Go structs in raft, protobuf types in proto/raftkvpb | Mapping in Stage 4 |
| Snapshot RPC | Unary chunk messages with offset/done; no resumable transfer implementation | Review transfer identity, retries, limits before freeze |
| Storage | Synchronous durable success; owner serializes calls; full active log recovered into memory | WAL implementation Stage 5 |
| Log append | Contiguous batch can replace an uncommitted suffix, never a committed one | Stage 5 |
| Snapshot install | Atomically publish metadata, payload and suffix-retention decision | Stage 5 |
| Compaction | Reclaim covered prefix after durable snapshot; recovery filters covered entries | Stage 5 |

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
4. Specify GET routing/consistency and whether RaftNode needs a separate read
   surface. No lease/read-index algorithm or bypass to state is implemented.
5. Specify membership administration boundary for HTTP join/remove, the admission
   versus promotion completion result, and configuration-change algorithm.
   RaftNode currently contains only the plan's Propose/Status/IsLeader surface.
6. Specify structured not-leader/unknown-leader errors and externally reachable
   leader HTTP address mapping. IsLeader cannot authorize a later proposal.
7. Approve inbound RPC dispatch and ownership of snapshot chunk assembly. The
   draft RPCHandler is not connected to the unimplemented server.
8. Approve Storage atomicity/recovery guarantees and memory-backed active-log
   assumption; define WAL constants only after the actual format is known.
9. Resolve snapshot membership metadata and transfer bounds/identity, including
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
