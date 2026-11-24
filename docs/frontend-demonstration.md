# Frontend demonstration

Open http://127.0.0.1:8080/demo/#cluster while the local demo launcher is running.
Use the served page, not the HTML file. Reload an old tab to load new controls.

1. Show **Current leader**, **Voting members**, **Membership: Stable**, and the
   quorum message. Reachability alone does not prove consensus progress.
2. Click **1 · Test PUT / GET / DELETE**. Show Passed and its evidence list:
   committed PUT, exact GET, DELETE, then GET 404. Every run uses a new key.
3. Click **Create node**. Show its unique ID, assigned HTTP/gRPC ports, and
   **Not a member** status. The new node is selected under Membership.
4. Under Membership, choose **Join as learner**, then **Apply membership
   change**. Wait for **Learner**, **0 behind**, and **Stable**. The voter count
   does not increase yet. Joining may catch up through a snapshot.
5. Choose **Promote to voter**, apply, and wait for Stable. The voter count
   increases by one. Repeat steps 3–5 to add more nodes, one change at a time.
6. Click **2 · Check replication progress**. It verifies a write/read and waits
   for every admitted node to apply through the captured commit index, then
   checks the retained value and cleans up. Applied indices are progress
   evidence, not independent state hashes or a proof of all Raft properties.
7. Select a console-created member under Membership. Click **3 · Restart
   selected managed node**, accept the confirmation, and show the evidence:
   acknowledged write, process offline, restart/catch-up, exact retained read,
   and cleanup. The test refuses to run without enough other reachable voters.
8. For a slower visual demonstration, use that node card's **Crash process**,
   observe Offline, run a replicated command while a majority remains, then
   **Restart process** and watch it catch up. Its ID, ports and data stay the same.
   Only if the crashed node was leader should an election be expected.

Do not use Remove member as a crash button: it changes consensus membership
and permanently retires that ID. Externally started nodes deliberately have no
process controls. Use their own terminals when demonstrating leader failure.

## Results and limits

On 2026-11-05, browser checks passed for CRUD, automatic creation, learner join,
voter promotion, and replication across five voters. The live API crash/restart
check verified Offline, recovery with the same identity/ports, applied-index
catch-up, and the existing `demo-welcome` value. The guided restart UI also completed with Passed and all six evidence steps
visible after the native confirmation. Go build/vet/test/race passed, including manager
ownership, distinct ports, persistent registry, and reopen intent tests.

These are real local processes. This is not unlimited scaling, a remote node
provisioner, a replacement for Docker Compose, or exhaustive failure testing.
Timeouts can leave an operation's outcome unknown; inspect status before retrying.
