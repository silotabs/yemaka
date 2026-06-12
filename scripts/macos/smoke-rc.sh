#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

: "${GOCACHE:=/private/tmp/yemaka-go-build-cache}"
: "${YEMAKA_SMOKE_MODEL:=}"

if [ -z "${YEMAKA_HOME:-}" ]; then
  YEMAKA_HOME=$(mktemp -d "${TMPDIR:-/private/tmp}/yemaka-rc-smoke.XXXXXX")
fi

export YEMAKA_HOME
export GOCACHE

run() {
  printf '\n==> %s\n' "$*"
  "$@"
}

run go test ./internal/config ./internal/memory ./internal/rag ./internal/workspace ./internal/safety ./internal/connectors ./internal/extensions ./internal/internet ./internal/scheduler ./internal/heartbeat ./internal/evaluation ./internal/release ./internal/tools
run npm --prefix frontend run test:smoke
run go run ./cmd/yemaka doctor
run go run ./cmd/yemaka tool doctor-status
run go run ./cmd/yemaka tool project-map .
run go run ./cmd/yemaka tool symbol-search Run .
run go run ./cmd/yemaka tool secret-scan .
run go run ./cmd/yemaka tool patch-preview smoke-preview.md "Yemaka smoke preview"
run go run ./cmd/yemaka connector list
run go run ./cmd/yemaka skill validate project_explainer
run go run ./cmd/yemaka memory write preference "RC smoke prefers local-first bounded context"
run go run ./cmd/yemaka memory search "bounded context"
run go run ./cmd/yemaka ingest ./docs
run go run ./cmd/yemaka rag search "model routing"
run go run ./cmd/yemaka eval run --mode low-memory --skip-model
if [ ! -x "build/bin/Yemaka.app/Contents/MacOS/Yemaka" ]; then
  run scripts/macos/build-app.sh
fi
run go run ./cmd/yemaka release check

if [ -n "$YEMAKA_SMOKE_MODEL" ]; then
  run go run ./cmd/yemaka model set low_memory "$YEMAKA_SMOKE_MODEL"
  run go run ./cmd/yemaka eval run --mode low-memory --model "$YEMAKA_SMOKE_MODEL"
  run go run ./cmd/yemaka chat "Say READY in one short sentence."
fi

printf '\nRC smoke complete. YEMAKA_HOME=%s\n' "$YEMAKA_HOME"
