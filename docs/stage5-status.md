# Stage 5 progress: durable storage and runnable nodes

Updated 29 October 2026. Implementation status is separate from required review.

## Delivered

- **5.1 WAL:** checksummed, versioned append records and HardState; synchronous
  writes; owned recovery data; suffix replacement; directory locking; fail-stop
  on I/O errors; incomplete-tail repair and corruption rejection.
- **5.2 Integration:** explicit JSON cluster configuration, stable bootstrap
  identity, exclusive per-node storage and production Node.Run + gRPC + HTTP
  wiring. The peer count is configurable; the set remains fixed across restarts.
- **Part of 5.4:** subprocess kill after acknowledged sync, real three-process
  leader kill, committed data check, full-cluster kill/restart and post-recovery
  reads/writes. These are actual process tests, not only memory-store simulations.
- **5.3 Snapshots:** applied-state checkpoints, chunked follower installation,
  fixed-configuration validation, snapshot restore and atomic WAL compaction.
  Durable nodes default to `--snapshot-entries 1000`; use `0` to disable automatic
  capture. See [ADR-0007](adr/0007-snapshots-and-compaction.md) for size limits,
  synchronous checkpoint costs and crash-ordering guarantees.

## Still pending

The rest of 5.4's failure matrix, including disk-full and further injected I/O
failure combinations. Dynamic membership remains Stage 6. Full Stage 5 completion
is not claimed. See ADR-0006 and ADR-0007 for the WAL format and limits.

## Checks performed

- `make build vet test race` passed, including the real-process tests.
- `go test -race ./cmd/raftkv ./internal/storage -count=3` passed all three runs.
- WAL tests cover suffix replacement, copy ownership, vote/log validation,
  directory lock contention, all byte cuts within an incomplete final record,
  checksum corruption, injected write/sync failures, and process termination
  after an acknowledged sync.
- Snapshot tests cover every byte cut of an installation record, corrupted
  records, suffix retention/discard, transfer ordering/retries, >4 MiB gRPC
  catch-up and real process crashes around checkpoint rename/directory sync.
- Three-process leader kill/full restart now runs with automatic snapshots.
  Docker acceptance and hardware power-loss behavior are not claimed.

## Run three durable nodes without Docker

Build with Go 1.27.1: `make build`. Run one command in each of three terminals
from the repository root:

```sh
./bin/raftkv --node-id node1 --cluster-config configs/local-three-node.json --data-dir data/node1 --http-addr 127.0.0.1:8101 --grpc-addr 127.0.0.1:9101
./bin/raftkv --node-id node2 --cluster-config configs/local-three-node.json --data-dir data/node2 --http-addr 127.0.0.1:8102 --grpc-addr 127.0.0.1:9102
./bin/raftkv --node-id node3 --cluster-config configs/local-three-node.json --data-dir data/node3 --http-addr 127.0.0.1:8103 --grpc-addr 127.0.0.1:9103
```

After a leader is elected:

```sh
./bin/raftctl --endpoint http://127.0.0.1:8101 status
./bin/raftctl --endpoint http://127.0.0.1:8101 put greeting hello
./bin/raftctl --endpoint http://127.0.0.1:8102 get greeting
```

The CLI follows leader redirects. Stop/restart processes with the same flags
and directories to retain data. Never share a data directory or change the
persisted voter list to simulate joining. Prepare a fresh configuration and
fresh per-node directories for a different static N; live changes require Stage 6.

The browser UI still uses `make demo` on port 8080 and is memory-only. It does not
show these durable nodes yet. The demo server must remain running for the page
and operations to work. A browser GitHub login does not automatically change
Git's SSH identity; verify Eternal-Spirit authentication before any push.
