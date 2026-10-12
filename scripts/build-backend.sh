#!/usr/bin/env bash
set -euo pipefail

SOREN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOREN_TARGET="${TAURI_ENV_TARGET_TRIPLE:-$(rustc --print host-tuple)}"

# Optional explicit platform arguments, useful for release pipelines.
if [[ $# -gt 0 ]]; then
  case "${1}/${2:-arm64}" in
    darwin/arm64|darwin/aarch64) SOREN_TARGET=aarch64-apple-darwin ;;
    darwin/amd64|darwin/x86_64) SOREN_TARGET=x86_64-apple-darwin ;;
    linux/arm64|linux/aarch64) SOREN_TARGET=aarch64-unknown-linux-gnu ;;
    linux/amd64|linux/x86_64) SOREN_TARGET=x86_64-unknown-linux-gnu ;;
    windows/amd64|windows/x86_64) SOREN_TARGET=x86_64-pc-windows-msvc ;;
    *) echo "Unsupported platform: $*" >&2; exit 1 ;;
  esac
fi
case "$SOREN_TARGET" in
  aarch64-apple-darwin) SOREN_GOOS=darwin; SOREN_GOARCH=arm64 ;;
  x86_64-apple-darwin) SOREN_GOOS=darwin; SOREN_GOARCH=amd64 ;;
  aarch64-unknown-linux-*) SOREN_GOOS=linux; SOREN_GOARCH=arm64 ;;
  x86_64-unknown-linux-*) SOREN_GOOS=linux; SOREN_GOARCH=amd64 ;;
  x86_64-pc-windows-*) SOREN_GOOS=windows; SOREN_GOARCH=amd64 ;;
  *) echo "Unsupported Tauri target: $SOREN_TARGET" >&2; exit 1 ;;
esac
SOREN_EXT=""
if [[ "$SOREN_GOOS" == windows ]]; then SOREN_EXT=.exe; fi
SOREN_OUTPUT="$SOREN_ROOT/frontend/src-tauri/binaries/soren-server-$SOREN_TARGET$SOREN_EXT"
mkdir -p "$(dirname "$SOREN_OUTPUT")"
echo "Building Soren backend: $SOREN_GOOS/$SOREN_GOARCH"
cd "$SOREN_ROOT/backend"
SOREN_LDFLAGS="-s -w"
if [[ -n "${SOREN_RELEASE_RELAY_URL:-}" || -n "${SOREN_RELEASE_MOBILE_URL:-}" ]]; then
  python3 - <<'CHECK_ENDPOINTS'
import os
from urllib.parse import urlsplit
for name in ('SOREN_RELEASE_RELAY_URL', 'SOREN_RELEASE_MOBILE_URL'):
    value = os.environ.get(name, '')
    u = urlsplit(value)
    if u.scheme != 'https' or not u.hostname or u.username or u.password or u.query or u.fragment or u.path not in ('', '/') or any(c.isspace() or c in "'\\\"" for c in value):
        raise SystemExit(f'{name} must be a plain HTTPS origin')
CHECK_ENDPOINTS
  SOREN_LDFLAGS+=" -X github.com/soren/backend/internal/mobile.DefaultRelayURL=${SOREN_RELEASE_RELAY_URL%/} -X github.com/soren/backend/internal/mobile.DefaultMobileURL=${SOREN_RELEASE_MOBILE_URL%/}"
fi
CGO_ENABLED=0 GOOS="$SOREN_GOOS" GOARCH="$SOREN_GOARCH" \
  go build -trimpath -ldflags="$SOREN_LDFLAGS" -o "$SOREN_OUTPUT" ./cmd/server
chmod +x "$SOREN_OUTPUT"
echo "Backend ready: $SOREN_OUTPUT"
