#!/bin/bash
# Install build-time dependencies for PairAdmin on Linux
# Usage: sudo ./scripts/install-deps.sh
set -e

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: Run as root (sudo ./scripts/install-deps.sh)" >&2
  exit 1
fi

. /etc/os-release

case "$ID" in
  ubuntu|debian)
    apt-get update -qq
    # NOTE: libwebkit2gtk-4.1-dev version 2.52.x pulls in a runtime
    # libjavascriptcoregtk that emits AVX instructions. On QEMU/KVM
    # virtual CPUs and older hardware without AVX, the WebKitWebProcess
    # crashes with SIGILL on launch. Until upstream provides a non-AVX
    # build, pin to the base release (2.44.x):
    #   sudo apt-mark hold libwebkit2gtk-4.1-0 libjavascriptcoregtk-4.1-0 \
    #                     libwebkit2gtk-4.1-dev libjavascriptcoregtk-4.1-dev
    apt-get install -y --no-upgrade \
      libwebkit2gtk-4.1-dev \
      at-spi2-core \
      gcc
    ;;
  fedora)
    dnf install -y \
      webkit2gtk4.1-devel \
      at-spi2-atk-devel \
      gcc
    ;;
  *)
    if echo "${ID_LIKE:-}" | grep -qE "rhel|centos|fedora"; then
      dnf install -y \
        webkit2gtk4.1-devel \
        at-spi2-atk-devel \
        gcc
    else
      echo "Unsupported distribution: ${ID}" >&2
      echo "Supported: Ubuntu, Debian, Fedora, RHEL/CentOS" >&2
      exit 1
    fi
    ;;
esac

echo "Dependencies installed successfully."
