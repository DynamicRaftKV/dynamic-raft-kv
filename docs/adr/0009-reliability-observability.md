# ADR-0009: Bounded proposal admission and local diagnostics

Status: implemented for review, 29 October 2026.

Protect an isolated leader from indefinitely appending client proposals by
checking capacity under the same node lock used for append. Default limits are
1024 uncommitted entries, 16 MiB of payload plus a 64-byte allowance per entry,
and 1024 pending callers. Zero configuration fields select those defaults;
negative pending limits are invalid. A rejected proposal does not enter the log.
HTTP exposes rejection as 429/Retry-After. Callers must still treat timeouts of
previously accepted proposals as unknown outcomes.

Recovery never discards durable entries just because they exceed new limits.
Replication and commitment drain an existing backlog; internal leadership no-ops
and membership finalization can proceed. This is an admission policy, not a hard
whole-process memory or disk quota. WAL growth from term/vote records, state
growth and transport memory require separate limits if the MVP needs them.

Raft exposes a locked local Diagnostics value and optional slog.Logger without
importing the HTTP or transport layers. The composition root supplies JSON
logging. Logs record transitions and boundaries, not command payloads. Synchronous
logger handlers must remain bounded/nonblocking in deployment. HTTP metrics and
WAL file-size inspection live in internal/observability, preserving import rules.
No metrics backend dependency or side database is added.

The finite-history checker is a test-only single-register model. It enumerates
orders compatible with invocation/return intervals, permits unknown-outcome
writes to be present or absent, and checks completed reads. Negative histories
test the oracle. Together with fault injection, recovery tests and race checks,
this supplies evidence, not a proof of arbitrary executions or production safety.
