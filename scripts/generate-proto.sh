#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
# Install these pinned tools separately; generation never silently installs.
# protoc 33.0; protoc-gen-go v1.36.10; protoc-gen-go-grpc v1.5.1.
test "$(protoc --version)" = "libprotoc 33.0"
test "$(protoc-gen-go --version)" = "protoc-gen-go v1.36.10"
test "$(protoc-gen-go-grpc --version)" = "protoc-gen-go-grpc 1.5.1"
protoc -I proto --go_out=. --go_opt=module=github.com/DynamicRaftKV/dynamic-raft-kv \
  --go-grpc_out=. --go-grpc_opt=module=github.com/DynamicRaftKV/dynamic-raft-kv proto/raftkv.proto
