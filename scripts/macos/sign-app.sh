#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

APP_PATH="${APP_PATH:-build/bin/Yemaka.app}"
IDENTITY="${DEVELOPER_ID_APPLICATION:-}"
SANDBOX="${SANDBOX:-0}"
if [ "${ENTITLEMENTS+x}" ]; then
  ENTITLEMENTS="$ENTITLEMENTS"
elif [ "$SANDBOX" = "1" ]; then
  ENTITLEMENTS="build/darwin/entitlements.sandbox.plist"
else
  ENTITLEMENTS="build/darwin/entitlements.plist"
fi

if [ ! -d "$APP_PATH" ]; then
  echo "Yemaka.app not found at $APP_PATH. Run scripts/macos/build-app.sh first." >&2
  exit 1
fi

if [ ! -f "$ENTITLEMENTS" ]; then
  echo "Entitlements file not found: $ENTITLEMENTS" >&2
  exit 1
fi

if [ "$SANDBOX" = "1" ] && ! grep -q "com.apple.security.app-sandbox" "$ENTITLEMENTS"; then
  echo "SANDBOX=1 requires App Sandbox entitlements." >&2
  exit 1
fi

if [ -z "$IDENTITY" ]; then
  echo "Set DEVELOPER_ID_APPLICATION to your Developer ID Application signing identity." >&2
  exit 1
fi

codesign --force --deep --options runtime --timestamp --entitlements "$ENTITLEMENTS" --sign "$IDENTITY" "$APP_PATH"

codesign --verify --deep --strict --verbose=2 "$APP_PATH"
echo "signed: $APP_PATH"
echo "entitlements: $ENTITLEMENTS"
if [ "$SANDBOX" = "1" ]; then
  echo "sandbox: enabled"
else
  echo "sandbox: disabled"
fi
