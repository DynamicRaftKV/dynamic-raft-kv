# Final evaluation draft

Prepared 25 November 2026 for reviewer and presentation use. Submission and
acceptance have not been recorded. This document does not certify production use.

## Outcome

The original standalone Go store now runs as a durable multi-process Raft
cluster. It supports ordered PUT/GET/DELETE, leader election, replication,
quorum commitment, WAL recovery, snapshots and dynamic learner/voter membership.
A browser console can create local nodes, allocate ports, join and promote them,
remove members and run guided checks. Docker Compose provides a reproducible
local deployment with persistent volumes.

## What the evidence establishes

| Question | Evidence | Interpretation |
|---|---|---|
| Do nodes agree after normal writes? | Replication tests and browser applied-index checks, consensus reads | Sampled convergence; the browser does not compare independent state hashes |
| Can a minority acknowledge operations? | Core/process tests and console quorum-loss acceptance | Tested GET/PUT cannot succeed without quorum; timed-out writes can commit later |
| Can membership change while running? | Learner catch-up, joint-consensus, restart tests; browser 5→6→5 demonstration | Growth/removal exercised; capacity remains machine-dependent |
| Does data survive failure? | WAL/snapshot tests, process restart and durable Compose acceptance | Tested recovery scenarios preserve acknowledged data |
| Can the project be reproduced? | Clean-checkout build/vet/test/race, isolated console acceptance, main CI | No dependency on the existing demo's data; shared dependency cache was used locally |
| Does repeated testing run automatically? | Successful scheduled three-seed reliability workflow | Repeated sampled fault coverage, not an exhaustive Raft proof |

## Measured behavior

Three runs for each of 3 and 5 nodes completed all 1,200 measured requests.
Observed throughput ranges were 124.75–137.21 requests/s (3 nodes) and
97.27–113.65 requests/s (5 nodes). Verified-read recovery after leader failure
ranged from 1.19–1.72 s and 0.96–1.04 s respectively. These are shared-host
integration measurements, not production throughput or an SLA. See
[evaluation data and method](evaluation/README.md) for latency ranges and raw data.

## Design choices and lessons

The project keeps its own Raft implementation and separates consensus from
storage, transport, HTTP and state-machine code. Reads use the replicated log;
ReadIndex and other optimizations remain optional. Learners do not contribute
to quorum until promotion. Membership removal retires an ID rather than
simulating failure. Cancellation ends a caller's wait, not a durable proposal.

Implementation testing found overly short managed-process readiness timing,
missing heartbeat retries in a TCP test harness, and a dashboard that counted
retired nodes' election terms. Each received a focused fix and regression
coverage. The console acceptance harness also needed a successful-read recovery
gate: a leader role report could arrive before quorum had recovered. Release tracking also needed correction: pushing to an already-merged
PR branch did not test or integrate later commits. Follow-up changes now receive
new PRs and current-head CI.

## Boundaries and remaining work

No MVCC, watches, transactions, leases, multi-machine deployment automation or
etcd product parity is claimed. The local supervisor manages only its own child
processes. Diagnostics and liveness are advisory, logs are synchronous, and
bounded tests do not cover every failure ordering. Independent consensus/SDD
review, final demonstration acceptance, report/deck acceptance and release
sign-off remain recorded gates in the [release checklist](release-checklist.md).

The [process visualizer](process-visualizer.md) explains elections, persistence,
replication, commitment, application, recovery and membership. Scripted teaching
scenarios are clearly separate from actual node-status observations; this is
not a packet trace or a replay of live operations.

## Suggested presentation order

Problem and scope → architecture → durable-write sequence → dynamic membership
→ fault/recovery demonstration → measured evaluation → lessons → known limits
and remaining acceptance. Use the [release-candidate demonstration script](release-candidate.md)
and [architecture diagram](adr/README.md) as supporting material. This outline
is not a completed slide deck or evidence of presentation submission.
