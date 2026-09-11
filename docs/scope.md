# Scope guardrail

Primary source: implementation-plan.md. The complete SDD exclusions have not
been supplied, so this is not claimed to reproduce SDD sections 4/5/43.

Current authorized work: Stage 1 repository, type/interface drafts, protocol and
HTTP/CLI stubs, deployment/CI scaffolding, review documentation, and the owner's
subsequently authorized Stage 2 standalone engine.

Stage 2 implements map-backed PUT/GET/DELETE, deterministic Apply, KV-only snapshot
and replacement restore, tests, and interface-driven test harness.

Do not implement Stage 3 in this scope. No TTL, CAS, Raft algorithms, WAL,
real membership changes, fault injection, authentication or additional features.
Authentication is explicitly deferred. Testing helpers must not ship business
logic or bypass consensus. Future UML work is tracked in uml/README.md.
