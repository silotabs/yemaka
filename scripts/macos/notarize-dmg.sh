#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

DMG_PATH="${DMG_PATH:-}"
APPLE_ID="${APPLE_ID:-}"
TEAM_ID="${APPLE_TEAM_ID:-}"
PASSWORD="${APPLE_APP_SPECIFIC_PASSWORD:-}"

if [ -z "$DMG_PATH" ]; then
  DMG_PATH=$(find dist/macos -name 'Yemaka-*.dmg' -type f -print 2>/dev/null | sort | tail -n 1 || true)
fi

if [ -z "$DMG_PATH" ] || [ ! -f "$DMG_PATH" ]; then
  echo "DMG not found. Run scripts/macos/package-dmg.sh first or set DMG_PATH." >&2
  exit 1
fi

if [ -z "$APPLE_ID" ] || [ -z "$TEAM_ID" ] || [ -z "$PASSWORD" ]; then
  echo "Set APPLE_ID, APPLE_TEAM_ID, and APPLE_APP_SPECIFIC_PASSWORD to notarize." >&2
  exit 1
fi

xcrun notarytool submit "$DMG_PATH" \
  --apple-id "$APPLE_ID" \
  --team-id "$TEAM_ID" \
  --password "$PASSWORD" \
  --wait

xcrun stapler staple "$DMG_PATH"
xcrun stapler validate "$DMG_PATH"
echo "notarized: $DMG_PATH"
