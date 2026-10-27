# Stage 4 progress: networking and client code

Date: 27 October 2026. Local changes, not a merge or deployment.

Follow-up: Stage 5.1–5.2 now supplies WAL and durable process startup. The
production-wiring items below are resolved in `--cluster-config` mode; actual
three-process crash/restart tests pass. Compose acceptance remains pending.
See [current Stage 5 status](stage5-status.md) and the [completion plan](completion-plan.md).

## Stage 3 recheck

Re-ran build, vet, tests, import boundaries and the race detector successfully
before proceeding. All four Stage 3 implementation phases remain delivered;
formal interface freeze and two-reviewer approval remain open. Stage 4 required
one core extension: bounded append batches and explicit oversized-command
rejection. See ADR-0005 for the reason and limits.

## Implemented in this follow-up

| Phase | Code delivered | Remaining |
|---|---|---|
| 4.1 gRPC transport | Concurrent reusable client connections; all four wire mappings; core dispatch; cancellation/error mapping; malformed-enum rejection | Attach a durable node in production startup |
| 4.2 HTTP API | PUT/GET/DELETE via consensus; local status; command validation, result/error mapping and timeouts | Production startup wiring; membership endpoints remain Stage 6 |
| 4.3 Routing and CLI | 307 leader redirect, unknown-leader response; put/get/delete CLI, escaped keys, bounded redirects preserving request bodies | End-to-end acceptance using separately launched durable processes |
| 4.4 Multi-process/Compose | TCP integration fixtures exercise independent nodes and HTTP/gRPC on real local ports | Separate-process deployment and Compose intentionally deferred |

This is not a claim that all of Stage 4 is finished. The current `raftkv` binary
still starts scaffold/demo mode. The new adapters are exercised by integration
tests; no fake durable Storage was added just to make the process launch.

## N-node and dynamic-membership status

There is no hard-coded three-node limit in the core or adapters. Tests instantiate
1, 3, 5 and 7 nodes with the same implementation. Peers are fixed for the lifetime
of each node; quorum uses configured voters. Static learners are supported.

Live joins/removals/promotions are **not implemented**. Mapping a JoinCluster RPC
does not implement its membership algorithm. The current core returns
Unimplemented for joins and snapshot installation; HTTP join/remove return 501.

## Verification

- `make build vet test race` passed, including the new network/HTTP/CLI tests.
- `go test -race ./internal/transport ./internal/api ./cmd/raftctl -count=5`
  passed all five repetitions.
- TCP tests cover 1/3/5/7 nodes, HTTP write/read through a follower, state
  convergence and leader replacement after stopping the old leader.
- A follower catches up after missing over 5 MiB of log payload, using bounded
  replication requests instead of one oversized gRPC message.
- Adapter tests cover field preservation, binary payloads, protocol rejection,
  deadline cancellation, connection close, unknown enums and unsupported calls.
- HTTP/CLI tests cover missing/empty values, validation, unknown leaders,
  escaped keys, redirects retaining PUT bodies, and redirect-loop bounds.

These tests use real sockets in one test process and test-only memory storage.
They are not kill -9 tests, a durable cluster, or a dynamic-membership demo.

## Next coding sequence

1. Implement Stage 5 WAL and durable term/vote/log recovery.
2. Connect storage, node lifecycle, peer configuration, gRPC and HTTP in
   `cmd/raftkv`; launch N separate processes without needing Docker first.
3. Implement snapshots/compaction and crash recovery tests.
4. Implement Stage 6 durable configuration entries, join as learner, catch-up,
   promotion and removal with quorum-safety tests.
5. Add Compose and connect the presentation UI to the working cluster.

Run the current tests with Go 1.27.1:

```sh
go test -v ./internal/transport -run TestTCPCluster
go test -race ./...
```
