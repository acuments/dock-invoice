#!/usr/bin/env bash
# Build and package Windows and/or Linux via fyne-cross (Docker).
#
# Wrapper around build/build.sh for backward compatibility.
# Usage: build/package-cross.sh [windows|linux|all] [output-dir]
set -euo pipefail
TARGET="${1:-all}"
OUT_DIR="${2:-dist}"
exec "$(dirname "$0")/build.sh" "$TARGET" --out "$OUT_DIR" --skip-tests
