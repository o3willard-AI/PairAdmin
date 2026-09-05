# ADR-0002: Trust-on-first-use SSH host-key pinning

- **Status:** Accepted
- **Date:** 2026-09-05
- **Deciders:** PairAdmin maintainers
- **Technical area:** Remote terminals / SSH / host-key trust

## Context

PairAdmin's "+ New" dialog opens interactive SSH sessions to user-specified
hosts (`services/remote_ssh.go`, `openSSHTerminal`). Every SSH client must
decide what to do with the server's host key. The classic alternatives are:

- **OpenSSH's `known_hosts` file** — a separate trust store with its own
  format and tooling, outside the app's control.
- **InsecureSkipVerify** — accept anything; no MITM protection at all.
- **Certificate-based trust** — requires a CA the user doesn't have.

PairAdmin is a per-user desktop app, not a shared server: it should own its
trust store in its own per-user data directory, and it cannot assume the user
has pre-populated anything.

The implementation (`services/remote_ssh.go`, `verifyHostKeyCallback` and
`PTYService.CheckHostKeyTrust` / `probeHostKey`) pins keys in
`~/.pairadmin/known_hosts.yaml` (`config.LoadKnownHosts` /
`config.SaveKnownHosts`), keyed by `host:port`
(`hostPortKey`, `net.JoinHostPort`), storing the key type and SHA-256
fingerprint (`ssh.FingerprintSHA256`).

## Decision

**Trust on first use (TOFU), with a later mismatched key ALWAYS rejected —
and an optional prompt for first-time keys.** The callback implements exactly
three rules (verbatim from `verifyHostKeyCallback`'s doc comment):

1. **A key matching what's already pinned** for this `host:port` is accepted
   silently — the common case on every connection after the first.
2. **A key that DIFFERS from what's pinned is always rejected** with
   `*HostKeyMismatchError` (carrying old and new fingerprints), regardless of
   any prompt setting or `trustNewKey` retry flag. This is the actual
   MITM defense, and it is unconditional — no UI flow can bypass it.
3. **An unrecognized `host:port` (first-ever connection)** is pinned
   immediately — *unless* `promptForNewKeys` is true and `trustNewKey` is
   false, in which case the connection is rejected with
   `*UnknownHostKeyError` so the caller can show an accept/reject prompt.
   The prompt flow works via `PTYService.CheckHostKeyTrust`, which uses
   `probeHostKey` to connect *just far enough* to observe the host key
   (aborting with an `errHostKeyCaptured` sentinel before any authentication,
   so probing can never trigger an auth-failure lockout on the remote), then
   shows the real fingerprint to the user; accepting retries the real
   connection with `trustNewKey` set, which pins the key.

`PromptNewHostKeys` is a user setting (`config.AppConfig.PromptNewHostKeys`,
Settings → Terminals). The default (`false`) is pin-silently; when enabled,
first-time keys require explicit acceptance. Either way, rule 2 above is
unchanged — accepting a *changed* key requires the user to consciously delete
the pinned entry from `known_hosts.yaml` first (the mismatch error message
says so).

## Consequences

### Positive
- **MITM detection:** any host-key change between connections is rejected
  with both fingerprints visible, which is the property that matters.
- **Zero-configuration first connect:** the default flow works against a new
  host with no pre-shared trust store.
- **The app owns its trust store:** a plain YAML file in the app's own config
  directory, inspectable and deletable by the user without knowing OpenSSH's
  formats.
- **Optional stricter onboarding:** `PromptNewHostKeys` upgrades
  first-connection trust from silent to explicit, without changing the
  mismatch rule.

### Negative / accepted trade-offs
- **First connection is unauthenticated** — the TOFU gap. A MITM present
  during the very first connection gets pinned and trusted from then on.
  This is inherent to TOFU; the mitigation is that the *later* mismatch
  detection still catches any subsequent attack, and `PromptNewHostKeys`
  makes even the first connection explicit for users who want it.
- **Per-application store:** a host also trusted in the user's OpenSSH
  `~/.ssh/known_hosts` is *not* automatically trusted here — its key is
  pinned again on first PairAdmin connection. Two stores can disagree.
- **Recovery from a legitimate key change is manual:** a real server rekey
  (or a rebuilt VM) surfaces as `*HostKeyMismatchError`; the user must delete
  the stale entry from `known_hosts.yaml` by hand. Deliberate: the app never
  silently re-pins over a mismatch.
- **Trust store is not protected against local tampering:** a local attacker
  who can write `~/.pairadmin/known_hosts.yaml` can replace a pinned
  fingerprint. This matches OpenSSH's own threat model for `known_hosts`
  (file permissions on the config dir are 0700).

## Alternatives considered

1. **`ssh.InsecureIgnoreHostKey()` (accept everything).**
   *Rejected.* No MITM detection at all; the mismatch rule is the whole
   security value of host-key checking.

2. **Use OpenSSH's `~/.ssh/known_hosts` directly.**
   *Rejected for now.* Its format (hashed hostnames, multiple key types per
   host, `@cert-authority` markers) is far richer than PairAdmin needs, and
   parsing/writing it correctly is a real compatibility risk (a bug could
   corrupt the user's system trust store, which affects ssh(1) itself, not
   just PairAdmin). A per-app YAML store is isolated and trivially correct.

3. **SSH certificates (`@cert-authority` / CA-signed host keys).**
   *Rejected for now.* Requires a CA infrastructure the target users don't
   run. Revisit if enterprise deployments ask for it.

4. **Pin on first use with NO prompt option (always silent pin).**
   *Rejected.* The `PromptNewHostKeys` setting exists because the silent
   default trains users to ignore first-contact risk. Keeping both modes
   costs nothing and serves both audiences (the backend's
   `maybeConnectToRemote` flow shows a materially scarier prompt when the key
   has *changed* — in that case there is no Accept button at all, matching
   the fail-closed mismatch rule).

5. **Require manual pre-provisioning of `known_hosts.yaml` (no TOFU at all).**
   *Rejected.* Breaks the "type a host, get a terminal" product promise for
   the common case, and the file format would still be app-owned anyway.

## References

- `services/remote_ssh.go` — `verifyHostKeyCallback`, `probeHostKey`,
  `HostKeyMismatchError`, `UnknownHostKeyError`, `hostPortKey`.
- `services/pty_service.go` — `CheckHostKeyTrust`, `HostKeyStatus` (the
  frontend-facing probe used by the accept/reject prompt).
- `services/config/config.go` — `KnownHostKey`, `LoadKnownHosts`,
  `SaveKnownHosts` (pinned-key store).
- `config.AppConfig.PromptNewHostKeys` — the stricter-onboarding setting.
- OpenSSH `StrictHostKeyChecking=yes` and `accept-new` semantics — the same
  TOFU family of trade-offs, for comparison.
