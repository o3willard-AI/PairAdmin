# PairAdmin

AI pair programming assistant for terminal workflows

## Overview

PairAdmin is a desktop application that brings AI assistance directly into your terminal workflow. The AI sees exactly what you see in the terminal — automatically, without copy/paste — so assistance is always in context. PairAdmin works with tmux and GNOME Terminal (via AT-SPI2), and supports multiple LLM providers including OpenAI, Anthropic, Ollama, LM Studio, and OpenRouter.

## Installation

### One-line installer (Linux)

```bash
# Install (auto-detects .deb / .rpm / AppImage)
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin-anth-CC-gsd/master/install.sh | bash

# Upgrade to latest version
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin-anth-CC-gsd/master/install.sh | bash -s upgrade

# Uninstall
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin-anth-CC-gsd/master/install.sh | bash -s uninstall
```

The installer detects your distro and picks the right package format automatically (`.deb` for Debian/Ubuntu, `.rpm` for Fedora/RHEL, `.AppImage` fallback for others). `sudo` is required during install/uninstall.

> **macOS:** Native builds are coming soon.

### Manual install

Download the latest release from the [Releases page](https://github.com/o3willard-AI/PairAdmin-anth-CC-gsd/releases/latest), then:

**Debian/Ubuntu (.deb)**
```bash
sudo apt install -y libwebkit2gtk-4.1-0 at-spi2-core
sudo dpkg -i pairadmin_*_linux_amd64.deb
```

**Fedora/RHEL (.rpm)**
```bash
sudo dnf install -y webkit2gtk4.1 at-spi2-atk
sudo rpm -Uvh pairadmin_*_linux_amd64.rpm
```

> **AppImage / macOS / Windows:** Coming in a future release.

## Verifying Downloads

```bash
sha256sum --check SHA256SUMS
```

## Prerequisites

Before using PairAdmin, you need:

- A terminal multiplexer: `tmux` (recommended) or a GTK3-based terminal (GNOME Terminal)
- An LLM provider: Ollama (local, private — no data leaves your machine), or a cloud API key for OpenAI, Anthropic, OpenRouter, or LM Studio

## Quick Start

1. Launch PairAdmin: `pairadmin`
2. Open Settings (gear icon in status bar) and configure your LLM provider
3. Start a tmux session in another terminal: `tmux new -s work`
4. PairAdmin auto-discovers the tmux pane — you'll see it in the left sidebar
5. Type a question in the chat input — the AI has full context of your terminal

## Building from Source

```bash
# Install build dependencies
sudo ./scripts/install-deps.sh

# Install Go 1.24+ and Node.js 20+
# Install Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Build
wails build -platform linux/amd64 -tags webkit2_41
# Binary at: build/bin/pairadmin
```

## License

MIT
