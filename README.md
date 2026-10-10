<div align="center">

<h1>
  <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"/>
  &nbsp; Dynamic Raft KV
</h1>

<h3>A fault-tolerant distributed key-value store â€” Raft consensus, built from scratch in Go</h3>

<p>
  <a href="https://github.com/DynamicRaftKV/dynamic-raft-kv/issues"><img src="https://img.shields.io/github/issues/DynamicRaftKV/dynamic-raft-kv?style=flat-square&color=e94560" alt="Issues"/></a>
  <a href="https://github.com/DynamicRaftKV/dynamic-raft-kv/milestones"><img src="https://img.shields.io/github/milestones/open/DynamicRaftKV/dynamic-raft-kv?style=flat-square&color=0f3460" alt="Milestones"/></a>
  <a href="https://github.com/orgs/DynamicRaftKV/projects/3"><img src="https://img.shields.io/badge/Project%20Board-Live-brightgreen?style=flat-square&logo=github" alt="Project Board"/></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/DynamicRaftKV/dynamic-raft-kv?style=flat-square&color=533483" alt="License"/></a>
  <img src="https://img.shields.io/badge/go-1.27+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version"/>
  <img src="https://img.shields.io/badge/gRPC-Protocol%20Buffers-4285F4?style=flat-square&logo=google&logoColor=white" alt="gRPC"/>
  <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker"/>
</p>

<p>
  <a href="#-what-is-this">What is this?</a> Â·
  <a href="#-architecture">Architecture</a> Â·
  <a href="#-raft-internals">Raft Internals</a> Â·
  <a href="#-quick-start">Quick Start</a> Â·
  <a href="#%EF%B8%8F-docker-cluster">Docker Cluster</a> Â·
  <a href="#-api-reference">API Reference</a> Â·
  <a href="#-project-roadmap">Roadmap</a> Â·
  <a href="#-team">Team</a>
</p>

---

</div>

## ðŸ§  What is this?

**Dynamic Raft KV** is a distributed, strongly-consistent key-value store built **from scratch in Go** â€” no Raft libraries, no shortcuts. It is a ground-up implementation of the [Raft consensus algorithm](https://raft.github.io/raft.pdf) (Ongaro & Ousterhout, 2014) applied to a practical key-value database.

> **This is not a toy.** It is a 9-stage engineering project targeting production-level correctness: leader election, log replication, crash recovery, state snapshotting, dynamic cluster membership, fault injection, and full observability.

### âœ¨ Key Properties

| Property | Description |
|:---|:---|
| **Strong Consistency** | Linearizable reads and writes â€” all nodes always agree on the same ordered log |
| **Fault Tolerance** | A 3-node cluster tolerates 1 simultaneous failure; a 5-node cluster tolerates 2 |
| **Automatic Leader Failover** | New leader elected within 150â€“600ms of detecting a failure |
| **Crash Recovery** | WAL + persistent state means a restarted node replays its log and rejoins the cluster |
| **Dynamic Membership** | Nodes can join, leave, or be promoted without downtime (Stages 6+) |
| **Fully Observable** | Prometheus metrics, structured logging, and health endpoints (Stage 7) |

---

## ðŸ—ï¸ Architecture

Each node in the cluster runs an identical process: an HTTP API, a Raft consensus engine, a state machine (the KV store), and persistent storage. All writes flow through the elected leader.

### High-Level System View

```mermaid
flowchart TD
    Client(["ðŸ‘¤ Client\n(curl / CLI / SDK)"])

    subgraph Cluster["RaftKV Cluster â€” Docker Compose"]
        direction TB

        subgraph N1["ðŸ“¦ raftkv-node1"]
            H1["HTTP API\n:8081"] --> R1[["RaftNode\n(Leader / Follower)"]]
            R1 --> KV1[("KVStore\nmap[string]string")]
            R1 --> WAL1[("Storage / WAL\n/data/node1/")]
            R1 --> G1["gRPC Transport\n:9001"]
        end

        subgraph N2["ðŸ“¦ raftkv-node2"]
            H2["HTTP API\n:8082"] --> R2[["RaftNode\n(Leader / Follower)"]]
            R2 --> KV2[("KVStore\nmap[string]string")]
            R2 --> WAL2[("Storage / WAL\n/data/node2/")]
            R2 --> G2["gRPC Transport\n:9002"]
        end

        subgraph N3["ðŸ“¦ raftkv-node3"]
            H3["HTTP API\n:8083"] --> R3[["RaftNode\n(Leader / Follower)"]]
            R3 --> KV3[("KVStore\nmap[string]string")]
            R3 --> WAL3[("Storage / WAL\n/data/node3/")]
            R3 --> G3["gRPC Transport\n:9003"]
        end

        G1 <-- "AppendEntries\nRequestVote (gRPC)" --> G2
        G1 <-- "AppendEntries\nRequestVote (gRPC)" --> G3
        G2 <-- "AppendEntries\nRequestVote (gRPC)" --> G3
    end

    Client -- "PUT / GET / DELETE\nHTTP/JSON" --> H1
    Client -. "HTTP :8082" .-> H2
    Client -. "HTTP :8083" .-> H3

    style Cluster fill:#0d1117,stroke:#30363d,stroke-width:2px,color:#c9d1d9
    style N1 fill:#161b22,stroke:#e94560,stroke-width:2px,color:#c9d1d9
    style N2 fill:#161b22,stroke:#0f3460,stroke-width:2px,color:#c9d1d9
    style N3 fill:#161b22,stroke:#0f3460,stroke-width:2px,color:#c9d1d9
```

### Component Breakdown

| Component | Role | Technology |
|:---|:---|:---|
| **HTTP API** | Client-facing REST interface. Forwards writes to Raft; redirects clients to leader if needed | Go `net/http` |
| **RaftNode** | Core consensus engine. Manages elections, log replication, heartbeats, and commit tracking | Pure Go |
| **StateMachine / KVStore** | The actual database â€” a `map[string]string` that receives committed log entries | Go map |
| **Storage / WAL** | Persists `currentTerm`, `votedFor`, and log entries to disk for crash recovery | JSON files on disk |
| **GRPCTransport** | Node-to-node communication for `RequestVote` and `AppendEntries` RPCs | gRPC + Protobuf |

---

## ðŸ”¬ Raft Internals

### Node State Machine

Every `RaftNode` transitions between three states. The transition logic is the heart of the Raft algorithm:

```mermaid
stateDiagram-v2
    direction LR
    [*] --> Follower : Node starts\n(load persisted state)

    state Follower {
        [*] --> WaitingForHeartbeat
        WaitingForHeartbeat --> WaitingForHeartbeat : AppendEntries received\n(reset election timer)
    }

    state Candidate {
        [*] --> RequestingVotes
        RequestingVotes --> RequestingVotes : Timeout, no winner\n(increment term, retry)
    }

    state Leader {
        [*] --> SendingHeartbeats
        SendingHeartbeats --> SendingHeartbeats : Heartbeat interval\nor client Propose()
    }

    Follower --> Candidate : Election timeout\n(no heartbeat received)\nIncrement term, vote for self
    Candidate --> Leader : Majority votes received\nBegin heartbeats immediately
    Candidate --> Follower : Valid leader found\nor higher term seen
    Leader --> Follower : Higher term discovered\nStep down immediately
```

| State | Behaviour |
|:---:|:---|
| **Follower** | Waits for heartbeats. Votes in elections. Never accepts writes directly. |
| **Candidate** | Requests votes from all peers. Increments term, votes for itself. |
| **Leader** | Accepts all writes. Replicates entries to followers. Sends heartbeats every ~100ms. |

### Write Path â€” PUT Request Flow

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant HTTP as HTTP API<br/>(Leader Node)
    participant Leader as RaftNode<br/>(Leader)
    participant Log as Replicated Log
    participant SM as KVStore
    participant F1 as RaftNode<br/>(Follower 1)
    participant F2 as RaftNode<br/>(Follower 2)

    rect rgb(15, 52, 96)
        Note over Client,F2: Normal Write â€” Client â†’ Leader
        Client->>+HTTP: PUT /kv/user:123<br/>{"value": "Aran"}
        HTTP->>+Leader: Propose(SET user:123 "Aran")
        Leader->>Log: Append entry {idx:1, term:1, cmd}
        Note over Leader: Entry appended locally.<br/>Not yet committed.
        par Replicate in parallel
            Leader->>+F1: AppendEntries (gRPC)
            F1->>F1: Validate & append to log
            F1-->>-Leader: {success: true}
        and
            Leader->>+F2: AppendEntries (gRPC)
            F2->>F2: Validate & append to log
            F2-->>-Leader: {success: true}
        end
        Note over Leader: Majority (3/3) confirmed.<br/>Advance commitIndex = 1.
        Leader->>+SM: Apply(entry)
        SM-->>-Leader: {success}
        Leader-->>-HTTP: Committed at index 1, term 1
        HTTP-->>-Client: HTTP 200 {"status":"ok","index":1,"term":1}
    end

    rect rgb(26, 26, 46)
        Note over Client,F2: Follower Redirect â€” Client hits wrong node
        Client->>+F1: PUT /kv/user:456 {"value":"Priya"}
        F1-->>-Client: HTTP 307 {"leader":"node1:8081"}
        Note over Client: Client retries with Leader
    end
```

### The Replicated Log

Every committed write is durably stored in an ordered, append-only log replicated across all nodes:

```mermaid
flowchart LR
    subgraph Log["Replicated Log (all 3 nodes hold identical copy)"]
        direction LR
        E1["Index 1\nTerm 1\nSET user:123\n'Aran'\nâœ… committed"]
        E2["Index 2\nTerm 1\nSET config:x\n'true'\nâœ… committed"]
        E3["Index 3\nTerm 2\nDELETE user:456\n\nâœ… committed"]
        E4["Index 4\nTerm 2\nSET z\n'99'\nðŸ”„ pending"]
        E1 --> E2 --> E3 --> E4
    end
    style Log fill:#0d1117,stroke:#30363d,color:#c9d1d9
    style E1 fill:#0f3460,stroke:#e94560,color:#c9d1d9
    style E2 fill:#0f3460,stroke:#e94560,color:#c9d1d9
    style E3 fill:#0f3460,stroke:#e94560,color:#c9d1d9
    style E4 fill:#161b22,stroke:#666,color:#888
```

> A write is **committed** only when a majority of nodes (â‰¥2 of 3) have appended it to their local log. This guarantees no committed entry is ever lost, even if up to `(n-1)/2` nodes fail simultaneously.

---

## âš¡ Quick Start

### Prerequisites

- Go 1.27+
- Docker (for cluster mode)
- `protoc` 33.0 (only if regenerating gRPC contracts)

### Build & Test

```sh
# Clone the repository
git clone https://github.com/DynamicRaftKV/dynamic-raft-kv.git
cd dynamic-raft-kv

# Build binaries, vet, run tests, check race conditions
make build vet test race

# Start a single node
./bin/raftkv --node-id node1 --http-addr :8001 --grpc-addr :9001
```

### Configuration

| Flag | Environment Variable | Default | Description |
|:---|:---|:---:|:---|
| `--node-id` | `RAFTKV_NODE_ID` | `node1` | Unique name for this node |
| `--peers` | `RAFTKV_PEERS` | `""` | Comma-separated peer list: `id=host:port,...` |
| `--data-dir` | `RAFTKV_DATA_DIR` | `./data` | Directory for WAL and snapshot files |
| `--http-addr` | `RAFTKV_HTTP_ADDR` | `:8001` | Client-facing HTTP address |
| `--grpc-addr` | `RAFTKV_GRPC_ADDR` | `:9001` | Inter-node gRPC address |

---

## ðŸ–¥ï¸ Demo UI

A local demo mode showcases the in-memory KV engine, snapshot capture, and architecture overview:

```sh
make demo
# Open: http://127.0.0.1:8080/demo/
```

> **Note:** The demo UI is for local presentation only. It does **not** implement consensus or replication. Data is held in-memory and lost on restart.

**Demo capabilities:**
- âœ… Interactive `PUT` / `GET` / `DELETE` on the in-memory KV store
- âœ… Live key inspection and value browsing
- âœ… Snapshot capture, download, and restore
- âœ… Architecture visualisation panels

---

## ðŸ³ Docker Cluster

Run a full 3-node Raft cluster locally using Docker Compose:

```sh
# Start a 3-node cluster
docker compose up -d --build

# Add a 4th or 5th node
docker compose --profile node4 --profile node5 up -d --build

# Add a node to a live cluster
sh scripts/add-node.sh 4

# Tear down (volumes preserved)
docker compose down
```

### Port Map

| Node | HTTP Port | gRPC Port | Data Volume |
|:---:|:---:|:---:|:---|
| `raftkv-node1` | `:8081` | `:9001` | `/data/node1/` |
| `raftkv-node2` | `:8082` | `:9002` | `/data/node2/` |
| `raftkv-node3` | `:8083` | `:9003` | `/data/node3/` |

---

## ðŸ“¡ API Reference

All endpoints use JSON. Write requests must be sent to the **leader**; followers return `HTTP 307` with the leader's address.

### Key-Value Operations

```sh
# Write a key
curl -X PUT http://localhost:8081/kv/user:123 \
  -H "Content-Type: application/json" \
  -d '{"value": "Aran"}'
# â†’ 200 {"status":"ok","index":1,"term":1}

# Read a key
curl http://localhost:8081/kv/user:123
# â†’ 200 {"key":"user:123","value":"Aran","found":true}

# Delete a key
curl -X DELETE http://localhost:8081/kv/user:123
# â†’ 200 {"status":"ok","index":2,"term":1}

# Key not found
curl http://localhost:8081/kv/unknown
# â†’ 404 {"key":"unknown","found":false}
```

### Cluster Status

```sh
curl http://localhost:8081/status
# â†’ 200 {"node_id":"node1","state":"Leader","term":3,"commit_index":42,"leader":"node1"}
```

---

## ðŸ› ï¸ Development

### Generating gRPC Contracts

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

make generate        # Regenerate from .proto files
make check-generated # Verify generated files match committed versions
```

### Tech Stack

| Layer | Technology |
|:---|:---|
| Language | Go 1.27 |
| Consensus | Raft â€” from-scratch implementation |
| Transport | gRPC + Protocol Buffers v3 |
| Client API | HTTP/JSON REST |
| Persistence | Write-Ahead Log (WAL) â€” JSON on disk |
| Containerisation | Docker + Docker Compose |
| Observability | Prometheus metrics (Stage 7) |
| Testing | Go `testing`, race detector, fault injection |

---

## ðŸ—ºï¸ Project Roadmap

The project is structured as 9 stages, tracked on the [GitHub Project Board](https://github.com/orgs/DynamicRaftKV/projects/3):

| Stage | Theme | Key Deliverables | Status |
|:---:|:---|:---|:---:|
| **1** | Foundation & System Design | SDD, Protobuf contracts, core interfaces, ADRs | ðŸ”„ In Progress |
| **2** | Standalone KV Engine | In-memory KV, WAL, state machine, Snapshot/Restore | âœ… Scaffolded |
| **3** | Raft Core | Leader election, AppendEntries, heartbeats, safety proofs | ðŸ“‹ Ready |
| **4** | Networking & Client | gRPC transport, follower redirect, client SDK, CLI | â³ Pending |
| **5** | Durability & Snapshots | Log compaction, InstallSnapshot RPC, crash recovery | â³ Pending |
| **6** | Dynamic Membership | Joint consensus, learner nodes, single-server changes | â³ Pending |
| **7** | Reliability & Observability | Chaos testing, fault injection, Prometheus dashboards | â³ Pending |
| **8** | Integration & Evaluation | End-to-end tests, benchmarks, runbooks, presentation | â³ Pending |
| **9** | Stretch Goals | Read leases, pipelining, multi-Raft, TLS | ðŸ”® Future |

> ðŸ“‹ View the full breakdown of **39 Epics** and **161 Tasks** on the [**Live Project Board â†’**](https://github.com/orgs/DynamicRaftKV/projects/3)

---

## ðŸ‘¥ Team

| Person | GitHub | Architectural Domain |
|:---|:---|:---|
| **Anubhav** | [@Anubhav2506](https://github.com/Anubhav2506) | Raft Core â€” consensus, election, log replication |
| **Eternal-Spirit** | [@Eternal-Spirit](https://github.com/Eternal-Spirit) | Storage & Membership â€” WAL, snapshots, dynamic membership |
| **Pratik** | [@pratikbatish](https://github.com/pratikbatish) | Networking & API â€” gRPC transport, client SDK, CLI |
| **Yuvraj** | [@Yuvraj-Kumar05](https://github.com/Yuvraj-Kumar05) | Reliability & DevOps â€” fault injection, CI/CD, observability |

---

## ðŸ“š Further Reading

| Document | Description |
|:---|:---|
| [ðŸ“– Complete Project Guide](raft_kv_complete_guide.md) | 1700-line deep-dive: what is Raft, why it exists, how it works |
| [ðŸ“ System Design Diagrams](raftkv_diagrams.md) | 6 UML/Mermaid diagrams: class, data flow, use case, sequence, state, deployment |
| [ðŸ“‹ Implementation Plan](docs/implementation-plan.md) | Primary specification: all stages, tasks, acceptance criteria |
| [âš–ï¸ Contracts & Interfaces](docs/contracts.md) | Interface decisions and architectural contract decisions |
| [ðŸŒ HTTP API Spec](docs/api.md) | Full HTTP endpoint specification |

---

## ðŸ“„ License

This project is licensed under the **MIT License** â€” see [LICENSE](LICENSE) for details.

---

<div align="center">

Built with â¤ï¸ by **DynamicRaftKV** Â·
[Project Board](https://github.com/orgs/DynamicRaftKV/projects/3) Â·
[Open Issues](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues) Â·
[Milestones](https://github.com/DynamicRaftKV/dynamic-raft-kv/milestones)

</div>
