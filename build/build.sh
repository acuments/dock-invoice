#!/usr/bin/env bash
# Automated release build for Invoice Generator (Go + Fyne).
#
# Runs tests, packages native binaries, optionally cross-builds via fyne-cross,
# and writes versioned archives under dist/.
#
# Usage:
#   build/build.sh [options] [targets...]
#
# Targets (default: native on current OS, plus windows+linux when Docker is available on macOS):
#   native   — package for the host OS only
#   darwin   — macOS .app (native on Mac only)
#   windows  — Windows via fyne-cross (Docker) or native on Windows
#   linux    — Linux via fyne-cross (Docker) or native on Linux
#   all      — every target available on this host
#
# Options:
#   --skip-tests     Do not run go test ./...
#   --out DIR        Base output directory (default: dist)
#   --no-archive     Skip creating .zip / .tar.gz release files
#   --no-cross       Do not attempt fyne-cross builds (native only)
#   -h, --help       Show this help
#
# Examples:
#   ./build/build.sh                    # smart default for current machine
#   ./build/build.sh all                # macOS: darwin + windows + linux (needs Docker)
#   ./build/build.sh native --skip-tests
#   ./build/build.sh darwin windows --out /tmp/releases
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

SKIP_TESTS=0
NO_ARCHIVE=0
NO_CROSS=0
OUT_BASE="dist"
TARGETS=()

usage() {
  sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-tests) SKIP_TESTS=1; shift ;;
    --no-archive) NO_ARCHIVE=1; shift ;;
    --no-cross) NO_CROSS=1; shift ;;
    --out)
      OUT_BASE="${2:-}"
      if [[ -z "$OUT_BASE" ]]; then
        echo "error: --out requires a directory" >&2
        exit 1
      fi
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    darwin|windows|linux|native|all)
      TARGETS+=("$1")
      shift
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

build_common_init
build_read_fyne_metadata
build_ensure_icon
build_sync_embedded_icon

HOST_OS="$(build_host_os)"
if [[ ${#TARGETS[@]} -eq 0 ]]; then
  if [[ "$HOST_OS" == darwin ]] && [[ "$NO_CROSS" -eq 0 ]] && build_check_docker; then
    TARGETS=(all)
  else
    TARGETS=(native)
  fi
fi

want_darwin=0
want_windows=0
want_linux=0

for t in "${TARGETS[@]}"; do
  case "$t" in
    native)
      case "$HOST_OS" in
        darwin) want_darwin=1 ;;
        windows) want_windows=1 ;;
        linux) want_linux=1 ;;
        *) echo "error: unsupported host OS for native build" >&2; exit 1 ;;
      esac
      ;;
    darwin) want_darwin=1 ;;
    windows) want_windows=1 ;;
    linux) want_linux=1 ;;
    all)
      want_darwin=1
      want_windows=1
      want_linux=1
      ;;
  esac
done

OUT_DIR="$OUT_BASE/${APP_SLUG}-${RELEASE_TAG}"
STAGING="$OUT_DIR/staging"
ARCHIVE_DIR="$OUT_DIR/archives"
mkdir -p "$STAGING"

echo "Building $APP_NAME $RELEASE_TAG on $HOST_OS" >&2
echo "Icon: $APP_ICON | ID: $APP_ID" >&2
echo "Staging: $STAGING" >&2

if [[ "$SKIP_TESTS" -eq 0 ]]; then
  build_run_tests
fi

build_clean_staging

# --- macOS -----------------------------------------------------------------
if [[ "$want_darwin" -eq 1 ]]; then
  if [[ "$HOST_OS" != darwin ]]; then
    echo "warning: darwin target skipped — run on macOS or use CI for native macOS builds" >&2
  else
    build_package_native darwin
    build_collect_app_bundle "$STAGING"
  fi
fi

# --- Windows ----------------------------------------------------------------
if [[ "$want_windows" -eq 1 ]]; then
  if [[ "$HOST_OS" == windows ]]; then
    build_package_native windows
    if compgen -G "$BUILD_ROOT/*.exe" > /dev/null; then
      mkdir -p "$STAGING/windows-native"
      mv "$BUILD_ROOT"/*.exe "$STAGING/windows-native/"
    fi
  elif [[ "$NO_CROSS" -eq 0 ]]; then
    if build_check_docker; then
      build_package_fyne_cross windows
      build_copy_fyne_cross_output "$STAGING"
    else
      echo "warning: windows target skipped — start Docker Desktop, then re-run" >&2
    fi
  else
    echo "warning: windows target skipped — cross-build disabled (--no-cross)" >&2
  fi
fi

# --- Linux ------------------------------------------------------------------
if [[ "$want_linux" -eq 1 ]]; then
  if [[ "$HOST_OS" == linux ]]; then
    build_package_native linux
    if compgen -G "$BUILD_ROOT/dock-invoice" > /dev/null || compgen -G "$BUILD_ROOT/dock-invoice.tar.xz" > /dev/null; then
      mkdir -p "$STAGING/linux-native"
      mv "$BUILD_ROOT/dock-invoice" "$STAGING/linux-native/" 2>/dev/null || true
      mv "$BUILD_ROOT/dock-invoice.tar.xz" "$STAGING/linux-native/" 2>/dev/null || true
    fi
  elif [[ "$NO_CROSS" -eq 0 ]]; then
    if build_check_docker; then
      build_package_fyne_cross linux
      build_copy_fyne_cross_output "$STAGING"
    else
      echo "warning: linux target skipped — start Docker Desktop, then re-run" >&2
    fi
  else
    echo "warning: linux target skipped — cross-build disabled (--no-cross)" >&2
  fi
fi

if [[ "$NO_ARCHIVE" -eq 0 ]]; then
  build_archive_release "$STAGING" "$ARCHIVE_DIR"
fi

build_print_summary
