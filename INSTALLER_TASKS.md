# Installer Tasks — Windows & macOS

**Audience:** whoever picks up native installer work for each platform — referred to
below as the **Windows builder** and the **macOS builder**. Items marked
**Shared** block both lanes and should be resolved first, by whoever gets to
them (they don't require either platform to build).

**Why this file exists:** Linux is the only platform with anything resembling
a real installer today (`.deb`, one-line `install.sh`), and even that has
gaps (see Current State). Windows and macOS only have `wails build`'s raw,
unsigned, unpackaged binary. This is the task breakdown to get both to the
same "download it, install it, forget it, upgrade it, uninstall it — with no
trust warnings and no manual cleanup" bar the `.deb` mostly already hits.

**Ground rules carried over from this project's other planning docs:** every
item below was checked against the actual repo state, not assumed. Re-verify
before starting — this file will go stale exactly like `PRE_INSTALLER_TASKS.md`
did. When a platform's checklist is fully done and verified on real hardware
(not just "the build didn't error"), delete that platform's section rather
than leaving stale checked boxes.

---

## Current state (verified 2026-08-30)

- **Linux `.deb`**: real. `SHA256SUMS` in repo root has exactly one entry —
  `pairadmin_1.0.0_linux_amd64.deb` — meaning a `.deb` has actually been
  built and checksummed at least once. It's also stale (v1.0.0; the project
  is past v2.0.0 now) and shouldn't live committed at repo root long-term —
  see Shared item below.
- **Linux `.rpm`**: claimed in README/RELEASE_NOTES, **never verified** —
  no `.rpm` entry has ever appeared in `SHA256SUMS`. Treat as unbuilt until
  someone actually produces and tests one on Fedora.
- **Linux AppImage**: claimed in older docs, **does not exist**. `install.sh`
  literally has no AppImage code path — it errors if `dpkg`/`rpm` aren't
  found. Fixed in README/install.sh this session (2026-08-30); still todo if
  AppImage support is ever wanted for real.
- **No CI/CD at all.** `.github/workflows/` doesn't exist. Every release
  asset that has ever existed was built and uploaded by hand.
- **Windows**: `build/windows/installer/project.nsi` and `wails_tools.nsh`
  are the **stock, unedited Wails NSIS template** — no PairAdmin branding,
  no code-signing hookup (the `signtool` lines are commented out), no EULA
  page (also commented out), no upgrade-detection logic beyond NSIS's
  default silent overwrite. `build/windows/info.json` still has unfilled
  `{{.Info.*}}` template placeholders because `wails.json` has no `info`
  block to fill them from — the compiled `.exe`'s file properties (Company,
  Product, Copyright) are currently blank or Wails defaults, not PairAdmin's.
- **macOS**: `build/darwin/Info.plist` is the **stock Wails template** too —
  `CFBundleIdentifier` is still the literal default `com.wails.{{.Name}}`,
  not a real PairAdmin bundle ID. **There is no `.icns` file anywhere in the
  repo** (`build/darwin/` has only the two `Info*.plist` files) — a macOS
  build today either fails at the icon-generation step or falls back to a
  generic icon; `PLATFORM_PARITY_NOTES.md` already flags this as an open
  action item for the macOS builder independent of this file.
- **No code signing or notarization anywhere**, any platform.
- **No update mechanism anywhere.** "Upgrade" on Linux means "run install.sh
  with `upgrade`, which uninstalls and reinstalls." Windows and macOS have
  no equivalent today, automated or manual.
- **Icon source asset**: `build/appicon.png`, 512×512, present. Fine as a
  master source. `build/windows/icon.ico`'s generated frames are oddly sized
  (`16x15`, `32x30` per `file`) rather than exact square dimensions — flagged
  below.

---

## Shared (do these first — they block both platforms)

- [ ] **Fill in `wails.json`'s `info` block** (`CompanyName`, `ProductName`,
      `Copyright`, `Comments`) — currently absent, so every downstream
      template (`build/windows/installer/project.nsi`,
      `build/windows/info.json`, `build/darwin/Info.plist`) is either blank
      or falling back to a Wails default. This one change fixes both
      platforms' file-properties/bundle metadata at once.
- [ ] **Pick a real bundle/app identifier** (e.g. reverse-DNS style, matching
      whatever domain/org the project settles on) — used for macOS's
      `CFBundleIdentifier` and worth reusing as the Windows installer's
      registry uninstall-key name too, instead of the NSIS default
      `${INFO_COMPANYNAME}${INFO_PRODUCTNAME}` string.
- [ ] **Decide the update-notification strategy for v2**: nothing automatic
      (user re-downloads / re-runs `install.sh`), an in-app "a new version is
      available" check against the GitHub Releases API (read-only, no
      auto-download), or a real auto-updater (Squirrel/Sparkle-equivalent —
      meaningfully more work, especially for signing implications). Recommend
      shipping the read-only "new version available" check first — it's
      cheap, and Ollama/LM Studio-style privacy users will not want a silent
      background updater phoning home by default.
- [ ] **Stand up a CI/CD release pipeline** (GitHub Actions, since the repo
      is on GitHub) that, on a version tag push: builds Linux `.deb`
      (+ `.rpm` once that's real), Windows installer, macOS `.dmg`; generates
      `SHA256SUMS` fresh per release; uploads all of it as release assets.
      Doing this by hand is exactly how `SHA256SUMS` ended up stuck at
      v1.0.0 with only one platform in it — a human remembering to
      regenerate and re-upload a checksum file for three platforms on every
      release will not stay reliable.
- [ ] **Stop committing `SHA256SUMS` to the repo root long-term.** It should
      be a per-release artifact attached to the GitHub Release, not a file
      that lives on `master` and silently goes stale between releases (it
      already has). Remove the stale v1.0.0 one once the pipeline above
      produces real per-release checksums instead.
- [ ] **Add GPG signing of release artifacts** — already listed as a
      "What's Next" item in both `RELEASE_NOTES.md` v1.0.0 and v2.0.0
      entries, never done. Decide who holds the signing key and how it's
      kept out of the CI pipeline's own logs/secrets exposure.
- [ ] **Decide the license/EULA text**, if any, to show during install on
      Windows and macOS (README already says MIT — a full install-time EULA
      page may be overkill for an MIT project; explicitly decide "no EULA
      page" rather than leaving the NSIS template's EULA page commented out
      by accident).
- [ ] **Decide the telemetry/crash-reporting posture explicitly** and state
      it in the README/installer: given the whole pitch is "your terminal
      output doesn't quietly leave the box," a Windows/macOS installer that
      later ships even opt-in crash reporting needs that decision made
      deliberately and disclosed, not defaulted to whatever a packaging tool
      does out of the box.
- [ ] **Regenerate `build/windows/icon.ico` and produce a real `build/darwin/
      icons.icns`** from `build/appicon.png` with exact square frames at each
      required size (16/32/48/256 for `.ico`; the full Apple `.icns` size set
      for macOS) rather than whatever produced the current off-by-one-pixel
      `.ico` frames.
- [ ] **Settle a minimum-OS support matrix** and put it in the README
      alongside the existing platform install sections (e.g. Windows 10
      1809+/11, macOS 12+) — both installer templates currently carry
      placeholder/default minimums (`LSMinimumSystemVersion` is hardcoded to
      `10.13.0` in `Info.plist`, likely older than anything actually tested).

## Windows builder

- [ ] **Confirm WebView2 Runtime handling.** The NSIS template already
      includes `wails.webview2runtime` — verify it actually installs/updates
      the runtime silently on a clean Windows 10 VM (not just Windows 11,
      which ships it preinstalled) and doesn't require a reboot mid-install.
- [ ] **Wire up code signing** in `project.nsi` (the `signtool` lines are
      present but commented out) — needs an Authenticode certificate (EV
      strongly preferred: standard OV certs still trigger SmartScreen
      "unrecognized publisher" warnings for a long reputation-building
      period; EV gets instant reputation but costs more and requires
      hardware-token/HSM key storage). Decide the certificate budget/vendor
      before this can move forward — flagging since it's a real recurring
      cost, not a one-time task.
- [ ] **Add an uninstall entry that behaves like a normal Windows app**:
      confirm it shows correctly in Settings → Apps with icon, publisher,
      version, and install size populated (driven by the `info.json`/
      `wails.json` fields above) — not just present-but-blank.
- [ ] **Verify uninstall leaves no residue**: registry uninstall key,
      `$INSTDIR`, Start Menu shortcut, Desktop shortcut (all already
      scripted in `project.nsi`'s uninstall section) — plus anything
      PairAdmin itself writes at runtime that the installer doesn't know
      about: `~/.pairadmin/` (config, known_hosts, audit logs), the
      encrypted-file keychain fallback store, and the WebView2 DataPath
      (`$AppData\${PRODUCT_EXECUTABLE}`, already RMDir'd in the template —
      confirm the path actually matches where WebView2 really writes it).
      Explicitly decide whether uninstall should offer to keep or wipe
      `~/.pairadmin/` (saved hosts, pinned commands, custom filters) — most
      installers default to "keep user data," which is probably right here
      too, but it should be a decision, not an accident.
- [ ] **Test the upgrade path specifically**, not just fresh install: install
      vN, then install vN+1 over it. NSIS's default behavior just overwrites
      files in `$INSTDIR` — confirm a running PairAdmin instance doesn't
      block the installer with a file-lock error, and that config/known_hosts
      survive the upgrade untouched.
- [ ] **ARM64 Windows**: decide in/out of scope for v2. The NSIS template
      already supports building an ARM64 or dual-arch installer
      (`ARG_WAILS_ARM64_BINARY`) — this is a scope decision, not a technical
      blocker, since Wails itself cross-compiles for `windows/arm64`.
- [ ] **Silent/unattended install flag** (`/S` is NSIS's standard silent
      switch) — verify it actually works end-to-end for anyone wanting to
      push PairAdmin via Intune/SCCM in a corporate fleet; document it if so.
- [ ] **Add the installer to the README's Windows install section**, which
      today only documents `wails build` from source — once a real signed
      `.exe` installer exists, Windows needs the same "download and run"
      framing Linux already has, not just a build-from-source path.

## macOS builder

- [ ] **Produce the missing `build/darwin/icons.icns`** (see Shared list) —
      this currently blocks a normal-looking build entirely, not just a
      cosmetic gap.
- [ ] **Set a real `CFBundleIdentifier`** in `Info.plist` (currently the
      literal Wails default) and confirm the bundle plist's remaining
      template fields (name, version, copyright) actually resolve once
      `wails.json`'s `info` block is filled in (Shared item).
- [ ] **Decide distribution channel**: direct-download `.dmg`/`.pkg` (almost
      certainly correct here — PairAdmin needs system-level access patterns
      like AT-SPI2-equivalent terminal reading and OS keychain integration
      that the Mac App Store's sandbox would fight) vs. Mac App Store.
      Recommend direct `.dmg` distribution, matching the Linux `.deb`/`.rpm`
      direct-download model already in place.
- [ ] **Apple Developer Program enrollment** ($99/yr) is required for both
      Developer ID code signing and notarization — without notarization,
      Gatekeeper blocks the app outright on first launch with no easy
      override for typical users (not just a warning dialog like Windows
      SmartScreen). This is a hard prerequisite for a distributable macOS
      build, not optional polish.
- [ ] **Code-sign with a Developer ID Application certificate, then notarize
      and staple the ticket** to both the `.app` bundle and the `.dmg`
      itself. Verify with `spctl -a -vvv` and `stapler validate` on a clean
      machine before calling this done — "it built" and "Gatekeeper actually
      accepts it" are different bars.
- [ ] **Wire up the real macOS OS Keychain backend.** Already flagged in
      `PLATFORM_PARITY_NOTES.md`: `services/keychain/keychain.go`'s
      `AllowedBackends` never includes `keyring.KeychainBackend`, so macOS
      today silently uses the same on-disk encrypted `FileBackend` as
      Windows/Linux — not real Keychain.app integration, despite the README
      claiming "credentials in the OS keychain." This is a correctness bug
      that specifically undercuts a security claim on macOS, worth fixing
      before or alongside packaging, not after.
- [ ] **First-run permission prompts**: identify what macOS actually prompts
      for at runtime (Keychain access confirmation the first time a saved
      credential is read/written, and — depending on how any
      AT-SPI2-equivalent external-terminal-reading feature is implemented on
      macOS — possibly an Accessibility permission grant in System Settings)
      and make sure the app doesn't silently fail or hang if the user denies
      one; a clear in-app message beats a mysterious no-op.
- [ ] **Universal binary vs separate Intel/Apple Silicon builds**: decide
      whether to ship a single `lipo`-merged universal `.app` (larger
      download, simplest UX) or two architecture-specific `.dmg`s (smaller,
      more release-pipeline complexity). Given the "download it and it just
      works" bar this file opened with, universal is the safer default
      unless a strong size/build-time reason says otherwise.
- [ ] **Verify uninstall is the normal macOS "drag to Trash" experience**:
      confirm nothing gets left in `~/Library/LaunchAgents`,
      `~/Library/Application Support/`, or a background helper process that
      would make a drag-to-Trash uninstall incomplete — unlike Windows/Linux,
      macOS has no installer-driven uninstall step by convention, so
      "leaves no residue" has to be true by construction, not by an
      uninstall script cleaning up after the fact. Decide (same as Windows)
      whether `~/.pairadmin/` should survive a Trash-based uninstall by
      default.
- [ ] **Test the upgrade path**: install vN, replace with vN+1's `.app` (the
      normal macOS "drag a new version over the old one in /Applications"
      flow), confirm `~/.pairadmin/` config/known_hosts/keychain state
      survives untouched and a previously-running instance doesn't corrupt
      anything mid-replace.
- [ ] **Add the installer to the README's macOS install section**, which
      today just says "Native builds are coming soon" — once a signed,
      notarized `.dmg` exists, mirror the Linux section's "download and
      install" framing.

---

## Explicitly out of scope for this pass (revisit later, don't block on them)

- Linux AppImage support (README/install.sh now correctly say it doesn't
  exist yet, rather than claiming a fallback that errors).
- Verifying the Linux `.rpm` path end-to-end on a real Fedora box — separate
  from Windows/macOS work, but equally unverified; worth its own pass.
- A full auto-updater (vs. the read-only "update available" check
  recommended above) — bigger scope, revisit once basic signed installers
  exist for all three platforms.
