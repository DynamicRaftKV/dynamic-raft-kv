# ADR-0008: Durable learners and joint consensus

Status: implemented for review; consensus review and remote CI remain required.

## Decision

Use the original Raft joint-consensus protocol, with configurations effective
when appended durably, including uncommitted configurations. This is deliberately
different from etcd Raft's committed-entry `ApplyConfChange` integration. The
[etcd joint quorum](https://github.com/etcd-io/raft/blob/main/quorum/joint.go) and
[configuration changer](https://github.com/etcd-io/raft/blob/main/confchange/confchange.go)
were algorithm references; this project does not import etcd Raft.

The configuration has current peers, outgoing voters during a transition, and
retired IDs. Elections and replication commitment require a majority of each
voter set separately. Learners never count. A removal retains outgoing voters
as replication targets until finalization. No voter demotion or address mutation
is exposed. Retired identities cannot be reused; a replacement needs a new ID.

## Ordering and persistence

Configuration entries contain versioned operations: add_learner, promote,
remove_learner, remove_voter, finalize. They use the existing configuration entry
kind and WAL, not a side database. The leader admits one change at a time, after
committing an entry in its current term. Promotion requires the learner's match
index to reach the current last log index under the node lock.

Promotion and voter removal append a joint configuration. Only after that entry
is applied does the leader append the final configuration. The final entry uses
the new voting set. A new leader resumes finalization after its current-term
entry commits. A removed leader steps down after applying the final entry.
Removing the last voter is rejected. Removing an unavailable node still requires
the existing quorum; this API cannot repair permanent quorum loss.

Runtime separates the latest durable configuration from the applied one.
Truncating an uncommitted suffix recomputes active membership from the snapshot
and surviving entries. Snapshot metadata records only applied membership. Recovery
restores that metadata and then replays the entire durable configuration suffix
for voting rules without applying uncommitted user commands.

Version-2 snapshot membership contains both voter sets and retired IDs. Existing
version-1 snapshots remain readable when they match the immutable bootstrap file.
Do not downgrade a directory after membership changes. Snapshot creation follows
applied configuration entries, so admission can bootstrap a fresh learner with a
complete state image. The existing 1 MiB configuration and 32 MiB state limits
remain. Those limits can prevent further growth and must be observed by operators.

## Joining and administration

`--join` starts a node whose ID is absent from its seed file. Before receiving a
snapshot it refuses append traffic and cannot campaign. Start it with reachable
seed voters, then submit its ID and addresses to `/cluster/join`. Admission
always makes a learner. A joining node must retain the same seed file and use
`--join` on restart; recovered membership, not that flag, determines voting.

HTTP join/promote/remove return 202 once the requested entry commits. This does
not promise catch-up or finalization. Poll `membership_pending` on leader status;
inspect `peers`, `outgoing_voters`, `last_applied` and `configuration_index`.
The latter is the latest configuration entry or the snapshot recovery boundary.
409 rejects invalid, overlapping or premature changes; 307 redirects to a known
leader; 503 means unavailable; 504 means the outcome may be unknown. Inspect
status before retrying. No exactly-once administration is claimed.

RaftNode's existing interface remains compatible. The HTTP adapter optionally
uses the separate ChangeMembership method; the existing JoinCluster RPC admits
a learner. Promotion/removal use HTTP. Peer endpoints and administrators are
trusted: the MVP has no authentication. A snapshot may introduce a promoted
leader unknown to an out-of-date follower, but its configuration must include
the sender as a voter. This check is not authentication.

An offline removed node may retain an old configuration and try elections;
current members reject its votes/append traffic. Stop retired processes and
retain their data for inspection; do not treat deletion/reuse as rejoining.

## Evidence and remaining gates

Tests cover exhaustive small-set joint quorum combinations, strict encoding,
learner admission/catch-up/promotion, overlap rejection, retired-ID rejection,
suffix rollback, empty-voter prevention, leader removal, interrupted finalization
and recovery. A four-process HTTP/gRPC/WAL test joins and promotes node four,
kills the leader, and hard-restarts the cluster while preserving data/membership.
Stage 7 still adds broader fault histories, resource limits and observability.
Docker acceptance and real-cluster frontend controls remain separate phases.
