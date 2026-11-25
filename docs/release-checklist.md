# Release acceptance checklist

The release is not complete until the open gates below have recorded evidence.
Do not infer approval from implementation or from this checklist.

## Implemented checks

- [x] Durable cluster, snapshots and dynamic membership implemented.
- [x] Phase 6 merged through PR 208.
- [x] Phase 7–8 baseline passed CI at `3aae4f3`, including durable Compose acceptance.
- [x] Live cluster console exposes real status, KV and membership operations.
- [x] Repeatable local 3/5-process evaluation script and baseline results exist.

## Final gates

- [ ] Current PR head passes build, vet, tests, race, UI status checks and Docker CI.
- [ ] Two Raft reviewers approve; findings and SDD/interface reconciliation resolved.
- [ ] Integrate the post-PR-211 logging, browser membership and acceptance follow-up against main.
- [ ] Confirm branch protections and ownership requirements.
- [x] Clean local checkout builds and passes isolated console API acceptance (af27b20; shared dependency cache).
- [ ] Fresh-checkout browser/Docker demonstration with documented host ports and persistent volumes is accepted.
- [ ] Review the three repeated 3/5-process measurements and their shared-host limitations.
- [ ] Final report and presentation accepted or submitted as required.
- [ ] Record release tag, known limits and retrospective.

## Demonstration sequence

1. Build and run `make demo-cluster`; open `/demo/#cluster` on port 8080.
2. PUT a new key, then GET it. The standalone workspace uses separate data.
3. On a fresh cluster, admit node4, wait for catch-up, promote and wait for stable
   membership. If it is already a voter, do not repeat admission.
4. Show diagnostics and metrics. A role or health result is not a quorum check.
5. Use the disposable Docker acceptance script for scripted leader kill and full
   restart. Do not run that destructive test against important data.
6. Show the implementation page and evaluation results, explaining which review
   and delivery gates remain open. Removal is permanent membership retirement,
   not a crash simulation; do not reuse retired IDs.

## Evidence recorded 25 November 2026

- PR 211 merged at `9d5592d`, containing `5f0ce6d`. Later commits `5766a1b`
  and `c7dcbdc` require a new PR; pushing them did not update the merged PR.
- [Scheduled extended reliability run](https://github.com/DynamicRaftKV/dynamic-raft-kv/actions/runs/45545674591)
  completed successfully on `9d5592d`. This satisfies observation of the scheduled
  workflow, not independent consensus review or proof of arbitrary fault schedules.
- The disposable console acceptance script now checks page serving, assigned
  ports, learner admission/promotion, managed identity and port retention after
  console restart, removal and consensus reads. It runs on isolated temporary
  data and ports, and is included in CI. Browser rendering remains a separate
  manual check; Docker acceptance remains its own CI step.

Run from a clean checkout after `make build`:

```sh
python3 scripts/console-acceptance.py --output /tmp/console-acceptance.json
```

Clean-checkout evidence: [console acceptance record](evaluation/console-acceptance-2026-11-25.json). Full build, vet, tests and race checks passed on the same checkout.
