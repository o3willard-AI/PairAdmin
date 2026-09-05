# ADR-0001: Disable the WebKit content-process sandbox on Linux

- **Status:** Accepted
- **Date:** 2026-09-05
- **Deciders:** PairAdmin maintainers
- **Technical area:** Linux packaging / rendering / Wails (WebKitGTK)

## Context

PairAdmin is a Wails v2 desktop app. On Linux the Wails window is rendered by
**WebKitGTK** (`webkit2gtk`); the Go binary embeds the React frontend via
`//go:embed all:frontend/dist` and spawns a single first-party WebView that
loads only bundled, trusted UI assets.

WebKitGTK ≥ 2.40 enables a **content-process sandbox** out of the box. That
sandbox is implemented with **bubblewrap**, an unprivileged sandbox that
creates a new Linux **user namespace** (`CLONE_NEWUSER`) and layers seccomp and
bind-mounts on top of it. If the web-content subprocess cannot be launched in
its sandbox, WebKit falls back to *not rendering at all* — the app shows a
**blank / empty window** and is unusable.

The sandbox fails on any configuration where the web process is not permitted
to create the namespaces/restrictions the sandbox requires. Those are common on
PairAdmin's primary Linux targets:

- Debian ships `kernel.unprivileged_userns_clone = 0` by default (user
  namespaces disabled for unprivileged users).
- Ubuntu 23.10+ gates user-namespace creation behind an AppArmor profile.
- Container / Docker / LXC hosts deny `CLONE_NEWUSER` via seccomp filters on
  the parent process.
- Locked-down desktop / corporate policies disable user namespaces or block
  `bwrap` (AppArmor profiles, seccomp).
- Parallel GPU failures (DMA-BUF renderer against VirtIO / Intel / AMD without
  full kernel DRM support) compound the symptom into a blank window even when
  the namespace leg would otherwise succeed.

WebKitGTK honors an explicit escape hatch to disable its sandbox via the
environment variable `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1`. In
WebKitGTK < 2.40 this variable was named `WEBKIT_FORCE_SANDBOX=0`; the new
name's alarming suffix is intentional upstream to discourage its uncondensed
use.

### OS matrix — which platforms are affected by this setting

| Platform | Renderer used by Wails v2 | Does `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS` apply? | Impact |
|----------|---------------------------|--------------------------------------------------------|--------|
| Linux (VirtIO GPU, Intel, AMD, no-dGPU, headless, containers) | WebKitGTK (`webkit2gtk`) | **Yes** — this is the only platform that reads the variable | **Affected**: sandbox blocked by seccomp/namespace → blank window without the disable |
| Linux (native desktop with unprivileged user namespaces enabled: fedora-shipped bwrap path, some distros) | WebKitGTK | **Yes** — variable is read and honored | Affected *by the setting*: sandbox would usually work, but we disable it anyway (see Decision) |
| macOS | WKWebView (Apple WebKit framework) | **No** — macOS does not use WebKitGTK or bubblewrap; the variable is a no-op | **Not affected** |
| Windows | WebView2 (Chromium/Edge) | **No** — Windows does not use WebKitGTK or bubblewrap; the variable is a no-op | **Not affected** |

The variable is therefore **Linux-only in effect**. On macOS and Windows the
current code path sets an environment variable that is silently ignored —
there is no sandbox disablement on those platforms, and they were never at
risk of the blank-window failure.

## Decision

**Keep the UNCONDITIONAL disable.** `main.go` continues to set
`WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1` before `wails.Run()`, for every
platform, without a runtime probe.

We do **not** implement a conditional / probed disable because **no reliable
pre-flight check exists**. The failure mode has multiple independent causes
(non-privileged user-namespace availability, AppArmor profiles, seccomp on the
process hierarchy, DMA-BUF/GPU rendering), and none of them can be detected
cheaply and correctly before the WebKit process launches. Any single signal we
could test (e.g. "is `unshare --user` allowed?") would:

- **False-positive** on hosts where a working namespace check still leads to a
  blank window through another leg (GPU/DMA-BUF), reproducing the bug we are
  trying to prevent; and
- **False-negative** on hosts where user namespaces are restricted in a way the
  probe does not model (AppArmor-gated userns, bwrap blocked by a policy),
  silently degrading security on systems where the sandbox would have worked.

An unreliable probe that guesses wrong is a worse outcome than an explicit,
documented, unconditional disable — it randomly re-introduces either a blank
window (app unusable) or a false sense of security. Upstream Wails / WebKitGTK
ecosystem apps make the same choice for the same reason.

The disable is set unconditionally (including on macOS/Windows) for simplicity
and cross-platform consistency; it is harmless there because the variable is a
no-op on those renderers.

## Consequences

### Positive
- **Usability is guaranteed** on PairAdmin's primary Linux targets (VMs with
  VirtIO/VirtIO GPU, cloud/no-GPU instances, headless, Docker). The app never
  shows a blank window due to sandbox-launch failure.
- Single, simple, deterministic code path — no probe, no branching, no
  platform-specific sandbox detection to maintain or get wrong.
- Other rendering env vars already set for the same environment
  (`LIBGL_ALWAYS_SOFTWARE`, `WEBKIT_DISABLE_COMPOSITING_MODE`,
  `WEBKIT_DISABLE_DMABUF_RENDERER`) remain consistent with a fully software,
  sandbox-free rendering stack.

### Negative / security trade-off
- On Linux — and only on Linux — the WebKit **content process is
  unsandboxed**. A compromise of the WebKit rendering / JS engine would not be
  confined by the sandbox's namespace/seccomp boundary.
- **Mitigations that bound this risk:**
  - The WebView loads **only bundled, first-party, trusted UI** (`go:embed`
    assets). It does not load arbitrary remote web content, so the primary
    threat a content sandbox defends against (malicious third-party web pages)
    is not present in the default app.
  - No remote content, no network-loaded JS, no third-party webviews are
    shipped. The app's attack surface for *web content* is therefore low.
  - The `_THIS_IS_DANGEROUS_` variable name is honored exactly as upstream
    intends: it is a deliberate, explicit opt-out by the application
    developer after weighing usability against sandboxing benefit, not an
    accidental configuration.
  - The choice is documented (this ADR) and the `main.go` comment points back
    to it, so a future maintainer can revisit if the app ever begins loading
    remote/untrusted content.

### Residual risk
- **Documented and accepted.** If PairAdmin ever starts rendering
  untrusted/remote web content on Linux, the unconditional sandbox disable must
  be revisited (ideally made conditional per-content, or removed once the
  sandbox works reliably in our shipping environments). This ADR and the
  `main.go` pointer are the tripwire for that future change.
- macOS and Windows are unaffected (variable is a no-op); their native
  helper sandboxes are entirely independent of this setting.

## Alternatives considered

1. **Conditional probe — "test `unshare(CLONE_NEWUSER)` and disable only if it fails."**
    *Rejected.* The probe does not model the real failure. Success creating a user
    namespace does not guarantee WebKit GTK will render (GPU/DMA-BUF and other
    legs can still blank the window), and restriction can be imposed by AppArmor/seccomp in ways a bare `unshare` test misses. It either re-introduces the bug
    or silently drops the sandbox — both bad. See Decision.

2. **Conditional probe — check a distribution/kernel knob
   (`kernel.unprivileged_userns_clone`, AppArmor userns profiles).**
    *Rejected.* These knobs are not present or not authoritative across all
    target distros and kernels; Debian sets the first to 0, older kernels lack
    it, and Ubuntu 23.10+ gate via AppArmor rather than the sysctl. It would be
    wrong on many real machines.

3. **Try/catch at runtime: launch, detect blank window, restart with the var set.**
    *Rejected.* Detection of a "blank window" is unreliable and platform-specific,
    and restarting the process mid-launch is poor UX and racy. Not a real,
    deterministic probe.

4. **Remove the disable entirely and require the user / distro to enable user namespaces.**
    *Rejected.* Rejects the primary target environment (VMs, containers,
    no-GPU) and makes the app unusable by default for the audience it serves.
    The usability requirement outweighs the sandbox here.

5. **Make the disable conditional via a build/launch flag or packaging toggle.**
    *Deferred as a future nicety, not pursued now.* The current unconditional
    behavior is correct and simple; a toggle adds configuration surface with no
    demonstrated need. If native desktop users later report a desire for the
    sandbox, this is the natural evolution (default remains disabled-and-safe,
    opt-in re-enables the sandbox).

## References

- WebKitGTK sandboxing / bubblewrap: WebKit relies on bubblewrap + user
  namespaces for the web-content process.
- Debian `kernel.unprivileged_userns_clone = 0` default; Ubuntu 23.10+ AppArmor
  gating of user namespaces.
- Blocking `CLONE_NEWUSER` via seccomp in containers.
- Wails v2 Linux rendering uses `webkit2gtk` (`main.go` comment, lines 30-38).
- WebKitGTK ≥ 2.40 variable rename: `WEBKIT_FORCE_SANDBOX` →
  `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS`.