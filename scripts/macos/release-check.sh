#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

: "${GOCACHE:=/private/tmp/yemaka-go-build-cache}"
export GOCACHE

if [ "${WAILS_BIN:-}" ]; then
  :
elif command -v wails >/dev/null 2>&1; then
  WAILS_BIN=$(command -v wails)
else
  WAILS_BIN="$HOME/go/bin/wails"
fi

if [ ! -x "$WAILS_BIN" ]; then
  echo "wails executable not found. Set WAILS_BIN or install Wails." >&2
  exit 1
fi

go test ./...
GOCACHE=/private/tmp/yemaka-go-vet-cache go vet ./...
npm --prefix frontend run build
GOCACHE=/private/tmp/yemaka-wails-go-build-cache "$WAILS_BIN" build -clean
go run ./cmd/yemaka eval run --mode low-memory
go run ./cmd/yemaka release check
