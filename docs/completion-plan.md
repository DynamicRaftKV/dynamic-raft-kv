# Remaining project completion plan

Updated 4 November 2026 after reliability and evaluation integration. Code status is
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
| 4 Network/client integration | gRPC, HTTP, CLI and durable startup implemented | Durable Compose acceptance passed at 3aae4f3; retain current-head regression checks |
| 5 Durability/recovery | 5.1–5.4 implemented, CI passed and milestone merged | Broader reliability is Stage 7; no deferred Stage 5 feature is implied |
| 6 Dynamic membership | 6.1–6.4 implemented locally: durable joint consensus, join, catch-up, promotion/removal and recovery tests | Merged in PR 208; retain consensus review and regression evidence; see [Stage 6](stage6-status.md) |
| 7 Reliability/observability | Fault histories, bounded register checker, admission limits, structured logs and metrics implemented | Consensus review and broader combined-fault acceptance; see [evidence and limits](stage7-8-status.md) |
| 8 Integration/evaluation | Live cluster UI, runbook, durable Compose acceptance script and benchmark baseline implemented | Container CI passed; local distributed evaluation added; repeated measurements, report/deck acceptance and release/review close-out remain |
| 9 Stretch | Not started | Optional post-MVP items only; not a prerequisite for required project completion |

## Order of remaining critical work

1. Review and integrate PR 209 against current main; recheck its current head.
2. Resolve targeted consensus findings and foundation governance close-out.
3. Review combined-fault coverage and repeated distributed measurements.
4. Exercise the documented demo from a fresh checkout.
5. Finish final evaluation, presentation and release acceptance.

See [release checklist](release-checklist.md) and [evaluation method](evaluation/README.md).

Docker packaging belongs to 4.4; the final reproducible container demonstration
and UI/documentation belong to Stage 8. Docker acceptance is being integrated with the working durable implementation. Separate local processes already exercise
real HTTP/gRPC and disk persistence without Docker.

Live changes to N use Stage 6's join/promote/remove endpoints. Do not add/remove
voters by editing the bootstrap JSON of a running or previously initialized
cluster. The Live cluster console exposes membership operations; standalone
snapshot/restore remains isolated. Compose acceptance now runs real operations.
