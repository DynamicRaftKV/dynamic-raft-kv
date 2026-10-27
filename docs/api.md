# HTTP and raftctl contract

## Stage 4 implementation

`NewRaftHandler(node)` implements PUT/GET/DELETE through Propose and local
cluster status. GET also goes through the log; no stale local reads are served.
Responses use `state.Result`: 200 for successful operations, 404 for missing
GET (`{"op":"GET"}`). An existing empty value has `found:true`, with value
omitted by the existing result codec. Invalid requests are 400. PUT body parsing
is limited to 1 MiB; encoded commands exceeding the core's 1 MiB limit are 413.

Follower responses use 307 Location when the leader address is known; otherwise
503. Proposal wait timeout/cancellation returns 504 and explicitly warns that
the outcome is unknown. Storage/node failures return 503. Join/remove remain
501. The executable uses `NewRaftHandler` in durable `--cluster-config` mode.
Without that flag it retains `NewHandler()` scaffold mode. The demo is separate.

The CLI now supports these forms against an attached Raft handler:

```sh
raftctl --endpoint http://localhost:8001 put greeting hello
raftctl --endpoint http://localhost:8001 get greeting
raftctl --endpoint http://localhost:8001 delete greeting
```

It follows bounded 307 leader redirects, preserving method/body, with no retries
after arbitrary failures. Keys are path-escaped; empty keys and standalone `.`
or `..` keys are rejected by the CLI. GET missing still exits nonzero (HTTP 404).
See ADR-0005 for the consistency, size-limit and integration decisions.

## Original scaffold behavior (still used by NewHandler)

Stage 1 exposes parsing and explicit stubs, not a working database API.

| Method | Route | JSON request | Stage 1 valid-request response |
|---|---|---|---|
| PUT | /kv/{key} | `{"value":"text"}` | 501 |
| GET | /kv/{key} | none | 501 |
| DELETE | /kv/{key} | none | 501 |
| GET | /cluster/status | none | 501 |
| POST | /cluster/join | `{"id":"node4","grpc_address":"node4:9004","http_address":"http://localhost:8004"}` | 501 |
| POST | /cluster/remove | `{"id":"node4"}` | 501 |

501 response: `{"error":"not implemented"}` with application/json.
Malformed JSON, unknown fields, trailing JSON, null/missing PUT value, or missing
join/remove fields return 400 with the same error envelope. Empty PUT values
are valid. Unsupported methods and unknown routes use Go ServeMux's 405/404
responses; these are not promised to use the JSON envelope. A key occupies one
URL path segment; full escaping/normalization policy is pending.

No stub calls Propose or alters membership. The missing read/admin surface is
recorded in contracts.md instead of a fake successful implementation.

Draft future response semantics: PUT and DELETE acknowledge, GET distinguishes
missing from empty value, status reports a local view. Exact success schemas,
missing-key HTTP status, redirect status, error codes, request limits, GET
consistency, and join/remove completion semantics require freeze review.

CLI forms (global flags precede subcommand):

```sh
raftctl --endpoint http://localhost:8001 --timeout 5s status
raftctl join node4 node4:9004 http://localhost:8004
raftctl remove node4
```

CLI prints response bodies to stdout and errors to stderr; it exits 1 for errors
or any non-2xx status, including 501. The original scaffold did not follow
redirects or offer KV subcommands; Stage 4 adds those as described above.
Authentication remains outside the current scope.
