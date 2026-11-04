#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/.."
mkdir -p data/demo-cluster
pids=()
cleanup() { for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done; }
trap cleanup EXIT INT TERM
for i in 1 2 3 4; do
  extra=()
  if [ "$i" = 4 ]; then extra=(--join); fi
  ./bin/raftkv --node-id "node$i" --cluster-config configs/local-three-node.json \
    --data-dir "data/demo-cluster/node$i" --http-addr "127.0.0.1:810$i" \
    --grpc-addr "127.0.0.1:910$i" "${extra[@]}" >"data/demo-cluster/node$i.log" 2>&1 &
  pids+=("$!")
  echo "$!" >"data/demo-cluster/node$i.pid"
done
./bin/raftkv --demo --demo-cluster-config configs/local-demo-nodes.json \
  --http-addr 127.0.0.1:8080 --grpc-addr 127.0.0.1:9080 &
pids+=("$!")
echo "Local console: http://127.0.0.1:8080/demo/#cluster"
echo "Node logs and PID files: data/demo-cluster/"
wait
