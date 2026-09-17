# Security Policy

PairAdmin takes the security of its users seriously. This document explains
how to report a vulnerability, what to expect after you report, and which
versions are supported.

## Reporting a vulnerability

Please report security vulnerabilities privately so the maintainers can
investigate and fix them before they are disclosed. Do not describe an
exploitable flaw in a public issue.

- **On GitHub:** use the **Security** tab of this repository →
  **Report a vulnerability** (GitHub private vulnerability reporting). This
  opens a private advisory visible only to the maintainers and you.
- **Outline the impact:** include the affected version, a minimal
  reproduction, and — where possible — whether the issue requires local
  access or a remote party.

The maintainers will acknowledge your report and work with you to confirm and
remediate it. You will be credited in the fix and any associated advisory
unless you ask otherwise.

## Response expectations

| State                     | Expected timeline |
|---------------------------|-------------------|
| Acknowledgment            | Within 3 business days of the report |
| Triage / confirmation     | Within 8 business days |
| Fix for a confirmed issue | As soon as a fix is ready; high-severity issues are prioritized |

If a fix touches any of the security-relevant areas of the codebase
(`services/keychain/`, `services/llm/filter/`, `services/remote_*.go`,
`.github/workflows/`, `services/config/config.go`), production and release are
paused for human review before the fix ships.

## Supported versions

PairAdmin is a desktop application distributed as releases. **Only the latest
release is supported.** Security fixes are delivered in new releases; fixes are
not backported to older versions. If you run an older release, update to the
latest version to receive a fix.

Each release ships with checksum verification for the one-line installer and
install packages — verify a download before installing
(`install.sh --sha256`).

## Threat model

PairAdmin's security claims are stated precisely, with the code that makes
each one true, in the repository threat model:

- **Repository threat model:** [`docs/security/threat-model.md`](docs/security/threat-model.md)
- **Website threat model:** <https://pairadmin.tech/security>

Every claim in the threat model is verifiable against the source; the
execution boundary in particular can be checked with the published
verification command. If any claim in the threat model does not hold, that is
a high-severity report.