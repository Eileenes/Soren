#!/usr/bin/env bash
set -euo pipefail
SOREN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SOREN_ROOT/frontend"
if [[ ! -d node_modules ]]; then npm ci; fi
exec npm run tauri:dev
