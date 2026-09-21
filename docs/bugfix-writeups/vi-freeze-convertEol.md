# Bug: `convertEol:true` corrupts cursor tracking for any program that uses scroll-region LFs

**Scope correction:** this was investigated as a hypothesis for the "vi appears totally frozen" report before the actual cause was found and fixed — see [vi-freeze-esbuild-decrqm.md](vi-freeze-esbuild-decrqm.md) for that. This bug is real, mechanically proven below, and independently worth fixing — but it is **not** what caused the freeze. A `convertEol` corruption produces garbled-but-changing output; the freeze report was a total, unchanging lockup, which pointed elsewhere once tested directly (see the other writeup's evidence trail). Keeping both fixes.

## Symptom
Reported during UAT: opening a file with `vi <filename>` in a PairAdmin terminal (reproduced over SSH; the mechanism is host-agnostic — see Root cause) draws the editor, but no keystroke has any visible effect — no cursor movement, no insert-mode entry, no `:` command line, nothing. There is no way out via the keyboard; the only recovery is force-closing the whole terminal session.

Plain shell interaction in the same kind of session (typing commands, seeing output, editing at the prompt) works completely normally. Only full-screen/curses programs are affected.

## Ruled out before finding the real cause
Investigated in order, each eliminated with evidence before moving to the next:
- **PairAdmin's own hotkey/key-interception code** (`useConfiguredHotkey`, `useQuickSelect`, the `Ctrl+C`/`Ctrl+V` handling in `attachCustomKeyEventHandler`) — all scoped to specific modifier combinations vi doesn't use; every other keystroke passes through untouched.
- **`WriteInput` dispatch** (`services/pty_service.go`) — routes correctly to the SSH/ConPTY/local-PTY write path with no silent-drop branch.
- **tmux copy-mode** — the user confirmed tmux was not involved in the reproduction.
- **The remote host / vi itself / SSH server-side terminfo** — the user connected to the *same host* with PuTTY and reproduced the identical operation successfully. This was the decisive elimination: it proves the bug is 100% client-side, in PairAdmin's own terminal-rendering code, not the remote program or the connection's server side.
- **SSH PTY dimension race** (`RequestPty("xterm-256color", 24, 80, ...)` is hardcoded and the real size only arrives via a later `WindowChange` call) — a real, separate, and worth-fixing latent issue (see Suggested follow-up), but a resize arriving after a curses program starts is normally handled by that program's own `SIGWINCH` handler and does not explain total, unrecoverable unresponsiveness. Deprioritized once the mechanism below was proven mechanically.

## Where
[`frontend/src/components/terminal/TerminalPreview.tsx`](../../frontend/src/components/terminal/TerminalPreview.tsx) — the `xterm.js` `Terminal` constructor options.

## Root cause
`convertEol: true` was set on the `Terminal` instance. This option forces xterm.js to treat every incoming bare `\n` (LF) as if it were `\r\n` — i.e., every linefeed also returns the cursor to column 0.

That is **not** real VT100/xterm terminal behavior. Under the actual standard (and under xterm.js itself with `convertEol: false`, its own default), a bare LF moves the cursor down one row **without changing its column** — except inside a defined scroll region (`DECSTBM`, `\x1b[<top>;<bottom>r`) at the bottom margin, where it instead scrolls the region's content up by one line, again without touching the column.

That exact mechanism — set a scroll region, then emit bare LFs at the bottom margin to scroll it — is the standard optimization full-screen/curses programs (vim, less, top, and effectively anything built on ncurses/terminfo) use for smooth, efficient redraws, because it's far cheaper than repositioning the cursor and rewriting every line on every scroll. A plain shell session essentially never exercises this path — ordinary prompt/command output doesn't set scroll regions — which is exactly why basic shell interaction was unaffected while vi broke completely.

With `convertEol: true`, every one of those scroll-triggering LFs also snaps the cursor to column 0. The running program's own internal model of where its cursor is (which it computed assuming standard, correct terminal behavior) immediately diverges from where xterm.js is actually drawing. This compounds with every subsequent scroll — which, in an editor, is on the order of every few keystrokes — so within a very short time the screen becomes irrecoverable, incoherent noise. Input is, mechanically, still reaching the remote program the entire time; there is simply no way to tell, because nothing on screen ever reflects it in a way a user can read.

### Mechanical proof (not just theory)
[`proof.mjs`](vi-freeze-convertEol/proof.mjs) in this directory is a self-contained, headless script using the exact `@xterm/xterm` version PairAdmin ships (`^6.0.0` — resolved `6.0.0`). It constructs a `Terminal` two ways (`convertEol: true` and `false`), feeds each an identical minimal transcript that (1) sets a scroll region, (2) places the cursor at a known non-zero column within it, then (3) emits one bare LF — and reads back the resulting cursor position from xterm.js's own buffer.

To run it:
```bash
mkdir /tmp/xterm-proof && cd /tmp/xterm-proof
npm init -y >/dev/null && npm install @xterm/xterm@^6.0.0
cp <path-to-this-repo>/docs/bugfix-writeups/vi-freeze-convertEol/proof.mjs .
node proof.mjs
```

Actual output when this was run:
```
convertEol: false (correct VT100 behavior) -> { cursorX: 15, cursorY: 8 }
convertEol: true  (PairAdmin's current setting) -> { cursorX: 0, cursorY: 8 }

PROVEN: convertEol:true snaps the cursor to column 0 after a scroll-region LF, where real VT100/xterm semantics (and convertEol:false) preserve the column. This is a real, mechanical corruption of cursor state for any curses program using LF-scroll inside a defined region.
```
`cursorY` (the row) is identical in both cases — the corruption is specifically and only in column-tracking after an LF, exactly as the option's own semantics would predict, and exactly the input curses redraw logic depends on being correct.

### How it got here
The `Terminal` instance — and `convertEol: true` with it — was introduced in the very first commit that created this file (`9cc6fe7`, `feat(01-02): xterm.js terminal preview with canvas addon and mock content`), which wrote **static mock demo content** using plain `"\n"` line breaks (ordinary JS string convention) before any real PTY existed to connect to. `convertEol: true` was the right call for that mock content. Real PTY output was wired in several commits later (`de783ee`, `feat(terminal): add interactive PTY shell to + New terminal tabs`), and the setting was never revisited — a real shell's own output already carries proper `\r\n`, so the redundant extra CR was silently harmless for ordinary prompt/command use, which is presumably why this has never surfaced before now: nobody had exercised a full-screen program through it until this UAT session.

## Fix
Set `convertEol: false` (xterm.js's own default — explicit here for a future reader, with the reasoning above as a comment so it isn't quietly re-enabled).

```diff
       scrollback: 1000,
-      convertEol: true,
+      convertEol: false, // see docs/bugfix-writeups/vi-freeze-convertEol.md
       cursorBlink: true,
```

(Applied in the working tree on local branch `experiment/vi-freeze-diagnosis`, not pushed — see `git diff master -- frontend/src/components/terminal/TerminalPreview.tsx` on that branch. The inline comment in the actual diff is longer than shown above; see the file.)

No other code path depends on `convertEol: true`. The one other place that manually formats line endings for `term.write()` — `GetWindowsContent`'s pull-based native-console content path, a few lines below in the same file — already explicitly joins lines with `\x1b[K\r\n` itself, so it supplies its own correct CRLF regardless of this setting and is unaffected by the change.

## Verification done
- Headless mechanical proof above, run against the exact shipped `@xterm/xterm` version — confirms the corruption exists and confirms `convertEol: false` avoids it, using xterm.js's own buffer state as the source of truth (not a visual/subjective check).
- `grep -rn "convertEol" frontend/src` — confirms the constructor option is the only reference in the codebase; no test or other file asserts on it, so nothing else needs updating.
- **Live reproduction — done, confirmed fixed.** `vi` is fully functional in the built-with-this-fix binary against the original reporting host (this fix shipped in the same build as `vi-freeze-esbuild-decrqm.md`'s terser fix, which was the actual cause of the total freeze; this one's contribution is correctness of scroll rendering rather than the freeze itself, and no corruption was observed during normal use). No dedicated scroll-stress test was run beyond ordinary editing — see follow-up below if a more targeted check is wanted.

## Suggested follow-up for the implementer
- Land the one-line `convertEol` change and the confirming comment; the diff above is ready to apply as-is.
- Consider adding `proof.mjs`'s technique as a permanent regression test (e.g. `frontend/src/components/terminal/__tests__/TerminalPreview.test.tsx` or a new dedicated test file) — it's fast, deterministic, needs no real PTY or browser, and would catch this exact regression forever if `convertEol` is ever flipped back without someone reading this file.
- Separately, worth its own follow-up (not blocking this fix): `RequestPty`/`WinConPtyStart` hardcode initial PTY dimensions (`24, 80` and `120, 40` respectively) rather than receiving the frontend's actual size at connect time, relying entirely on a `WindowChange`/`ResizeTerminal` call arriving afterward. Not the cause of this bug, but a real latent race worth closing — thread the real cols/rows through `RemoteConnectParams` into the initial PTY/ConPTY creation instead of a placeholder.
