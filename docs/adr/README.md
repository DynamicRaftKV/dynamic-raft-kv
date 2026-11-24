# Architecture decision index

These records explain the implemented boundaries and tradeoffs. Their presence
is not evidence of reviewer approval; retain each record's stated status.

| Record | Decision |
|---|---|
| [0001](0001-learner-as-attribute.md) | Learner as a membership attribute |
| [0002](0002-interface-contracts.md) | Shared interface contracts |
| [0003](0003-stage2-state-semantics.md) | Standalone state-machine semantics |
| [0004](0004-stage3-core-boundaries.md) | Raft core boundaries |
| [0005](0005-network-and-client-integration.md) | Network and client integration |
| [0006](0006-wal-and-durable-startup.md) | WAL and durable startup |
| [0007](0007-snapshots-and-compaction.md) | Snapshots and compaction |
| [0008](0008-dynamic-membership.md) | Dynamic membership |
| [0009](0009-reliability-observability.md) | Admission limits and local diagnostics |

## From a write to an observable commit

```mermaid
sequenceDiagram
    participant C as Client
    participant L as Leader
    participant D as Leader WAL
    participant F as Followers
    participant S as State machine
    C->>L: Propose command
    L->>D: Persist entry
    D-->>L: Durable append succeeds
    L->>F: Replicate entry
    F-->>L: Acknowledge durable matching prefix
    Note over L,F: Current-term quorum permits commit advance
    L->>L: Emit commit_advanced (source=quorum)
    L->>S: Apply committed entries in order
    S-->>L: Result
    L-->>C: Return result
    L->>F: Advertise commit in replication
    F->>F: Advance matched prefix; emit source=leader_append
    F->>F: Apply committed entries
```

This diagram shows a successful write. Without quorum, the leader cannot take
the commit step. A timeout is an unknown outcome: the durable proposal can still
commit later. The commit event precedes application and contains indices and
node metadata, never the command's keys or values. Followers can learn the commit
before or after the client receives its result; the arrows illustrate one order.
Snapshot installation is recorded separately and does not replay a commit event
for every entry covered by the snapshot.
