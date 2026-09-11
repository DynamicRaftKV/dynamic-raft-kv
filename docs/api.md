# HTTP and raftctl draft contract

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
or any non-2xx status, including 501. Redirects are not followed in this skeleton.
No KV subcommands or authentication have been added.
