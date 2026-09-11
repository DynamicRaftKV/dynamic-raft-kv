# Stage 2: standalone KV engine

Implementation and local checks complete. Formal team review and hosted CI
execution are not claimed. The owner authorized Stage 2 separately from the
unresolved Stage 1 system-wide freeze; see ADR-0003.

| Phase | Delivered |
|---|---|
| 2.1 | Zero-value-safe map store, mutex-protected private put/get/delete, command codec and validation |
| 2.2 | Deterministic Apply, JSON results, compile-time StateMachine conformance |
| 2.3 | Version-1 JSON snapshots, atomic replacement restore and format documentation |
| 2.4 | CRUD, invalid input, concurrency, snapshot and model/replay tests; CI import-boundary check |
| 2.5 | Test-only driver using StateMachine exclusively after fixture construction |

## Operation contract

- PUT sets/overwrites and returns `{"op":"PUT"}`. Empty values are supported.
- GET does not mutate. Missing returns `{"op":"GET"}`; present returns found
  true and value (empty value omitted by existing Result JSON tags). Decode into
  Result to distinguish Found=false from Found=true with an empty Value.
- DELETE removes if present and always returns `{"op":"DELETE"}`. Missing or
  repeated deletion succeeds; no previous value is returned.
- Command JSON has op/key/value. Omitted value means empty; null is invalid.
  Keys cannot be empty. No arbitrary key/value size limit was introduced.
- Errors wrap ErrInvalidCommand or ErrInvalidSnapshot; callers can use errors.Is.
- The in-memory read is not a distributed consistency guarantee. Callers must
  serialize committed Apply invocations in log order.

## Verification

`make build vet test race` passed using Go 1.27.1. The test target includes
`scripts/check-state-imports.sh`, so the existing CI workflow runs the boundary
check without path filtering. State production dependencies include no Raft,
transport, API, membership, net, or gRPC packages.

Property checks replay 100 seeds of 200 commands against two instances and an
independent map model, including intermediate restore and operation results.
Snapshot tests cover empty, small and 5,000-key stores, replacement, caller byte
ownership, and failure atomicity. Concurrent operations, Apply, Snapshot and
Restore run under the race detector. Valid-command tests fail on any panic.

No raftkv server wiring, Raft algorithm, WAL, network calls, membership logic,
TTL, CAS, or authentication was added. Stage 3 has not started.
