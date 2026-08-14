#!/usr/bin/env bash
# Copy Icon.png (from FyneApp.toml) into the embedded UI asset used by go build / go run.
# Run this after replacing Icon.png, then rebuild or re-run the app.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"
build_common_init
build_read_fyne_metadata
build_ensure_icon
build_sync_embedded_icon
echo "Done. Rebuild to apply: go run .   or   ./build/build.sh darwin" >&2
