# Stage 6: live membership

Implemented locally: 6.1 durable configuration entries/recovery, 6.2 learner
admission, 6.3 snapshot catch-up and gated promotion, and 6.4 core plus real-process
membership tests. Review/remote CI acceptance is separate from implementation.
See [ADR-0008](adr/0008-dynamic-membership.md) for the exact algorithm and limits.

## Run a fourth node without Docker

First start the three durable nodes using [Stage 5 instructions](stage5-status.md).
Keep their original cluster JSON unchanged. The example below assumes it is
`cluster.json`, contains node1–node3, and node1's HTTP port is 8101. Substitute
your actual seed path, leader URL and unused listening ports.

```sh
./bin/raftkv --join --node-id node4 --cluster-config cluster.json \
  --data-dir ./data/node4 --http-addr 127.0.0.1:8104 \
  --grpc-addr 127.0.0.1:9104
```

In another terminal:

```sh
./bin/raftctl --endpoint http://127.0.0.1:8101 join node4 \
  127.0.0.1:9104 http://127.0.0.1:8104
./bin/raftctl --endpoint http://127.0.0.1:8104 status
./bin/raftctl --endpoint http://127.0.0.1:8101 promote node4
./bin/raftctl --endpoint http://127.0.0.1:8101 status
```

Promotion returns a conflict until the learner catches up. Wait and retry after
checking status. Join success alone does not mean promotion. After promotion,
wait for `membership_pending: false` on the leader and `learner: false` for node4.
The joining process must use the same seed file and `--join` after restart.
Do not add node4 to that seed file: membership now comes from its durable state.

Remove a node only while quorum is available:

```sh
./bin/raftctl --endpoint http://127.0.0.1:8101 remove node4
./bin/raftctl --endpoint http://127.0.0.1:8101 status
```

Wait for finalization, then stop the retired process. Use a new ID and fresh data
directory for replacements. A timeout has an uncertain outcome; inspect leader
status before retrying. Keep peer endpoints on a trusted network.

## Validation

```sh
make build vet test race
go test -race ./cmd/raftkv -run TestDynamicMembershipProcess -count=3
```

The second test uses four independent child processes, real HTTP/gRPC, real WAL
directories and hard process kills. It verifies join, promotion, leader failure,
full restart and retained key-value data. The standalone `/demo/` workspace does
not control these processes. Its live cluster integration belongs to Stage 8.

Next: consensus review/CI, Stage 7 fault histories and metrics, Stage 4.4 durable
Docker Compose acceptance, then Stage 8 cluster dashboard and evaluation.
