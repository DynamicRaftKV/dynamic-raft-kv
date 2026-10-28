# Stage 5.4 recovery validation and merge gate

29 October 2026. The defined storage/recovery fault matrix is implemented.
Review, GitHub CI and deployment acceptance are separate from local test success.

## Fault matrix

| Boundary | Injection/evidence | Required result |
|---|---|---|
| Term/vote, append, snapshot records | ENOSPC before write, partial write, EIO on sync | No success; handle poisoned; no later writes; reopen yields a consistent state |
| Checkpoint file creation | ENOSPC and EIO | Prior durable snapshot and suffix recover |
| Checkpoint header/record write and file sync | ENOSPC and EIO, including partial writes | Incomplete temporary file never replaces active WAL |
| Checkpoint rename | ENOSPC and EIO | Prior WAL remains usable after reopening |
| Directory sync after rename | ENOSPC and EIO | Handle stops; reopened complete generation has the same snapshot, term/vote and suffix |
| Reopening replacement / closing old WAL | ENOSPC and EIO | No further operations on uncertain handle; recover and append safely |
| Real process termination during compaction | Before rename, after rename, after directory sync | Complete checkpoint plus suffix, followed by successful append/recompact/reopen |
| Snapshot transfer | Restart or higher term during assembly | No partial publication; reject continuation; transfer from zero succeeds |
| Follower install / compaction failure | ENOSPC on install, EIO on compact | No final acknowledgement; no subsequent votes/appends; restart and retry recover |
| Local checkpoint failure with pending proposal | ENOSPC | Node steps down and releases waiting client with the storage error |
| Cluster crash/restart | Three real processes, automatic snapshots | Committed keys survive leader kill and full restart |

`internal/storage/failure_test.go` covers 16 checkpoint error combinations and
9 record-write error combinations. Raft-level failure tests verify protocol
responses as well as storage behavior. Existing Stage 5.3 tests cover record
corruption, every torn-tail byte cut, suffix policy, uncommitted-state exclusion,
lost final acknowledgement and >4 MiB snapshot catch-up over gRPC.

The private per-WAL checkpoint I/O seam defaults to the existing OS operations.
Only package tests inject alternatives; there is no fault-injection API or flag.
Tests inject ENOSPC rather than filling the developer's real disk. A failed sync
has unknown durability: callers must stop, and recovery may include a complete
unacknowledged operation. Process tests do not certify hardware power-loss safety.

## Merge recommendation

Merge the Stage 3–5 branch before layering Stage 6 dynamic membership on top,
once all of these are true:

1. A PR from `codex/stage3-raft-core` targets current `main` without conflicts.
2. GitHub CI passes on its current head: build, vet, tests, race checks, formatting,
   generated-protobuf check and the existing Compose scaffold smoke check.
3. Two reviewers approve the Raft changes, as required by
   `.github/PULL_REQUEST_TEMPLATE.md`; review findings are resolved.
4. Reviewers accept ADR-0004 through ADR-0007 and the documented limits: fixed
   membership, 32 MiB snapshots, synchronous checkpoints and standalone demo UI.

Local passing tests alone do not authorize bypassing branch rules or establish
that the remotely configured protections are enabled. CI currently triggers on
pull requests and pushes to main, so a feature-branch push alone does not run it.
The Compose check still tests scaffold startup, not a working container cluster.
Main remains an incremental integration branch, not a declaration that all nine
stages or production-readiness work are complete.

## Next work

Stage 6 needs one precise, documented membership protocol before implementation:
when configuration entries take effect, quorum transitions, one change in flight,
learner admission/catch-up/promotion, removal, identity reuse and restart rules.
Broader adversarial histories, operational metrics, resource admission limits and
performance remain Stage 7; Docker acceptance is 4.4 and cluster UI/evaluation is 8.
