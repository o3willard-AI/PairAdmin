# PairAdmin

<p align="center">
  <img src="assets/PA_Repo_img.jpg" alt="PairAdmin" width="644" />
</p>

**The terminal for the agentic age.**

## Overview

PairAdmin is a real terminal emulator — local shells, SSH, and remote Windows,
all in one tabbed window — with an AI assistant that already knows what's on
your screen. No copy/paste, no re-explaining context, no alt-tabbing to a chat
window to ask what a stack trace means.

It's built for the way people actually work now: several sessions open at once,
long-running jobs on remote boxes, agents and scripts producing more output than
anyone wants to read, and a constant low-grade worry about what you're pasting
into someone else's model. PairAdmin is designed around all four.

**A terminal first.** Real PTYs (ConPTY on Windows), a proper tabbed session
list, per-tab renaming, full scrollback, and keyboard focus that stays where you
expect it — when you start typing, it goes to the terminal, not to whatever
button you clicked last. When you don't want the assistant, collapse it and
you've got a clean, capable terminal with a persistent command palette.

**tmux, minus the papercuts.** tmux is the right answer for sessions that
outlive your connection, and a rough experience the moment you try to use it
casually. PairAdmin does the create-or-attach dance for you (`tmux new-session
-A`) so reconnecting to a named session is a checkbox, not a memorized
incantation. It auto-discovers panes from tmux sessions you started outside the
app. And because the single most common tmux complaint is that the scroll wheel
stops working, connecting with tmux enabled auto-pins one-click `mouse on` /
`mouse off` toggles to the sidebar. Sessions survive disconnects; you don't have
to think about it.

**Quick Commands — a runbook that builds itself.** Every command the AI suggests
becomes a one-click *Copy to Terminal* / *Execute in Terminal* / *Save to
Commands* button. Saved commands live in a sidebar shared across every tab, so a
debugging session quietly turns into a reusable runbook. Pin the ones you keep
reaching for and drag them into the order you want. Add your own by hand, or hit
a hotkey to turn whatever's on your clipboard into a saved command instantly.
Edit a command permanently — or use *Edit/Append for next use* to tweak it just
for the next run (add a `--dry-run`, change one hostname) without touching the
saved version.

**A genuine SSH client, not a shell-out.** Saved connections with host, port,
username, and password or key-file auth; credentials in the OS keychain, never
in a config file; friendly names that stick; optional tmux auto-attach per
connection. Remote Windows hosts over WinRM too. It's the "replace PuTTY and
stop keeping a text file of IP addresses" tier of useful.

**Your secrets stay yours.** Terminal output is scrubbed *before* it's sent to
any model: AWS keys, GitHub tokens, OpenAI/Anthropic keys, bearer tokens, and
generic API-key patterns are redacted in-process, and you can add your own regex
patterns to redact matches or drop whole lines. API keys are held in encrypted,
mlock'd memory (memguard) rather than plain variables. Every prompt and response
is written to a local rotating JSONL audit log, so you can answer "what did we
send them?" with a file instead of a shrug. Prefer nothing leave the box at all?
Point it at Ollama or LM Studio — and the Ollama path is enforced localhost-only,
so a typo in a host field can't quietly ship your terminal to someone else's GPU.

**Bring your own model.** OpenAI, Anthropic, Ollama, LM Studio, and OpenRouter,
switchable mid-session with `/model`. Each terminal tab keeps its own separate
conversation, so the chat about your flaky CI box doesn't bleed into the one
about your database migration. Ten slash commands cover the rest: `/context`
to size the context window, `/filter` to manage redaction patterns, `/export` to
save a session transcript, `/refresh`, `/clear`, `/theme`, and more.

Light and dark themes throughout, including the terminal itself.

## Installation

### One-line installer (Linux)

```bash
# Install (auto-detects .deb / .rpm)
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash

# Upgrade to latest version
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s upgrade

# Uninstall
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s uninstall
```

The installer detects your distro and picks the right package format automatically (`.deb` for Debian/Ubuntu, `.rpm` for Fedora and other dnf/yum-based distros). `sudo` is required during install/uninstall. No AppImage build exists yet, so a system without `dpkg` or `rpm` isn't supported by the one-line installer — use [Building from Source](#building-from-source) instead.

> **macOS & Windows:** see the manual install sections below.

### Manual install

Prebuilt binaries are **not in this repository** — CI builds them and attaches them as assets to the [Releases page](https://github.com/o3willard-AI/PairAdmin/releases/latest). Download the latest release from there, then:

**Debian/Ubuntu (.deb)**
```bash
sudo apt install -y libwebkit2gtk-4.1-0 at-spi2-core
sudo dpkg -i pairadmin_*_linux_amd64.deb
```

**Fedora (.rpm)**
```bash
sudo dnf install -y webkit2gtk4.1 at-spi2-atk
sudo rpm -Uvh pairadmin_*_linux_amd64.rpm
```

**Windows (.exe installer)**

1. Download `pairadmin-amd64-installer.exe` from the [latest release](https://github.com/o3willard-AI/PairAdmin/releases/latest) and run it.
2. Windows SmartScreen may show "Windows protected your PC" (the app is unsigned during alpha) — click **More info → Run anyway**.
3. Requires the WebView2 Runtime (preinstalled on Windows 11; the installer fetches it on Windows 10). Windows 10 1809+ supported.

**macOS (alpha — unsigned)**

macOS builds ship **unsigned** during the alpha. Download `PairAdmin-v*.dmg` from the [latest release](https://github.com/o3willard-AI/PairAdmin/releases/latest), mount it, and drag PairAdmin to Applications. Gatekeeper will block the app on first launch, so approve it once with:

```bash
xattr -dr com.apple.quarantine /Applications/PairAdmin.app
```

…or via **System Settings → Privacy & Security → "Open Anyway"**. Re-run this on every update (each new download re-triggers Gatekeeper). Apple code signing + notarization is planned for the stable release. Requires **macOS 12+**.

> **AppImage:** coming in a future release.

## Verifying Downloads

```bash
sha256sum --check SHA256SUMS
```

## Prerequisites

PairAdmin opens its own terminals, so nothing else is strictly required to
start. Optionally:

- **`tmux`** — for sessions that survive disconnects, and to have PairAdmin
  auto-discover panes from tmux sessions started outside the app. Install it on
  whichever machine you want persistent sessions on (locally, or on the remote
  host you're SSH-ing into).
- **A GTK3-based terminal** (GNOME Terminal) — Linux only; lets PairAdmin read
  an already-open external terminal via AT-SPI2.
- **An LLM provider** — Ollama or LM Studio for fully local, private inference
  with no data leaving your machine, or an API key for OpenAI, Anthropic, or
  OpenRouter. The terminal works without any provider configured; you just
  won't have the assistant.

## Quick Start

1. Launch PairAdmin.
2. Open Settings (gear icon in the status bar) and configure your LLM provider.
3. Click **+ New** in the terminal list and pick **Local** — you've got a shell.
4. Run something. Ask a question in the chat input; the AI already has the
   terminal output as context, so "what does this error mean?" just works.
5. Click **Save to Commands** on anything it suggests — it's in the sidebar for
   one-click reuse from any tab, for the rest of the session and beyond.

To connect somewhere else, choose **Unix / Linux (SSH)** or **Remote Windows
(WinRM)** from the same **+ New** dialog instead. Tick **Save Terminal** to keep
the connection (credentials go to the OS keychain), and **Use tmux if available**
to land in a persistent, reattachable session every time you connect.

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

**macOS:**

```bash
wails build -platform darwin/universal
# App bundle at: build/bin/pairadmin.app
```

## License

[Apache License 2.0](LICENSE)
