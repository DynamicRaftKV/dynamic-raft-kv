# Issue audit — 2026-11-05

GitHub inventory: **200 open issues, 0 closed issues** (PRs excluded). Open does
not mean unimplemented. Many original tickets retain outdated interface details.
No tickets were automatically closed by this audit. Task definitions include
reviewed/merged PR requirements; passing local tests does not satisfy that gate.
PR #210 remains draft and open. Keep release acceptance separate from code coverage.

## Closure candidates after review evidence

State-machine CRUD, deterministic Apply and snapshot/restore are implemented
(phase parents #6, #7, #194, #195; tasks #64–84 need individual acceptance checks).
Raft, transport, storage and membership have substantial implementation and tests;
use the roadmap and linked tests to review each criterion rather than closing
entire phases because a frontend demonstration passes.

## Refine stale acceptance criteria

| Issues | Required clarification |
|---|---|
| #99 | Match the actual proposal result/error contract. |
| #111, #114 | GET goes through the log; follower redirection uses HTTP 307. |
| #123, #124 | Distinguish incomplete final-frame repair from fatal corruption; update storage interface names. |
| #132, #189 | Chunked snapshot transfer supersedes the original single-message scope. |
| #138 | Follow ADR 0008: distinguish durable active configuration from committed applied configuration. |
| #145 | Promotion uses the implemented catch-up gate; do not imply an unimplemented configurable threshold. |
| #161 | Assert no commitment without quorum; timeout/overload/unavailable outcomes are not uniformly HTTP 503. |
| #176 | Replace stale-read and single-message limitations with current limitations. |
| #185 | Reconcile the old self-removal rejection task with supported safe leader removal. |

These are proposed ticket refinements, not silent changes to requirements.

## Work still requiring implementation or acceptance

- Critical: independent review of consensus, recovery and membership changes;
  complete PR #210 acceptance and verify CI on the final commit before merging.
- #150–151: strengthen explicit concurrent-write joining and learner-failure
  scenarios; existing catch-up tests do not automatically cover every schedule.
- #156–157: reconcile the planned scenario DSL with the current deterministic
  harness, and validate the harness itself before claiming that requirement done.
- #166: nightly stress scheduling remains separate from ordinary pull-request CI.
- #168: review structured logging coverage, including explicit commit-advance events.
- #174–175: finish sequence-diagram and ADR-index deliverables.
- #178–182: presentation, video, release/tag, retrospective and project archive
  require final delivery; do not equate an interactive demo with those artifacts.
- #57: verify branch protection/review settings independently.
- Optional scope (#183, #187, #188): ReadIndex, join authentication and additional
  histograms should not block the agreed core unless explicitly promoted in scope.

## Full inventory (GitHub state at audit time)

| Issue | Title | State |
|---|---|---|
| [#2](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/2) | [Phase 1.1] Repository & Tooling Bootstrap | open |
| [#3](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/3) | [Phase 1.2] Core Interface Contracts (Storage, Transport, StateMachine) | open |
| [#4](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/4) | [Phase 1.3] Protobuf Contracts & gRPC Service Skeleton | open |
| [#5](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/5) | [Phase 1.7] SDD Ratification, ADRs, Diagrams & Scope Guardrail | open |
| [#6](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/6) | [Phase 2.2] Apply(command) Implementation & Determinism | open |
| [#7](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/7) | [Phase 2.3] Snapshot & Restore | open |
| [#8](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/8) | [Phase 3.1] Raft Node Skeleton, State & Timers | open |
| [#9](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/9) | [Phase 3.2] Leader Election | open |
| [#10](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/10) | [Phase 3.3] Log Replication & Commit | open |
| [#11](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/11) | [Phase 3.4] In-Process Transport & Raft Test Cluster | open |
| [#12](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/12) | [Phase 4.1] Real gRPC Transport | open |
| [#13](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/13) | [Phase 4.2] Real HTTP API | open |
| [#14](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/14) | [Phase 4.4] Multi-Node End-to-End on Docker Compose | open |
| [#15](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/15) | [Phase 5.1] Write-Ahead Log | open |
| [#16](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/16) | [Phase 5.2] Raft and Storage Integration | open |
| [#17](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/17) | [Phase 5.3] Snapshots & Log Compaction | open |
| [#18](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/18) | [Phase 5.4] Crash Recovery Validation | open |
| [#19](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/19) | [Phase 6.1] Config-Change Entries & Durable Membership | open |
| [#20](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/20) | [Phase 6.2] Join Protocol | open |
| [#21](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/21) | [Phase 6.3] Learner Catch-Up & Promotion | open |
| [#22](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/22) | [Phase 6.4] Membership Test Suite | open |
| [#23](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/23) | [Phase 7.1] Fault Injection Harness | open |
| [#24](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/24) | [Phase 7.2] Failure Scenario Suite | open |
| [#25](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/25) | [Phase 7.3] Correctness Properties & Race-Clean Build | open |
| [#26](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/26) | [Phase 8.1] End-to-End Integration & Demo | open |
| [#27](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/27) | [Phase 8.3] Evaluation Deliverables | open |
| [#28](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/28) | [Phase 9.1] Read-Index Linearizable Reads | open |
| [#29](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/29) | [Phase 9.2] Safe Node Removal | open |
| [#30](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/30) | [S1.1-01] Initialize Go module, .gitignore, LICENSE, README stub | open |
| [#31](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/31) | [S1.1-02] Create internal/* package skeletons with doc.go per SDD 12 | open |
| [#32](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/32) | [S1.1-03] Create cmd/raftkv and cmd/raftctl entry points | open |
| [#33](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/33) | [S1.1-04] Makefile with build, test, vet and race targets | open |
| [#34](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/34) | [S1.1-05] Add golangci-lint configuration | open |
| [#35](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/35) | [S1.2-01] Define StateMachine interface (Apply, Snapshot, Restore) | open |
| [#36](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/36) | [S1.2-02] Define Storage interface (append, read-on-startup, compact) | open |
| [#37](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/37) | [S1.2-03] Define Transport interface inside internal/raft (dependency inversion) | open |
| [#38](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/38) | [S1.2-04] Define RaftNode public surface (Propose, Status, IsLeader) | open |
| [#39](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/39) | [S1.2-05] ADR-0001: Learner is a membership attribute, not a fourth Raft state | open |
| [#40](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/40) | [S1.2-06] Synchronous interface review against coupling rules; declare interfaces FROZEN | open |
| [#41](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/41) | [S1.3-01] Write .proto messages for the four RPC pairs | open |
| [#42](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/42) | [S1.3-02] Set up protoc/buf generation pipeline in Makefile | open |
| [#43](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/43) | [S1.3-03] gRPC server skeleton returning Unimplemented | open |
| [#44](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/44) | [S1.3-04] gRPC client wrapper skeleton implementing raft.Transport | open |
| [#45](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/45) | [S1.3-05] Cross-check proto fields against Raft algorithm needs | open |
| [#46](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/46) | [S1.4-01] Define HTTP route table (kv, cluster status/join/remove) | open |
| [#47](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/47) | [S1.4-02] Handler skeletons with defined JSON schema (501 stubs) | open |
| [#48](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/48) | [S1.4-03] raftctl command skeleton (status, join, remove) | open |
| [#49](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/49) | [S1.4-04] Write docs/api.md HTTP API contract | open |
| [#50](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/50) | [S1.4-05] ADR note: bearer-token on join is out of MVP scope for now | open |
| [#51](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/51) | [S1.5-01] Multi-stage Dockerfile building cmd/raftkv | open |
| [#52](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/52) | [S1.5-02] docker-compose.yml for configurable 3-5 node cluster with named volumes | open |
| [#53](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/53) | [S1.5-03] Node bootstrap flags and env vars (node ID, peers, data dir) | open |
| [#54](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/54) | [S1.5-04] scripts/add-node.sh skeleton for adding a 4th/5th node | open |
| [#55](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/55) | [S1.5-05] Verify docker compose up brings up N running containers | open |
| [#56](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/56) | [S1.6-01] CI workflow: build, vet, test, race on PR and push to main | open |
| [#57](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/57) | [S1.6-02] Branch protection: CI + review required (2 reviewers for raft/membership) | open |
| [#58](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/58) | [S1.6-03] Docker Compose smoke test in CI | open |
| [#59](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/59) | [S1.6-04] golangci-lint step in CI | open |
| [#60](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/60) | [S1.7-01] Ratify the SDD in a team session; record amendments as ADRs | open |
| [#61](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/61) | [S1.7-02] Set up docs/adr with a context/decision/consequences template | open |
| [#62](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/62) | [S1.7-03] Set up docs/uml and schedule the four pending sequence diagrams | open |
| [#63](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/63) | [S1.7-04] Pin the OUT_OF_SCOPE list as a standing scope-creep check | open |
| [#64](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/64) | [S2.1-01] Map-backed store with Put/Get/Delete guarded by a mutex | open |
| [#65](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/65) | [S2.1-02] Define command encoding {OpType, Key, Value} | open |
| [#66](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/66) | [S2.1-03] Isolate GET as a read-only non-mutating path | open |
| [#67](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/67) | [S2.1-04] Input validation (empty key, size limits) | open |
| [#68](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/68) | [S2.2-01] Implement Apply(command) dispatching Put/Get/Delete | open |
| [#69](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/69) | [S2.2-02] Guarantee determinism (no clocks, randomness or map-order dependence) | open |
| [#70](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/70) | [S2.2-03] Document Apply return contract for each op type | open |
| [#71](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/71) | [S2.2-04] Compile-time assertion that Store satisfies StateMachine | open |
| [#72](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/72) | [S2.3-01] Choose and document the snapshot serialization format (with version field) | open |
| [#73](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/73) | [S2.3-02] Implement Snapshot() returning a complete byte representation | open |
| [#74](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/74) | [S2.3-03] Implement Restore([]byte) fully replacing state | open |
| [#75](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/75) | [S2.3-04] Round-trip test: Restore(Snapshot()) equals original | open |
| [#76](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/76) | [S2.3-05] Document that snapshot payload is KV-internal; Raft adds index/term metadata outside | open |
| [#77](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/77) | [S2.4-01] Unit tests for Put/Get/Delete/Apply edge cases | open |
| [#78](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/78) | [S2.4-02] Property-based tests: determinism, snapshot round-trip, no panics | open |
| [#79](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/79) | [S2.4-03] Import-boundary check for internal/state (go list -deps) | open |
| [#80](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/80) | [S2.4-04] Concurrent Put/Get/Delete tests under -race | open |
| [#81](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/81) | [S2.4-05] Wire the state test suite into CI | open |
| [#82](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/82) | [S2.5-01] Test-only driver using the StateMachine interface type | open |
| [#83](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/83) | [S2.5-02] Confirm harness never references the concrete Store type | open |
| [#84](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/84) | [S2.5-03] Capture interface friction as ADR amendment or close as not needed | open |
| [#85](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/85) | [S3.1-01] RaftNode struct, role enum and persistent/volatile state per SDD 24 | open |
| [#86](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/86) | [S3.1-02] Randomized election timer (150-300ms) and heartbeat ticker with injectable clock | open |
| [#87](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/87) | [S3.1-03] Term management: step down on higher term in any RPC | open |
| [#88](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/88) | [S3.1-04] Persist currentTerm/votedFor before responding (via Storage interface, in-memory fake) | open |
| [#89](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/89) | [S3.1-05] Unit tests: timer reset, randomization range, step-down | open |
| [#90](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/90) | [S3.2-01] RequestVote handler: one vote per term and log up-to-date check | open |
| [#91](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/91) | [S3.2-02] Candidate flow: increment term, self-vote, broadcast, tally majority | open |
| [#92](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/92) | [S3.2-03] Split-vote handling via randomized retry | open |
| [#93](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/93) | [S3.2-04] Leader init: nextIndex/matchIndex, immediate heartbeat, no-op entry | open |
| [#94](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/94) | [S3.2-05] Election tests incl. property test: at most one leader per term | open |
| [#95](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/95) | [S3.3-01] AppendEntries follower handler with consistency check and conflict truncation | open |
| [#96](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/96) | [S3.3-02] Leader replication loop with nextIndex backoff and conflict backtracking | open |
| [#97](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/97) | [S3.3-03] Commit advancement: majority matchIndex, current-term-only rule (Raft 5.4.2) | open |
| [#98](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/98) | [S3.3-04] Apply loop: ordered apply to StateMachine, lastApplied tracking, idempotent retries | open |
| [#99](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/99) | [S3.3-05] Propose(): leader-only append returning index, term, isLeader | open |
| [#100](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/100) | [S3.3-06] Replication tests: divergent followers, quorum-only commit, monotonic commit index | open |
| [#101](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/101) | [S3.4-01] In-process Transport implementation with drop/delay hooks | open |
| [#102](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/102) | [S3.4-02] N-node in-process cluster harness (3 and 5 nodes) | open |
| [#103](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/103) | [S3.4-03] Run Stage 3 suite under -race in CI and fix races | open |
| [#104](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/104) | [S3.4-04] ADR: Raft implementation decisions and deviations from the paper | open |
| [#105](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/105) | [S3.4-05] UML sequence diagrams: leader election and log replication | open |
| [#106](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/106) | [S4.1-01] gRPC server handlers for RequestVote/AppendEntries delegating to Raft | open |
| [#107](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/107) | [S4.1-02] gRPC client implementing raft.Transport with bounded RPC timeouts | open |
| [#108](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/108) | [S4.1-03] Connection management: reconnect after peer restart | open |
| [#109](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/109) | [S4.1-04] Transport tests: round-trip, timeout and reconnect-after-restart | open |
| [#110](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/110) | [S4.2-01] PUT/DELETE: encode command, Propose, wait for commit+apply, respond (503 on no quorum) | open |
| [#111](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/111) | [S4.2-02] GET served from leader applied state (MVP consistency, documented gap) | open |
| [#112](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/112) | [S4.2-03] /cluster/status, /cluster/leader, /cluster/members endpoints | open |
| [#113](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/113) | [S4.2-04] JSON error model and status codes (400, 409, 503) | open |
| [#114](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/114) | [S4.3-01] Non-leader write: 409 with {error: not leader, leaderHint} | open |
| [#115](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/115) | [S4.3-02] raftctl put/get/delete and cluster status/leader/members with one auto-redirect | open |
| [#116](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/116) | [S4.3-03] Document redirect behaviour and consistency caveat in docs/api.md | open |
| [#117](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/117) | [S4.4-01] internal/server wiring: peers to Raft + transport + API | open |
| [#118](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/118) | [S4.4-02] 3-node compose: leader elected, write replicated, reads consistent | open |
| [#119](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/119) | [S4.4-03] Integration test: PUT via raftctl to a follower, redirect, value readable | open |
| [#120](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/120) | [S4.4-04] CI smoke test hitting /cluster/status on a booted compose cluster | open |
| [#121](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/121) | [S5.1-01] WAL record format: length, checksum, recordType, payload | open |
| [#122](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/122) | [S5.1-02] Write path: append, fsync, then return | open |
| [#123](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/123) | [S5.1-03] Recovery path: sequential read, stop at first bad or truncated frame | open |
| [#124](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/124) | [S5.1-04] FileStorage: SaveState, LoadState, AppendEntries, TruncateFrom, GetEntries, LastIndex | open |
| [#125](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/125) | [S5.1-05] WAL tests: crash round-trip, torn write, corrupt tail, concurrent read-during-append | open |
| [#126](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/126) | [S5.2-01] Swap in FileStorage; persist-before-respond for votes and AppendEntries | open |
| [#127](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/127) | [S5.2-02] Node restart: reload state, replay to rebuild lastApplied, rejoin as follower | open |
| [#128](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/128) | [S5.2-03] Storage error policy: fatal, crash loudly (SDD 29) | open |
| [#129](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/129) | [S5.3-01] Snapshot file format: lastIncludedIndex/Term, KV payload, checksum, latest pointer | open |
| [#130](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/130) | [S5.3-02] Snapshot trigger every N committed entries (configurable) | open |
| [#131](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/131) | [S5.3-03] Log compaction without losing entries beyond lastApplied | open |
| [#132](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/132) | [S5.3-04] InstallSnapshot sender and handler (single message) | open |
| [#133](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/133) | [S5.3-05] Startup from snapshot plus WAL tail; inconsistency detected as fatal | open |
| [#134](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/134) | [S5.4-01] Test: kill -9 a follower mid-write, restart, converge | open |
| [#135](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/135) | [S5.4-02] Test: full-cluster restart preserves all committed data | open |
| [#136](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/136) | [S5.4-03] Test: crash during WAL replay is re-entrant | open |
| [#137](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/137) | [S6.1-01] Config commands (add learner, promote voter) and membership data model | open |
| [#138](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/138) | [S6.1-02] Apply config entries only when committed; quorum from voters only | open |
| [#139](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/139) | [S6.1-03] Persist cluster configuration with WAL/snapshot | open |
| [#140](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/140) | [S6.1-04] Enforce a single in-flight membership change (durable flag) | open |
| [#141](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/141) | [S6.2-01] JoinCluster RPC and /cluster/join with validation rules | open |
| [#142](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/142) | [S6.2-02] Seed-node NotLeader redirect and idempotent retries | open |
| [#143](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/143) | [S6.2-03] raftkv --join startup flow and raftctl cluster join | open |
| [#144](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/144) | [S6.2-04] Leader replicates to learner via AppendEntries or InstallSnapshot | open |
| [#145](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/145) | [S6.3-01] Track learner matchIndex and configurable promotion threshold | open |
| [#146](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/146) | [S6.3-02] Promote learner to voter via committed config entry; recompute quorum | open |
| [#147](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/147) | [S6.3-03] Real scripts/add-node.sh joining a running compose cluster | open |
| [#148](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/148) | [S6.3-04] ADR: promotion threshold and learner-as-attribute confirmation | open |
| [#149](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/149) | [S6.4-01] Test: join, learner, promoted voter | open |
| [#150](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/150) | [S6.4-02] Test: join during concurrent client writes | open |
| [#151](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/151) | [S6.4-03] Test: restart learner or leader during catch-up | open |
| [#152](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/152) | [S6.4-04] Test: membership survives full-cluster restart | open |
| [#153](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/153) | [S6.4-05] Test: duplicate or late join retries are rejected or idempotent | open |
| [#154](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/154) | [S7.1-01] Fault controller and Transport decorator: drop, delay, partition | open |
| [#155](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/155) | [S7.1-02] Process-level faults: in-process stop/restart and docker stop/start helpers | open |
| [#156](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/156) | [S7.1-03] Data-driven scenario DSL (at tick N kill leader) | open |
| [#157](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/157) | [S7.1-04] Harness self-tests: faults delivered exactly as configured | open |
| [#158](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/158) | [S7.2-01] Leader crash: re-election, no committed loss | open |
| [#159](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/159) | [S7.2-02] Follower crash and restart: catch-up | open |
| [#160](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/160) | [S7.2-03] Network partition: minority cannot commit; heal causes step-down and resync | open |
| [#161](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/161) | [S7.2-04] Quorum loss: writes refused with 503 and resume on recovery | open |
| [#162](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/162) | [S7.2-05] Delayed and dropped packet scenarios (second wave) | open |
| [#163](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/163) | [S7.2-06] Crash during recovery and during membership change (second wave) | open |
| [#164](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/164) | [S7.3-01] Property tests across randomized fault runs (SDD 14 invariants) | open |
| [#165](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/165) | [S7.3-02] Clean go test -race ./... across all packages; fix flakes | open |
| [#166](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/166) | [S7.3-03] Nightly extended property workflow in GitHub Actions | open |
| [#167](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/167) | [S7.4-01] Structured JSON logging interface (timestamp, node, role, term, event) | open |
| [#168](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/168) | [S7.4-02] Log key events: role/term changes, election results, commit advance, snapshot, membership | open |
| [#169](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/169) | [S7.4-03] /metrics: role, term, commit and applied index, leader changes, log size | open |
| [#170](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/170) | [S8.1-01] Demo script: 3 nodes, writes, kill leader, re-election, restart, consistency check | open |
| [#171](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/171) | [S8.1-02] Demo script: join 4th node as learner, promote, writes continue | open |
| [#172](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/172) | [S8.1-03] Full regression green on CI (unit, integration, fault, membership, race) | open |
| [#173](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/173) | [S8.2-01] Finish README: architecture, quick start, API, limitations | open |
| [#174](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/174) | [S8.2-02] UML sequence diagrams: network partition and crash recovery | open |
| [#175](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/175) | [S8.2-03] ADR set complete and indexed in docs/adr/README | open |
| [#176](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/176) | [S8.2-04] Known-limitations doc (stale-read gap, single-message snapshot, no auth/TLS) | open |
| [#177](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/177) | [S8.3-01] Update the mid-semester report with implementation results and real test evidence | open |
| [#178](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/178) | [S8.3-02] Presentation deck: architecture, live demo, results, project-management evidence | open |
| [#179](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/179) | [S8.3-03] Record backup demo video | open |
| [#180](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/180) | [S8.4-01] Tag v0.1.0 and publish release notes | open |
| [#181](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/181) | [S8.4-02] Scope retrospective: shipped vs SDD MUST/SHOULD | open |
| [#182](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/182) | [S8.4-03] Post final project status update and archive board views | open |
| [#183](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/183) | [S9.1-01] Read-index (or leader-lease) linearizable reads | open |
| [#184](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/184) | [S9.2-01] /cluster/remove validation and CONFIG_REMOVE entry | open |
| [#185](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/185) | [S9.2-02] Reject leader self-removal with a clear error | open |
| [#186](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/186) | [S9.2-03] Removal tests incl. failure paths | open |
| [#187](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/187) | [S9.3-01] Shared-secret bearer token on the join endpoint | open |
| [#188](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/188) | [S9.3-02] Extra metrics: commit latency histogram, snapshot duration, learner lag | open |
| [#189](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/189) | [S9.3-03] Chunked snapshot transfer | open |
| [#190](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/190) | [S9.3-04] Benchmark suite and short results write-up | open |
| [#191](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/191) | [Phase 1.4] HTTP Client API & raftctl CLI Skeletons | open |
| [#192](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/192) | [Phase 1.5] Docker Compose & Multi-Node Bootstrap Skeleton | open |
| [#193](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/193) | [Phase 1.6] CI Pipeline & Quality Gates | open |
| [#194](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/194) | [Phase 2.1] KV Store Core Data Structure & CRUD | open |
| [#195](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/195) | [Phase 2.4] Isolation Enforcement & Property Test Suite | open |
| [#196](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/196) | [Phase 2.5] Standalone Integration Stub (No-Raft Harness) | open |
| [#197](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/197) | [Phase 4.3] Leader Redirect & raftctl | open |
| [#198](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/198) | [Phase 7.4] Observability (Structured Logs & Basic Metrics) | open |
| [#199](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/199) | [Phase 8.2] Documentation | open |
| [#200](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/200) | [Phase 8.4] Release & Retrospective | open |
| [#201](https://github.com/DynamicRaftKV/dynamic-raft-kv/issues/201) | [Phase 9.3] Hardening & Polish | open |
