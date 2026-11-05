# Local console

Build using `make build`, then run `bash scripts/run-local-demo.sh` from the
repository. Open http://127.0.0.1:8080/demo/#cluster (not the HTML file).

The script starts nodes 1–3 on HTTP ports 8101–8103 and gRPC ports 9101–9103.
Node4 starts in joining mode on 8104/9104. The console uses 8080/9080. Its
allowlist is `configs/local-demo-nodes.json`; this does not modify membership.
Raft data and node logs remain in `data/demo-cluster/`. Existing membership is
recovered on restart. Stop the script with Ctrl-C; it stops the processes it owns.

## Interface

- **Live cluster:** real leader, term, voter count, commit/applied indices and
  learner lag. Updates every two seconds while visible.
- **Replicated commands:** PUT/GET/DELETE go to the observed leader through the
  normal Raft HTTP API. Try key `greeting`, then GET it.
- **Membership:** select node4, Join as learner, wait for catch-up, then Promote
  to voter. Wait for Stable before another change. A joining process that was
  previously promoted will already be a voter after restart.
- **Remove member:** requires confirmation and quorum. Removed IDs cannot be
  reused. Removal is permanent membership administration, not a crash test.
- **KV workspace / Snapshots:** isolated, memory-only state and capture/restore.
  These controls do not restore arbitrary data into the durable cluster.

Live snapshots and WAL recovery happen in the backend. The console does not add
a bulk listing endpoint, metrics that are not implemented, arbitrary snapshot
restoration, or simulated process failure. The optional manager kills and restarts real, console-created processes. Node failures/elections become visible
when an actual node process stops. PID files identify the locally launched nodes;
restart a node with the same flags/data directory, retaining `--join` for node4.

The bridge only routes to configured nodes, refuses redirected requests and
blocks browser cross-site mutations. Keep this development console bound to
127.0.0.1. It is not an authenticated remote administration service.

## Automatic node creation and visible tests

The launcher now enables the local process manager with `--demo-node-seed
configs/local-three-node.json --demo-node-dir data/demo-managed`. This requires
the console HTTP listener to bind to 127.0.0.1. Each Create node assigns a random
unique identity and two available loopback ports, persists the registry, and
starts a joining process with its own durable directory. Membership is changed
separately through the existing Raft APIs. Repeat to grow beyond the original
four cards; practical capacity is limited by this machine, ports and resources.

See [the step-by-step demonstration](frontend-demonstration.md).

Only console-created children have process controls. Existing processes are not
adopted or killed. A normal console shutdown stops its children; enabled children
restart with their saved identities, ports and data when the console reopens.
An explicitly crashed child stays stopped until Restart process is selected.
Do not remove the registry or data to restart a member. A removed member ID must
never be reused. If the console itself is forcibly killed, children may remain
alive: stop those known processes before reopening the manager. Occupied saved
ports produce an error; they are never silently changed.

The manager is a local development convenience, not a distributed orchestrator.
Docker Compose remains a separate deployment/acceptance path.
