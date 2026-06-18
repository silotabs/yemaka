#!/bin/sh
set -eu
export COPYFILE_DISABLE=1

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

APP_PATH="${APP_PATH:-build/bin/Yemaka.app}"
DIST_DIR="${DIST_DIR:-dist/macos}"
VERSION="${VERSION:-1.0-rc}"
ARCH="$(uname -m)"
DMG_NAME="Yemaka-${VERSION}-macos-${ARCH}.dmg"
DMG_PATH="$DIST_DIR/$DMG_NAME"
STAGING_DIR="$DIST_DIR/staging"
MANIFEST_PATH="$DIST_DIR/manifest.json"

if [ ! -d "$APP_PATH" ]; then
  echo "Yemaka.app not found at $APP_PATH. Run scripts/macos/build-app.sh first." >&2
  exit 1
fi

if [ ! -x "$APP_PATH/Contents/MacOS/Yemaka" ]; then
  echo "Yemaka executable missing at $APP_PATH/Contents/MacOS/Yemaka. Run scripts/macos/build-app.sh first." >&2
  exit 1
fi

if ! command -v hdiutil >/dev/null 2>&1; then
  echo "hdiutil is required to package a macOS DMG." >&2
  exit 1
fi

rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR" "$DIST_DIR"
cp -R "$APP_PATH" "$STAGING_DIR/Yemaka.app"
ln -s /Applications "$STAGING_DIR/Applications"
find "$STAGING_DIR" -name .DS_Store -type f -delete
find "$STAGING_DIR" -name __MACOSX -type d -prune -exec rm -rf {} \;
find "$STAGING_DIR" \( -path '*/frontend/playwright-report' -o -path '*/frontend/test-results' -o -path '*/node_modules' \) -type d -prune -exec rm -rf {} \;
find "$STAGING_DIR" \( -name '*.sqlite' -o -name '*.sqlite3' -o -name '*.db' -o -name '*.db-shm' -o -name '*.db-wal' -o -name '*.log' -o -name '*.tmp' \) -type f -delete

rm -f "$DMG_PATH" "$DMG_PATH.sha256" "$MANIFEST_PATH"
hdiutil create \
  -volname "Yemaka" \
  -srcfolder "$STAGING_DIR" \
  -ov \
  -format UDZO \
  "$DMG_PATH"

if command -v shasum >/dev/null 2>&1; then
  CHECKSUM=$(shasum -a 256 "$DMG_PATH" | awk '{print $1}')
elif command -v openssl >/dev/null 2>&1; then
  CHECKSUM=$(openssl dgst -sha256 "$DMG_PATH" | awk '{print $2}')
else
  echo "shasum or openssl is required to write checksums." >&2
  exit 1
fi

printf '%s  %s\n' "$CHECKSUM" "$DMG_NAME" > "$DMG_PATH.sha256"
SIZE_BYTES=$(wc -c < "$DMG_PATH" | tr -d ' ')
CREATED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
CODESIGN_STATUS="unsigned"
if codesign -dv "$APP_PATH" >/dev/null 2>&1; then
  CODESIGN_STATUS="signed"
fi

cat > "$MANIFEST_PATH" <<EOF
{
  "name": "Yemaka",
  "version": "$VERSION",
  "platform": "macos",
  "arch": "$ARCH",
  "created_at": "$CREATED_AT",
  "app_path": "$APP_PATH",
  "dmg": "$DMG_NAME",
  "size_bytes": $SIZE_BYTES,
  "sha256": "$CHECKSUM",
  "codesign_status": "$CODESIGN_STATUS",
  "notarization_status": "not_submitted"
}
EOF

rm -rf "$STAGING_DIR"

echo "DMG: $DMG_PATH"
echo "SHA256: $DMG_PATH.sha256"
echo "Manifest: $MANIFEST_PATH"
