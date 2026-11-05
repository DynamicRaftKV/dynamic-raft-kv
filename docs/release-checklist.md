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
- [ ] Integrate PR 209 against current main after reviewing its final diff.
- [ ] Confirm branch protections and ownership requirements.
- [ ] Fresh-checkout demo succeeds with documented host ports and persistent volumes.
- [ ] Review repeated distributed measurements and their limitations.
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
