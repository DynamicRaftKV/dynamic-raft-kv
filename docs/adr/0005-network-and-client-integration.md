# ADR-0005: Network and client adapters for configurable node counts

Status: implemented under owner authorization; team freeze/review pending.

## Context and plan

The owner requested a Stage 3 recheck, advancement to the next stage, and an
eventual N-node system with dynamic membership. Docker Compose should wait.
Re-run build/vet/test/race for Stage 3, then implement Stage 4.1 transport,
4.2 HTTP and 4.3 CLI adapters. Exercise them together over TCP with independent
nodes/state machines before connecting production startup to Stage 5 storage.

## Decisions

- Preserve the existing Go interfaces and protobuf schema. The gRPC server
  delegates to RPCHandler; the client reuses connections by address and supports
  concurrent requests, caller deadlines and explicit Close. Default transport
  is plaintext on a trusted network; no new authentication/TLS feature is added.
- Do not hard-code a three-node cluster. Tests construct 1, 3, 5 and 7 nodes
  using the same code and peer-list configuration. Quorum remains a majority
  of configured voters; changing that configuration at runtime is Stage 6.
- GET uses Propose, like PUT/DELETE. A successful GET is applied in the committed
  log order. It needs quorum and appends a log entry; it does not use local reads,
  leases or ReadIndex. This is a deliberately simple consistency policy.
- HTTP returns the existing state.Result JSON: 200 for successful PUT/DELETE
  and found GET, 404 for a missing GET. Empty values remain distinct from absent
  keys. Invalid commands return 400; encoded proposals over 1 MiB return 413.
  PUT parsing reads at most 1 MiB. No storage/state-domain limit is introduced.
- Not-leader responses return 307 with the configured leader HTTP address, or
  503 if unknown/unusable. A proposal timeout returns 504 with an explicit
  unknown-outcome message. Other core failures return 503 without disk details.
  Status is local/advisory; join/remove remain 501.
- CLI adds put/get/delete, escapes keys, follows only 307 redirects, retains
  method/body, allows at most five attempts, and disallows HTTPS downgrade.
  It does not automatically retry arbitrary network errors or timeouts.
- Bound append batches to 128 entries and approximately 2 MiB (payload plus
  conservative per-entry overhead). Reject commands larger than 1 MiB before
  append. This explicit addition prevents catch-up requests growing beyond
  gRPC's default 4 MiB receive limit. No proto or durable-record format changes.
  Existing oversized recovered entries are rejected instead of silently skipped.

## Boundaries

Update: ADR-0006 subsequently implements WAL and production startup. The
following records the adapter-stage boundary before that integration.

The executable and standalone UI are not switched to fake durable storage.
Until WAL integration, the production composition root retains scaffold/demo
mode. Tests alone use memory-backed Storage doubles. Real TCP in one test
process demonstrates adapter interoperability, not multi-process crash safety.

Snapshot and join messages are mapped faithfully, but the current core still
returns Unimplemented for these operations. This does not implement dynamic
membership. Required order: disk storage + process wiring, snapshot recovery,
then durable configuration changes, learner catch-up and promotion.
