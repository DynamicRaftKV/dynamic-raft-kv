# RaftKV Implementation Plan — Stage 1 & Stage 2

**Scope of this document:** Full phased implementation plan for **Stage 1 (Foundation, Interfaces & System Design)** and **Stage 2 (Standalone Key-Value Engine)**, including tasks, ownership, dependencies, parallel work, testing, deliverables, Definition of Done, and Exit Criteria for every phase.

This plan is written against the ratified SDD (§10–§47) and preserves the dependency model: Stage 1 gates all later work; Stage 2 and Stage 3 (Raft Core) run **in parallel** once Stage 1's interface contracts are frozen.

---

## How Stage 1 and Stage 2 fit the whole system

- **Connection A (Interfaces):** Stage 1 is *entirely* about defining `Storage`, `Transport`, and `StateMachine` (the `Apply(command) result` contract) as Go interfaces with no implementations behind the "do not start coding" constraint (§12). Stage 2 delivers the first real implementation of `StateMachine` — proving Connection A actually works end-to-end for one of the three interfaces before Raft (Stage 3) starts consuming it.
- **Connection B (Data Flow):** Stage 2's `Apply()` is the *last hop* of the client→Raft→log→replication→commit→StateMachine→response pipeline. Nothing about Raft, replication, or networking is implemented yet, but Stage 2 must produce the exact contract that pipeline will call into.
- **Connection D (Snapshots):** Stage 2 implements `Snapshot()`/`Restore()` on the KV engine itself — this is the producer/consumer end of the snapshot flow that Stage 5 (durability) and Stage 6 (membership catch-up) depend on later.
- **Coupling rule enforcement (§11):** `internal/state` must have **zero knowledge** of Raft, transport, or networking. Stage 2 is the first real test of whether that isolation boundary actually holds in code, not just on paper.
- **Why Stage 2 can run in parallel with Stage 3:** Raft Core (Stage 3) only needs the *signature* of `StateMachine.Apply()`, not its implementation, to build against. Stage 1 Phase 1.2 freezing that signature is therefore the single most important unlock in this plan — it is what lets two developers work simultaneously starting week 2 instead of serially.

---

# STAGE 1 — Foundation, Interfaces & System Design

## Stage Overview

**Objective:** Establish repository structure, all cross-package interfaces (`Storage`, `Transport`, `StateMachine`), Protobuf/gRPC contracts, HTTP API and `raftctl` CLI skeletons, Docker Compose scaffolding, CI, and the ratified SDD/ADRs — with **no business logic implemented**, per the project's "do not start coding" constraint on interface sketches (§12).

**Position in dependency graph:** Root of the entire project. Stage 2 and Stage 3 cannot productively start until Phase 1.2 (interfaces) is merged. Stage 4 needs Phase 1.3 (Protobuf/gRPC). Stage 5 needs the `Storage` interface signature from Phase 1.2. Every later stage needs Phase 1.6 (CI) and Phase 1.7 (docs/ADR conventions) to already be running.

**Value to final system:** Without a stable, reviewed set of interfaces, later stages will build incompatible assumptions into Raft, storage, and networking, causing expensive rework exactly at the point (weeks 4–8) the roadmap's own risk table (§44) identifies as highest-risk. Stage 1 is insurance against that.

**Importance:** CRITICAL — this is the only stage where a mistake is nearly free to fix (nothing depends on it yet) and where a mistake left uncaught becomes exponentially more expensive per stage that builds on it.

**Ownership overview for this stage:**
- **Person 1 (Raft):** Co-authors the `Transport` and `Storage` interface signatures (from the *consumer* side — what Raft core needs to call), and owns the ADR confirming the "Learner is a membership attribute, not a fourth Raft state" design decision (§13 assumption) before anyone builds against it.
- **Person 2 (Storage/Durability/Membership):** Owns the `Storage` interface definition (append/read/compact contract per §20–21) and the WAL record-format constants that Stage 5 will implement against.
- **Person 3 (Networking/Client API/Tooling):** Owns Protobuf schema, gRPC service skeleton, HTTP API route skeleton, and `raftctl` CLI skeleton (separate binary per §11).
- **Person 4 (Reliability/DevOps/Testing):** Owns repo scaffolding, CI pipeline, Docker Compose skeleton, and the Fault Injection Hook *placeholder* (test-build-only, per §11/§34) so Stage 7 has somewhere to grow into starting as early as week 3–4.

**Parallel work unlocked by this stage:** Once Phase 1.2 is merged, Stage 2 (KV engine, Person assigned) and Stage 3 (Raft core, Person 1) begin in parallel. Phase 1.5 (Docker) and Phase 1.6 (CI) can run concurrently with Phase 1.2/1.3 since they don't depend on interface content, only on repo layout from Phase 1.1.

---

### Phase 1.1 — Repository & Tooling Bootstrap

**Objective:** Create the Go module, package skeleton, and baseline tooling so every subsequent phase has somewhere to put code.

**Value to the final system:** Nothing else can start without this; it's the physical substrate for every package boundary named in the coupling rules (§11).

**Importance:** CRITICAL
**Priority:** MUST HAVE

**Inputs / Dependencies:** SDD §11 (component architecture), §12 (module responsibilities).

**Components involved:** Repo root, `go.mod`, `internal/raft`, `internal/storage`, `internal/transport`, `internal/state`, `internal/api`, `internal/membership`, `internal/observability`, `cmd/raftkv`, `cmd/raftctl`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S1.1-01 | Initialize Go module, set Go version, add `.gitignore`, `LICENSE`, root `README.md` stub | Person 4 | MUST | — | Initialized repo |
| S1.1-02 | Create empty package skeletons for `internal/raft`, `internal/storage`, `internal/transport`, `internal/state`, `internal/api`, `internal/membership`, `internal/observability`, each with a `doc.go` stating its responsibility per §12 | Person 4 | MUST | S1.1-01 | Package skeleton committed |
| S1.1-03 | Create `cmd/raftkv/main.go` and `cmd/raftctl/main.go` as separate binary entry points (empty `main()` for now) per §11's "CLI raftctl separate binary" rule | Person 3 | MUST | S1.1-01 | Two buildable stub binaries |
| S1.1-04 | Set up `Makefile` or `justfile` with `build`, `test`, `vet`, `race` targets | Person 4 | SHOULD | S1.1-01 | Local dev commands |
| S1.1-05 | Add `golangci-lint` config matching team conventions | Person 4 | SHOULD | S1.1-01 | Lint config committed |

**Developer owner:** Person 4
**Supporting developers:** Person 3 (cmd skeletons)
**Parallel work:** Fully parallelizable internally; blocks nothing external until done.
**Blocked work:** All of Phase 1.2–1.7 wait on S1.1-02.
**Integration points:** None yet — this phase has no runtime behavior.
**Testing:** `go build ./...` succeeds; `go vet ./...` clean.
**Deliverables:** Buildable empty repo with correct package boundaries and two stub binaries.
**Definition of Done:** Repo compiles; package doc comments present and match §12 responsibilities; PR reviewed and merged.
**Exit Criteria:** All later Stage 1 phases can begin.
**Risks:** Low risk, but getting package boundaries wrong here silently violates the coupling rules later — worth one extra reviewer pass specifically checking package names against §11's dependency diagram.

---

### Phase 1.2 — Core Interface Contracts (Storage, Transport, StateMachine)

**Objective:** Define the three cross-cutting Go interfaces — `Storage`, `Transport`, `StateMachine` — as method signatures only, with no implementations, following the "interface sketches are Go-style signatures for design clarity — not implementation" constraint (§12).

**Value to the final system:** This is the single highest-leverage phase in the whole project. It is what makes Stage 2 (KV engine) and Stage 3 (Raft core) parallelizable, and it is what the coupling rules (§11) are actually enforcing: `internal/raft` may depend on `internal/storage` but not on `internal/transport`/`internal/api`/`internal/membership` directly — it talks to transport through an interface it defines itself (dependency inversion, §12).

**Importance:** CRITICAL
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 1.1 package skeleton; SDD §12 coupling rules; §20 storage architecture (append/read/compact); §13 Raft state model.

**Components involved:** `internal/raft` (interface *consumer* and, per dependency inversion, *definer* of the Transport interface it needs), `internal/storage`, `internal/state`, `internal/transport`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S1.2-01 | Define `StateMachine` interface in `internal/state` with `Apply(command) (result, error)`, `Snapshot() ([]byte, error)`, `Restore([]byte) error` — no method bodies | Person 1 + Person 2 (joint review) | MUST | S1.1-02 | `internal/state/interface.go` |
| S1.2-02 | Define `Storage` interface in `internal/storage` covering append, read-on-startup, and compact operations per §20's Raft↔️Storage arrows | Person 2 | MUST | S1.1-02 | `internal/storage/interface.go` |
| S1.2-03 | Define `Transport` interface **inside `internal/raft`** (dependency inversion per §12) exposing `SendAppendEntries`, `SendRequestVote`, `SendInstallSnapshot`, and a `JoinCluster` hook, matching the four RPCs called out in the system diagram (§10) | Person 1 | MUST | S1.1-02 | `internal/raft/transport.go` (interface only) |
| S1.2-04 | Define the shared `RaftNode` public surface (`Propose`, `Status`, `IsLeader`) that `internal/api` will call — this is the boundary the HTTP layer depends on | Person 1 | MUST | S1.2-03 | `internal/raft/node.go` (interface only) |
| S1.2-05 | Write an ADR confirming "Learner is a membership attribute on the standard Follower state, not a fourth Raft role" (§13 assumption) and get sign-off from all four members | Person 1 | MUST | — | `docs/adr/0001-learner-as-attribute.md` |
| S1.2-06 | Circulate interface diffs for cross-review; explicitly check each interface against the coupling table in §12.1/§11 before merge | All | MUST | S1.2-01..04 | Reviewed interface PR |

**Developer owner:** Person 1 (Transport, RaftNode), Person 2 (Storage, co-owns StateMachine)
**Supporting developers:** Person 3 (reviews Transport interface against what gRPC will need to implement in Stage 4), Person 4 (reviews for testability — can these interfaces be mocked cleanly?)
**Parallel work:** S1.2-01 and S1.2-02 can be drafted simultaneously; S1.2-03/04 depend on nothing but package skeleton.
**Blocked work:** Stage 2 cannot begin real implementation until S1.2-01 merges. Stage 3 cannot begin until S1.2-02/03/04 merge.
**Integration points:** This *is* the integration point — every future stage's boundary. No runtime integration yet.
**Testing:** No unit tests possible yet (interfaces only); verify interfaces compile and that a trivial mock implementation of each can be written and satisfies the interface (`var _ StateMachine = (*mockSM)(nil)` style compile-time assertions).
**Deliverables:** Three frozen interface files + one ADR.
**Definition of Done:** All four team members have reviewed and approved the interfaces in a synchronous session (not just async PR approval, given how much rides on this); ADR merged; compile-time mock-satisfaction checks present.
**Exit Criteria:** Interfaces are declared **frozen** for the MVP timeline — any future change requires a new ADR and a cross-team sync, not a silent PR edit. This freeze is what Stage 2/3 parallel work depends on.
**Risks:** Getting this wrong is the single most expensive mistake in the project (§44's log-replication risk assumes a stable Raft/Storage boundary). Mitigation: treat this phase's exit criteria as a hard gate, not a soft one — do not let Stage 2/3 work start against draft interfaces.

---

### Phase 1.3 — Protobuf Contracts & gRPC Service Skeleton

**Objective:** Define the `.proto` schema for `AppendEntries`, `RequestVote`, `InstallSnapshot`, and `JoinCluster` RPCs (per the system context diagram, §10), and generate Go stubs. No handler logic.

**Value to the final system:** This is the wire contract for Connection B (data flow) once Stage 4 makes replication real, and for Connection E (membership) once Stage 6 implements `JoinCluster`. Defining it now (rather than during Stage 4) lets Stage 3's Raft core be written against the *real* message shapes it will eventually send over gRPC, instead of an in-process stand-in that has to be reconciled later.

**Importance:** HIGH
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 1.2 (Transport interface signature — the RPC names should match); §10 system diagram.

**Components involved:** `proto/raftkv.proto`, generated Go stubs, `internal/transport`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S1.3-01 | Write `.proto` messages for `AppendEntriesRequest/Response`, `RequestVoteRequest/Response`, `InstallSnapshotRequest/Response`, `JoinClusterRequest/Response` | Person 3 | MUST | S1.2-03 | `proto/raftkv.proto` |
| S1.3-02 | Set up `protoc`/`buf` generation pipeline, wire into `Makefile` | Person 3 | MUST | S1.3-01 | Generated `*.pb.go` files, repeatable generation |
| S1.3-03 | Define gRPC service interface (server-side) skeleton in `internal/transport` with unimplemented handler stubs returning `Unimplemented` | Person 3 | MUST | S1.3-02 | `internal/transport/grpc_server.go` (stubs) |
| S1.3-04 | Define gRPC client wrapper skeleton implementing the `internal/raft` `Transport` interface from S1.2-03, with unimplemented method bodies (`TODO: Stage 4`) | Person 3 | SHOULD | S1.2-03, S1.3-02 | `internal/transport/grpc_client.go` (stub) |
| S1.3-05 | Cross-check every proto field against what Stage 3's Raft algorithm actually needs (e.g. `prevLogIndex`, `prevLogTerm`, `leaderCommit`) so Stage 4 doesn't need a schema change mid-implementation | Person 1 + Person 3 | MUST | S1.3-01 | Reviewed proto |

**Developer owner:** Person 3
**Supporting developers:** Person 1 (field-level review against Raft algorithm needs)
**Parallel work:** Fully parallel with Phase 1.2's Storage work and Phase 1.5 (Docker).
**Blocked work:** Stage 4 (real gRPC networking) depends on this; Stage 3 does *not* need it (Stage 3 uses an in-process transport initially, per the parallel-path note in the dependency model).
**Integration points:** Proto message shapes must match the semantics of the `Transport` interface from Phase 1.2 exactly (field-for-field), otherwise Stage 4 will need a breaking proto change.
**Testing:** `protoc`/`buf` generation succeeds cleanly in CI; a compile-time check that the generated gRPC client stub satisfies `internal/raft.Transport`.
**Deliverables:** Compilable proto schema + generated stubs + unimplemented gRPC skeleton.
**Definition of Done:** Proto reviewed by Person 1 for algorithmic completeness; generation is scripted and reproducible; stubs compile.
**Exit Criteria:** Stage 4 can start wiring real handler logic into the stubs without touching the schema.
**Risks:** MEDIUM — under-specifying a field now (e.g. forgetting `leaderCommit`) causes a schema change later that touches both Raft core and transport simultaneously. Mitigation: S1.3-05's explicit cross-check task exists specifically to catch this early.

---

### Phase 1.4 — HTTP Client API & raftctl CLI Skeletons

**Objective:** Stand up the HTTP/JSON API surface (`PUT`/`GET`/`DELETE`, cluster status/join/remove) and the `raftctl` CLI as thin wrappers over HTTP, with route handlers present but returning stub/`501` responses.

**Value to the final system:** Establishes the client-facing contract (§10: "Clients never talk directly to followers for writes... see §28") before any handler logic exists, so Stage 4's leader-redirect logic and Stage 6's join/remove logic have a stable route surface to fill in.

**Importance:** HIGH
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 1.2 (`RaftNode` interface, S1.2-04); §10 system context diagram.

**Components involved:** `internal/api` (HTTP handlers), `cmd/raftctl`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S1.4-01 | Define HTTP route table: `PUT/GET/DELETE /kv/{key}`, `GET /cluster/status`, `POST /cluster/join`, `POST /cluster/remove` | Person 3 | MUST | S1.2-04 | `internal/api/routes.go` |
| S1.4-02 | Implement handler skeletons that parse requests/responses per a defined JSON schema, but call into stub/`RaftNode` interface returning `not implemented` | Person 3 | MUST | S1.4-01 | Compilable handler stubs |
| S1.4-03 | Build `raftctl` command skeleton (`raftctl status`, `raftctl join`, `raftctl remove`) wrapping the HTTP routes above | Person 3 | MUST | S1.4-01 | `cmd/raftctl` stub commands |
| S1.4-04 | Draft OpenAPI/JSON schema doc for the HTTP API so Stage 4/6 implement against an agreed contract | Person 3 | SHOULD | S1.4-01 | `docs/api.md` |
| S1.4-05 | Note the §"cheap hardening" bearer-token option as an explicit **out-of-MVP-scope-for-now** decision in an ADR, so it isn't silently forgotten nor silently added early | Person 3 | NICE TO HAVE | — | ADR note |

**Developer owner:** Person 3
**Supporting developers:** Person 1 (confirms `RaftNode` interface is sufficient for handlers to call)
**Parallel work:** Fully parallel with Phase 1.3 (same owner, but no hard dependency between them) and Phase 1.5/1.6.
**Blocked work:** Stage 4's leader-redirect logic and Stage 6's join/promote handlers need these routes to exist first.
**Integration points:** Route handlers call `internal/raft.RaftNode` (from Phase 1.2) — this is the Connection B entry point.
**Testing:** HTTP route tests confirming correct status codes for stubbed responses; request/response JSON schema validated against `docs/api.md`.
**Deliverables:** Stubbed but structurally complete HTTP API and CLI.
**Definition of Done:** Routes match §10's diagram; `docs/api.md` merged; stub handlers return sensible placeholder responses (not panics).
**Exit Criteria:** Stage 4 can implement real handler bodies without changing route shapes.
**Risks:** LOW — mostly a naming/consistency risk if routes diverge from what raftctl expects; mitigated by S1.4-03 building both sides together.

---

### Phase 1.5 — Docker Compose & Multi-Node Bootstrap Skeleton

**Objective:** Create the `docker-compose.yml` and per-node Dockerfile that can bring up an N-node cluster skeleton (processes that start, bind ports, and exit cleanly), per the deployment architecture (§33).

**Value to the final system:** This is the platform Stage 4 (real distributed system), Stage 6 (dynamic join against a live compose cluster), and Stage 8 (final integration) all run on. Standing it up empty now means later stages are adding logic to a working deployment, not debugging Docker *and* Raft simultaneously.

**Importance:** HIGH
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 1.1 (buildable `cmd/raftkv` binary); §33 deployment diagram (gRPC:900x, HTTP:800x, named volume `/data` per node).

**Components involved:** `Dockerfile`, `docker-compose.yml`, `scripts/`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S1.5-01 | Write multi-stage `Dockerfile` building `cmd/raftkv` | Person 4 | MUST | S1.1-03 | `Dockerfile` |
| S1.5-02 | Write `docker-compose.yml` for a configurable N-node cluster (3–5 nodes), each with its own named volume (`/data`), gRPC port `900x`, HTTP port `800x` per §33 | Person 4 | MUST | S1.5-01 | `docker-compose.yml` |
| S1.5-03 | Add node bootstrap flags/env vars (node ID, peer list, data dir) to `cmd/raftkv/main.go` stub so compose can parameterize nodes | Person 4 | MUST | S1.5-02 | CLI flags wired |
| S1.5-04 | Write a `scripts/` helper to dynamically add a 4th/5th node to an already-running compose cluster (stub for now; real join logic lands in Stage 6) per §33's SHOULD | Person 4 | SHOULD | S1.5-02 | `scripts/add-node.sh` skeleton |
| S1.5-05 | Verify `docker compose up` brings up N containers that start and stay running (even with no real Raft logic yet) | Person 4 | MUST | S1.5-02, S1.5-03 | Verified local run |

**Developer owner:** Person 4
**Supporting developers:** Person 3 (port/flag conventions matching HTTP API from Phase 1.4)
**Parallel work:** Fully parallel with Phases 1.2–1.4 (only depends on Phase 1.1).
**Blocked work:** Stage 4's real Docker-networked integration tests and Stage 7's Docker-based fault injection depend on this being solid early.
**Integration points:** Compose env vars ↔️ `cmd/raftkv` flags ↔️ eventual `internal/raft` node configuration (Stage 3).
**Testing:** `docker compose up` / `docker compose down` cycle tested locally and in CI if feasible; container logs show clean startup with no crash loops.
**Deliverables:** Working empty-cluster Docker Compose setup.
**Definition of Done:** N-node cluster (N configurable) starts cleanly, each node has isolated storage, ports don't collide.
**Exit Criteria:** Stage 4/6/7/8 can build directly on this deployment shape without re-architecting it.
**Risks:** LOW at this stage since there's no real logic to fail; risk shifts to Stage 4 when nodes actually need to reach each other over the compose network.

---

### Phase 1.6 — CI Pipeline & Quality Gates

**Objective:** Stand up CI running `go build`, `go vet`, `go test ./...`, and `go test -race ./...` on every PR, matching the MVP requirement (§42: "CI running build/vet/test/race on every PR").

**Value to the final system:** Directly implements one of the roadmap's explicit MVP checklist items and is the mechanism that catches concurrency bugs early — the risk table (§44) specifically calls out that distributed-systems bugs are "slower to debug than typical coursework bugs," making early, continuous race detection disproportionately valuable.

**Importance:** CRITICAL
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 1.1 (Makefile targets).

**Components involved:** CI config (GitHub Actions or equivalent).

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S1.6-01 | Set up CI workflow file running build/vet/test/race on every PR and push to main | Person 4 | MUST | S1.1-04 | `.github/workflows/ci.yml` |
| S1.6-02 | Add branch protection requiring CI pass + review approval before merge (two approvals for `internal/raft`/`internal/membership` per developer model) | Person 4 | MUST | S1.6-01 | Branch protection configured |
| S1.6-03 | Add `docker compose up`/build smoke test to CI if runner supports it | Person 4 | SHOULD | S1.5-02, S1.6-01 | CI Docker smoke test |
| S1.6-04 | Add lint step (`golangci-lint`) to CI | Person 4 | SHOULD | S1.1-05, S1.6-01 | Lint gate in CI |

**Developer owner:** Person 4
**Supporting developers:** None required; low-coordination task.
**Parallel work:** Fully parallel with all other Stage 1 phases.
**Blocked work:** Nothing blocks on this directly, but every future PR benefits from it existing before Stage 2/3 work lands.
**Integration points:** None (infra-only).
**Testing:** CI itself is the testing infrastructure; validate it by deliberately introducing a failing test/race in a throwaway branch and confirming CI catches it.
**Deliverables:** Working CI pipeline enforcing the MVP's quality bar from day one.
**Definition of Done:** CI runs on a real PR and correctly passes/fails; branch protection active.
**Exit Criteria:** All subsequent PRs (Stage 2 onward) are gated by this pipeline.
**Risks:** LOW — mostly a "don't skip this" risk; the risk table (§44) explicitly flags students under-investing in tests, and a CI gate is the cheapest structural defense against that.

---

### Phase 1.7 — SDD Ratification, ADRs, Diagrams & Coupling Enforcement

**Objective:** Formally ratify the SDD as the team's binding reference, establish the ADR process (incremental, per-PR habit per §37, not batched at the end), and set up the `/docs/uml` location for the diagrams flagged as still-pending (§46: election, replication, partition, crash-recovery sequence diagrams).

**Value to the final system:** Prevents the two documentation-related risks the roadmap itself names: "underestimating documentation/report-writing time" and scope creep toward "make it more like etcd" (§44) — both mitigated by having a living, referenced document from week 1 rather than a retrospective one.

**Importance:** MEDIUM
**Priority:** MUST HAVE

**Inputs / Dependencies:** None technical; organizational.

**Components involved:** `docs/adr/`, `docs/uml/`, root SDD.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S1.7-01 | Ratify SDD in a team session; record any amendments as ADRs, not silent edits | All | MUST | — | Ratified SDD + amendment ADRs |
| S1.7-02 | Set up `docs/adr/` with a template (context/decision/consequences) | Person 4 | MUST | — | ADR template |
| S1.7-03 | Set up `docs/uml/` and schedule the four pending sequence diagrams (election/replication for weeks 3–4, partition/recovery for week 9) as tracked follow-up tasks, not forgotten action items | Person 4 | SHOULD | — | Tracked backlog items |
| S1.7-04 | Re-read and post the explicit OUT_OF_SCOPE list (§4/§5) somewhere visible (e.g. pinned issue) as a standing scope-creep check per the risk mitigation in §44 | Person 4 | SHOULD | — | Pinned scope reference |

**Developer owner:** Person 4 (process), All (content)
**Supporting developers:** All four members equally.
**Parallel work:** Fully parallel with all technical phases.
**Blocked work:** Nothing technical; this is a governance phase.
**Integration points:** None.
**Testing:** N/A (process phase).
**Deliverables:** Ratified SDD, ADR process live, scope-creep guardrail visible.
**Definition of Done:** Team has explicitly agreed the SDD is authoritative; ADR-0001 (learner-as-attribute, from Phase 1.2) is the first entry proving the process works.
**Exit Criteria:** Every future design decision across Stage 2 onward is expected to produce an ADR if it changes or clarifies the SDD.
**Risks:** LOW technically, MEDIUM organizationally if skipped — the roadmap explicitly names this as a recurring failure mode for student teams.

---

## Stage 1 Exit Summary

Stage 1 is complete when: repo builds (1.1), the three core interfaces are frozen and reviewed (1.2), proto/gRPC skeleton compiles (1.3), HTTP/raftctl skeleton compiles (1.4), Docker Compose brings up an empty N-node cluster (1.5), CI enforces build/vet/test/race on every PR (1.6), and the SDD/ADR process is live (1.7).

**Hard gate:** Stage 2 and Stage 3 implementation work must not begin until **Phase 1.2 is merged and frozen**. This is the one dependency in this stage that is not soft.

---
---

# STAGE 2 — Standalone Key-Value Engine

## Stage Overview

**Objective:** Implement `internal/state` as a real, deterministic, in-memory key-value engine satisfying the `StateMachine` interface frozen in Phase 1.2 — `PUT`/`GET`/`DELETE`, `Apply(command)`, `Snapshot()`, `Restore()` — with **zero dependency on Raft, transport, or networking** (§12 coupling rule).

**Position in dependency graph:** Runs in parallel with Stage 3 (Raft Core). Both consume Phase 1.2's interfaces independently; neither blocks the other. Stage 2's output (`internal/state`) becomes a direct dependency of Stage 4 (client integration — real commands flowing through Raft into this engine) and Stage 5 (its `Snapshot()`/`Restore()` are exactly what gets persisted to and read from disk).

**Value to final system:** This is the actual "database" in the key-value store — everything else (Raft, networking, durability, membership) exists to get commands to this component correctly, in order, and durably. It is also the first real proof that the coupling rules from §11 are enforceable in practice, not just on a diagram.

**Importance:** HIGH — not on the critical path the way Raft core is (§44 identifies Raft as the hardest, highest-risk chunk), but it is a hard **prerequisite** for Stage 4's end-to-end data flow and for validating Stage 5's snapshot/restore correctness later.

**Connections engaged:**
- **Connection B (Data Flow):** Implements the final hop — `StateMachine` — of client→Raft→log→replication→commit→StateMachine→response.
- **Connection D (Snapshots):** Implements the producer/consumer of `Snapshot()`/`Restore()`, which Stage 5 will wrap in file I/O and Stage 6 will stream to catching-up learners.

**Ownership overview for this stage:**
- **Primary owner:** Person 2 (Storage/Durability/Membership) — natural fit since this person also owns `Storage` and will own snapshot file I/O in Stage 5; building the in-memory snapshot format here directly informs that later work.
- **Supporting:** Person 1 reviews `Apply(command)` semantics for exact alignment with what Raft log entries will contain (command encoding, idempotency expectations); Person 4 builds out the unit/property test harness pattern this stage establishes, since it will be reused by Stage 3 and beyond.
- **Person 3:** Not directly involved in this stage's core work; available for a later Stage 4 integration hook design discussion.

**Parallel work:** Entirely parallel with Stage 3 (Raft Core). Internally, Phase 2.1 (CRUD) and initial test scaffolding can start immediately after Phase 1.2 merges; Phase 2.3 (snapshot format) can be drafted concurrently with 2.1 since it only needs to know the internal data structure shape, which can be pinned early via a short design note.

---

### Phase 2.1 — KV Store Core Data Structure & CRUD Operations

**Objective:** Implement the underlying in-memory key-value data structure and `Put`/`Get`/`Delete` operations, fully isolated from any Raft/networking concept — these are plain Go methods operating on a map (or similar), not yet wired to `Apply()`.

**Value to the final system:** This is the literal "store" in key-value store. Every other RaftKV feature exists to get data into and out of this structure correctly.

**Importance:** HIGH
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 1.2 (`StateMachine` interface).

**Components involved:** `internal/state`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S2.1-01 | Implement internal map-backed store with `put(key, value)`, `get(key) (value, found)`, `delete(key)` methods, guarded by a mutex for concurrent-safe access | Person 2 | MUST | S1.2-01 | `internal/state/store.go` |
| S2.1-02 | Define the command encoding this store will `Apply()` — a small struct/enum for `{OpType: PUT/GET/DELETE, Key, Value}` that mirrors what Raft log entries will eventually carry | Person 2 + Person 1 (review) | MUST | S1.2-01 | `internal/state/command.go` |
| S2.1-03 | Ensure `GET` is handled as a **read-only, non-mutating** path distinct from `PUT`/`DELETE`, since read semantics may differ from write semantics once Raft leader-lease/read-index questions arise later (§36 consistency caveat, out of scope now but don't foreclose it) | Person 2 | SHOULD | S2.1-01 | Read-path isolated in code |
| S2.1-04 | Add basic input validation (empty key handling, max value size if any) matching whatever the HTTP API in Phase 1.4 expects to pass through | Person 2 | SHOULD | S1.4-01 | Validation logic |

**Developer owner:** Person 2
**Supporting developers:** Person 1 (command shape review)
**Parallel work:** Fully parallel with Stage 3's early work (election/term logic) — no shared files.
**Blocked work:** Phase 2.2 (`Apply()` wiring) depends on S2.1-01/02.
**Integration points:** `command.go`'s shape must be something Stage 3's Raft log entries can carry as an opaque `[]byte` payload without Raft needing to understand its contents (coupling rule: Raft doesn't know about KV semantics).
**Testing:** Unit tests for `put`/`get`/`delete` covering: basic set/get, overwrite, delete-then-get, get-on-missing-key, concurrent access via `go test -race`.
**Deliverables:** Working, concurrency-safe, isolated CRUD core.
**Definition of Done:** Unit tests pass including `-race`; no import of `internal/raft` or `internal/transport` anywhere in `internal/state` (enforced by code review, per §11).
**Exit Criteria:** Phase 2.2 can wire this into the `Apply()` interface method.
**Risks:** LOW. Main risk is scope creep toward features like TTL/CAS explicitly marked out-of-MVP (§43) — reviewers should reject anything beyond PUT/GET/DELETE here.

---

### Phase 2.2 — `Apply(command)` Interface Implementation & Determinism Guarantees

**Objective:** Wire the CRUD core from Phase 2.1 behind the `StateMachine.Apply(command)` method frozen in Phase 1.2, and explicitly verify determinism: applying the same sequence of commands on any node must produce identical state.

**Value to the final system:** This is the exact method Stage 3/4's committed log entries will call. Determinism here is a **Raft safety precondition** — if `Apply()` isn't deterministic, replicated state will diverge across nodes even with perfectly correct consensus, silently breaking the whole system's core guarantee.

**Importance:** CRITICAL
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 2.1 (CRUD core); Phase 1.2 (`StateMachine` interface).

**Components involved:** `internal/state`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S2.2-01 | Implement `Apply(command) (result, error)` dispatching to `put`/`get`/`delete` based on `command.OpType` | Person 2 | MUST | S2.1-01, S2.1-02 | `internal/state/apply.go` |
| S2.2-02 | Ensure `Apply()` has no dependency on wall-clock time, randomness, map-iteration order, or any other non-deterministic Go behavior | Person 2 | MUST | S2.2-01 | Deterministic implementation |
| S2.2-03 | Define and document what `Apply()` returns for each op type (e.g. previous value on delete, found/not-found on get) so Stage 4's HTTP handlers know what to expect back through the pipeline | Person 2 | MUST | S2.2-01 | Documented return contract |
| S2.2-04 | Add a compile-time assertion (`var _ state.StateMachine = (*Store)(nil)`) proving the concrete type satisfies the Stage 1 interface exactly | Person 2 | MUST | S2.2-01 | Compile-time check |

**Developer owner:** Person 2
**Supporting developers:** Person 1 (signs off specifically on determinism — this is a Raft safety concern even though it's implemented in Stage 2)
**Parallel work:** Independent of Stage 3's concurrent work.
**Blocked work:** Stage 4's end-to-end write path cannot be demonstrated without this.
**Integration points:** This method signature is exactly what Stage 3's future "apply committed entries to state machine" loop will call — no changes should be needed here when Stage 4 wires them together.
<br>
**Testing:** Property-based test: generate random sequences of PUT/GET/DELETE commands, apply the same sequence twice (fresh instances), assert identical resulting state. Unit tests for each op type's return contract.
**Deliverables:** Working, deterministic `Apply()` implementation satisfying `StateMachine`.
**Definition of Done:** Determinism property test passes across many random seeds; compile-time interface assertion present; Person 1 has explicitly reviewed and signed off on determinism.
**Exit Criteria:** Stage 4 can call `Apply()` from the committed-entries loop without any further changes to this method's contract.
**Risks:** MEDIUM — subtle non-determinism (e.g. accidentally relying on Go map iteration order somewhere) is easy to introduce and easy to miss without a dedicated property test; mitigated by S2.2-02 being a named task and by the property-based test in this phase's testing plan.

---

### Phase 2.3 — Snapshot & Restore Implementation

**Objective:** Implement `Snapshot() ([]byte, error)` and `Restore([]byte) error`, producing and consuming a complete, serialized representation of the KV store's state.

**Value to the final system:** This is the exact mechanism Stage 5 will wrap in file I/O (writing `Snapshot()`'s output to a snapshot file per §20) and Stage 6 will stream to a learner that's too far behind to catch up via log replay alone (Connection D). Getting the serialization format right and stable now avoids a breaking format change once Stage 5/6 depend on it.

**Importance:** HIGH
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phase 2.1/2.2 (working store); Phase 1.2 interface.

**Components involved:** `internal/state`.

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S2.3-01 | Choose and document a serialization format for the full KV map (e.g. length-prefixed key/value pairs or JSON — pick the simpler one; this is an MVP, not a performance project per §5 scope) | Person 2 | MUST | S2.1-01 | Documented format (ADR if non-trivial) |
| S2.3-02 | Implement `Snapshot()` producing a complete, self-contained byte representation of current state | Person 2 | MUST | S2.3-01 | `internal/state/snapshot.go` |
| S2.3-03 | Implement `Restore([]byte)` fully replacing current state with the deserialized snapshot contents (not merging) | Person 2 | MUST | S2.3-01 | Restore logic |
| S2.3-04 | Verify round-trip correctness: `Restore(Snapshot())` on a fresh instance produces state identical to the original | Person 2 | MUST | S2.3-02, S2.3-03 | Round-trip test |
| S2.3-05 | Document that this snapshot format is **KV-engine-internal only** — Stage 5 will layer `lastIncludedIndex`/`lastIncludedTerm` metadata *around* this payload, not inside it (per §20's MUST on snapshot metadata), keeping the coupling boundary clean | Person 2 + Person 1 (review) | MUST | S2.3-02 | Documented boundary in code comments/ADR |

**Developer owner:** Person 2
**Supporting developers:** Person 1 (confirms the Raft-facing metadata boundary is respected — Raft/Storage owns `lastIncludedIndex`/`lastIncludedTerm`, not `internal/state`)
**Parallel work:** Can be drafted concurrently with Phase 2.1/2.2 once the format is agreed, since it mainly needs to know the internal map shape.
**Blocked work:** Stage 5 (durable snapshot files) and Stage 6 (learner catch-up via `InstallSnapshot`) both depend on this being stable.
**Integration points:** Output of `Snapshot()` will later be wrapped by Stage 5's storage layer with the `lastIncludedIndex`/`lastIncludedTerm` header (§20 MUST) — `internal/state` must remain unaware of that wrapping.
**Testing:** Round-trip unit test (snapshot → restore → compare); test restoring into a non-empty store correctly *replaces* rather than merges state; test snapshotting an empty store.
**Deliverables:** Working, round-trip-verified `Snapshot()`/`Restore()`.
**Definition of Done:** Round-trip test passes for empty, small, and large (many-key) stores; format documented; boundary with future Raft-side metadata explicitly noted in code/ADR.
**Exit Criteria:** Stage 5 can wrap this payload in a file format without needing changes to `internal/state`.
**Risks:** MEDIUM — if the format isn't self-describing/versioned at all, a future change becomes a breaking one for Stage 5/6. Mitigation: even for an MVP, include a simple format-version byte/field so this doesn't become a silent landmine later (flag as a SHOULD if not already planned).

---

### Phase 2.4 — Isolation Enforcement & Determinism/Property Test Suite

**Objective:** Build out the full unit and property-based test suite for `internal/state`, and mechanically verify the package's isolation from Raft/transport/networking holds — not just by convention, but by a checkable rule.

**Value to the final system:** Directly implements the coupling rule (§11) that `internal/state` "has no knowledge of Raft, transport, or networking — it only implements an `Apply(command) result` interface." This phase is what makes that rule enforceable rather than aspirational, and it establishes the property-based testing pattern that Stage 3 (Raft correctness) will lean on heavily later.

**Importance:** HIGH
**Priority:** MUST HAVE

**Inputs / Dependencies:** Phases 2.1–2.3 (implementation to test).

**Components involved:** `internal/state`, CI (Phase 1.6).

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S2.4-01 | Write comprehensive unit tests for `Put`/`Get`/`Delete`/`Apply` covering edge cases (missing keys, empty values, repeated deletes) | Person 2 | MUST | S2.1, S2.2 | Unit test suite |
| S2.4-02 | Write property-based tests (e.g. using `gopter` or hand-rolled random command generation) asserting: determinism across replay, snapshot/restore round-trip equivalence, and that `Apply()` never panics on any valid command | Person 4 (harness pattern) + Person 2 (domain assertions) | MUST | S2.2, S2.3 | Property test suite |
| S2.4-03 | Add a static-analysis or code-review checklist item verifying `internal/state` has zero imports from `internal/raft`, `internal/transport`, `internal/api`, `internal/membership` (a simple `go list -deps` check can be scripted) | Person 4 | MUST | S1.1-02 | Import-boundary check (script or CI step) |
| S2.4-04 | Add `-race` coverage specifically for concurrent `Put`/`Get`/`Delete` access from multiple goroutines, since Raft will eventually call `Apply()` from its own goroutine while HTTP reads may occur concurrently later | Person 2 | MUST | S2.1-01 | Race-tested concurrency |
| S2.4-05 | Wire this test suite into CI (Phase 1.6) so it runs on every PR touching `internal/state` | Person 4 | MUST | S1.6-01 | CI coverage confirmed |

**Developer owner:** Person 2 (test content), Person 4 (test infrastructure/CI wiring)
**Supporting developers:** None further needed.
**Parallel work:** Can run alongside Phase 2.3 as tests are written incrementally per feature, per the project's stated Definition of Done habit (§45: tests alongside implementation, not deferred).
**Blocked work:** Nothing blocks on this beyond general CI confidence, but skipping it directly contradicts the roadmap's explicit anti-pattern warning (§44: "students under-invest in tests... happy path works and failure paths are silently broken").
**Integration points:** This test suite becomes the regression safety net Stage 4 relies on when it starts calling `Apply()` from real committed Raft entries.
**Testing:** (This phase *is* the testing phase — see tasks above.)
**Deliverables:** Full unit + property test suite, import-boundary check, CI integration.
**Definition of Done:** All tests green including `-race`; import-boundary check passes with zero violations; CI runs this suite automatically.
**Exit Criteria:** `internal/state` is considered feature-complete and trustworthy enough for Stage 4 to build the real end-to-end write path against it.
**Risks:** LOW technically; the real risk is a schedule risk — skipping thoroughness here to "move faster" is exactly the trap the roadmap's risk table warns about (§44).

---

### Phase 2.5 — Standalone Integration Stub (No-Raft Test Harness)

**Objective:** Build a small, throwaway in-process harness that exercises `internal/state` through the exact `StateMachine` interface shape (not the concrete type) — simulating "commands arriving and being applied" without any real Raft — to prove the interface boundary from Phase 1.2 works end-to-end before Stage 3/4 exist to prove it for real.

**Value to the final system:** This is the first live validation of Connection A (interfaces) and Connection B (data flow) for the `StateMachine` leg specifically, ahead of Stage 3/4. It gives early, concrete confidence that the interface frozen in Phase 1.2 was well-designed, while there's still time to raise concerns via an ADR rather than after Stage 4 has been built on top of it.

**Importance:** MEDIUM
**Priority:** SHOULD HAVE

**Inputs / Dependencies:** Phases 2.1–2.4.

**Components involved:** `internal/state`, a temporary test-only harness (not shipped in `cmd/raftkv`).

**Implementation tasks:**

| ID | Task | Owner | Priority | Dependencies | Deliverable |
|----|------|-------|----------|---------------|-------------|
| S2.5-01 | Write a small test-only driver that holds a `state.StateMachine` (typed as the interface, not the concrete struct) and feeds it a scripted sequence of commands, printing/asserting results | Person 2 | SHOULD | S2.2-01 | `internal/state/harness_test.go` or similar |
| S2.5-02 | Confirm the harness only ever references the interface type, never the concrete `Store` type, as a live check that Stage 3/4 will be able to swap in a real Raft-driven caller with no code changes to `internal/state` | Person 1 (review) | SHOULD | S2.5-01 | Interface-only usage confirmed |
| S2.5-03 | Capture any friction found while doing this (e.g. missing context in `Apply()`'s signature) as feedback into an ADR amendment to Phase 1.2's frozen interface, if truly necessary — otherwise defer | Person 2 + Person 1 | NICE TO HAVE | S2.5-01 | ADR amendment or "no changes needed" note |

**Developer owner:** Person 2
**Supporting developers:** Person 1 (interface-fidelity review)
**Parallel work:** Independent of Stage 3.
**Blocked work:** Nothing depends on this directly; it's a confidence-building phase, not a critical-path one.
**Integration points:** Same interface boundary Stage 4 will use for real.
**Testing:** The harness itself is a test; no further tests needed beyond what Phase 2.4 already covers.
**Deliverables:** A small internal proof that the `StateMachine` interface is implementable and drivable exactly as designed.
**Definition of Done:** Harness runs a scripted command sequence successfully against the real `Store` via the interface type; any interface friction is either resolved via ADR amendment or explicitly closed as "not needed."
**Exit Criteria:** Stage 2 is considered fully done; Stage 4 can proceed with full confidence in the `StateMachine` boundary.
**Risks:** LOW — this phase exists specifically to *reduce* risk elsewhere by surfacing interface problems early and cheaply.

---

## Stage 2 Exit Summary

Stage 2 is complete when: CRUD core works and is concurrency-safe (2.1), `Apply()` is implemented, deterministic, and interface-verified (2.2), `Snapshot()`/`Restore()` round-trip correctly with a documented, stable format (2.3), the full unit/property/isolation test suite is green in CI (2.4), and a standalone harness has proven the interface boundary works end-to-end (2.5).

**Hard gate:** Stage 5 (durable storage) cannot begin wrapping `Snapshot()`/`Restore()` in file I/O until Phase 2.3's format is documented and frozen. Stage 4 cannot demonstrate a real write path until Phase 2.2 is done.

---

# Combined Stage 1 → Stage 2 Handoff

| Contract frozen in Stage 1 | Consumed starting in Stage 2 | Also required later by |
|---|---|---|
| `StateMachine` interface (Phase 1.2) | Phase 2.2 (`Apply()` implementation) | Stage 4 (commit→apply loop) |
| Snapshot boundary assumption (Phase 1.2 ADR-0001 context) | Phase 2.3 (`Snapshot`/`Restore`) | Stage 5 (file wrapping), Stage 6 (learner catch-up) |
| CI quality gates (Phase 1.6) | Every Stage 2 PR | All future stages |
| Package coupling rules (Phase 1.1/1.2) | Phase 2.4 import-boundary check | All future stages, enforced via code review |

**What must remain stable for Stage 3 (Raft Core) to proceed safely in parallel:** The `StateMachine.Apply()` signature and the `Transport`/`RaftNode` interfaces from Phase 1.2. Stage 2's internal implementation details (map structure, serialization format) are free to evolve without affecting Stage 3, since Stage 3 only ever sees Stage 2 through the frozen interface — which is precisely the point of Phase 1.2's design.