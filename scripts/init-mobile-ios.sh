#!/usr/bin/env bash
set -euo pipefail
SOREN_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SOREN_ROOT"
if [[ ! -f mobile/src-tauri/gen/apple/project.yml ]]; then
  npm --prefix mobile run tauri -- ios init --ci
fi
# Xcode embeds these as simulated entitlements on Simulator, and signs them
# with the selected developer team's prefix for a physical device.
python3 - <<'PY'
from pathlib import Path
p = Path('mobile/src-tauri/gen/apple/project.yml')
s = p.read_text()
needle = 'path: soren-mobile_iOS/soren-mobile_iOS.entitlements'
if 'keychain-access-groups:' not in s:
    if needle not in s:
        raise SystemExit('Cannot find generated iOS entitlements; review project.yml')
    s = s.replace(needle, needle + '\n      properties:\n        application-identifier: $(AppIdentifierPrefix)$(PRODUCT_BUNDLE_IDENTIFIER)\n        keychain-access-groups:\n          - $(AppIdentifierPrefix)$(PRODUCT_BUNDLE_IDENTIFIER)')
    p.write_text(s)
PY
xcodegen generate --spec mobile/src-tauri/gen/apple/project.yml
npm --prefix mobile run tauri -- icon ../assets/app-icon-1024.png -o src-tauri/icons --ios-color '#FFFFFF'
