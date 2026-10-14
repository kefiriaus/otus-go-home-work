#!/bin/sh
set -eu
calendar_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
PATH="$calendar_root/bin:$PATH"
export PATH
PROTOC=${PROTOC:-protoc}
command -v "$PROTOC" >/dev/null 2>&1 || { echo "Install protoc or set PROTOC to its path" >&2; exit 1; }
for plugin in protoc-gen-go protoc-gen-go-grpc; do
  command -v "$plugin" >/dev/null 2>&1 || { echo "Run make install-generate-deps" >&2; exit 1; }
done
cd "$calendar_root"
exec "$PROTOC" -I api --go_out=. \
  --go_opt=module=github.com/fixme_my_friend/hw12_13_14_15_calendar \
  --go-grpc_out=. --go-grpc_opt=module=github.com/fixme_my_friend/hw12_13_14_15_calendar \
  api/EventService.proto
