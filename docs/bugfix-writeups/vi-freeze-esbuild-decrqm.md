# Bug: opening a full-screen curses program (vi, and likely any program that probes terminal modes) freezes the terminal completely and permanently, requiring a hard-kill of the session

## Symptom
Reported during UAT, over SSH (reproduced against a real remote Linux host — the mechanism is host-agnostic, see Root cause). Opening a file with `vi <filename>` draws the editor, but from that point on:
- No keystroke has any visible effect anywhere on screen, including at the cursor's own starting position — not garbled, not delayed, just completely static.
- Resizing the PairAdmin window *does* cause a partial repaint (of stale, already-drawn content), but the content itself never advances.
- The only way out is force-closing the whole terminal session; nothing typed reaches an exit path (`Escape`, `:q!`, etc. all appear equally dead).
- The identical operation against the identical host works perfectly in PuTTY, immediately eliminating the remote host, vi itself, and the SSH server side as causes.

## Where
Not a bug in PairAdmin's own source at all — it's in the frontend's **production build tooling**. Specifically: [`frontend/package.json`](../../frontend/package.json)'s `vite` devDependency, which pins `esbuild ^0.21.3` for its build-time minification, and [`frontend/vite.config.ts`](../../frontend/vite.config.ts), which had no `build.minify` override and so used that default.

## Investigation trail (see also `vi-freeze-convertEol.md` for a real, separate bug found and fixed along the way)
Ruled out in order, each with direct evidence, before the real cause surfaced:
1. **PairAdmin's own key-handling/hotkey code** — traced every keydown listener; none intercept vi's keys.
2. **`WriteInput` dispatch and the SSH write path** — inspected the Go source; correctly routes to the SSH session's stdin.
3. **tmux copy-mode** — ruled out directly by the user (no tmux involved).
4. **The remote host, vi itself, the SSH server** — eliminated conclusively: PuTTY against the identical host worked. This was the pivot point that narrowed everything to PairAdmin's own client code.
5. **The SSH PTY's window size never updating** (a real, separate concern raised earlier: `RequestPty` hardcodes `24, 80`) — directly disproven with hard evidence: `stty -F /dev/pts/N size` on the actual running `vi` process's controlling tty, read independently via a second SSH session, returned a real, resize-driven value (`53 130`), not the hardcoded placeholder. **`WindowChange` resize is working correctly.**
6. **`convertEol: true` corrupting cursor tracking on scroll-region LFs** — a real bug, mechanically proven with a headless test against the exact xterm.js version shipped (see `vi-freeze-convertEol.md`), and fixed. But it does not match this symptom: that corruption produces wrong-but-changing output, and the report was a *total*, unchanging freeze, confirmed by the next step.
7. **Whether input was even reaching vim at all** — checked directly via `/proc/<pid>/io` on the live `vi` process, read before and after a burst of keystrokes in the frozen PairAdmin session:
   ```
   before: rchar 242588  wchar 35036   syscr 145  syscw 27
   after:  rchar 242603  wchar 35366   syscr 156  syscw 39
   ```
   `rchar`/`syscr` increased (**+15 bytes across 11 reads** — vim received the keystrokes) and, critically, `wchar`/`syscw` also increased (**+330 bytes across 12 writes** — vim genuinely produced new output in response). Both directions of the underlying pipe were provably intact. This isolated the bug to exactly one place: **PairAdmin's own consumption/rendering of already-arrived SSH output**, not the remote, not the connection, not the write path.
8. **Rebuilt with `wails build -debug`** (devtools disabled by default in production builds; this flag enables them) and reproduced live with the WebView2 DevTools console open. The console showed:
   ```
   Uncaught ReferenceError: r is not defined
       at o$.requestMode (index-BG619nQ3.js:197:103687)
       at Array.<anonymous> (index-BG619nQ3.js:197:77895)
       at t$.parse (index-BG619nQ3.js:197:68053)
       at o$.parse (index-BG619nQ3.js:197:83386)
       at c$._action (index-BG619nQ3.js:197:121793)
       at c$._innerWrite (index-BG619nQ3.js:197:117130)
   ```
   An uncaught exception inside xterm.js's own parser, in its `requestMode` handler — the implementation of **DECRQM** (`CSI Ps $ p` / `CSI ? Ps $ p`, "request terminal mode"), the escape sequence a terminal-aware program sends to probe capabilities (bracketed paste, synchronized output, cursor-key mode, etc.) before deciding how to behave. A plain shell prompt never sends this; a real curses program's startup capability-probing does — which is exactly why ordinary shell interaction was unaffected while vi broke immediately and totally. Once xterm.js's parser throws mid-write, it never recovers: every subsequent `write()` call for that terminal instance — including the 330 bytes of real vim output already confirmed above — silently does nothing. `resize()` is a separate, unrelated code path, which is why resize still produced a (stale) repaint while nothing else ever updated again.

## Root cause
This exact failure — `xterm.js`'s DECRQM/`requestMode` handler throwing `ReferenceError: r is not defined` specifically when built through Vite's esbuild-based minifier — is a known, documented upstream bug: [`agentmuxai/agentmux#3343`](https://github.com/agentmuxai/agentmux/pull/3343), *"fix(build): vim freezes the terminal — esbuild miscompiled xterm's DECRQM handler."* The mechanism, per that report: esbuild's minifier constant-folds the read of an uninitialized `let r;` binding to `void 0`, concludes the declaration is therefore "unused," and drops it — while leaving the later **write** to `r` in place. The result parses fine (it's valid JS) but throws at runtime the moment that code path actually executes, which for this specific handler means the moment any program sends a DECRQM query.

Confirmed this applies here: PairAdmin's frontend resolves `esbuild@0.21.5` (well before the fix), and manually extracting the built `requestMode` method from the actual shipped bundle shows precisely the corrupted shape the upstream report describes:
```js
requestMode(e,t){let n;(n||(n={}))[r.NOT_RECOGNIZED=0]="NOT_RECOGNIZED",...
```
(`r` read and written, never declared — the `var r;`/`let r;` that should precede it is gone.)

The upstream fix landed in **esbuild 0.28.2**.

## Fix attempted and rejected
The obvious first fix — force esbuild to `0.28.2` via an `overrides` entry in `frontend/package.json` — **does not work as a drop-in change**. Vite `5.4.21` (the version this project's `devDependencies` currently resolve) calls esbuild with a set of target-environment transform options that esbuild 0.28.2 no longer supports the same way; the build fails outright:
```
Transforming destructuring to the configured target environment
("chrome87", "edge88", "es2020", "firefox78", "safari14" + 2 overrides)
is not supported yet
```
Upgrading esbuild alone, without also upgrading Vite to a version built against that newer esbuild, is not viable here.

## Fix applied
Switched the production minifier from esbuild's (default) to **terser**, which has no equivalent bug and needs no esbuild version change at all:

```diff
--- a/frontend/vite.config.ts
+++ b/frontend/vite.config.ts
@@
   resolve: {
     alias: { "@": path.resolve(__dirname, "./src") },
   },
+  build: {
+    minify: "terser",
+  },
```

```diff
--- a/frontend/package.json
+++ b/frontend/package.json
@@
     "jsdom": "^29.0.1",
+    "terser": "^5.51.2",
     "typescript": "^5.9.3",
```

(Applied on local branch `experiment/vi-freeze-diagnosis`, not pushed.)

Rebuilding after this change and re-extracting the same `requestMode` method from the new bundle shows the correct, intact pattern:
```js
requestMode(e,t){let n;var r;(r=n||(n={}))[r.NOT_RECOGNIZED=0]="NOT_RECOGNIZED",...
```
`r` is now properly declared (`var r;`) and assigned (`r=n||(n={})`) before any property access — the exact shape the esbuild bug destroyed.

## Verification done
- Root cause confirmed against a live, running `vi` process via `/proc/<pid>/io` (proves input/output were both intact at the OS level) and a live DevTools console capture (proves the exact uncaught exception and its call stack).
- Matched to a specific, named, already-fixed upstream issue with an exact mechanism match (constant-folded read, dropped declaration, kept write) — not a guess.
- Confirmed the project's resolved `esbuild` version (`0.21.5`) predates the fix (`0.28.2`).
- Confirmed the naive fix (bumping esbuild directly) breaks Vite's own build pipeline — documented so nobody re-attempts it and burns the same cycle.
- Rebuilt with the `terser` fix; `npm run build` completes cleanly with no errors or new warnings.
- Manually extracted and compared the compiled `requestMode` method before and after — the corrupted pattern is gone; the correct declare-then-use shape is present.
- **Live reproduction — done, confirmed fixed.** Reopened `vi` against the same host in the binary built with this fix: fully functional, insert mode works, the ruler redraws, editing behaves normally. This closes the report.

## Suggested follow-up for the implementer
- This is worth reporting upstream if it hasn't been already for this exact combination (esbuild 0.21.x + this xterm.js version) — the linked PR is against a different project (`agentmuxai/agentmux`), so xterm.js's own tracker and/or Vite's may not have this specific combination flagged. Low priority, but it's a real correctness bug affecting anyone shipping xterm.js through an unpatched esbuild 0.21.x-era Vite build, which is plausibly a meaningful population.
- Longer-term, PairAdmin's own Vite/esbuild versions should be upgraded together (not esbuild alone) so the project can eventually drop the `terser` workaround and use current, non-buggy esbuild minification again — esbuild is meaningfully faster than terser at build time. Not urgent; the terser fix is correct and sufficient on its own.
- Worth a quick check of `frontend/package.json`'s other devDependencies for whether a coordinated Vite 6/7 upgrade is low-risk enough to bundle with that later esbuild bump, given how many packages (`@vitejs/plugin-react`, `vitest`, `@tailwindcss/vite`, etc.) have version constraints tied to the Vite major version.
- The `convertEol: true` fix from the parallel investigation (`vi-freeze-convertEol.md`) should still land — it's a real, separate, proven bug, independent of this one.
