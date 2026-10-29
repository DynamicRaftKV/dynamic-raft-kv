# Stages 7 and 8: reliability and integration

Updated 29 October 2026. Implementation, test evidence and final acceptance are
tracked separately. Passing this suite is not a proof of all possible schedules.

## Stage 7 evidence

| Area | Implemented evidence | Limits |
|---|---|---|
| 7.1 Fault injection | Test-only partitions, dropped and duplicated append messages; existing WAL error and interrupted snapshot seams | No production fault-injection endpoint |
| 7.2 Failure scenarios | Minority isolation, majority leader replacement, follower restart, lagging snapshot catch-up, real process leader kill/full restart | Broader mixed membership/network/disk schedules remain review work |
| 7.3 Correctness | Twelve seeded concurrent single-key histories checked against an exhaustive register model, including unknown-outcome writes; negative tests for the checker; race detector | Bounded histories (at most 16 operations), not a general-purpose multi-key proof or exhaustive scheduler |
| 7.4 Observability | JSON election/role/membership/snapshot/stop events, Prometheus text metrics, local diagnostic JSON, liveness endpoint | Counters reset on restart; liveness and role do not prove quorum |

The [etcd Raft reference](https://github.com/etcd-io/raft) emphasizes deterministic
testing and protection against unbounded proposal growth. This implementation
retains its own algorithm and adopts explicit client proposal admission limits:
1024 uncommitted entries, approximately 16 MiB of uncommitted payload plus entry
allowance, and 1024 pending callers. Limits are configurable in raft.Config.
HTTP returns 429 with Retry-After when capacity is reached. Cancellation removes
the waiting caller but does not cancel or erase a durable log entry.

These limits bound new client proposals on an isolated leader. They do not bound
the whole WAL, all RPC memory, internal election/no-op entries, state-machine
growth or disk use under arbitrary faults. Internal no-op/finalization entries
are allowed to make progress. Disk failures retain the existing fail-stop rule.

Endpoints on each durable node:

- `/metrics`: term, leadership/election counts, commit/applied/last/snapshot
  indices, pending callers, uncommitted entries/bytes, acceptance/rejection/wait
  timeout counts and WAL bytes.
- `/debug/raft`: local diagnostic snapshot, without key/value payloads.
- `/healthz`: process/node liveness, not readiness for linearizable operations.

## Stage 8 integration

The [local console](local-console.md) now includes real node status, replicated
KV commands, learner join/promotion/removal and diagnostic gauges. Standalone
snapshot/restore remains separate. Data lives in per-node WAL directories.

Compose now starts durable nodes using `configs/compose-three-node.json` and
separate volumes. Its HTTP addresses target clients on the Docker host; gRPC
addresses use Compose DNS internally. HTTP ports 8001–8005 are bound to loopback.
Container-internal HTTP clients must use explicit service addresses and cannot
rely on the host-facing redirect URLs. This is a local-host deployment profile,
not a multi-machine deployment configuration.

```sh
docker compose --profile node4 --profile node5 up -d --build
python3 scripts/compose-acceptance.py
docker compose --profile node4 --profile node5 down
```

The acceptance script is for a disposable test cluster. It expands 3→5 nodes,
promotes learners, kills the elected leader and then all nodes with SIGKILL,
restarts the cluster, verifies prior data and performs a new write/read. It does
not delete volumes. Use fresh test volumes for a new acceptance run: the initial
configuration expectation is three voters. Do not run it against important data.
CI owns a fresh environment and runs this instead of checking scaffold HTTP 501.

## Evaluation baseline

`BenchmarkDurableSingleton` measures one node, real fsync WAL and checkpoints
every 128 operations. On this developer's Apple M5 / macOS arm64, one 1-second
sample produced **3,793,682 ns/op, 3,657 B/op, 50 allocs/op** (344 iterations).
This is a baseline, not distributed throughput, latency percentiles or an SLA.
Reproduce with:

```sh
go test ./internal/raft -run '^$' -bench BenchmarkDurableSingleton -benchtime=1s -benchmem
make build vet test race
```

Stage 8 close-out still needs reviewed distributed benchmark results, the final
report/deck, consensus approvals and release acceptance. No report submission,
review approval, production readiness or completed Stage 9 is implied.
