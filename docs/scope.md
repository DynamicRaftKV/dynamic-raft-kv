# Scope guardrail

Primary source: implementation-plan.md. The complete SDD exclusions have not
been supplied, so this is not claimed to reproduce SDD sections 4/5/43.

Current authorized work: Stage 1 repository, type/interface drafts, protocol and
HTTP/CLI stubs, deployment/CI scaffolding, review documentation, and the owner's
subsequently authorized Stage 2 standalone engine, and Stage 3 consensus core
(owner authorization 27 October 2026; see stage3-plan.md and ADR-0004).
The owner's subsequent request to verify Stage 3 and proceed authorizes the
Stage 4 network/client adapters; see ADR-0005 and stage4-status.md. Docker Compose
deployment is deferred at the owner's request.
The next authorization adds Stage 5.1 WAL and 5.2 durable startup integration;
see ADR-0006. The owner's 29 October continuation authorizes Stage 5.3
snapshot/compaction work (ADR-0007). Dynamic membership remains the next stage.

Stage 2 implements map-backed PUT/GET/DELETE, deterministic Apply, KV-only snapshot
and replacement restore, tests, and interface-driven test harness.

Stage 3 adds fixed-membership elections/replication and an in-process test
cluster. Stage 4 adds gRPC dispatch, consensus HTTP handlers and KV CLI commands.
Production process wiring, WAL and snapshot transfer/compaction are implemented;
dynamic membership remains pending. No TTL, CAS, authentication or
additional features are authorized by this scope.
Authentication is explicitly deferred. Testing helpers must not ship business
logic or bypass consensus. Future UML work is tracked in uml/README.md.
