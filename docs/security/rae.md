# Registered Accountable Entity (RAE)

The canonical definition of RAE lives in a standalone, versioned specification:

- **Specification:** <https://github.com/o3willard-AI/RAE>

## PairAdmin implementation

PairAdmin's free tier **displays an RAE at L0 (declared, unverified)**: the Settings
Security tab resolves the logged-in OS account and displays it with the
explicit label `declared, unverified (L0)`. No identity verification or
registration is performed in the free build — the display makes the accountable
human visible without claiming a verification it does not perform.

The core definition, normative clauses (N1–N8), assurance levels, and
conformance criteria live in the spec, not here. This file is a pointer; if the
two ever diverge, the spec is authoritative.

## Claim boundary (in-repo)

The free build's only conformant claim is **displays an RAE at L0 (declared,
unverified)**. Prohibited: "RAE-compliant," "implements RAE," "accountable by
design," or any RAE term without its level attached. The spec's Conformance
section (§4) is authoritative; this restates the PairAdmin-specific application
of it.
