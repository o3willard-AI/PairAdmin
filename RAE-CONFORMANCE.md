# RAE Conformance Statement — PairAdmin

**Specification:** [Registered Accountable Entity (RAE)](https://github.com/o3willard-AI/RAE), v1.0.4
**Product:** PairAdmin (this repository, the open-source build)
**Claim:** *displays an RAE at L0 (declared, unverified)* — **not** *implements RAE*
**Date:** 2026-09-22

Per RAE §4, an L0 display is not an implementation and "publishes no
conformance statement." This file is therefore **not** a §4 conformance
declaration — the open-source PairAdmin makes no claim to implement the RAE
practice, whose attribution machinery (N1, N3, N4, N6a/N6b, N7, N8) begins at
RAE L1. It is published voluntarily, at the conventional location §4 names, to
state exactly what PairAdmin does and does not assert, so no reader has to
infer the boundary. It also brings PairAdmin in line with the rest of the
o3willard-AI portfolio, where Chaperone, Machina-Parousia, MR-Krabs, and
linus-deployment-specialist each carry the same voluntary L0 file.

Companion in-repo docs: [docs/security/rae.md](docs/security/rae.md) (the
implementation note and claim boundary) and SECURITY.md's Accountability
section. This file is the §4-convention statement; those remain the
product-facing docs. If they ever disagree, the spec is authoritative.

## The claim, precisely

The open-source PairAdmin displays an RAE at L0 (declared, unverified):

- **What exists:** the Settings → Security tab resolves the logged-in OS
  account (`SettingsService.GetCurrentUsername`, `services/settings_service.go`)
  and displays it as `Registered Accountable Entity (RAE): <username> -
  declared, unverified (L0)` — the level suffix is a hardcoded constant
  (`RAE_L0_SUFFIX`, `frontend/src/components/settings/SecurityTab.tsx`), never
  dynamic, so the label cannot drift or be omitted. Audit-log entries
  (`services/audit/audit.go`) carry the session identity, and the assistant's
  hard execution boundary (the model can never execute anything — see
  docs/security/threat-model.md) means every consequential action is a
  human-initiated one.
- **What is missing for L1:** the displayed identity is the OS account —
  claimed, not verified. There is no organization-verified identity, no
  signing credential bound to authorization records, and no pre-attribution
  records with declared scope. Per RAE §4 an L0 claim "asserts only the
  display criterion," and that is the entire claim.

## Clause status (informational — L0 declares no enforcement tier)

| Clause | Status in the open build | Note |
|---|---|---|
| N1 Pre-attribution | Not implemented | No sponsorship/approval records exist; the OS account is resolved at display time, not bound in advance of actions. This is expected at L0 — the machinery "begins at L1." |
| N2 Agents are never RAEs | Honored | The display names a human OS account, never the assistant. Reinforced structurally by the execution boundary: the model produces text only; every action is operator-initiated (threat-model.md). |
| N3 No-RAE invariant | Not applicable at L0 | PairAdmin does not claim a tier; the assistant can be disabled entirely (MESSAGING §2.7), and the RAE display degrades to "unknown" rather than blanking when resolution fails. |
| N4 Influenced actions | Not implemented | No provenance-metadata recording of agent influence on operator actions. |
| N5 Natural-person resolution | Honored | The display resolves to a single OS account (a natural person standing behind the login), never an organization. |
| N6a/N6b Sponsorship scope | Not applicable | No sponsorship concept at L0. |
| N7 Sponsorship lifecycle | Not applicable | No sponsorship concept at L0. |
| N8 Agent-to-agent delegation | Not applicable | PairAdmin hosts one assistant; it does not orchestrate agent-invokes-agent chains. |

## Proof location

- **Display:** `frontend/src/components/settings/SecurityTab.tsx`
  (`RAE_L0_SUFFIX` constant; resolution states resolving/unknown/displayed).
- **Identity resolution:** `services/settings_service.go`,
  `GetCurrentUsername()` — OS account via `os/user.Current()` with `USER`/
  `LOGNAME` fallback; documented as a declared, unverified identity.
- **Audit:** `services/audit/audit.go` — append-only JSON-lines, session-scoped
  entries with redaction statistics (content itself is never logged).
- **Claim discipline:** `docs/security/rae.md` and `docs/MESSAGING.md` §2.10/§3
  (approved: "displays an RAE at L0 (declared, unverified)"; prohibited:
  "RAE-compliant," "implements RAE," "accountable by design," or any RAE term
  without its level attached).

## What would change the claim

Raising a PairAdmin build to *implements RAE 1.0.x L1, \<tier\> tier*
requires: organization-verified operator identity; an operator signing
credential bound to authorization records; pre-attribution records with
declared scope (N1, N6a/N6b); a declared N3 tier with no-RAE blocking;
provenance references per N4; and a true §4 conformance statement. None of
that is planned for the open-source build; the open-core line (MESSAGING §8.1)
is that the free tier's guarantees are unconditional and complete on their
own, not a crippled preview.
