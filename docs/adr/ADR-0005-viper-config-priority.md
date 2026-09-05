# ADR-0005: Viper-first config priority (persisted settings > env vars for user settings, env > nothing for LLM backend)

- **Status:** Accepted
- **Date:** 2026-09-05
- **Deciders:** PairAdmin maintainers
- **Technical area:** Configuration management

## Context

PairAdmin has two distinct kinds of configuration, which the D-04 note was
the first to separate cleanly:

1. **User app settings** — provider, model, theme, font size, hotkeys,
   context-lines count, custom filter patterns, saved remote hosts, pinned
   commands, sidebar widths, `PromptNewHostKeys`. The user sets these in the
   Settings UI (or via slash commands) and expects them to persist across
   launches. These live in `~/.pairadmin/config.yaml`, managed by Viper
   (`services/config/config.go`: `AppConfig` with `mapstructure`/`yaml`
   tags, `LoadAppConfig` / `SaveAppConfig`).
2. **LLM backend environment configuration** — `PAIRADMIN_PROVIDER`,
   `PAIRADMIN_MODEL`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`,
   `OPENROUTER_API_KEY`, `OLLAMA_HOST`, `OLLAMA_API_KEY`,
   `LMSTUDIO_HOST`. These come from the process environment
   (`services/llm_service.go` `LoadConfig()`), which is how headless/CI/CLI
   users configure the LLM backend without touching a UI.

Before D-04, the two collided: only `LoadConfig()` (env) existed, so
provider/model changes made in the Settings UI were **silently ignored** —
the next `LoadConfig` call re-read the stale environment and reverted the
user's choice. The Settings UI was writing `config.yaml` that nothing read.

## Decision

**Priority: persisted Viper config (`config.yaml`) takes precedence over
environment variables for every field that exists in both worlds; env vars
remain the fallback (and the only source for the secrets env vars carry).**

Implemented concretely in `services/settings_service.go`'s
`LoadConfigWithViper()` (called by `main.go` at startup and by
`LLMService.RebuildProvider()` after every save):

```go
provider := envCfg.Provider
if appCfg != nil && appCfg.Provider != "" {
    provider = appCfg.Provider   // Viper wins when non-empty
}
model := envCfg.Model
if appCfg != nil && appCfg.Model != "" {
    model = appCfg.Model
}
// ...same pattern for OllamaHost / LMStudioHost
```

- **Viper-first, not Viper-only:** an *empty* persisted field falls through
  to the environment variable (the checks test for non-empty values, not
  presence) — so a fresh install with no `config.yaml` is still fully
  configured by env vars alone, and API keys always come from
  `envCfg.*`/the keychain, which is where `SaveAPIKey` puts them (never in
  `config.yaml`).
- **Re-resolution on every settings save:** `SettingsService.SaveSettings`
  calls `LLMService.RebuildProvider()` after writing `config.yaml`;
  `RebuildProvider` re-reads via `LoadConfigWithViper` and rebuilds the
  active provider — settings changes take effect immediately without an app
  restart.
- **Keychain is out of this hierarchy entirely.** API keys never live in
  `config.yaml` (see ADR-0003); they resolve through
  `buildProvider`'s `keyFn` (keychain Enclave) with the env var as fallback,
  which is a deliberate exception to "Viper > env" because secrets in a
  YAML file would defeat the keychain.
- **`SaveAppConfig` preserves foreign fields.** `SettingsService.SaveSettings`
  re-loads the existing config and re-attaches `RemoteHosts` /
  `PinnedCommands` before writing, because `SaveAppConfig` writes the whole
  struct — each Settings tab sends only the fields it manages, and without
  this guard a tab save would wipe every other persisted setting.
- **Env-var reads stay scoped.** Per the repo convention (AGENTS.md), user
  app settings are never read via `os.Getenv` in `services/`; the only env
  reads are `LoadConfig()`'s LLM backend vars and a few OS-adjacent probes
  (`SHELL`, `WAYLAND_DISPLAY`, `LOCALAPPDATA`/`XDG_DATA_HOME`).

## Consequences

### Positive
- **The Settings UI actually works.** Provider/model/host changes made in
  the UI stick across restarts and take effect immediately (via
  `RebuildProvider` on save) — the original bug (env silently overriding the
  UI) is structurally impossible now.
- **Headless / automation configuration still works.** A user who has never
  opened Settings configures the LLM purely via `PAIRADMIN_*` /
  provider-key env vars; every persisted field is empty, so every env value
  falls through.
- **Deterministic, single resolution point.** `LoadConfigWithViper` is the
  only place the two sources meet; the priority is readable in one function
  rather than smeared across call sites.
- **Secrets stay out of YAML.** The keychain/Enclave path (ADR-0003) is
  orthogonal to this hierarchy, and `SaveAppConfig`'s foreign-field
  preservation means adding new persisted fields can't wipe saved hosts or
  pinned commands.

### Negative / accepted trade-offs
- **"Unset" is represented as empty string, not absence.** The Viper-wins
  rule is `appCfg.X != ""`, so a user cannot express "persist that I want
  the env var to win" — they must clear the field in the UI (which saves an
  empty value, correctly falling through). Acceptable: the UI is the primary
  surface, and env-var-driven users are by definition not using it.
- **Field-by-field priority code is repetitive.** Each new persisted LLM
  field needs the same four-line block in `LoadConfigWithViper`. A
  reflection-based merge was considered and rejected (see Alternatives) —
  explicit beats clever for a function this small and security-adjacent.
- **No env override for a *saved* value.** Unlike 12-factor apps where
  `ENV_VAR=other` overrides a config file, here the file wins once a value
  is saved. Deliberate: the UI is the source of truth for user intent, and a
  stray environment variable silently overriding an explicit UI choice is
  the exact original bug.
- **API keys are the odd ones out** (env/keychain only, no `config.yaml`
  representation) — a user reading `config.yaml` won't find where keys come
  from. Documented here and in ADR-0003.

## Alternatives considered

1. **Env vars always win (env > Viper).**
   *Rejected — this was the original bug.* The Settings UI became
   write-only: `LoadConfig()` re-read the environment and reverted every
   user choice. The product promise is "the UI configures the app."

2. **Viper-only: stop reading LLM env vars entirely.**
   *Rejected.* Breaks headless/CI/automation configuration and the documented
   `PAIRADMIN_*` contract; the env fallback is what makes a fresh install
   work without opening Settings.

3. **Reflection / mapstructure merge of env + Viper into one struct.**
   *Rejected.* The merge rule (non-empty Viper wins) is five explicit blocks
   in `LoadConfigWithViper` today; a reflection-based merge would hide the
   priority, be easy to get subtly wrong for bool/int fields (is `false`
   "unset"?), and the function is small enough that explicit code is
   auditable at a glance.

4. **Single unified config struct persisted entirely in YAML (env vars
   dropped).**
   *Rejected.* Collapses the user-settings vs backend-env distinction that
   D-04 drew, puts API-key-adjacent fields on disk (even if only host/model,
   the precedent is bad), and breaks env-var-only workflows.

5. **Runtime precedence flags / per-request config resolution.**
   *Rejected.* `RebuildProvider` already re-resolves at the only moments the
   config can change (startup + settings save); per-request resolution adds
   lock contention and re-read cost in the streaming hot path for zero
   benefit.

## References

- `services/settings_service.go` — `LoadConfigWithViper()` (the priority
  implementation), `SaveSettings` (foreign-field guard + `RebuildProvider`
  call), `LoadConfig` lives in `services/llm_service.go`.
- `services/llm_service.go` — `Config` struct (the two worlds' fields side
  by side), `LoadConfig()` (env source), `RebuildProvider()`
  (re-resolution on save), `buildProvider` (keyFn/keychain resolution).
- `main.go` — `NewLLMService(services.LoadConfigWithViper())` (startup
  priority in action).
- `services/config/config.go` — `AppConfig`, `LoadAppConfig`,
  `SaveAppConfig` (the Viper-persisted layer).
- The D-04 note — original priority decision this ADR records.
