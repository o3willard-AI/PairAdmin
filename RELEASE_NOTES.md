# PairAdmin v2.0.0

**The terminal for the agentic age.** v1.0.0 shipped a tool that *watched* a
terminal you started elsewhere via tmux capture-pane / AT-SPI2 polling. v2.0.0
is a real, native, multi-session terminal emulator in its own right — local
shells, remote SSH, and remote Windows via WinRM, all in one tabbed window —
with the same AI assistant built in, now backed by a much more capable UI
around it. Windows moves from "deferred pending hardware access" to the
most-tested platform in this release.

## Highlights

- **Native terminal emulation, not just capture.** Real PTYs — ConPTY on
  Windows, `creack/pty` on Linux/macOS — power a proper tabbed terminal
  session list with per-tab rename, full scrollback, and resize that actually
  reaches the shell. The old tmux/AT-SPI2 capture adapters are still there for
  reading a terminal you started elsewhere, but they're no longer the only way
  in.
- **Remote terminals: real SSH, plus WinRM for Windows.** Open a live,
  interactive SSH session (password or private-key auth, `~` paths expand
  correctly) or a WinRM command/response session to a remote Windows host,
  straight from "+ New." Save a connection and its credentials go to the OS
  keychain, never a config file; reconnect later from a "Recent" list under a
  friendly name that persists across renames. Check "Use tmux if available"
  and PairAdmin runs `tmux new-session -A` for you on connect — creates the
  session the first time, reattaches every time after, so a dropped
  connection is a non-event.
- **Quick Commands, rebuilt.** The sidebar is now a real command manager:
  right-click any saved command for Pin/Unpin, Edit, *Edit/Append for next
  use* (a one-time override that doesn't touch the saved version), or Remove;
  drag to reorder pinned commands; a pencil icon opens the same editor with a
  single click. Editing now opens a proper multi-line dialog instead of a
  cramped single-line field. A new "+ Add Command" button lets you type or
  paste a command directly, and a configurable hotkey (`Ctrl+Shift+A` by
  default) turns whatever's on your clipboard into a saved command in one
  keystroke — no dialog, no AI round-trip required. Connecting with tmux
  enabled auto-pins one-click `mouse on`/`mouse off` toggles, since that's the
  single most common tmux papercut.
- **Per-tab chat isolation.** Each terminal tab now keeps its own separate AI
  conversation — previously deferred, now shipped.
- **Light and Dark themes that actually work.** Toggling theme now changes
  the entire app — sidebar, dialogs, chat, status bar, context menus, and the
  terminal itself (including syntax-highlighted code blocks) — not just a
  CSS class with nothing wired to it.
- **A calmer, more reliable chat experience.** AI responses no longer
  visibly jitter while streaming; code-block action buttons (Copy to
  Terminal / Execute in Terminal / Save to Commands) now appear on every
  code block regardless of language tag, so a command the model mislabels
  is never stuck uncopyable; the system prompt no longer bundles independent
  commands into one block in a way that silently drops later steps if an
  earlier one fails.
- **The terminal keeps keyboard focus by default.** Click anywhere that
  isn't a genuine text field — a button, a tab, a checkbox — and the next
  keystroke still goes to the terminal instead of re-triggering whatever you
  clicked. Collapse the whole assistant with "Hide PairAdmin" when you don't
  need it and get a bigger terminal instead; the preference persists.
- **`/exit`** closes every terminal and quits the app; terminal tabs can be
  renamed via right-click. **10 slash commands** in total now (`/model`,
  `/context`, `/refresh`, `/filter`, `/export`, `/rename`, `/theme`, `/clear`,
  `/exit`, `/help`).
- **Several crash- and data-loss-class bugs fixed:** a nil-`http.Client` bug
  in the Ollama provider that panicked the entire application on the first
  real chat request; a Settings save path that could silently wipe other
  tabs' fields; a test-isolation bug that, on Windows, was overwriting the
  *real* user's `~/.pairadmin/config.yaml` every time the test suite ran; a
  keychain bug where colons in a saved host's key name broke password
  storage outright on Windows; and a tmux-over-SSH bug where sessions opened
  at roughly half their real width until an unrelated later action nudged
  them straight (root cause: a resize-event listener was being registered
  one call too late to catch the terminal's very first resize).

Everything from v1.0.0 — automatic tmux/AT-SPI2 capture, multi-provider LLM
support, pre-LLM credential filtering, memguard-protected API keys, the local
audit log, and OS-keychain-backed settings storage — is still here and
unchanged in behavior.

## Installation

### Ubuntu / Debian (.deb)

```bash
sudo apt install -y libwebkit2gtk-4.1-0 at-spi2-core
sudo dpkg -i pairadmin_2.0.0_linux_amd64.deb
pairadmin
```

### Fedora / RHEL (.rpm)

```bash
sudo dnf install -y webkit2gtk4.1 at-spi2-atk
sudo rpm -i pairadmin_2.0.0_linux_amd64.rpm
pairadmin
```

### AppImage

```bash
chmod +x pairadmin_2.0.0_linux_amd64.AppImage
./pairadmin_2.0.0_linux_amd64.AppImage
```

> **Note:** AppImage may fail at runtime due to WebKit2GTK subprocess path isolation (Wails Issue [#4313](https://github.com/wailsapp/wails/issues/4313)). The `.deb` package is the recommended install path on Ubuntu/Debian.

### Windows

No installer package yet — build from source:

```powershell
wails build -platform windows/amd64
# Binary at: build\bin\pairadmin.exe
```

WebView2 Runtime is required (preinstalled on Windows 11; installable
separately on Windows 10). This is the platform this release was most
thoroughly tested on.

### macOS

Not yet packaged or built. See `PLATFORM_PARITY_NOTES.md` for what's already
been fixed proactively (e.g. `~` expansion in SSH key paths) versus what
still needs a real Mac to verify (native Keychain integration, `Cmd`-based
hotkey/copy-paste conventions, app icon `.icns` generation).

## Prerequisites

- **tmux** (optional but recommended) — for sessions that survive
  disconnects, and for auto-discovering panes from sessions started outside
  the app. No special permissions required.
- **Ollama** (optional) — for fully local AI with no data leaving your
  machine: `ollama pull llama3`
- **Cloud provider API key** (optional) — OpenAI, Anthropic, OpenRouter, or
  LM Studio

PairAdmin opens its own local terminal by default, so none of the above are
required just to get a working terminal.

## Verify Checksums

```bash
sha256sum --check SHA256SUMS
```

## Known Limitations

- **macOS is unbuilt and unverified.** Everything in this release has been
  tested on Windows and/or Linux only. `PLATFORM_PARITY_NOTES.md` tracks
  specific known gaps (no native Keychain.app backend yet — falls back to
  the same encrypted file store as Windows; `Ctrl`-based default hotkey and
  copy/paste handling that should probably be `Cmd`-based on macOS) and a
  full manual verification checklist.
- **WinRM has only been exercised with PairAdmin itself running on
  Windows.** The client library is pure Go and should work identically
  regardless of the host OS, but this hasn't been verified from a Linux or
  macOS build yet.
- **SSH host key verification is not implemented.** Remote SSH connections
  do not verify the server's host key, so they are not protected against
  MITM attacks on untrusted networks. A visible warning is shown in the
  connection dialog; a fast-follow fix is planned.
- AppImage webkit runtime issue — use `.deb` or `.rpm` as primary install
  path on Linux.
- **WebKitGTK 2.52+ JIT crash on older CPUs** — `libwebkit2gtk-4.1-0`
  version 2.52.x (noble-updates) uses AVX instructions in JavaScriptCore's
  JIT compiler. QEMU/KVM virtual CPUs and older physical CPUs that lack AVX
  support will crash with SIGILL on launch. Pin to 2.44.x
  (`sudo apt-mark hold libwebkit2gtk-4.1-0 libjavascriptcoregtk-4.1-0`) or
  run with `JSC_useFTLJIT=false` as a workaround.

## What's Next

- macOS build: native Keychain.app backend, `Cmd`-based hotkey/copy-paste
  conventions, verified `.icns` app icon generation
- SSH host key verification (`known_hosts`-style, replacing the current
  `InsecureIgnoreHostKey`)
- SQLite chat history persistence (carried over from v1.0.0's roadmap)
- Wails v3 migration
- GPG artifact signing

---

# PairAdmin v1.0.0

AI pair programming assistant for terminal workflows. PairAdmin reads your terminal automatically — no copy/paste — and provides an AI chat interface with full terminal context injected into every message.

## Highlights

- **Automatic terminal capture** — tmux panes discovered and captured at 500ms polling; no manual copy/paste ever required
- **Multi-provider LLM support** — OpenAI, Anthropic, Ollama (local), LM Studio, OpenRouter; switch providers with `/model`
- **Pre-LLM credential filtering** — AWS keys, GitHub tokens, private keys, bearer tokens redacted before any content reaches a cloud API; Ollama enforces localhost-only
- **AT-SPI2 adapter** — GNOME Terminal and Konsole support via Linux accessibility bus
- **Security hardening** — API keys protected with memguard (mlock, encrypted in process memory); local audit log at `~/.pairadmin/logs/audit-YYYY-MM-DD.jsonl`
- **Settings dialog** — 5-tab UI: LLM config, prompts, terminals, hotkeys, appearance; OS keychain storage via `99designs/keyring`
- **8 slash commands** — `/model`, `/context`, `/refresh`, `/filter`, `/export`, `/rename`, `/theme`, `/help`

## Installation

### Ubuntu / Debian (.deb)

```bash
sudo apt install -y libwebkit2gtk-4.1-0 at-spi2-core
sudo dpkg -i pairadmin_1.0.0_linux_amd64.deb
pairadmin
```

### Fedora / RHEL (.rpm)

```bash
sudo dnf install -y webkit2gtk4.1 at-spi2-atk
sudo rpm -i pairadmin_1.0.0_linux_amd64.rpm
pairadmin
```

### AppImage

```bash
chmod +x pairadmin_1.0.0_linux_amd64.AppImage
./pairadmin_1.0.0_linux_amd64.AppImage
```

> **Note:** AppImage may fail at runtime due to WebKit2GTK subprocess path isolation (Wails Issue [#4313](https://github.com/wailsapp/wails/issues/4313)). The `.deb` package is the recommended install path on Ubuntu/Debian.

## Prerequisites

- **tmux** — primary terminal adapter (no special permissions required)
- **Ollama** (optional) — for fully local AI with no data leaving your machine: `ollama pull llama3`
- **Cloud provider API key** (optional) — OpenAI, Anthropic, OpenRouter, or LM Studio

## Verify Checksums

```bash
sha256sum --check SHA256SUMS
```

## Known Limitations

- CHAT-05/06 (per-tab chat isolation, `/clear`), CMD-02/05 (sidebar order, clear history) — deferred to v2
- macOS and Windows adapters — deferred pending hardware/VM access for QA
- AppImage webkit runtime issue — use `.deb` or `.rpm` as primary install path
- **WebKitGTK 2.52+ JIT crash on older CPUs** — `libwebkit2gtk-4.1-0` version 2.52.x (noble-updates) uses AVX instructions in JavaScriptCore's JIT compiler. QEMU/KVM virtual CPUs and older physical CPUs that lack AVX support will crash with SIGILL on launch. Pin to 2.44.x (`sudo apt-mark hold libwebkit2gtk-4.1-0 libjavascriptcoregtk-4.1-0`) or run with `JSC_useFTLJIT=false` as a workaround. Future `.deb` packages will declare a versioned `Breaks` on 2.52+ for affected architectures.

## What's Next (v2)

- macOS Terminal.app adapter (CGO/Accessibility API)
- SQLite chat history persistence
- Wails v3 migration
- GPG artifact signing
