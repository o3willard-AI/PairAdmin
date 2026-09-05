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
incantation. Sessions survive disconnects; you don't have
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
any model. In-process redaction covers: `aws-access-key-id`,
`github-token`, `gitlab-personal-access-token`, `openai-api-key`,
`anthropic-api-key`, `slack-token`, `google-api-key`,
`google-service-account` (service-account JSON private keys), `azure-account-key`
(AccountKey / SharedAccessKey), `bearer-token`, `jwt` (bare, no Bearer
prefix needed), `pem-private-key` (RSA/EC/OpenSSH private-key blocks),
`password-assignment`, `generic-api-key`, and `connection-string-credentials`
(URI-style `scheme://user:pass@host`). You can also add your own regex
patterns to redact matches or drop whole lines. API keys are held in encrypted,
mlock'd memory (memguard) rather than plain variables. Every prompt and response
is written to a local rotating JSONL audit log, so you can answer "what did we
send them?" with a file instead of a shrug. Prefer nothing leave the box at all?
Point it at Ollama or LM Studio on this machine — both default to loopback
(Ollama to `http://localhost:11434`), so a typo in a host field can't quietly
ship your terminal elsewhere.

**Ollama on a remote host?** Also supported — point the Server URL at a team
GPU box or any other instance you control. Two things to know: terminal output
*will* leave your machine, so only point at a host you trust, and if the remote
requires authentication, set `OLLAMA_API_KEY` (or paste a key into the Ollama
API key field in Settings → LLM Config; it's stored in the OS keychain and sent
as `Authorization: Bearer`). PairAdmin shows a warning in Settings whenever a
non-loopback Ollama host is configured.

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
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s install

# Upgrade to latest version
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s upgrade

# Uninstall
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s uninstall

# Verify a release's checksum before installing (no sudo needed)
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s -- --sha256
```

The installer detects your distro and picks the right package format automatically (`.deb` for Debian/Ubuntu, `.rpm` for Fedora and other dnf/yum-based distros). The checksum is **always verified** before installation — `install`, `upgrade`, and `--sha256` all download `SHA256SUMS` from the same release and abort on mismatch. `sudo` is required during install/uninstall. No AppImage build exists yet, so a system without `dpkg` or `rpm` isn't supported by the one-line installer — use [Building from Source](#building-from-source) instead.

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

### Automated checksum verification (install.sh)

The `install.sh` script supports a `--sha256` mode that downloads the release
asset **and** the release's `SHA256SUMS` file, then verifies the checksum
before any installation. A mismatch aborts with a clear error and does **not**
install.

```bash
# Verify the latest release (auto-detects .deb/.rpm):
bash install.sh --sha256

# Verify a specific version:
bash install.sh --sha256 v2.3.0

# Via the one-line installer (note the -- before args):
curl -fsSL https://raw.githubusercontent.com/o3willard-AI/PairAdmin/master/install.sh | bash -s -- --sha256 v2.3.0

# The plain "verify" command is an alias for the latest:
bash install.sh verify
```

The regular `install` command also verifies the checksum automatically before
installing — the verify step is built into the install flow.

### Manual verification

```bash
# Download both the asset and SHA256SUMS from the same release:
curl -fsSL -o pairadmin_2.3.0_linux_amd64.deb \
  https://github.com/o3willard-AI/PairAdmin/releases/download/v2.3.0/pairadmin_2.3.0_linux_amd64.deb
curl -fsSL -o SHA256SUMS \
  https://github.com/o3willard-AI/PairAdmin/releases/download/v2.3.0/SHA256SUMS

# Verify:
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

## Configuration & Data Storage

A properly-installed release build (the one-line installer, the packaged
`.deb`/`.rpm`, or the official `.exe`/`.dmg`) keeps its settings, saved
connections, SSH known-host pins, and audit log under an OS-conventional
per-user data directory:

- **Windows:** `%LOCALAPPDATA%\PairAdmin\`
- **macOS:** `~/Library/Application Support/PairAdmin/`
- **Linux:** `$XDG_DATA_HOME/pairadmin/` (falls back to `~/.local/share/pairadmin/`)

A binary you build yourself from source (`wails build` without the release
pipeline's flags, or `wails dev`) instead uses the legacy `~/.pairadmin/`,
so local dev/QA testing never mixes its data with a real install on the same
machine.

## Building from Source

**Linux:**

```bash
# Install build dependencies
sudo ./scripts/install-deps.sh

# Install Go 1.26.6 and Node.js 20+
# Install Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Build
wails build -platform linux/amd64 -tags webkit2_41
# Binary at: build/bin/pairadmin
```

**Windows:**

```powershell
# Install Go 1.26.6, Node.js 20+, and the Wails CLI (same as above)
# WebView2 Runtime is required (preinstalled on Windows 11; installable on Windows 10)

wails build -platform windows/amd64
# Binary at: build\bin\pairadmin.exe
```

**macOS:**

```bash
wails build -platform darwin/universal
# App bundle at: build/bin/pairadmin.app
```

## Testing

The frontend uses Vitest (jsdom) with `@vitest/coverage-v8`. To run the test
suite with a coverage report locally:

```bash
cd frontend
npm ci
npx vitest run --coverage
```

This emits an HTML-friendly Console/text report plus a machine-readable
`coverage/coverage-final.json` that the CI diff-coverage gate consumes.

### Diff-coverage gate

CI hard-fails a pull request whose **changed feature lines** (intersection of
`git diff <base> HEAD` ranges with the coverage report's statement lines) fall
below an 80% floor. This deliberately measures coverage of what a PR touched,
not a gameable whole-project total. To run the same gate locally against `main`:

```bash
cd frontend
npx vitest run --coverage
node scripts/check-diff-coverage.mjs origin/master
```

See `frontend/scripts/check-diff-coverage.mjs` for details.
## Security Model

PairAdmin makes four security guarantees. Each is backed by specific code you
can point at.

### 1. SSH host-key TOFU (trust-on-first-use)

When you connect to a new SSH host, PairAdmin pins the server's public key.
This is the same trust-on-first-use model as `ssh` / `~/.ssh/known_hosts`.

- **First connect to a host:** the key is pinned silently (no prompt); a
  mismatch on a later connection is always rejected.
- **To require confirmation on first connect** (security-team review path),
  set `prompt_new_host_keys: true` in config (Settings → Terminals →
  "Prompt on new host keys"). The first connection pauses for explicit
  approval; subsequent connections without a prompt behave as above.
- A key mismatch on any later connection is **always rejected** — the setting
  only affects whether the *first* connect asks for approval, not whether
  later mismatches are caught.

See `services/config/config.go` (`PromptNewHostKeys` field, default `false`),
`services/remote_ssh.go` (key pinning logic), and
`services/remote_types.go` (the saved-host struct).

### 2. Credential redaction (filter pipeline)

Terminal output, user input, and any content sent to an LLM is scrubbed
through an in-process filter pipeline **before** it leaves the process —
whether to a model, a log file, or the audit trail. The pipeline lives in
`services/llm/filter/` and is constructed in `services/llm_service.go`
(line ~189: `pipeline := filter.NewPipeline(...)`).

Built-in redaction patterns (`services/llm/filter/credential.go`):

| Pattern ID | What it catches |
|---|---|
| `aws-access-key-id` | AWS access key IDs (AKIA...) |
| `github-token` | GitHub PATs (`ghp_...`, `github_pat_...`) |
| `gitlab-personal-access-token` | GitLab PATs (`glpat-...`) |
| `openai-api-key` / `anthropic-api-key` | Provider API keys |
| `slack-token` | Slack tokens |
| `google-api-key` / `google-service-account` | Google API keys + service-account JSON private keys |
| `azure-account-key` | Azure AccountKey / SharedAccessKey |
| `bearer-token` | Bare bearer/JWT tokens |
| `jwt` | JWT tokens |
| `pem-private-key` | RSA/EC/OpenSSH private-key blocks |
| `password-assignment` | `password = value` / `passwd: value` patterns |
| `generic-api-key` | Generic `api_key=...` / `apiKey: ...` assignments |
| `connection-string-credentials` | URI-style `scheme://user:pass@host` |

You can add custom regex patterns in Settings → Prompts → Redaction Patterns
(persisted as `CustomPatterns` in `services/config/config.go`).

### 3. OS keychain for secrets

Credentials (API keys, SSH key passphrases, saved remote-host passwords) are
stored in the **OS keychain**, never in config files:

- **macOS:** real Keychain.app (`keyring.KeychainBackend` with
  `KeychainTrustApplication: true`).
- **Windows:** Windows Credential Manager (`keyring.WinCredBackend`).
- **Linux:** Secret Service / gnome-keyring / kwallet
  (`keyring.SecretServiceBackend`).
- **Fallback:** if no OS backend opens *or* passes a write probe, PairAdmin
  falls back to an encrypted on-disk `FileBackend`
  (`~/.pairadmin/keyring/` for dev builds, the OS data dir for releases)
  unlocked by a **user-chosen master password**.

API keys held in memory use `memguard.Enclave` (sealed, mlock'd, not plain
variables). See `services/keychain/keychain.go` (allow-listed backends,
`FileBackend` fallback) and `services/llm_service.go` (`apiKeyEnclaves`).

### 4. WebKit content-process sandbox (Linux)

On Linux, Wails renders the UI via WebKitGTK. WebKitGTK ≥ 2.40 enables a
content-process sandbox (bubblewrap + user namespaces) by default. This
sandbox is **disabled unconditionally** in `main.go` via
`WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1` (a no-op on macOS/Windows,
which use WKWebView / WebView2 respectively).

**Why:** the sandbox fails — producing a **blank window** — on PairAdmin's
primary Linux targets (VMs with VirtIO GPU, containers, headless hosts,
hosts with user namespaces disabled by kernel/AppArmor/seccomp). There is
no reliable pre-flight check that models all failure legs, so the disable
is unconditional and documented.

**Risk is bounded** because the WebView loads **only bundled, first-party,
trusted UI** (`//go:embed all:frontend/dist`) — no remote web content, no
third-party webviews, no network-loaded JS.

Rationale, OS matrix, and alternatives are recorded in
[`docs/adr/ADR-0001-webkit-sandbox-disable.md`](docs/adr/ADR-0001-webkit-sandbox-disable.md).

## License

[Apache License 2.0](LICENSE)
