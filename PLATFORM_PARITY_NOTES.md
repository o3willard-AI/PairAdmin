# Platform Parity Notes (TEMPORARY — delete before v1.0)

**Audience:** coding agents building/maintaining the Linux and macOS targets.
**Purpose:** remaining per-platform verification items. This file was burned
down on 2026-09-05: everything verifiable from the Linux build/test suite or
by reading the shipped code was verified and deleted (see the "Resolved and
removed" section at the bottom for what was checked and why it was removed).
What remains genuinely requires hardware/OS access a Linux build agent does
not have.

**Lifecycle:** once the remaining items below are verified on real macOS and
Windows machines (and against real SSH/WinRM targets), delete this file. It
is not meant to ship in a v1.0 release.

---

## Remaining items — need a real macOS / Windows machine

### macOS

- **Terminal Cmd+C / Cmd+V semantics.** `TerminalPreview.tsx`'s
  `attachCustomKeyEventHandler` treats `ctrlKey || metaKey` identically for
  copy/paste-vs-SIGINT. On macOS, `Cmd+C` should always copy and `Ctrl+C`
  should always send SIGINT (even with a selection active), and paste is
  conventionally `Cmd+V`. The current code does not distinguish which
  modifier was pressed. Needs a real macOS + WKWebView machine to verify the
  actual key events xterm receives before changing the handler — guessing
  the fix without measuring was exactly the mistake documented in the
  original notes (the tmux width bug).
- **App icon rendering.** `build/appicon.png` is now square (512×512 — the
  698×665 non-square source called out earlier was replaced), so the known
  distortion risk is resolved *at the source-image level*, but the generated
  `.icns`/dock rendering still needs one visual check on a real Mac.
- **Hotkey defaults on macOS.** All shipped hotkey defaults are `Ctrl`-based
  (`Ctrl+Shift+A` clipboard-command, `Ctrl+Shift+N` new terminal, etc.),
  chosen for Windows/Linux AltGr and WebView2 reasons — not macOS
  conventions. They are user-rebindable in Settings → Hotkeys and the
  rebinding path handles `Meta` (Cmd) correctly (`buildKeyCombo` records
  `metaKey`, `matchesHotkey` compares it), so nothing is broken — but
  whether the *defaults* should be `Cmd`-based on macOS needs a real Mac to
  check for collisions with WKWebView/system shortcuts.
- **macOS Keychain backend.** `keyring.KeychainBackend` IS in the
  `AllowedBackends` list (`services/keychain/keychain.go`,
  `KeychainTrustApplication: true` is set, so the app will not re-prompt per
  access) — the original "never added" note is now stale code-wise. But it
  has still never been observed on a real Mac: verify that API keys and
  saved remote-host passwords actually land in Keychain Access, and that the
  canary probe (`probeBackend`) passes against a real Keychain on first run.
- **General smoke test.** The manual checklist from the original file is
  preserved below — it needs a human with a real Mac clicking through it.

### Windows

- **App icon rendering in taskbar/window title bar** after the 512×512
  source change (`build/windows/icon.ico` was purpose-built, so risk is low,
  but confirm once).
- **General smoke test** (checklist below).

### Real remote targets (from any Linux/macOS/Windows build)

- **WinRM against a real Windows host.** The client path is pure Go
  (`masterzen/winrm` + `Azure/go-ntlmssp`, cgo-free) and the WinRM TLS
  backend + endpoint tests now run in Linux CI, but no test connects to a
  real WinRM-enabled Windows host. Do one real connect-and-run-a-command
  test (TLS and plaintext) before trusting remote-Windows support.
- **SSH against a real Linux host.** `services/remote_ssh_test.go` covers
  the full connect/PTY/resize/exit path against an in-process SSH server,
  and tmux create-or-attach is plain keystrokes over the channel — but one
  manual connect against a real SSH host (password, private-key with
  `~/.ssh/...` path, and tmux enabled) is still the honest final check.

---

## Manual verification checklist (carry-over, unchanged in substance)

Run on each platform before considering it at parity:

- [ ] Open 2+ terminal tabs, generate output, switch between them — history
      persists in each.
- [ ] Type `exit` in a terminal — only that tab closes, app stays running.
- [ ] Open/close/reopen terminals — tab names never repeat.
- [ ] Right-click a terminal tab → Rename — works, persists.
- [ ] Status bar shows real model name, "Connected" after a response, and a
      token count that updates.
- [ ] Code-block actions ("Copy to Terminal" / "Execute in Terminal" /
      "Save to Commands") behave distinctly and appear on every block.
- [ ] Commands sidebar: pin/unpin, edit (permanent + one-time), remove,
      drag-reorder pinned commands, Clear History preserves pinned.
- [ ] Copy/paste with the platform's native shortcuts — paste lands exactly
      once.
- [ ] Cancel a running command with the platform's native SIGINT shortcut —
      works even with text selected (the macOS Cmd/Ctrl risk above).
- [ ] `/exit` closes all terminals and quits the app.
- [ ] A misconfigured/unreachable LLM provider produces a graceful error in
      chat, not a crash.
- [ ] Light/Dark theme switch applies everywhere; light-mode code blocks use
      light syntax colors.
- [ ] SSH + tmux fills the full terminal width immediately on connect.
- [ ] Ollama API key + remote-host warning behave as documented (Settings →
      LLM Config).
- [ ] WinRM: TLS on by default (port 5986), plaintext opt-in shows the loud
      unencrypted-traffic warning, skip-verify checkbox only visible with
      TLS on.

---

# Resolved and removed (2026-09-05 burn-down)

Everything below was in the original file and has been verified and deleted
from the active checklist. Verification method: reading the shipped code
with the specific commit/behavior cited, and the Linux build/test pass
(`go test ./services/... -count=1`, `go build ./...`, `npx vitest run`,
`npx tsc --noEmit` — all green at commit `1a979f7`).

## Section 1 items (cross-platform by construction — verified in code)

- **Terminal history persistence + ResizeObserver zero-size guard:** shipped
  in `TerminalPreview.tsx` (fit is skipped when the container has no size);
  the per-tab persistently-mounted xterm instances are structural
  (`TerminalTab`/`TerminalPreview` keep instances mounted across tab
  switches).
- **Status bar real data:** implemented across `StatusBar.tsx` (model name,
  connection status incl. the LLM activity indicator), `chatStore.ts`
  (tokenCount estimated in `finalizeMessage`), `useLLMStream.ts` (stream
  events drive it). Unit-tested (`StatusBar.test.tsx`, `useLLMStream.test.ts`).
- **Chat auto-scroll:** single `ResizeObserver`-driven in
  `ChatMessageList.tsx`.
- **Tailwind `@theme` mapping:** present in `frontend/src/index.css`
  (`@theme inline` surface-scale tokens); the entire component tree uses the
  `surface-*` scale (swept in the light-theme work, section 6).
- **CodeBlock chat actions on every block:** shipped in `CodeBlock.tsx`;
  Execute no longer auto-logs to Commands.
- **Commands sidebar (pin/edit/one-time-edit/remove/drag-reorder/Clear
  History preserves pinned + EditCommandDialog modal):** all shipped and
  unit-tested (`CommandSidebar.test.tsx`, `CommandCard.test.tsx`,
  `EditCommandDialog` flows). The focus-steal race fix (deferred focus to
  next animation frame) is present in `EditCommandDialog`/`TerminalTab`.
- **`/exit` slash command:** shipped in `ChatPane.tsx`
  (`CloseTerminal` for all tabs + Wails `Quit()`).
- **Right-click rename on terminal tabs:** shipped in `TerminalTab.tsx`
  (with the same deferred-focus fix for the inline input).
- **Ollama nil-`http.Client` crash:** moot — the Ollama SDK was replaced
  entirely by a self-owned `net/http` client (R-04, `services/llm/ollama.go`)
  which constructs its own `*http.Client`; the SDK's nil-client bug cannot
  recur. `recover()` guards remain in the streaming goroutine.
- **System prompt tweaks (fenced-blocks-only-runnable, no duplicate
  commands, one-command-per-block, no `&&` chaining of independent
  commands):** all present in `services/llm/context.go` `SystemPrompt`.

## Section 2 items (Windows ConPTY-specific — Unix path verified by code + tests)

- **Shell `exit` hang/crash (ConPTY pipes don't signal EOF):** not applicable
  on Linux/macOS — `pumpPTYOutput` (the shared Unix read loop in
  `services/pty_service.go`) gets a real EOF/read error from the PTY master
  when the child exits, and its cleanup path removes the session, closes the
  master (guarded by the map-presence check the notes asked for), and emits
  `pty:closed`. The map-presence-check defensive consistency item **was
  applied** to the Unix path as suggested. Covered by PTY read-loop tests.
- **Console-window flash on subprocess spawn:** the `HideWindow` /
  `CREATE_NO_WINDOW` attributes are Windows-only `SysProcAttr` settings,
  present and unchanged in `pty_windows.go` / `clipboard_windows.go`. The
  Unix clipboard path (`wl-copy` / Wails native clipboard) spawns no window
  by construction — nothing to port.
- **Hand-rolled ConPTY syscalls:** no Unix analog (`creack/pty` is the
  mature library). No action possible or needed on Linux/macOS.

## Section 3 items

- **App icon aspect ratio:** the source image is now square —
  `build/appicon.png` measured at 512×512 (was 698×665). The
  source-level defect is resolved; remaining visual confirmation on real
  macOS/Windows is retained above.
- **Terminal Ctrl+C / Cmd+C on macOS:** **retained** above — needs a real
  Mac (see Remaining items). The current handler is unchanged.
- **Windows-only test files / thin Unix PTY coverage:** the Unix path now
  has test files (`remote_ssh_test.go` with an in-process SSH server
  covering connect/PTY/resize/exit/auth-methods/tmux-attach,
  `remote_winrm_test.go` + `remote_winrm_tls_test.go` covering
  line-buffering/auth/TLS-endpoint construction, and the PTY read-loop
  exercised via `openLocalTMTerminal` tests). The parity gap this item
  called out is closed to the extent unit tests can close it (real remote
  boxes are retained above).

## Section 4 (manual checklist)

- The checklist itself is retained (above), with items that referenced
  resolved code (non-square icon, TLS-absent WinRM, missing Ollama key)
  updated or replaced.

## Section 5 items (remote terminals)

- **`expandHomeDir` `~` expansion:** shipped and unit-tested
  (`TestExpandHomeDir`, `TestBuildSSHAuthMethods_PrivateKey_TildeExpansion`
  in `services/remote_ssh_test.go`). The "confirm `os.UserHomeDir()` on a
  real box" residual is minimal and folded into the real-SSH-host check.
- **Keychain keys containing colons:** `sanitizeKey` shipped long ago;
  no-op on Linux/macOS by construction. Context only — no action.
- **Test `$HOME`/`%USERPROFILE%` isolation:** present in all affected test
  files (`t.Setenv("USERPROFILE", ...)` alongside `HOME`). Context only.
- **No macOS Keychain backend:** **stale — resolved in code.**
  `keyring.KeychainBackend` is in `AllowedBackends` with
  `KeychainTrustApplication: true`. Retained above only as a
  verify-on-real-Mac item.
- **WinRM client never run off Windows:** the pure-Go argument stands, and
  the WinRM TLS endpoint construction is now unit-tested on Linux; the real
  connect test is retained above.
- **`useDefaultTerminalFocus.ts`:** shipped; uses only standard DOM APIs.
  The click-through verification is folded into the manual checklist.
- **tmux create-or-attach without remote tmux:** visible-error behavior is
  by design (backend writes the command; the shell reports the missing
  binary). Context only — no action.
- **Remote-terminal manual checklist:** retained (above).

## Section 6 items

- **Light theme (surface scale + xterm theme + MutationObserver + shiki
  light/dark pair):** all shipped in `index.css`, `TerminalPreview.tsx`,
  `CodeBlock.tsx`; visually verified by the frontend test suite only to the
  extent code assertions can go — the visual smoke test is folded into the
  manual checklist.
- **tmux half-width bug (onResize registered after first fit):** the fix is
  present and commented in `TerminalPreview.tsx` (listener registration
  explicitly ordered before the first `fitAddon.fit()` call). The real-host
  tmux check is retained in the manual checklist.
- **System prompt anti-chaining:** present in `services/llm/context.go`.
- **EditCommandDialog:** shipped and unit-tested.
- **macOS hotkey default question:** **retained** above (needs a real Mac).
- **Hotkey capture Meta handling:** `buildKeyCombo` records `metaKey` and
  `matchesHotkey` compares it — rebinding works as described. Folded into
  the macOS hotkey item above.
- **Section 6 manual checklist:** retained (above).

## On testing rigor

The original file's closing lesson stands and is worth keeping verbatim in
spirit: unit tests prove the code does what the tests assume — not that it
works against a real server, a real remote host, or a real WebView engine.
That is exactly why the checklists above are retained even after the
code-verifiable items were burned down.
