#!/usr/bin/env bash
set -euo pipefail
SOREN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SOREN_ROOT"
node scripts/check-release.mjs
SOREN_VERSION="$(node -p "require('./package.json').version")"
SOREN_BUNDLE="${SOREN_BUNDLE_DIR:-frontend/src-tauri/target/release/bundle}"
SOREN_DMG="$SOREN_BUNDLE/dmg/Soren_${SOREN_VERSION}_aarch64.dmg"
SOREN_APP="$SOREN_BUNDLE/macos/Soren.app"
[[ -f "$SOREN_DMG" && -d "$SOREN_APP" ]] || { echo 'Build the macOS arm64 app and DMG first.' >&2; exit 1; }
[[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$SOREN_APP/Contents/Info.plist")" == "$SOREN_VERSION" ]]
lipo "$SOREN_APP/Contents/MacOS/soren" -verify_arch arm64
lipo "$SOREN_APP/Contents/MacOS/soren-server" -verify_arch arm64
codesign --verify --deep --strict "$SOREN_APP"
hdiutil verify "$SOREN_DMG" >/dev/null
mkdir -p release
cp "$SOREN_DMG" "release/Soren-${SOREN_VERSION}-macOS-arm64.dmg"
cd release
shasum -a 256 "Soren-${SOREN_VERSION}-macOS-arm64.dmg" > "SHA256SUMS-${SOREN_VERSION}.txt"
echo "Release package verified: Soren-${SOREN_VERSION}-macOS-arm64.dmg"
