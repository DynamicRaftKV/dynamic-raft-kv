# Stage 1 status

Status: scaffolding implemented; interface freeze and full Stage 1 exit pending.

| Phase | Local deliverable | Remaining exit requirement |
|---|---|---|
| 1.1 | Module, seven packages, binaries, Makefile, lint config; GitHub module path and existing MIT license | SDD package review |
| 1.2 | Draft interfaces/types and compile-time mocks, two ADRs | SDD reconciliation, open decisions, synchronous review/freeze |
| 1.3 | Proto, generated Go, unimplemented server/client | Field-level sign-off, transfer-shape freeze |
| 1.4 | Routes, JSON parsing, CLI, route tests and API docs | Read/admin interface and final HTTP behavior decisions |
| 1.5 | Dockerfile, 3–5 node Compose, add-node helper, server lifecycle | Docker runtime verification; bootstrap/address convention review |
| 1.6 | Build/vet/test/race and generation/Compose CI workflow | Real PR execution, branch protection and approval policy |
| 1.7 | Preserved plan, ADR process, scope reference, diagram backlog | Actual SDD, ratification, real reviewers |

WAL record constants and test-build fault-hook placeholder appear only in the
plan's ownership prose; exact shape/location await the referenced SDD. No format
or injection framework was invented. Optional CI lint awaits a validated pinned
golangci-lint version. The remote is DynamicRaftKV/dynamic-raft-kv; its original
history, MIT license and proposal are preserved. No review approvals are claimed.
Stage 2 was subsequently authorized and implemented (see
stage2-status.md and ADR-0003). Stage 3 has not begun; Stage 1 freeze remains open.

## Local verification

- Go 1.27.1: `make build vet test race` passed.
- HTTP schema/stub tests passed; gRPC in-memory calls to all four RPCs returned
  Unimplemented as required.
- Protobuf regeneration reproduced both generated files byte-for-byte using
  protoc 33.0, protoc-gen-go v1.36.10 and protoc-gen-go-grpc 1.5.1.
- Local process smoke check passed: HTTP/gRPC listeners bound loopback ports,
  raftctl status/join/remove reported 501 and exited nonzero, and SIGTERM
  stopped the server with exit status zero.
- The local main branch is based on the existing GitHub repository history.
- Docker/Compose are not installed here, so the Docker smoke workflow has not
  been executed locally. Hosted CI and branch protection remain unverified.

Go was downloaded to `/private/tmp/raftkv-toolchain/go`; protoc and generators
are under `/private/tmp/raftkv-protoc` and `/private/tmp/raftkv-tools`. This is
temporary verification tooling, not a system installation. Dependencies were
cached in `/private/tmp/raftkv-gopath` and build outputs are ignored under bin/.
