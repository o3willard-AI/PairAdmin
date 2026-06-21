# Platform Parity Notes (TEMPORARY — delete before v1.0)

**Audience:** coding agents building/maintaining the Linux and macOS targets.
**Purpose:** a long Windows-focused debugging/feature session (see `git log`
from roughly `a141259` through `1fc9d60`) fixed a lot of bugs and added
several features. Most of the work was in shared frontend code and applies
to every platform automatically, but a few fixes were Windows-specific and
need an equivalent check — or an equivalent fix — on Linux and macOS.

**Lifecycle:** once Linux and macOS builds have been verified to have
equivalent (or better) functionality for everything below, delete this file.
It is not meant to ship in a v1.0 release.

---

## 1. Already cross-platform — verify by smoke test only

Everything below lives in `frontend/src/**` or platform-agnostic Go and
should "just work" identically on every OS once built. No porting needed —
just confirm during your platform's build/test pass:

- **Terminal history persists across tab switches.** Each tab keeps its own
  persistently-mounted `xterm.js` instance; switching tabs toggles CSS
  visibility rather than unmounting. Also fixed: a `ResizeObserver` bug
  where a hidden tab collapsing to 0×0 would send a near-zero resize down to
  the PTY and truncate its scroll buffer — `TerminalPreview.tsx` now skips
  fitting when the container has zero size. Verify: open 2+ terminals,
  generate output in each, switch back and forth, confirm nothing is lost.
- **Status bar shows real data.** Model name, connection status, and token
  count were all permanently-wrong hardcoded placeholders ("No model",
  "Disconnected", no token count ever updating). Fixed in
  `ThreeColumnLayout.tsx`, `StatusBar.tsx`, `useLLMStream.ts`,
  `chatStore.ts`. Verify: status bar reflects your actual configured
  provider/model, shows "Connected" after a successful response, and the
  token count updates.
- **Chat auto-scroll.** Old logic mixed a throttled instant-jump against a
  smooth animated scroll, causing visible jitter, and didn't always land at
  the true bottom once async syntax highlighting in code blocks changed the
  layout after the last scroll fired. Now driven by a single
  `ResizeObserver` on the message content, in `ChatMessageList.tsx`. Verify:
  ask a question that returns a code block; confirm no jitter and the view
  ends up fully scrolled to the bottom.
- **Tailwind theme tokens were silently broken app-wide.** There was no
  `@theme` mapping from the project's `--muted`/`--foreground`/etc CSS
  variables to Tailwind v4 color tokens, so `bg-muted`, `text-foreground`,
  `hover:bg-muted-foreground/30`, and every other semantic-color utility
  compiled to **nothing** — verified via the compiled CSS output containing
  zero generated rules for these classes. This affected chat bubbles,
  buttons, badges, and code-block hover states everywhere, not just one
  reported symptom. Fixed with an `@theme inline` block in `index.css`.
  This is a pure CSS fix with no OS dependency, but it's significant enough
  that every platform should double check buttons/bubbles render with
  visible hover states and correct colors, not just inherited/default ones.
- **CodeBlock chat actions.** "Copy to Terminal" / "Execute in Terminal" /
  "Save to Commands" with pipe separators and visible hover shading; blocks
  tagged `text`/`plaintext`/unspecified no longer render action buttons
  (those are meant to be illustrative, not actionable, per the system
  prompt in `services/llm/context.go`); Execute no longer auto-logs to
  Commands (Save is the only thing that does, to avoid duplicate entries).
- **Commands sidebar.** Shared across all terminal tabs (not scoped to
  whichever tab is active); right-click context menu for Pin/Unpin, Edit
  (permanent), Edit/Append for next use (one-time override), Remove; pinned
  commands sit above the scrolling list and are drag-reorderable; "Clear
  History" preserves pinned commands. Watch for: a focus race where the
  context menu's focus-return-to-trigger can steal focus back from the
  inline rename/edit `<input>` before the user types anything, silently
  discarding the edit — fixed by deferring `.focus()` to the next animation
  frame in both `CommandCard.tsx` and `TerminalTab.tsx`. This is a generic
  React/focus-management race, not Windows-specific, but it's exactly the
  kind of thing that can resurface if either component is touched again.
- **`/exit` slash command** — closes every open terminal tab via
  `PTYService.CloseTerminal`, then calls the Wails runtime's `Quit()`.
  `Quit()` is part of the Wails runtime itself, not OS-specific.
- **Right-click rename on terminal tabs** (`TerminalTab.tsx`) — same
  pattern/fix as the Commands sidebar edit.
- **Ollama nil-`http.Client` crash** (`services/llm/ollama.go`) —
  `ollamaapi.NewClient` was constructed with `nil` for its `*http.Client`
  argument; the SDK stores that as-is with no nil-check, so the first real
  chat request through Ollama dereferenced a nil pointer and panicked the
  *entire process* (unrecoverable, since it ran in a detached goroutine).
  This is pure Go with zero OS dependency — affects every platform equally
  if Ollama is configured as the provider. Also added a `recover()` guard
  to all three providers' (Ollama/Anthropic/OpenAI) streaming goroutines so
  a future SDK panic becomes a chat error message instead of taking the app
  down — this matters on every platform, since an unrecovered goroutine
  panic kills the whole process regardless of OS.
- **System prompt tweaks** (`services/llm/context.go`) — tells the model to
  only use fenced code blocks for commands it's actually recommending, and
  never show the same command twice across two different fences. Pure
  prompt text, no platform dependency.

## 2. Windows-specific fixes — need an equivalent check

These addressed bugs specific to Windows' ConPTY/console plumbing. The
Linux/macOS path uses real PTYs via `github.com/creack/pty`
(`services/pty_service.go`, non-Windows branch), which behaves differently
at the OS level — most of these bugs likely don't exist there, but **verify
rather than assume**:

- **Exiting a shell (`exit`) used to hang the session in limbo, then later
  crashed the whole app.** Root cause was ConPTY-specific: its pipes don't
  signal EOF just because the child process exited, so a watcher goroutine
  had to be added (`cpty.Wait()` + forced `Close()`), and an early version
  of that watcher raced with the read loop and double-closed the same
  Windows handles, corrupting the heap (`STATUS_HEAP_CORRUPTION`) and
  killing the app instantly.
  **On Linux/macOS:** real PTYs *do* signal EOF/an error on the master fd
  when the child exits and the slave end closes, so the read loop in
  `pty_service.go`'s non-Windows branch should already detect this and
  clean up correctly without needing an equivalent watcher. **Verify this
  explicitly** — open a terminal, type `exit`, confirm only that tab closes
  and the app stays running, including with 2+ terminals open. If it works,
  no code change needed; if it hangs, the watcher pattern in
  `services/pty_windows.go` is the reference fix.
  Note also: `CloseTerminal` and the read-loop's error branch can both call
  `session.ptmx.Close()` on Unix today — unlike Windows, double-closing a Go
  `*os.File` on Unix is low-severity (returns `EBADF`, doesn't corrupt
  anything), so this is *not* an urgent fix, but applying the same
  map-presence-check pattern used in `pty_windows.go` (whichever side
  observes the session first deletes it under the lock, and only that side
  performs the close) would be good defensive consistency.
- **A visible external console window would flash when spawning the
  fallback shell or the OS clipboard helper.** Fixed by adding
  `CREATE_NO_WINDOW`/`HideWindow` to the relevant `exec.Command` calls in
  `services/pty_service.go` and `services/clipboard_windows.go`.
  **On Linux/macOS:** the clipboard path (`services/clipboard_unix.go`)
  uses `wl-copy` on Wayland or the Wails runtime's native clipboard API on
  X11 — neither spawns a visible window, so this specific bug shouldn't
  exist there. Worth a quick sanity check anyway, especially if any new
  subprocess-spawning code gets added to the Unix path later.
- **Hand-rolled ConPTY syscalls never actually attached the child process to
  the pseudoconsole**, despite every Win32 API call reporting success —
  this took a lot of debugging to find (see commit history around
  `af25265`/`9c55f43`) and was ultimately replaced with a vendored,
  corrected implementation (`services/conpty_windows.go`, adapted from
  `github.com/UserExistsError/conpty`, MIT licensed). This has no Linux/Mac
  analog — `creack/pty` is a mature, correct library — but if either
  platform's PTY integration ever needs similar low-level work, the
  debugging approach that worked here was: write a minimal standalone
  reproduction outside the GUI app, and compare against a known-working
  reference implementation byte-for-byte rather than trusting that Win32
  API success codes mean what they say.

## 3. Concrete gaps found — these likely *do* need platform-specific work

- **App icon aspect ratio.** `build/appicon.png` (the cross-platform source
  icon, used for Linux and as the base for macOS's `.icns` generation) is
  currently **698×665 — not square**. The previous icon was 1024×1024
  (square). macOS's `.icns` generation (via Wails' build pipeline, which
  shells out to `iconutil`/`sips`) generally expects a square source image;
  a non-square source can produce a distorted or cropped app icon, dock
  icon, or `.icns` build failure. **Action for the macOS builder:** verify
  the icon renders correctly after `wails build`; if it's distorted, either
  get a square crop of `PA_Icon1.png` or generate a proper square
  `.icns`/iconset directly rather than relying on Wails' auto-conversion of
  the non-square source. Windows is unaffected because `build/windows/icon.ico`
  was set directly from a purpose-built `.ico` file
  (`pair_admin.ico`), not auto-generated from `appicon.png`.
- **Terminal Ctrl+C / Cmd+C semantics on macOS.** The clipboard fix in
  `TerminalPreview.tsx` treats `ctrlKey || metaKey` identically: copy the
  selection if one exists, otherwise fall through as SIGINT. On Windows/
  Linux that's fine (`Ctrl+C` is the only relevant modifier). **On macOS,
  `Cmd+C` and `Ctrl+C` are conventionally different shortcuts** — `Cmd+C`
  should always copy (standard macOS app-wide copy shortcut) and `Ctrl+C`
  should always send SIGINT (standard Unix terminal convention,
  independent of whether anything is selected). The current logic doesn't
  distinguish which modifier was pressed, so on macOS:
  - `Ctrl+C` with a selection active would incorrectly copy instead of
    sending SIGINT (surprising if a user has text selected but wants to
    cancel a running command).
  - `Cmd+C` with no selection would incorrectly send SIGINT instead of
    being a no-op or copying nothing.
  **Action for the macOS builder:** split the handling — `event.metaKey`
  (Cmd) always attempts copy (no-op if no selection), `event.ctrlKey`
  (without metaKey) always sends SIGINT regardless of selection state. Same
  applies to paste: macOS conventionally pastes with `Cmd+V`, not `Ctrl+V`;
  verify which one xterm's native paste event actually fires for, and
  ensure the explicit `Ctrl+V` handling in the same file matches macOS
  convention (likely needs `Cmd+V` instead of, or in addition to, `Ctrl+V`).
- **Windows-only test files** (`go:build windows` tag) obviously don't run
  in Linux/macOS CI. Equivalent coverage for the Unix PTY path
  (`pty_service.go`'s non-Windows branch) is comparatively thin — consider
  adding tests there mirroring what `pty_windows.go`/`conpty_windows.go`
  cover (session open/close, exit detection, write/resize) if not already
  present.

## 4. Manual verification checklist

Repeat this on each platform before considering it at parity:

- [ ] Open 2+ terminal tabs, generate output, switch between them — history
      persists in each.
- [ ] Type `exit` in a terminal — only that tab closes, app stays running.
- [ ] Open/close/reopen terminals — tab names never repeat (e.g. no two
      "Terminal 2"s).
- [ ] Right-click a terminal tab → Rename — works, persists.
- [ ] Status bar shows real model name, "Connected" after a response, and a
      token count that updates.
- [ ] Ask a question that returns a runnable command — "Copy to Terminal",
      "Execute in Terminal", and "Save to Commands" all behave distinctly;
      hovering each shows a visible shade change.
- [ ] Ask a question that returns a non-runnable/illustrative example — no
      action buttons appear on it.
- [ ] Commands sidebar: log a command, switch tabs, confirm it's still
      there. Right-click → Pin, Edit, Edit/Append for next use, Remove —
      each behaves as described in section 1. Drag-reorder two pinned
      commands. Clear History — pinned commands survive.
- [ ] Copy a terminal selection with the platform's native copy shortcut,
      paste it elsewhere — works. Paste into the terminal with the
      platform's native paste shortcut — pastes exactly once, not twice or
      zero times.
- [ ] Cancel a running command (e.g. `ping -t localhost` / `ping localhost`)
      with the platform's native SIGINT shortcut — still works even with
      text selected in the terminal (this is the macOS risk called out in
      section 3).
- [ ] `/exit` in the chat input — closes all terminals and quits the app.
- [ ] If `provider: ollama` (or any provider) is misconfigured/unreachable,
      sending a chat message produces a graceful error message in chat, not
      an app crash.
- [ ] App icon renders correctly and un-distorted in the taskbar/dock and
      window title bar.
