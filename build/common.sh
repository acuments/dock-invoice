# Shared helpers for dock-invoice packaging scripts. Source from build/*.sh — do not execute directly.

build_common_init() {
  BUILD_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  cd "$BUILD_ROOT"
  FYNE_CMD=""
  FYNE_CROSS_CMD=""
  FYNE_CROSS_WORK_ROOT=""
  build_ensure_go_bin_on_path
}

build_read_fyne_metadata() {
  local details=0
  APP_WEBSITE=""
  APP_ICON=""
  APP_NAME=""
  APP_ID=""
  APP_VERSION=""
  APP_BUILD=""

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line//$'\r'/}"
    if [[ "$line" == "[Details]" ]]; then
      details=1
      continue
    fi
    if [[ $details -eq 0 ]]; then
      if [[ "$line" =~ ^Website[[:space:]]*=[[:space:]]*\"(.*)\" ]]; then
        APP_WEBSITE="${BASH_REMATCH[1]}"
      fi
      continue
    fi
    if [[ "$line" =~ ^Icon[[:space:]]*=[[:space:]]*\"(.*)\" ]]; then
      APP_ICON="${BASH_REMATCH[1]}"
    elif [[ "$line" =~ ^Name[[:space:]]*=[[:space:]]*\"(.*)\" ]]; then
      APP_NAME="${BASH_REMATCH[1]}"
    elif [[ "$line" =~ ^ID[[:space:]]*=[[:space:]]*\"(.*)\" ]]; then
      APP_ID="${BASH_REMATCH[1]}"
    elif [[ "$line" =~ ^Version[[:space:]]*=[[:space:]]*\"(.*)\" ]]; then
      APP_VERSION="${BASH_REMATCH[1]}"
    elif [[ "$line" =~ ^Build[[:space:]]*=[[:space:]]*([0-9]+) ]]; then
      APP_BUILD="${BASH_REMATCH[1]}"
    fi
  done < FyneApp.toml

  if [[ -z "$APP_ICON" ]]; then
    APP_ICON="Icon.png"
  fi
  if [[ -z "$APP_NAME" ]]; then
    APP_NAME="Invoice Generator"
  fi
  if [[ -z "$APP_ID" ]]; then
    APP_ID="com.dock.invoice.desktop"
  fi
  if [[ -z "$APP_VERSION" ]]; then
    APP_VERSION="0.0.0"
  fi
  if [[ -z "$APP_BUILD" ]]; then
    APP_BUILD="0"
  fi

  APP_SLUG="$(printf '%s' "$APP_NAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd 'a-z0-9-.')"
  RELEASE_TAG="${APP_VERSION}-b${APP_BUILD}"
}

build_ensure_icon() {
  if [[ ! -f "$APP_ICON" ]]; then
    echo "error: app icon not found at $APP_ICON (from FyneApp.toml)" >&2
    exit 1
  fi
}

build_sync_embedded_icon() {
  # The Fyne UI embeds internal/ui/assets/appicon.png at compile time.
  # Copy the packaging icon (FyneApp.toml → Icon.png) so go run / tests match
  # the shipped .app icon after you replace Icon.png.
  local embed_dest="$BUILD_ROOT/internal/ui/assets/appicon.png"
  if [[ ! -f "$APP_ICON" ]]; then
    return 0
  fi
  mkdir -p "$(dirname "$embed_dest")"
  cp -f "$APP_ICON" "$embed_dest"
  echo "Synced $APP_ICON → internal/ui/assets/appicon.png (embedded UI icon)" >&2
}

build_check_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "error: docker CLI not found. Install Docker Desktop for Mac:" >&2
    echo "  https://www.docker.com/products/docker-desktop/" >&2
    return 1
  fi

  local err_file
  err_file="$(mktemp)"
  if docker info >/dev/null 2>"$err_file"; then
    rm -f "$err_file"
    return 0
  fi

  echo "error: Docker is installed but the engine is not reachable." >&2
  if [[ -s "$err_file" ]]; then
    sed 's/^/  /' "$err_file" >&2
  fi
  rm -f "$err_file"

  case "$BUILD_ROOT" in
    /Volumes/*)
      echo "Note: repo is on an external volume ($BUILD_ROOT)." >&2
      echo "  fyne-cross builds will use a temporary copy under ~/.cache/." >&2
      ;;
  esac

  echo "" >&2
  echo "Try:" >&2
  echo "  1. Open Docker Desktop and wait until it shows \"Docker Desktop is running\"" >&2
  echo "  2. Then run: docker ps" >&2
  echo "  3. Re-run: ./build/build.sh windows" >&2
  echo "" >&2
  echo "If you use Colima instead of Docker Desktop:" >&2
  echo "  colima start && docker context use colima" >&2
  return 1
}

build_ensure_go_bin_on_path() {
  local gobin=""
  if [[ -n "${GOBIN:-}" ]]; then
    gobin="$GOBIN"
  else
    gobin="$(go env GOPATH)/bin"
  fi
  if [[ -n "$gobin" && -d "$gobin" ]]; then
    case ":$PATH:" in
      *":$gobin:"*) ;;
      *) export PATH="$gobin:$PATH" ;;
    esac
  fi
}

build_resolve_go_tool() {
  local name="$1"
  local install_pkg="$2"
  build_ensure_go_bin_on_path
  if command -v "$name" >/dev/null 2>&1; then
    command -v "$name"
    return 0
  fi
  local candidate="$(go env GOPATH)/bin/$name"
  if [[ -x "$candidate" ]]; then
    echo "$candidate"
    return 0
  fi
  echo "Installing $name ($install_pkg)..." >&2
  go install "$install_pkg@latest"
  build_ensure_go_bin_on_path
  if [[ -x "$candidate" ]]; then
    echo "$candidate"
    return 0
  fi
  if command -v "$name" >/dev/null 2>&1; then
    command -v "$name"
    return 0
  fi
  echo "error: $name not found after go install." >&2
  echo "Add Go's bin directory to your PATH:" >&2
  echo "  export PATH=\"\$(go env GOPATH)/bin:\$PATH\"" >&2
  exit 1
}

build_ensure_fyne() {
  FYNE_CMD="$(build_resolve_go_tool fyne fyne.io/tools/cmd/fyne)"
}

build_ensure_fyne_cross() {
  FYNE_CROSS_CMD="$(build_resolve_go_tool fyne-cross github.com/fyne-io/fyne-cross)"
}

build_host_os() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    Linux) echo linux ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) echo unknown ;;
  esac
}

build_run_tests() {
  echo "Running go test ./..." >&2
  go test ./...
}

build_clean_staging() {
  rm -rf "$BUILD_ROOT/fyne-cross/dist"
  rm -rf "$BUILD_ROOT/Invoice Generator.app"
  rm -rf "$BUILD_ROOT/Dock Invoice Generator.app"
  # Remove any .app left at repo root matching the packaged app name.
  find "$BUILD_ROOT" -maxdepth 1 -name '*.app' -print0 2>/dev/null | while IFS= read -r -d '' app; do
    rm -rf "$app"
  done
}

build_package_native() {
  local os="$1"
  build_ensure_fyne
  echo "Packaging for $os (native, release)..." >&2
  "$FYNE_CMD" package -os "$os" -release -icon "$APP_ICON"
}

build_collect_app_bundle() {
  local dest="$1"
  local bundle="$BUILD_ROOT/${APP_NAME}.app"
  if [[ ! -d "$bundle" ]]; then
    echo "error: expected ${APP_NAME}.app after fyne package" >&2
    exit 1
  fi
  mkdir -p "$dest"
  rm -rf "$dest/${APP_NAME}.app"
  mv "$bundle" "$dest/"
  echo "Wrote $dest/${APP_NAME}.app"
}

FYNE_CROSS_WORK_ROOT=""

build_resolve_fyne_cross_workdir() {
  # Colima/Docker on macOS often cannot write to bind mounts on external
  # drives (/Volumes/*). fyne-cross needs to create fyne-cross/ inside the
  # project tree — stage a copy under $HOME when needed.
  case "$BUILD_ROOT" in
    /Volumes/*)
      FYNE_CROSS_WORK_ROOT="${HOME}/.cache/dock-invoice-fyne-cross-work/${APP_SLUG}-${RELEASE_TAG}"
      echo "Project is on an external drive; fyne-cross will run from a copy at:" >&2
      echo "  $FYNE_CROSS_WORK_ROOT" >&2
      rm -rf "$FYNE_CROSS_WORK_ROOT"
      mkdir -p "$FYNE_CROSS_WORK_ROOT"
      if command -v rsync >/dev/null 2>&1; then
        rsync -a \
          --exclude 'dist/' \
          --exclude 'fyne-cross/' \
          --exclude '.git/' \
          "$BUILD_ROOT/" "$FYNE_CROSS_WORK_ROOT/"
      else
        tar -C "$BUILD_ROOT" \
          --exclude='dist' --exclude='fyne-cross' --exclude='.git' \
          -cf - . | tar -C "$FYNE_CROSS_WORK_ROOT" -xf -
      fi
      return 0
      ;;
  esac
  FYNE_CROSS_WORK_ROOT="$BUILD_ROOT"
}

build_sync_fyne_cross_output_back() {
  if [[ "$FYNE_CROSS_WORK_ROOT" == "$BUILD_ROOT" ]]; then
    return 0
  fi
  if [[ -d "$FYNE_CROSS_WORK_ROOT/fyne-cross" ]]; then
    rm -rf "$BUILD_ROOT/fyne-cross"
    cp -R "$FYNE_CROSS_WORK_ROOT/fyne-cross" "$BUILD_ROOT/"
  fi
}

build_package_fyne_cross() {
  local os="$1"
  build_ensure_fyne_cross
  build_resolve_fyne_cross_workdir
  echo "Packaging for $os via fyne-cross (Docker)..." >&2
  (
    cd "$FYNE_CROSS_WORK_ROOT"
    # amd64 covers most Windows/Linux PCs; skip arm64 to halve cross-build time.
    "$FYNE_CROSS_CMD" "$os" -arch=amd64 -app-id "$APP_ID" -icon "$APP_ICON" \
      -env GOTOOLCHAIN=auto
  )
  build_sync_fyne_cross_output_back
}

build_copy_fyne_cross_output() {
  local dest="$1"
  if [[ ! -d "$BUILD_ROOT/fyne-cross/dist" ]]; then
    echo "error: fyne-cross/dist not found after build" >&2
    return 1
  fi
  mkdir -p "$dest"
  cp -R "$BUILD_ROOT/fyne-cross/dist/." "$dest/"
  echo "Copied fyne-cross artifacts into $dest/"
}

build_archive_release() {
  local staging="$1"
  local archive_dir="$2"
  mkdir -p "$archive_dir"
  local archive_abs="$(cd "$archive_dir" && pwd)"
  local staging_abs="$(cd "$staging" && pwd)"

  if [[ -d "$staging_abs/${APP_NAME}.app" ]]; then
    local zip_name="${APP_SLUG}-${RELEASE_TAG}-macos.zip"
    echo "Creating $archive_abs/$zip_name" >&2
    if command -v ditto >/dev/null 2>&1; then
      ditto -c -k --sequesterRsrc --keepParent "$staging_abs/${APP_NAME}.app" "$archive_abs/$zip_name"
    else
      (
        cd "$staging_abs"
        zip -r -q "$archive_abs/$zip_name" "${APP_NAME}.app"
      )
    fi
  fi

  for dir in "$staging_abs"/*; do
    [[ -d "$dir" ]] || continue
    local base="$(basename "$dir")"
    case "$base" in
      windows-*|windows-native)
        local zip_name="${APP_SLUG}-${RELEASE_TAG}-${base}.zip"
        echo "Creating $archive_abs/$zip_name" >&2
        (
          cd "$staging_abs"
          zip -r -q "$archive_abs/$zip_name" "$base"
        )
        ;;
      linux-*|linux-native)
        local tar_name="${APP_SLUG}-${RELEASE_TAG}-${base}.tar.gz"
        echo "Creating $archive_abs/$tar_name" >&2
        tar -czf "$archive_abs/$tar_name" -C "$staging_abs" "$base"
        ;;
    esac
  done
}

build_print_summary() {
  echo "" >&2
  echo "=== Build complete ===" >&2
  echo "App:     $APP_NAME" >&2
  echo "Version: $APP_VERSION (build $APP_BUILD)" >&2
  echo "Output:  $OUT_DIR" >&2
  if [[ -d "$OUT_DIR" ]]; then
    find "$OUT_DIR" -maxdepth 2 \( -name '*.zip' -o -name '*.tar.gz' -o -name '*.app' -o -name '*.exe' \) -print 2>/dev/null | sort
  fi
}
