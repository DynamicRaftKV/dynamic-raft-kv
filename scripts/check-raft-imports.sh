#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
deps=$("${GO:-go}" list -deps ./internal/raft)
if printf '%s\n' "$deps" | awk '/^github\.com\/DynamicRaftKV\/dynamic-raft-kv\/(internal\/(transport|api|membership|demo)|proto)(\/|$)|^net($|\/)|^google.golang.org\/(grpc|protobuf)($|\/)/ { print; bad=1 } END { exit !bad }'; then
  echo 'raft import boundary violated' >&2
  exit 1
fi
