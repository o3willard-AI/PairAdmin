# Registered Accountable Entity (RAE)

> **Registered Accountable Entity (RAE):** The single identified natural person to whom an action taken by an AI agent or agentic process is attributable, and who is answerable for that action by virtue of advance sponsorship or approval covering it — attribution that holds regardless of the degree or speed of automation employed. How firmly the attribution is established varies by assurance level (see below); that it is established, and to exactly one person, is what the term asserts.

**Why it matters:** An action with no RAE is an action no one owns. The practice exists to make that state impossible — or at minimum loudly visible — wherever RAE is enforced.

## Normative clauses

- **N1 — Pre-attribution.** Registration precedes the action. "In advance" is load-bearing: naming someone after the fact is *post-hoc audit attribution*, not RAE. That is the line between accountability and blame-allocation.
- **N2 — Agents are never RAEs.** An agent is the *subject* of attribution, never the *object*. Forecloses the "the AI decided" deferral — the failure mode RAE exists to eliminate.
- **N3 — The no-RAE invariant.** In a system practicing RAE, every agent action has an RAE at execution time. Actions without one are *blocked* (enforcement tier) or *flagged unattributed* (informational tier) — never silent.
- **N4 — Influenced actions.** A human who acts on an agent's output is trivially the RAE of their own action; the agent's contribution is recorded as *provenance metadata* ("agent-influenced"), not as separate RAE attribution. Two RAEs is no RAE.
- **N5 — Natural-person resolution.** "Entity" remains the term of art, but every RAE resolves to a single natural person. An organization participates only through designated humans; responsibility never terminates at the organizational boundary.

## Assurance levels

| Level | Name | Meaning |
|---|---|---|
| L0 | Declared | Claimed identity only (e.g., a bare OS username). Below the registration bar — a seed of the practice, not the practice. |
| L1 | Registered | Identity verified by the deploying organization + a signing credential bound to authorization records. |
| L2 | Registered (verified) | Identity verified by a third party (KYC/eID-grade) + a tamper-evident registry. |

## Glossary

- **Attribution (the property)** — The durable, independently reconstructable linkage of a specific action to a specific person. The core asserts attribution as a *property*; the assurance ladder carries the *mechanism*.
- **Accountable / Answerable** — Obligated to explain, justify, and bear the consequences of the action. Non-delegable to software: automation transfers *execution*, never *answerability*. Operational accountability only — the term neither confers nor disclaims legal liability, which is jurisdiction-specific.
- **In advance (pre-attribution)** — Sponsorship or approval recorded before the action executes (N1).
- **Entity vs. natural person** — "Entity" kept in the name as registry/legal idiom; normatively pinned to "natural person," the precise legal term that cleanly excludes organizations.
- **Agent** — A software system that selects and executes actions toward a goal with some autonomy (tool calls, code execution, message sending, system changes).
- **Agentic process** — Any workflow in which one or more agents act, or whose outputs *materially shape* a subsequent action. "Materially shapes" = the agent output changed the action's target, nature, or consequence, such that a reasonable person would treat the action as co-determined.
- **Sponsored / Approved** — *Sponsorship*: standing advance authorization of an agent or action class. *Approval*: specific authorization of a particular action. Either establishes the RAE link; the record must show which one covered a given action.
- **Registered (the institution)** — Recorded in a registry that binds identity to authorization records so attribution outlives the runtime. Graded by the assurance levels above.
- **Proof (the mechanism)** — Evidence sufficient for an independent party to reconstruct person → authorization → action. L0 = unsigned claim; L1 = cryptographic signature over the action/approval record by an org-verified credential + timestamped log; L2 = as L1, over a tamper-evident registry. The core definition never requires a specific level — only that *some declared level is met and stated*.

## Display convention

Any UI showing an RAE must state its assurance level. PairAdmin's free tier shows:

```
Registered Accountable Entity (RAE): <username> - declared, unverified (L0)
```

The free/enterprise gap becomes a visible assurance difference, not a hidden feature gate.

## PairAdmin implementation

The free tier implements RAE at **L0 (declared)**: it resolves the logged-in OS username and displays it in the Settings Security tab, explicitly labeled as declared and unverified. There is no enforcement in the free tier — the display is informational, making the RAE visible without claiming a verification it does not perform. Enterprise tiers implement L1/L2 (verified identity + signed attribution).

## Design lineage

Attribution regimes that inform the concept: git signed commits (author + credential + signature) and RACI (exactly one accountable party). Three-leg separation is maintained throughout: **attribution (property) / proof (mechanism) / registration (institution)** — conflating these is what lets "we have audit logs" products overclaim accountability.
