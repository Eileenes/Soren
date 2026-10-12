#!/usr/bin/env bash
set -euo pipefail
SOREN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SOREN_ROOT/frontend"
if [[ ! -d node_modules ]]; then npm ci; fi
npm run tauri:build -- "$@"
echo "Soren bundles: $SOREN_ROOT/frontend/src-tauri/target/release/bundle/"
