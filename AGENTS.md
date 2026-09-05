# AGENTS.md

**Audience:** coding agents (human or AI) building or maintaining PairAdmin.
**Purpose:** a single source of truth for how this repo is built, tested, and
conventionally changed. Read this before writing code or opening a PR.

---

## 1. Build & Test

These exact commands must pass before any PR. The CI
(`.github/workflows/ci.yml`) runs the same set — if it's green locally, it
will be green in CI.

### Go backend + Wails bindings

```bash
# From repo root (not frontend/):
go test ./services/... -count=1
go build ./...
```

> **Important:** `go build ./...` requires `frontend/dist` to exist because
> `main.go` line 22 has `//go:embed all:frontend/dist`. A fresh checkout has
> no `frontend/dist` (it's gitignored). To build the Go binary locally, build
> the frontend first:
> ```bash
> cd frontend && npm ci && npm run build && cd .. && go build ./...
> ```
> In CI, the `go-test` job builds frontend assets before `go build` (see
> `.github/workflows/ci.yml` lines 60–72).

**Go version:** `go1.26.6` (pinned in CI and `go.mod`). `go vet` runs
automatically as part of `go test`.

### Frontend (React + TypeScript)

```bash
# From frontend/:
npm ci              # clean install — requires package-lock.json (committed)
npx vitest run      # run all tests (non-watch mode, 37 test files total (including
                     # 3 named *.disabled.test.* — despite the "disabled" name,
                     # vitest's exclude pattern does NOT skip them))
npx tsc --noEmit    # TypeScript type-check (also run in CI, line 101)
npm run build       # = tsc && vite build — production build
```

**Node version:** Node 20 (pinned in CI). `npm ci` is used (not `npm install`)
because `package-lock.json` is committed and CI pins to it for reproducibility.

### CI pipeline (4 jobs, all hard gates)

| Job          | What it does                                         |
|--------------|------------------------------------------------------|
| `go-test`    | `npm ci` + `npm run build` in frontend, then `go build ./...` and `go test ./services/... -count=1` |
| `frontend`   | `npm ci`, `npx tsc --noEmit`, `npx vitest run`       |
| `govulncheck`| Supply-chain vulnerability scan — **HARD gate**      |
| `gitleaks`   | Secret scanner on the working tree — **HARD gate**   |

All four must pass for CI to be green.

---

## 2. Conventions

### Injectable seams for tests
Go services accept dependencies via function fields or interfaces so tests
can inject mocks:

- `LLMService` has `emitFn func(ctx context.Context, event string, ...)`
  (defaults to `runtime.EventsEmit`). Tests replace it with a no-op.
- `buildProviderFn` is a **package-level** `var` in
  `services/settings_service.go` line 23 (not a field on `LLMService`):
  `var buildProviderFn func(Config, func(string) string) llm.Provider = buildProvider`.
  `SettingsService.TestConnection` (line 269) calls it; tests swap it via
  `TestSettingsService_TestConnection_Success` and its siblings.
- `SettingsService` accepts an injectable `keychainClient *keychain.Client`
  via `NewSettingsService(kc)`.
- `CaptureManager` exposes `buildFilterPipeline()` for testability.

**Pattern:** prefer injectable fields with package-level defaults over
hardcoded `runtime.EventsEmit` / `os.Getenv` calls inside struct methods.

### Event naming
Wails runtime events use `component:action` colon-separated naming:

| Event               | Emitter                  | Consumer            |
|---------------------|--------------------------|---------------------|
| `llm:chunk`         | `LLMService`             | frontend stream     |
| `llm:error`         | `LLMService`             | frontend stream     |
| `llm:done`          | `LLMService`             | frontend stream     |
| `llm:usage`         | `LLMService`             | frontend stream     |
| `settings:changed`  | `SettingsService.SaveSettings` | frontend     |
| `settings:model-changed` | `SettingsService.SetModel` | frontend  |
| `terminal:rename`   | `SettingsService.RenameTab` | frontend         |
| `pty:output`        | `PTYService`             | frontend terminal   |
| `app:warning`       | `CommandService`         | frontend            |

### Configuration via Viper ONLY (no direct env reads for user settings)
User-facing app settings (hotkeys, theme, font size, provider, model, etc.)
live in `services/config/config.go` and are persisted to
`~/.pairadmin/config.yaml` via Viper (mapstructure + YAML tags). **Never**
read user settings directly from environment variables in `services/`.

The only `os.Getenv` calls in `services/*.go` (outside `_test.go`) are:
- `services/llm_service.go` `LoadConfig()` — reads **runtime/LLM provider**
  env vars (`PAIRADMIN_PROVIDER`, `PAIRADMIN_MODEL`, `OPENAI_API_KEY`,
  `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`, `OLLAMA_HOST`,
  `OLLAMA_API_KEY`, `LMSTUDIO_HOST`). These are LLM backend configuration,
  not user app settings.
- `services/config/config.go` `releaseDataDirForGOOS()` — reads
  `LOCALAPPDATA` / `XDG_DATA_HOME` for OS-conventional data-dir resolution
  (not configuration).
- `services/commands.go` — reads `WAYLAND_DISPLAY` for display-server
  detection.
- `services/pty_service.go` — reads `SHELL` (Unix) for shell selection.
- `services/pty_windows.go` — reads `USERNAME` (Windows log header, line 17)
  and `ComSpec` (Windows shell) for terminal session logging and shell
  selection.

### Structured logging
Go services do **not** use a structured-logging framework (`slog`,
`zap`, etc.) yet. Logging is informal: `fmt.Printf` statements appear in
`services/capture/manager.go` (lines 94–180) for capture lifecycle
diagnostics. Do not expand this pattern into new code — if you add a
diagnostic path, prefer the existing `audit` package
(`services/audit/audit.go`) for security-relevant events, or at minimum
keep `fmt.Printf` usage scoped and minimal. This file will be updated when
a structured-logging framework is adopted.

---

## 3. Redaction contract

Terminal output, user input, and any content sent to an LLM **must** pass
through the credential-redaction pipeline before leaving the process — whether
it's going to a model, a log file, or the audit trail.

The filter pipeline lives in `services/llm/filter/` and is constructed in
`services/llm_service.go` (line 189: `pipeline := filter.NewPipeline(...)`).
It includes:

1. `filter.NewANSIFilter()` — strips ANSI escape sequences from terminal
   output.
2. `filter.NewCredentialFilter()` — detects and redacts credentials
   (GitHub PATs `ghp_...`, AWS keys, generic `key=value` patterns, etc.).
3. An optional custom-pattern pipeline from `AppConfig.CustomPatterns`
   (loaded via `customFilterPipeline()` in `services/llm_service.go`,
   line 283).

**Rule:** Any new code path that sends terminal/user content to a model or
persistent log **must** route through `filter.Pipeline`. The existing
`CaptureManager.buildFilterPipeline()` (line 286 of
`services/capture/manager.go`) is the reference implementation for
non-LLM capture paths.

The audit logger (`services/audit/`) writes redacted content only — see
`services/llm_service.go` line 197 for the comment: "must not leave it in
plaintext in the audit log."

---

## 4. Security-lane rule

Changes in the following directories or files require **human review before
merge** — they touch secrets, credentials, or CI:

| Path                                      | Rationale                              |
|------------------------------------------|----------------------------------------|
| `services/keychain/`                      | Master-password storage, OS keychain backends, credential encryption |
| `services/llm/filter/`                    | Credential redaction logic — a bypass leaks secrets to LLMs/logs |
| `services/remote_*.go`                    | SSH/WinRM connections — auth, key handling, host-key pinning |
| `services/remote_service.go`              | Saved remote-host credentials, keychain keys |
| `.github/workflows/`                      | CI/CD configuration — supply-chain surface |
| `.gitleaks.toml`                         | Secret-scanning allowlist — widening it can hide real leaks |
| `services/config/config.go`              | Default values and persisted config schema |

If a PR touches any of these, explicitly request review from Hephaestus
(@hephaestus on Matrix, reviewer on GitHub). Do not self-merge.

---

## 5. Testing rule — anti-gaming

Every behavior change must ship with a test that **fails if the behavior is
removed**. This is enforced via mutation-style test design: each security or
correctness test includes an inline comment explaining what production-code
deletion would turn it red.

**Reference pattern** — `services/keychain/security_properties_test.go`
pins three security properties (K2/K3/K4) with comments like:

```
// Mutation check: deleting the `os.IsNotExist(err) { return false, nil }`
// branch in verifyMasterPasswordHash makes this test fail (the call returns
// a read error instead of a clean false).
```

**Your tests must follow this pattern.** For a new function `doX`:

```go
// Mutation check: removing the `if input == "" { return ErrEmpty }`
// guard in doX makes this test pass without error — the test asserts
// on that specific guard being present.
func TestDoX_RejectsEmptyInput(t *testing.T) {
    _, err := doX("")
    if err == nil {
        t.Fatal("expected error for empty input")
    }
}
```

For TypeScript/React tests, the equivalent is asserting on the **store
state** or **rendered output** that the behavior would not produce if the
code path were removed. Use `@testing-library/react`'s `render` +
`screen.getByText` / `screen.getByRole` and assert on the result — not on
implementation details like "a function was called".

**No test? No merge.** CI runs `go test ./services/... -count=1` and
`npx vitest run` — if a behavior change has no test, it will be caught in
review.

---

## 6. Secret hygiene

- **Never commit `.env`** — there is no `.env` in the committed tree. If you
  need to reference environment variable names, use them directly in code
  (`os.Getenv("OPENAI_API_KEY")`) or document them in a `.env.example`
  pattern. **Do not** check `.env` into git under any circumstances.
- **Never paste real API keys into test fixtures.** The project uses a
  fixed, inert placeholder for GitHub PAT redaction tests:
  `ghp_1234567890abcdefghij1234567890abcdef12` (in
  `services/llm/filter/filter_test.go`, lines 93 and 98). This literal is
  allowlisted in `.gitleaks.toml` — **the identical placeholder, not a real
  token**. Any new secret-looking string in tests must use a similarly
  fake value and be allowlisted if gitleaks flags it.
- **Never print secrets in logs.** The credential filter pipeline
  (`services/llm/filter/credential.go`) redacts `ghp_...` patterns from
  content sent to LLMs. If you add a new diagnostic `fmt.Printf` or
  `t.Logf`, sanitize token-like strings first — or better, use the
  `audit` package which redacts by default.
- **`.gitleaks.toml`** runs in CI as a hard gate on every PR (scanning only
  the PR's working tree) and on every push to master (scanning full history).
  It uses the default rule set — do not add broad ignores to bypass it.

---

## 7. Commit attribution

Agent-authored commits are attributed to the `o3willard-AI` identity. The
repository's git remote is `https://github.com/o3willard-AI/PairAdmin.git`.
Commits on master are authored as `Stephen Blankenship
<stephen.blankenship@gmail.com>`; agent commits should use the same
repository identity (the `o3willard-AI` GitHub org).

**Local git config for committing:**
```bash
git config user.name "Mike"
git config user.email "mike@nousresearch.com"
```

Use **conventional commits** (the repo's existing style):
- `feat(scope): ...` — new feature
- `fix(scope): ...` — bug fix
- `docs(scope): ...` — documentation only
- `chore(...)` — maintenance, deps, tooling

**Before committing:**
1. `git pull --rebase origin master` (stay current — see below)
2. Run `go test ./services/... -count=1` and `cd frontend && npx vitest run`
3. `git status` — no stray files, no `.env`, no `package-lock.json` changes
   unless you intentionally updated deps
4. `git push origin <branch>` and open a PR

**Pre-push checklist:**
- [ ] Branch based on latest master (not an old commit)
- [ ] All tests pass locally
- [ ] `npx tsc --noEmit` is clean
- [ ] No secrets committed (`.gitleaks.toml` is the gate)
- [ ] PR description explains what changed and why
- [ ] Security-lane files (section 4) have a human reviewer tagged

---

## 8. Verify, don't assume

This rule is carried over from `PLATFORM_PARITY_NOTES.md` (section "On
testing rigor generally"): a green `go test ./...` / `npx vitest run` run
only proves the code does what the **tests assume**, not that it works
against a real server, a real remote host, or a real WebView engine.

Before trusting any change — especially anything touching:

- **PTY/terminal behavior** — test against a real terminal (open 2+ tabs,
  `exit` one, confirm the app stays running; copy/paste with native
  shortcuts; SIGINT with text selected).
- **Remote SSH/WinRM** — `services/remote_ssh_test.go` uses an in-process SSH
  server, not a live host. Connect to a real Linux box by hand before
  trusting it.
- **Keychain backends** — macOS uses real Keychain.app (KeychainBackend with
  `KeychainTrustApplication: true`), Windows uses Credential Manager
  (WinCredBackend), Linux uses Secret Service / gnome-keyring / kwallet
  (SecretServiceBackend). The `FileBackend` (encrypted on-disk file, requires
  a master password via `FilePasswordFunc`) is the **fallback** used only when
  no OS backend opens and passes `probeBackend` — see
  `services/keychain/keychain.go` lines 160–170 for the allowed backend list
  and lines 168–178 for the FileBackend fallback. Verify on target platform.
- **Hotkeys** — the default `Ctrl+Shift+A` (clipboard command) was not
  chosen with macOS conventions in mind. Verify on each platform that the
  default combo doesn't collide with OS-level shortcuts.
- **Theme toggles** — switching Light/Dark should recolor every panel
  (sidebar, terminal chrome, dialogs, chat, status bar). Toggle back and
  confirm dark mode is pixel-identical to before.
- **tmux integration** — verify tmux fills the entire terminal width
  immediately on connect, with no layout resize needed.

**When in doubt:** measure the real behavior, don't reason from the code.
