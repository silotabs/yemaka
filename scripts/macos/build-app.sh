#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT_DIR"

: "${GOCACHE:=/private/tmp/yemaka-wails-go-build-cache}"
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

render_info_plist() {
  plist_path=$1
  cat > "$plist_path" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleDisplayName</key>
  <string>Yemaka</string>
  <key>CFBundleExecutable</key>
  <string>Yemaka</string>
  <key>CFBundleIconFile</key>
  <string>iconfile</string>
  <key>CFBundleIdentifier</key>
  <string>com.yemaka.agent</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Yemaka</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0.0</string>
  <key>CFBundleVersion</key>
  <string>1.0.0</string>
  <key>LSApplicationCategoryType</key>
  <string>public.app-category.productivity</string>
  <key>LSMinimumSystemVersion</key>
  <string>13.0.0</string>
  <key>NSAppTransportSecurity</key>
  <dict>
    <key>NSAllowsLocalNetworking</key>
    <true/>
  </dict>
  <key>NSHighResolutionCapable</key>
  <true/>
  <key>NSHumanReadableCopyright</key>
  <string>Copyright 2026 Yemaka</string>
</dict>
</plist>
PLIST
}

build_icon() {
  icon_path=$1
  source_icon="build/appicon.png"

  if [ ! -f "$source_icon" ]; then
    echo "app icon source is missing: $source_icon" >&2
    exit 1
  fi

  if command -v sips >/dev/null 2>&1 && command -v iconutil >/dev/null 2>&1; then
    icon_tmp=$(mktemp -d "${TMPDIR:-/private/tmp}/yemaka-icon.XXXXXX")
    iconset="$icon_tmp/icon.iconset"
    mkdir -p "$iconset"

    sips -z 16 16 "$source_icon" --out "$iconset/icon_16x16.png" >/dev/null
    sips -z 32 32 "$source_icon" --out "$iconset/icon_16x16@2x.png" >/dev/null
    sips -z 32 32 "$source_icon" --out "$iconset/icon_32x32.png" >/dev/null
    sips -z 64 64 "$source_icon" --out "$iconset/icon_32x32@2x.png" >/dev/null
    sips -z 128 128 "$source_icon" --out "$iconset/icon_128x128.png" >/dev/null
    sips -z 256 256 "$source_icon" --out "$iconset/icon_128x128@2x.png" >/dev/null
    sips -z 256 256 "$source_icon" --out "$iconset/icon_256x256.png" >/dev/null
    sips -z 512 512 "$source_icon" --out "$iconset/icon_256x256@2x.png" >/dev/null
    sips -z 512 512 "$source_icon" --out "$iconset/icon_512x512.png" >/dev/null
    sips -z 1024 1024 "$source_icon" --out "$iconset/icon_512x512@2x.png" >/dev/null

    if iconutil -c icns "$iconset" -o "$icon_path" >/dev/null 2>&1; then
      rm -rf "$icon_tmp"
      return
    fi

    echo "iconutil could not create iconfile.icns; using appicon.png as development icon placeholder" >&2
    cp "$source_icon" "$icon_path"
    rm -rf "$icon_tmp"
    return
  fi

  cp "$source_icon" "$icon_path"
}

build_dev_bundle() {
  app_path="build/bin/Yemaka.app"
  macos_path="$app_path/Contents/MacOS"
  resources_path="$app_path/Contents/Resources"

  if [ -f "$app_path" ]; then
    echo "cannot create app bundle because $app_path is a file" >&2
    exit 1
  fi

  mkdir -p "$macos_path" "$resources_path"
  go build -buildvcs=false -tags desktop,wv2runtime.download,production -ldflags "-w -s" -o "$macos_path/Yemaka"
  render_info_plist "$app_path/Contents/Info.plist"
  build_icon "$resources_path/iconfile.icns"

  if command -v codesign >/dev/null 2>&1; then
    codesign --force --deep --options runtime --sign - "$app_path" >/dev/null 2>&1 || \
      codesign --force --deep --sign - "$app_path" >/dev/null 2>&1 || true
  fi
}

npm --prefix frontend run build
if "$WAILS_BIN" build -clean; then
  :
else
  echo "wails build failed; creating local unsigned development app bundle fallback" >&2
  build_dev_bundle
fi

release_home=$(mktemp -d "${TMPDIR:-/private/tmp}/yemaka-build-release-check.XXXXXX")
trap 'rm -rf "$release_home"' EXIT
YEMAKA_HOME="$release_home" GOCACHE=/private/tmp/yemaka-go-build-cache go run ./cmd/yemaka release check
