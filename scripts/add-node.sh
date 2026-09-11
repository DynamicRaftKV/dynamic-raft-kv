#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
case "${1:-}" in
  4|5) ;;
  *) echo 'usage: sh scripts/add-node.sh 4|5' >&2; exit 2 ;;
esac
echo "Starting node$1 process only. Membership admission is unimplemented until Stage 6."
docker compose --profile "node$1" up -d --build "node$1"
