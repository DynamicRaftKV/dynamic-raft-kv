# Stage 3 implementation and verification

Date: 27 October 2026. Branch: `codex/stage3-raft-core`.
Implementation is local; this document does not claim a merge, GitHub CI run,
interface freeze, or the required two Raft reviewer approvals.

## Four phases implemented

| Phase | Delivered | Evidence |
|---|---|---|
| 3.1 Node state and timers | Validated fixed configuration; recovered term/vote/log; status; randomized election ticks; Run/Stop lifecycle | core.go; timer, recovery, runner and ownership tests |
| 3.2 Leader election | Durable self-vote; log-freshness checks; one vote per term; voter majority; higher-term step-down; learners cannot vote/campaign | election.go; election, split-vote, learner and restart tests |
| 3.3 Replication and commit | Append consistency; suffix repair; next/match indices; current-term quorum commit; leader no-op; ordered application; proposal results | replication.go; partition, conflict, commit-bound, cancellation and concurrent-proposal tests |
| 3.4 In-process cluster | Independent state machines, test storage, routed RPCs, unavailable links, delayed replies and node replacement | harness_test.go and core_test.go |

All four are critical prerequisites for the later stages. This is an implemented,
tested core, not a claim of production readiness or a formal safety proof.

## Verification performed

Using the pinned Go 1.27.1 toolchain on darwin/arm64:

- `make build vet test race` — passed, including state and Raft import boundaries.
- `go test -race ./internal/raft -count=20` — passed all 20 repetitions.
- `go test ./internal/raft -coverprofile=...` — passed; 88.5% statement coverage.
- Formatting and `git diff --check` — clean.

The 21 top-level Raft tests include 1/3/5-node clusters, two unavailable peers
in a five-node cluster, split elections, leader replacement, minority isolation,
learner exclusion from quorum, conflicting suffix repair, immutable committed
history, replay with retained test storage, durable-write failures, unexpected
application failure, invalid commands, slow peers, delayed replies after
step-down, 50 concurrent proposals, and cancellation with later commitment.
Twenty seeded schedules each exercise fifteen write/failover rounds. These
tests sample execution histories; they do not establish correctness for every
possible schedule.

Stress testing exposed a harness synchronization issue: it could inspect state
while an earlier asynchronous append was still in flight. The flush helper now
waits for those requests before advancing replication. Repeated race tests were
rerun successfully after that fix.

Docker smoke tests and protobuf regeneration were not rerun: no container,
wire-schema or generated-code changes were made. No kill -9/disk durability or
real-network tests are claimed; those require later implementations.

## What the user can run now

```sh
go test -v ./internal/raft
go test -race ./...
make demo
```

The first command demonstrates consensus scenarios through test output.
The browser at `http://127.0.0.1:8080/demo/` still exercises the standalone KV
engine. It does not run multiple Raft nodes or demonstrate real node crashes.
There is no production Storage implementation to construct a durable Node yet.

## Critical next work

1. **Stage 4.1–4.4: networking and clients.** Map protobuf to the core RPCs,
   implement gRPC client/server dispatch, wire the node lifecycle into the
   process, implement HTTP operations and leader routing/CLI, and add real
   multi-process tests. Settle GET consistency and error/redirect mapping before
   exposing those routes. Multi-process durability acceptance depends on Stage 5.
2. **Stage 5.1–5.4: durability and recovery.** Implement WAL/HardState writes,
   integrate disk-backed storage, snapshot installation/compaction and recovery,
   then test process kills and full-cluster restart. Core tests only model
   persistence; they do not substitute for fsync or corruption testing.
3. **Stage 6.1–6.4: dynamic membership.** After Stages 3–5 are validated,
   implement durable configuration changes, join, catch-up and promotion.
   Static learner support in Stage 3 is not dynamic membership.
4. **Stages 7–8:** extend fault scenarios to real processes, add observability,
   complete end-to-end validation, documentation and evaluation. Stage 9 remains
   stretch work after the MVP.

Stage 1's governance close-out remains open; Stage 2 is implemented. Stage 3's
code and local verification are delivered here, with review outstanding.
Stages 4–9 are not marked complete by this change.

Design decisions and reference sources: [ADR-0004](adr/0004-stage3-core-boundaries.md).
