# Raft process visualizer

Open `/demo/#architecture` (Process visualizer in the sidebar). The old URL is
retained so existing bookmarks continue to work.

## Demonstration

1. Choose **Write: PUT to response** and 3 voters. Step through WAL persistence,
   follower persistence, quorum commitment, application and the client response.
   Notice that the last log index advances before commit, and apply follows commit.
2. Choose **Read through consensus**. Reads in this implementation use the log;
   this example does not pretend ReadIndex has been implemented.
3. Choose **Quorum loss and recovery**. The leader can append while isolated,
   but cannot commit. The caller times out with an unknown outcome.
4. Choose **Leader crash and failover**. Show a surviving majority electing a
   new leader and the restarted old leader catching up in the newer term.
5. Choose **Snapshot and catch-up**. Explain the checkpoint boundary, prefix
   compaction and complete snapshot installation for a lagging follower.
6. Choose **Join, promote and remove**, then 7 voters. A learner does not count
   toward quorum. Promotion to 8 voters requires both old (4/7) and new (5/8)
   majorities during joint consensus. Removal also uses joint then final configs.
7. Click **Read current status** to compare real roles, terms and indices.
   These observations are separate from fictional nodes N1–N7 and L.

Play/pause, previous/next, restart, playback speed and a keyboard-operable step
slider support presentation use. Playback stops at the end or when the tab/view
is hidden; reduced-motion preferences disable animated message dashes.

## Meaning and limitations

The animation contains scripted teaching examples, not a second Raft engine,
network capture or live timeline. It sends no writes, membership changes or
process-control requests. Example voter counts are 3/5/7; they do not constrain
the real cluster's node count. A single step can summarize multiple RPCs and
some valid interleavings. Only illustrated messages have arrows; missing arrows
do not imply missing replication in the implementation.

The status panel reads the existing allowlisted console API on demand, timestamps
its observation and clears previous results on failure. It does not infer packet
traffic, fsyncs, votes, per-entry client values or quorum readiness from polling.
Processes outside the observed leader's membership are identified explicitly.
No automatic refresh or live message recording is implied.

Twenty model tests (13 scenario checks plus seven dashboard checks) cover index
ordering, immutable frames, quorum-loss behavior, client response ordering,
learner/joint transitions and status interpretation. They validate teaching data;
backend safety remains covered by the independent Go and process test suites.
