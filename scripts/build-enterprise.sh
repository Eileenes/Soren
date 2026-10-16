#!/usr/bin/env bash
set -euo pipefail

SOREN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOREN_IMAGE="${1:-}"
if [[ ! "$SOREN_IMAGE" =~ ^portus\.in\.chaitin\.net/[a-z0-9._-]+/[a-z0-9._/-]+:[a-zA-Z0-9._-]+$ ]] || [[ "$SOREN_IMAGE" == *:latest ]]; then
  echo 'Usage: bash scripts/build-enterprise.sh portus.in.chaitin.net/<project>/soren-enterprise:<version>' >&2
  echo 'Provide an explicit version tag, not latest. This command builds locally; it does not push.' >&2
  exit 1
fi

docker buildx build --platform linux/amd64 --load \
  -f "$SOREN_ROOT/deploy/enterprise/Dockerfile" -t "$SOREN_IMAGE" "$SOREN_ROOT"
echo "Built $SOREN_IMAGE (linux/amd64). Push this version separately when ready."
