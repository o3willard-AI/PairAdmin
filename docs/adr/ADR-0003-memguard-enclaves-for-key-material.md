# ADR-0003: memguard enclaves for in-memory key material

- **Status:** Accepted
- **Date:** 2026-09-05
- **Deciders:** PairAdmin maintainers
- **Technical area:** Secrets handling / memory hygiene

## Context

PairAdmin holds live secrets in memory for the lifetime of a session:

- **LLM API keys** (OpenAI / Anthropic / OpenRouter / Ollama), stored in the
  OS keychain (or the encrypted file backend) and needed on every LLM request.
- **The file-backend master password**, once verified, needed by
  `keychain.ring()`'s `FilePasswordFunc` to decrypt keyring items.

A plain Go `string` holding a secret is a poor container:

- Strings are **immutable and shared** — the runtime can duplicate them
  during slicing/concatenation, and nothing can reliably scrub every copy.
- The garbage collector **copies and moves** backing arrays without notice,
  leaving stale plaintext in freed heap pages.
- **Core dumps and swap** can capture the plaintext indefinitely (unless
  mlock'd or swapped to an encrypted pagefile — neither guaranteed).

`github.com/awnumar/memguard` exists to fix exactly this: buffers allocated
outside the Go heap via `mmap`, `mlock`ed so they cannot be swapped to disk,
zeroed on `Destroy()`, and process-wide **purged on SIGINT/SIGTERM** via
`memguard.CatchInterrupt()`.

## Decision

**All live key material is held in memguard Enclaves, never in long-lived
Go strings.** Concretely, as implemented:

1. **Sealing at load time.** The master password and the keychain loader
   wrap secrets with `memguard.NewBufferFromBytes(...)` and immediately
   `.Seal()` them into an `*memguard.Enclave` (encrypted, read-protected
   memory). `SettingsService.SaveAPIKey` / `LoadAPIKeys` do this and hand the
   Enclave to `LLMService.SetAPIKeyEnclave(provider, enc)`, which keeps a
   `map[provider]*memguard.Enclave` (`services/llm_service.go`,
   `apiKeyEnclaves`).
2. **Open → use → Destroy immediately.** `LLMService.getAPIKeyString` opens
   an Enclave only for the duration of `buildProvider`, extracts the key
   string for the provider constructor, and calls `buf.Destroy()` — the
   plaintext exists for microseconds per provider rebuild, not for the
   process lifetime. The Enclave itself is kept sealed and can be reopened.
3. **Process shutdown purge.** `main.go` calls `memguard.CatchInterrupt()`
   **before any Enclave is created** (so SIGINT/SIGTERM handlers are armed
   early) and `memguard.Purge()` in Wails' `OnBeforeClose` — every buffer's
   contents are wiped when the app exits by either path.
4. **No plaintext at rest.** Long-term storage is the OS keychain / encrypted
   file backend (see the keychain architecture), never a config file.

The one intentional plaintext window: `keychain.go`'s `masterPW` field and
`ring()`'s `FilePasswordFunc` closure hold the master password as a plain
string, because `keyring.Config.FilePasswordFunc`'s signature demands a
`string` return — the 99designs/keyring API boundary makes an enclave
impossible there without forking the library. This is a documented,
single-field exception, not a pattern.

## Consequences

### Positive
- **Keys are wiped deterministically** on destroy and on process exit
  (SIGINT/SIGTERM via `CatchInterrupt`, normal quit via `Purge` in
  `OnBeforeClose`) — the heap no longer carries recoverable key copies after
  the app closes.
- **mlock'd memory:** enclaved key material cannot leak to swap or core
  dumps while the process lives.
- **Sealed-by-default:** the Enclave map never holds plaintext; opening is
  an explicit, short-lived, localized step (`getAPIKeyString`).
- **Simple call-site contract:** services pass Enclaves, not strings; the
  only string conversion happens inside `getAPIKeyString` immediately before
  provider construction.

### Negative / accepted trade-offs
- **The API boundary forces one plaintext string.** `llm.New*Provider(key,
  ...)` constructors take a `string` (matching every HTTP auth API), so the
  key is momentarily in a Go string inside the provider. The enclave shortens
  but cannot eliminate this window without changing the `Provider`
  constructors to take enclaves — judged not worth the churn for the
  microseconds involved.
- **CGO / platform support:** memguard uses `mmap`/`mlock` syscalls; it is
  pure Go (no cgo) and works on all three target platforms, but it does add a
  dependency whose platform quirks (e.g. mlock limits) PairAdmin inherits.
- **`masterPW` string exception** (see Decision §4) — a single field survives
  as plaintext for the session lifetime, by API necessity.
- **Crash windows:** a hard crash (panic, SIGKILL) cannot run the purge;
  memguard's mlock still prevents swap capture, but a core dump would contain
  the enclaved (encrypted) bytes rather than wiped memory. Accepted: no app
  can defend against SIGKILL entirely.

## Alternatives considered

1. **Plain Go `string` / `[]byte` fields for keys.**
   *Rejected.* No way to zero them reliably (immutability, GC copying, no
   mlock); a heap dump or swap capture yields usable secrets indefinitely.

2. **Re-derive keys from the keychain on every request (no in-memory hold).**
   *Rejected.* Keychain/file-backend reads on every streamed LLM request add
   per-request cost and failure modes (locked collection, master-password
   prompts) to the hot path, and the 99designs file backend still needs the
   master password each time — the plaintext-hold problem just moves.

3. **OS keychain as the only memory representation (query at request time).**
   *Same as 2 at a smaller scale.* Also macOS Keychain prompts on every access
   without `KeychainTrustApplication`; per-request UI interruption is worse
   than a sealed enclave.

4. **Fork/patch 99designs/keyring to accept an enclave-backed
   `FilePasswordFunc`.**
   *Rejected.* Fork maintenance outweighs closing the single documented
   `masterPW` string exception.

5. **Encrypt keys under a session key held in an enclave (double encryption).**
   *Rejected as redundant.* The Enclave is already encrypted in-memory; the
   keychain is already encrypted at rest; a third layer adds key-management
   complexity with no threat model it defends that isn't already covered.

## References

- `services/llm_service.go` — `apiKeyEnclaves`, `SetAPIKeyEnclave`,
  `getAPIKeyString` (open → use → destroy).
- `services/settings_service.go` — `SaveAPIKey` / `LoadAPIKeys` (seal at
  load), `apiKeysProviders`.
- `main.go` — `memguard.CatchInterrupt()` before enclave creation,
  `memguard.Purge()` in `OnBeforeClose`.
- `services/keychain/keychain.go` — the documented `masterPW` string
  exception at the `FilePasswordFunc` API boundary.
- `github.com/awnumar/memguard` — enclave semantics, `CatchInterrupt`,
  `Purge`, `mlock`/`mmap` internals.
