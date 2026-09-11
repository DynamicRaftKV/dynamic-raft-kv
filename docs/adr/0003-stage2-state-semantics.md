# ADR 0003: Standalone Stage 2 state engine

Status: implemented following the owner's explicit "start stage2" instruction.
This records the authorization to proceed and implementation choices; it does
not claim the missing SDD or four-person system-wide freeze was approved.

## Context

Stage 1 had already declared Apply([]byte) ([]byte, error), Snapshot and Restore,
plus JSON Command/Result structs. The owner explicitly requested Stage 2 while
other system-wide contract decisions remained open.

## Decision

Implement the existing interface with an RWMutex-protected map and private CRUD
helpers. NewStore and the zero Store value are usable. Preserve opaque bytes at
the boundary, strings as KV data, and the existing JSON tags. Omitted command
value means empty, which supports the existing omitempty encoding; HTTP still
requires its distinct request's value field. GET/DELETE reject nonempty values.

Invalid commands wrap ErrInvalidCommand and leave state unchanged. Invalid
snapshots wrap ErrInvalidSnapshot and leave state unchanged. No domain size
limit is invented. Reject null, unknown struct fields, invalid raw UTF-8,
trailing JSON, unsupported operations/versions, and empty keys. Otherwise Go's
encoding/json decoding semantics apply (including duplicate-key handling).

Use a version-1 JSON object containing the complete map. Restore builds and
validates a new map, then replaces the old one under the lock. Snapshot holds a
read lock through serialization. Raft metadata remains external to the payload.

## Consequences

Stage 2 is independently testable without network, storage, or Raft code. Tests
use seeded standard-library random generation rather than another dependency.
The existing CI test target enforces the state import boundary on every run.
The API remains a stub and cannot bypass consensus to mutate the store. Future
SDD reconciliation may require an explicit amendment, not a silent API change.
