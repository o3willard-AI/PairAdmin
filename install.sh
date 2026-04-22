#!/usr/bin/env bash
set -euo pipefail

REPO="o3willard-AI/PairAdmin-anth-CC-gsd"
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

do_install() {
  local version
  version="$(get_latest_version)"
  [ -z "$version" ] && error "Could not determine latest version. Check your internet connection."

  info "Installing PairAdmin v${version}..."

  local pkg_mgr tmpdir
  pkg_mgr="$(detect_pkg_manager)"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  case "$pkg_mgr" in
    deb)
      local file="${BINARY_NAME}_${version}_linux_${ARCH}.deb"
      info "Downloading ${file}..."
      curl -fsSL -o "${tmpdir}/${file}" \
        "https://github.com/${REPO}/releases/download/v${version}/${file}"
      info "Installing (requires sudo)..."
      sudo dpkg -i "${tmpdir}/${file}"
      ;;
    rpm)
      local file="${BINARY_NAME}_${version}_linux_${ARCH}.rpm"
      info "Downloading ${file}..."
      curl -fsSL -o "${tmpdir}/${file}" \
        "https://github.com/${REPO}/releases/download/v${version}/${file}"
      info "Installing (requires sudo)..."
      sudo rpm -Uvh "${tmpdir}/${file}"
      ;;
    appimage)
      local file="${BINARY_NAME}_${version}_linux_${ARCH}.AppImage"
      info "Downloading ${file}..."
      curl -fsSL -o "${tmpdir}/${file}" \
        "https://github.com/${REPO}/releases/download/v${version}/${file}"
      chmod +x "${tmpdir}/${file}"
      info "Installing to ${INSTALL_DIR} (requires sudo)..."
      sudo mv "${tmpdir}/${file}" "${INSTALL_DIR}/${BINARY_NAME}"
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
      if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        sudo rm -f "${INSTALL_DIR}/${BINARY_NAME}" && info "Uninstalled."
      else
        warn "PairAdmin not found at ${INSTALL_DIR}/${BINARY_NAME}."
      fi
      ;;
  esac
}

do_upgrade() {
  info "Upgrading PairAdmin..."
  do_uninstall
  do_install
}

CMD="${1:-install}"
case "$CMD" in
  install)   do_install ;;
  uninstall) do_uninstall ;;
  upgrade)   do_upgrade ;;
  *) error "Unknown command: $CMD\nUsage: $0 [install|uninstall|upgrade]" ;;
esac
