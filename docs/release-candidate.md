# Release candidate: reviewer and demonstration handoff

Prepared 25 November 2026. This is a review candidate, not a tagged release or
an assertion that the project has received final acceptance.

## What the project now does

RaftKV is a Go key-value store with its own Raft implementation. Each durable
node has an HTTP client interface, gRPC peer transport, WAL and state machine.
Writes and reads are ordered through Raft. Learners catch up before promotion;
voter changes use joint consensus. Restart recovers node identity, log and
snapshots. The local console can create processes with assigned ports and drive
join, promotion, removal and guided correctness demonstrations.

## Evidence and limits

- The merged baseline and Docker membership/crash acceptance passed CI at
  `5f0ce6d`; PR 211 integrated it into main at `9d5592d`.
- The scheduled three-seed, race-checked reliability workflow passed on main:
  [run 45545674591](https://github.com/DynamicRaftKV/dynamic-raft-kv/actions/runs/45545674591).
- The browser demonstrated 5→6→5 voters, followed by successful replicated
  PUT/GET/DELETE and applied-index catch-up checks. See the
  [frontend demonstration](frontend-demonstration.md).
- [Repeated evaluation](evaluation/README.md) records 1,200 successful measured
  requests across three runs of each 3/5-node configuration, plus failover and
  catch-up checks. Shared hardware and background load limit interpretation.
- [Architecture records](adr/README.md) describe decisions and the durable-write
  sequence. [Reliability evidence](stage7-8-status.md) separates test coverage
  from proof or production claims.

The latest logging/frontend/acceptance follow-up requires its own PR and CI.
No previous CI result is evidence for an untested later revision.

## Demonstration script

1. Show Implementation: explain implemented features versus pending acceptance.
2. Open Live cluster. Identify leader, voters, term and stable membership.
3. Run the guided CRUD test. Explain that success follows commit and application.
4. Enable automatic join/promotion, create one node and show assigned ports.
5. Wait for stable membership; run the replication progress check.
6. Select that managed node and run the restart test while other voters retain
   quorum. Verify the retained value and applied-index progress.
7. Remove that test member using the in-page confirmation. Explain that removal
   retires the ID; it is not a crash and does not delete files.
8. Show metrics and commit-progress logs. Explain that observations alone do not
   prove quorum; a successful consensus operation is the stronger check.
9. Show Docker CI evidence and the measured evaluation, including limitations.
10. End with the outstanding release checklist; do not describe the review
    candidate as fully approved or production-ready.

## Lessons and exact causes

| Observed problem | Cause established by checks | Change |
|---|---|---|
| Managed startup timed out during suite load | Three-second readiness deadline expired; isolated starts passed | Eight-second bounded deadline, retaining the last readiness error |
| TCP proposal test could time out | Test waited synchronously without ongoing heartbeat retries | Continue leader ticks while test requests wait |
| Removed node hid a healthy leader in UI | Term comparison included a retired process that kept campaigning | Compare current members and outgoing joint voters; retain conflict checks |
| Later pushes did not run PR CI | PR 211 had already merged at an earlier head | New follow-up PR containing subsequent commits |

## Remaining acceptance

Independent Raft review, SDD/interface reconciliation and repository-protection
verification remain team gates. Repeatable scripts do not substitute for those
reviews. Final report/deck submission and acceptance, release tagging and team
retrospective must be recorded rather than inferred. A fresh-checkout console
API check complements (but does not replace) browser and container checks.

The requested full process visualizer is a subsequent UI feature. It should
clearly distinguish explanatory animation from recorded real-node events and
must not manufacture replication messages or quorum evidence from status polls.
It is intentionally deferred until this Phase 7/8 acceptance work is reviewed.
