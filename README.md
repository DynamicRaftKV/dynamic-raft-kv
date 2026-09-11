# RaftKV

Go implementation from scratch: **Stage 1 scaffolding and Stage 2 standalone KV**.
The state engine implements PUT/GET/DELETE, deterministic Apply, Snapshot and
Restore. An opt-in local demo UI calls the engine; the normal client API remains
stubbed. There is no Raft algorithm, WAL,
membership implementation, or authentication. Stage 3 has not started.
System-wide contracts remain draft pending SDD review; Stage 2 was explicitly
authorized by the owner using the existing state interface.

The [supplied plan](docs/implementation-plan.md) is the primary specification.
See [contract decisions](docs/contracts.md), [HTTP API](docs/api.md), and
[Stage 1 status](docs/stage1-status.md), and [Stage 2 status](docs/stage2-status.md).

## Build and run

Install Go 1.27.1, then:

```sh
make build vet test race
./bin/raftkv
./bin/raftctl status
```

The server binds HTTP :8001 and gRPC :9001. Valid HTTP requests return 501;
all gRPC methods return Unimplemented. raftctl reports the server body and exits
nonzero for the expected stub responses. Stop the server with Ctrl-C/SIGTERM.

Flags override environment variables:

| Flag | Environment | Default |
|---|---|---|
| `--node-id` | `RAFTKV_NODE_ID` | `node1` |
| `--peers` | `RAFTKV_PEERS` | empty |
| `--data-dir` | `RAFTKV_DATA_DIR` | `./data` |
| `--http-addr` | `RAFTKV_HTTP_ADDR` | `:8001` |
| `--grpc-addr` | `RAFTKV_GRPC_ADDR` | `:9001` |

Peer list and data directory are reserved configuration, not consumed yet.
Compose provisionally uses `id=grpc-host:port` comma-separated peers. The SDD
must confirm bootstrap and advertised-address conventions before freeze.

## Prototype UI

After building, run:

```sh
./bin/raftkv --demo --http-addr 127.0.0.1:8080 --grpc-addr 127.0.0.1:9090
```

Open http://127.0.0.1:8080/demo/ (not the HTML file directly). The console supports
PUT/GET/DELETE, live key inspection, snapshot capture/download/restore, and views
of implementation progress and architecture. Data is held in memory and lost on
server restart. Captured snapshots stay in the browser tab until downloaded;
reloading the page clears the tab's captured snapshot.

The demo adapter is disabled by default. It is for local presentation only and
does not implement consensus, replication, or disk persistence. `make demo` is
an alternative when Go is on PATH.

## Compose

Docker with Compose is required separately:

```sh
docker compose up -d --build
docker compose --profile node4 up -d --build
docker compose --profile node4 --profile node5 up -d --build
sh scripts/add-node.sh 4
docker compose --profile node4 --profile node5 down
```

Default: three processes. Profiles add a fourth/fifth. Each node has ports
800N/900N and a distinct named /data volume. Adding a process does not join it
to a Raft cluster. `down` preserves volumes.

## Generated contracts

Generated Go files are committed with the schema. Generation requires protoc
33.0 and these binaries on PATH:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
make generate
make check-generated
```

The generation script checks exact versions and does not install tools.
`make check-generated` requires generated files tracked in Git. `make lint` expects a compatible
golangci-lint v2 installation; lint is optional until its version is validated.

## Pending project settings

The module is `github.com/DynamicRaftKV/dynamic-raft-kv`. This project preserves
the repository's MIT license and existing proposal document.
The actual SDD and required team approvals are not available yet.
