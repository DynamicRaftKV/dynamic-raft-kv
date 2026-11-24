# Stage 7 combined-fault acceptance

This follow-up targets the remaining explicit scenarios in issues #150, #151
and #166. It changes tests and scheduling, not the consensus algorithm.

- `TestMembershipJoinDuringConcurrentWrites` holds voter responses to establish
  an in-flight write, appends learner admission before releasing them, completes
  24 writes from two concurrent clients, compares replica contents, promotes the
  learner and reads every acknowledged key through consensus.
- `TestMembershipRestartDuringCatchup` keeps a newly admitted learner behind,
  rejects premature promotion, restarts either the learner or leader, restores
  communication and checks full state equality. It then promotes and restarts
  the new voter again to verify recovered membership.
- Existing interrupted-snapshot tests separately check rejection of an orphaned
  chunk tail after restart or term change. The new catch-up tests do not claim
  to interrupt every possible snapshot chunk boundary.

The test storage is an in-memory retained-disk double. Real disk/process recovery
continues to be covered separately by the executable and Compose acceptance
suites. These bounded scenarios do not prove correctness for every schedule.

## Scheduled checks

`Extended reliability` runs daily at 21:17 UTC (02:47 IST the next day), and can
be started manually. Three explicit test-order seeds (17, 73, 211) each repeat
the selected membership, history, quorum, partition and snapshot tests 20 times
under the race detector. Failure does not cancel other seeds. Artifacts retain
commit, Go version, options and JSON test results for 14 days. A failed Go test
remains a failed job despite output being piped to a file.

The schedule becomes active only when this workflow reaches the default branch.
A local successful run is not evidence that the scheduled workflow has run.

Reproduce a job with the pinned Go toolchain:

```sh
go test -json -race -count=20 -shuffle=17 -timeout=15m \
  -run 'Test(Membership|ConcurrentHistories|HistoryOracle|SeededPartitions|QuorumLoss|Snapshot)' \
  ./internal/raft
```

Next: run current-head CI and Compose acceptance, complete consensus review,
then finalize Stage 8 documentation, presentation, demo video and release.

## Validation fixes

Full-suite execution exposed two timing weaknesses. Local managed processes now
have an eight-second readiness window (previously three seconds), below the
console's ten-second request limit; failures include the last readiness error
and the node log name. Isolated startup tests passed while loaded full-suite
runs exhausted the old deadline.

The real TCP cluster test now continues leader heartbeat ticks while client
requests wait. Previously, a timed-out append could leave a proposal waiting
until the test deadline because no retry tick was scheduled. Production node
runtimes already tick continuously; this change corrects the test harness.

## Local validation — 25 November 2026

Passed: `make build vet test race`; all five dashboard model checks; frontend
script syntax; the extended selection at count 20 for each shuffle seed 17,
73 and 211; and `TestTCPCluster` at count 10 with race detection after the
heartbeat fix. Docker is unavailable on this machine, so current-commit Compose
acceptance remains a GitHub CI gate. No issue closure or release is implied.
