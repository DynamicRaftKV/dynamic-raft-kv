# KV snapshot format, version 1

The payload is UTF-8 JSON:

```json
{"version":1,"data":{"hello":"world","empty":""}}
```

An empty store is `{"version":1,"data":{}}`. Version and data are required;
null data is invalid. Keys are nonempty strings and values are strings, including
empty strings. JSON escaping handles embedded control characters and Unicode.
Snapshot uses encoding/json, which orders map keys; byte output is repeatable
for a given logical state. The caller owns the returned byte buffer.

Restore accepts version 1 only and rejects malformed/trailing JSON, unknown
envelope fields, nulls, invalid raw UTF-8, empty keys and wrong field types. It
validates into a fresh map before publishing, so errors preserve the old state.
It replaces all keys; it never merges. Input bytes are not retained. Snapshot
and Restore are safe against concurrent access, but the future Raft caller
must coordinate snapshots with application order and applied position.

No index, term, membership, checksum, file path, or persistence mechanism is in
this payload. Stage 5 will wrap it with Raft-owned metadata. No streaming or
compression is implemented. Changing this format requires an explicit version
and compatibility decision before existing snapshots are consumed differently.
