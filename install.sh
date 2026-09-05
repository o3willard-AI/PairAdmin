#!/usr/bin/env bash
set -euo pipefail

REPO="o3willard-AI/PairAdmin"
BINARY_NAME="pairadmin"
INSTALL_DIR="/usr/local/bin"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[pairadmin]${NC} $*"; }
warn()  { echo -e "${YELLOW}[pairadmin]${NC} $*"; }
error() { echo -e "${RED}[pairadmin]${NC} $*" >&2; exit 1; }

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)          ARCH="amd64" ;;
  aarch64|arm64)   ARCH="arm64" ;;
  *)               error "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  Linux)  ;;
  Darwin) error "macOS builds are not yet available. Coming soon!" ;;
  *)      error "Unsupported OS: $OS" ;;
esac

get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": *"v?([^"]+)".*/\1/'
}

detect_pkg_manager() {
  if command -v dpkg &>/dev/null && command -v apt-get &>/dev/null; then
    echo "deb"
  elif command -v rpm &>/dev/null && (command -v dnf &>/dev/null || command -v yum &>/dev/null); then
    echo "rpm"
  else
    echo "appimage"
  fi
}

check_webkit_version() {
  if ! command -v dpkg &>/dev/null; then
    return 0
  fi
  local ver
  ver="$(dpkg -l libwebkit2gtk-4.1-0 2>/dev/null | awk '/^ii/{print $3}' | cut -d. -f1-2)"
  if [ -n "$ver" ] && dpkg --compare-versions "$ver" ge "2.52"; then
    warn "WebKitGTK 2.52+ detected — this version uses AVX instructions"
    warn "that may crash on QEMU/KVM virtual CPUs and older hardware."
    warn "If PairAdmin crashes on launch, downgrade WebKitGTK to the base"
    warn "Ubuntu 24.04 release version (2.44.x) or run with:"
    warn "  JSC_useFTLJIT=false pairadmin"
    warn ""
    info "WebKitGTK version: $(dpkg -l libwebkit2gtk-4.1-0 2>/dev/null | awk '/^ii/{print $3}')"
  fi
}

# Downloads the release asset (package or SHA256SUMS) for the given version
# and architecture into the given tmpdir. Prints the downloaded filename.
fetch_release_asset() {
  local version="$1" asset_name="$2" tmpdir="$3"
  local url="https://github.com/${REPO}/releases/download/v${version}/${asset_name}"
  local dest="${tmpdir}/${asset_name}"
  info "Downloading ${asset_name}..."
  curl -fsSL -o "$dest" "$url"
  echo "$asset_name"
}

# Verifies the local package file against the SHA256SUMS file from the same
# release. Aborts with error if the checksum does not match or the asset is
# not listed in SHA256SUMS at all.
verify_sha256() {
  local version="$1" pkg_name="$2" tmpdir="$3"
  local sums_file="${tmpdir}/SHA256SUMS"

  # Fetch the release's SHA256SUMS alongside the package.
  fetch_release_asset "$version" "SHA256SUMS" "$tmpdir" >/dev/null

  # Extract the expected hash for this package from SHA256SUMS.
  # SHA256SUMS format: "<hash>  <filename>"
  local expected_hash
  expected_hash="$(grep -E "[0-9a-f]{64}  ${pkg_name}" "$sums_file" | awk '{print $1}')"
  if [ -z "$expected_hash" ]; then
    error "Package ${pkg_name} not found in SHA256SUMS for v${version}.\nThe release may not include a checksum for this platform — aborting for safety."
  fi

  # Compute the actual hash and compare (portable: works with both GNU and BSD sha256sum).
  local actual_hash
  if command -v sha256sum &>/dev/null; then
    actual_hash="$(sha256sum "${tmpdir}/${pkg_name}" | awk '{print $1}')"
  else
    actual_hash="$(shasum -a 256 "${tmpdir}/${pkg_name}" | awk '{print $1}')"
  fi

  if [ "$actual_hash" != "$expected_hash" ]; then
    error "SHA256 checksum mismatch for ${pkg_name}!\n  Expected: $expected_hash\n  Got:      $actual_hash\nThe release asset may have been tampered with or corrupted — aborting install."
  fi

  info "SHA256 verified: ${pkg_name} OK"
}

do_install() {
  local version
  version="$(get_latest_version)"
  [ -z "$version" ] && error "Could not determine latest version. Check your internet connection."

  info "Installing PairAdmin v${version}..."

  local pkg_mgr tmpdir
  pkg_mgr="$(detect_pkg_manager)"
  tmpdir="$(mktemp -d)"
  # Expand $tmpdir now (double quotes) so the value is baked into the trap string.
  # Using single quotes would defer expansion to EXIT time when the local var is out of scope.
  trap "rm -rf '$tmpdir'" EXIT

  case "$pkg_mgr" in
    deb)
      local file="${BINARY_NAME}_${version}_linux_${ARCH}.deb"
      fetch_release_asset "$version" "$file" "$tmpdir" >/dev/null
      # Verify checksum before installing — only on amd64 (release ships deb for amd64).
      verify_sha256 "$version" "$file" "$tmpdir"
      info "Installing (requires sudo)..."
      sudo dpkg -i "${tmpdir}/${file}"
      check_webkit_version
      ;;
    rpm)
      local file="${BINARY_NAME}_${version}_linux_${ARCH}.rpm"
      fetch_release_asset "$version" "$file" "$tmpdir" >/dev/null
      verify_sha256 "$version" "$file" "$tmpdir"
      info "Installing (requires sudo)..."
      sudo rpm -Uvh "${tmpdir}/${file}"
      ;;
    appimage)
      error "No supported package manager found (dpkg or rpm required).\nSupported distros: Debian, Ubuntu, Fedora, and other dnf/yum-based distros.\nNo AppImage build exists yet — build from source instead: https://github.com/${REPO}#building-from-source"
      ;;
  esac

  info "PairAdmin v${version} installed. Run: pairadmin"
}

do_uninstall() {
  info "Uninstalling PairAdmin..."
  local pkg_mgr
  pkg_mgr="$(detect_pkg_manager)"

  case "$pkg_mgr" in
    deb)
      if dpkg -l pairadmin &>/dev/null 2>&1; then
        sudo dpkg -r pairadmin && info "Uninstalled."
      else
        warn "PairAdmin not found in dpkg."
      fi
      ;;
    rpm)
      if rpm -q pairadmin &>/dev/null 2>&1; then
        sudo rpm -e pairadmin && info "Uninstalled."
      else
        warn "PairAdmin not found in rpm."
      fi
      ;;
    appimage)
      warn "No supported package manager found. Nothing to uninstall."
      ;;
  esac
}

do_upgrade() {
  info "Upgrading PairAdmin..."
  do_uninstall
  do_install
}

# --- Verify mode: download and check a release asset's checksum ---
do_verify() {
  local version="$1"
  local pkg_mgr
  pkg_mgr="$(detect_pkg_manager)"

  case "$pkg_mgr" in
    deb) local ext="deb" ;;
    rpm) local ext="rpm" ;;
    *)   error "No supported package manager found (dpkg or rpm required). Cannot verify." ;;
  esac

  local file="${BINARY_NAME}_${version}_linux_${ARCH}.${ext}"
  local tmpdir
  tmpdir="$(mktemp -d)"
  trap "rm -rf '$tmpdir'" EXIT

  info "Verifying PairAdmin v${version} (${ext})..."
  fetch_release_asset "$version" "$file" "$tmpdir" >/dev/null
  verify_sha256 "$version" "$file" "$tmpdir"

  info "Verification PASSED. The release asset matches the published checksum."
  info "To install: curl -fsSL https://raw.githubusercontent.com/${REPO}/master/install.sh | bash -s install"
}

# --- Argument parsing ---
# Supports: install | uninstall | upgrade | --sha256 [version] | verify
# When piping via curl|bash, pass args as: bash -s -- --sha256 v2.3.0
# (the "--" tells bash "end of options", so --sha256 reaches the script as $1)
CMD="${1:-install}"
case "$CMD" in
  install)   do_install ;;

  # --sha256 <version>  : verify a release asset's checksum without installing
  --sha256|sha256)
    shift
    version="${1:-}"
    if [ -z "$version" ]; then
      version="$(get_latest_version)"
      info "No version specified, using latest: v${version}"
    else
      # Strip leading 'v' if the user passed a tag like "v2.3.0"
      version="${version#v}"
      info "Verifying release v${version}..."
    fi
    do_verify "$version"
    ;;

  verify)
    # Alias for --sha256 with latest version
    version="$(get_latest_version)"
    info "Verifying latest release: v${version}"
    do_verify "$version"
    ;;

  uninstall) do_uninstall ;;
  upgrade)   do_upgrade ;;
  *) error "Unknown command: $CMD\nUsage: $0 [install|uninstall|upgrade]\n       $0 --sha256 [version]\n       $0 verify" ;;
esac
