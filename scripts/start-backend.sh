#!/usr/bin/env bash
set -euo pipefail
SOREN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SOREN_ROOT/backend"
# SQLite and the in-process cache are defaults; PostgreSQL/Redis are optional.
# The Go configuration loader reads backend/.env without executing shell input.
exec go run ./cmd/server
