#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
# Inspect production dependencies, not networking tools used by test runners.
deps=$("${GO:-go}" list -deps ./internal/state)
if printf '%s\n' "$deps" | awk '/^github\.com\/DynamicRaftKV\/dynamic-raft-kv\/internal\/(raft|transport|api|membership)(\/|$)|^net($|\/)|^google.golang.org\/grpc($|\/)/ { print; bad=1 } END { exit !bad }'; then
  echo 'state import boundary violated' >&2
  exit 1
fi
