# Remaining project completion plan

Updated 29 October 2026 after snapshot and compaction work. Code status is
separate from reviews, deployment and final acceptance.

The [etcd comparison and readiness review](raft-readiness-review.md) adds
concrete snapshot, membership, history-testing and deployment acceptance checks.
Resource admission limits are a proposed reliability task; optional etcd-style
extensions remain separate from the MVP.

| Stage | Current position | Work left |
|---|---|---|
| 1 Foundation | Scaffold/interfaces built | Complete SDD reconciliation, interface freeze, team/governance review and confirm repository protections |
| 2 Standalone KV | Implemented and tested | No additional feature work planned; retain regression checks |
| 3 Raft core | Four phases implemented and tested | Required Raft reviews and broader system-level validation |
| 4 Network/client integration | gRPC, HTTP, CLI and durable startup implemented | Phase 4.4 Docker Compose deployment and acceptance through published ports; failure/restart checks in containers |
| 5 Durability/recovery | 5.1–5.3 WAL, startup, snapshots and compaction implemented/tested | Finish 5.4 disk-full and additional failure combinations; Raft review |
| 6 Dynamic membership | Not implemented | 6.1 durable configuration-change algorithm/entries; 6.2 joining; 6.3 learner catch-up/promotion; 6.4 membership/removal safety and restart tests |
| 7 Reliability/observability | Initial core, TCP and process failure tests exist | 7.1 broader fault injection; 7.2 failure combinations; 7.3 invariant/property checks; 7.4 structured logging, metrics and diagnostics |
| 8 Integration/evaluation | Standalone presentation UI exists | 8.1 connect UI to the real cluster and reproducible full demo; 8.2 accurate user/operator docs; 8.3 evaluation/benchmarks/report/deck; 8.4 release/review close-out |
| 9 Stretch | Not started | Optional post-MVP items only; not a prerequisite for required project completion |

## Order of remaining critical work

1. Finish remaining recovery fault injection; snapshot/compaction is now implemented (ADR-0007).
2. Freeze and implement the membership-change protocol, then join as learner,
   catch-up, promotion and removal. Do not count a working JoinCluster wire
   adapter as a working membership algorithm.
3. Expand fault/property testing and observability alongside those changes.
4. Complete Docker Compose integration (Stage 4.4), using the working durable
   executable, persistent per-node volumes and reachable advertised addresses.
   Test 3/5/N configurations, leader failure, container restart and full restart.
   Existing Compose files still launch scaffold mode and are not this deliverable.
5. Connect the frontend to real cluster APIs and status, including accurate
   membership/failure controls only after the backend supports them.
6. Finish end-to-end acceptance, documentation and evaluation/release materials.

Docker packaging belongs to 4.4; the final reproducible container demonstration
and UI/documentation belong to Stage 8. Docker is deferred at the owner's request,
not omitted from project completion. Separate local processes already exercise
real HTTP/gRPC and disk persistence without Docker.

N nodes currently means a configurable, fixed peer set. Live changes to N are
Stage 6. Do not add/remove voters by editing the bootstrap JSON of a running or
previously initialized cluster.
