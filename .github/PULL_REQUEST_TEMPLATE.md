## Summary
<!-- What does this PR change and why? -->

Closes #<issue>   ·   Task ID: S_._-__

## Raft safety invariants touched (SDD section 14)
- [ ] None
- [ ] S1 Election safety  - [ ] S2 Leader append-only  - [ ] S3 Log matching  - [ ] S4 Leader completeness
- [ ] S5 State machine safety  - [ ] S6 Membership-change safety  - [ ] S7 No commit without quorum

## Definition of Done (SDD section 45)
- [ ] Unit tests added/updated
- [ ] Integration tests (multi-node) where applicable
- [ ] Failure tests where the feature has failure-relevant behaviour
- [ ] Docs / ADR updated (link: )
- [ ] Logs/metrics present where relevant
- [ ] `go test ./...` and `go test -race ./...` pass
- [ ] No import-boundary violations (`internal/state` and `internal/raft` coupling rules)

## Review
- [ ] At least 1 reviewer approved (2 for `internal/raft` and `internal/membership`)

## Evidence
<!-- Test output, logs, or screenshot. This feeds the final report and presentation. -->
