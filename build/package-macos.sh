#!/usr/bin/env bash
# Build and package the macOS app bundle natively.
#
# Wrapper around build/build.sh for backward compatibility.
# Usage: build/package-macos.sh [output-dir]
set -euo pipefail
OUT_DIR="${1:-dist}"
exec "$(dirname "$0")/build.sh" darwin --out "$OUT_DIR"
