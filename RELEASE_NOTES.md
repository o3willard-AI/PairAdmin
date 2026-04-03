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

## What's Next (v2)

- macOS Terminal.app adapter (CGO/Accessibility API)
- SQLite chat history persistence
- Wails v3 migration
- GPG artifact signing
