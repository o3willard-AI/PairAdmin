# PairAdmin

<p align="center">
  <img src="assets/PA_Repo_img.jpg" alt="PairAdmin" width="644" />
</p>

AI pair programming assistant for terminal workflows

## Overview

PairAdmin is a desktop application that brings AI assistance directly into your terminal workflow. The AI sees exactly what you see in the terminal — automatically, without copy/paste — so assistance is always in context. PairAdmin works with tmux and GNOME Terminal (via AT-SPI2), and supports multiple LLM providers including OpenAI, Anthropic, Ollama, LM Studio, and OpenRouter.

Beyond local terminals, PairAdmin can also open and manage **remote SSH (Unix/Linux) and WinRM (Windows) sessions** directly from the "+ New" terminal dialog, with saved connections, OS-keychain-backed credential storage, and optional tmux auto-attach. A **Commands sidebar** keeps a running, pinnable history of every command the AI suggests (or that you save yourself, via a dialog or a configurable clipboard hotkey) for one-click copy/execute later. The UI supports both Light and Dark themes.

## Installation

### One-line installer (Linux)

```bash
# Install (auto-detects .deb / .rpm / AppImage)
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash

# Upgrade to latest version
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s upgrade

# Uninstall
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s uninstall
```

The installer detects your distro and picks the right package format automatically (`.deb` for Debian/Ubuntu, `.rpm` for Fedora/RHEL, `.AppImage` fallback for others). `sudo` is required during install/uninstall.

> **macOS:** Native builds are coming soon.

### Manual install

Download the latest release from the [Releases page](https://github.com/o3willard-AI/PairAdmin/releases/latest), then:

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

**Linux:**

```bash
# Install build dependencies
sudo ./scripts/install-deps.sh

# Install Go 1.24+ and Node.js 20+
# Install Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Build
wails build -platform linux/amd64 -tags webkit2_41
# Binary at: build/bin/pairadmin
```

**Windows:**

```powershell
# Install Go 1.24+, Node.js 20+, and the Wails CLI (same as above)
# WebView2 Runtime is required (preinstalled on Windows 11; installable on Windows 10)

wails build -platform windows/amd64
# Binary at: build\bin\pairadmin.exe
```

**macOS:** not yet packaged — see `PLATFORM_PARITY_NOTES.md` for the current
state of cross-platform work and what still needs verification there.

## License

MIT
