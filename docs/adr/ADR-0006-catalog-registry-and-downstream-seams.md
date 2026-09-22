# ADR-0006: Catalog-driven provider registry, compiled-in protocol adapters, and the downstream-override seam

- **Status:** Accepted
- **Date:** 2026-09-22
- **Deciders:** PairAdmin maintainers
- **Technical area:** LLM provider architecture / extensibility boundaries

## Context

Until v2.6.0, provider selection was a hardcoded `switch` in
`buildProvider()` (`services/llm_service.go`) over five provider strings, the
model was a free-text field in the Settings UI, and adding a provider meant
writing a new Go adapter and touching the switch. Three pressures made that
shape untenable:

1. **Breadth.** Users want providers beyond the five — DeepSeek, Groq,
   Mistral, GLM, Gemini, and the long tail of OpenAI-compatible gateways
   (LM Studio, vLLM, llama.cpp servers, internal proxies). Nearly all of the
   long tail speaks one of a handful of wire protocols.
2. **Model discovery.** A free-text model field cannot show what a provider
   offers, what each model costs, its context window, or whether it supports
   reasoning/tool-calls/attachments. Typo'd model ids fail only at request
   time.
3. **Downstream builds.** A future licensed build will need to *restrict*
   which providers and models an installation may use, under control its
   users cannot edit. Whatever shape the open-source provider layer takes, it
   must leave a clean place for such a downstream to hook in without the open
   repo carrying any of that machinery — the same split the codebase already
   uses elsewhere (see Decision 3).

We reviewed the methodology of opencode (anomalyco/opencode, TypeScript) as
prior art — catalog-driven providers from the community models.dev feed,
protocol adapters separated from providers, and declarative auth methods. No
code was reused; PairAdmin is Go/Wails and the design below is adapted to it
(and deliberately diverges where opencode's choices don't fit a desktop
security posture).

## Decision

Three decisions, shipped across PRs #50–#63 (scanner) and #60–#63 (LLM):

### 1. Catalog-driven registry replaces the hardcoded switch

Provider and model metadata lives in a **static, compiled-in catalog**
(`services/llm/catalog/`): a curated top-11 provider list with models,
capabilities (reasoning, tool-call, attachments), context limits, and costs,
embedded in the binary. The registry (`services/llm/registry.go`) resolves
`provider/model` ids against it; the Settings UI is a searchable picker fed
by the registry, not a text field. Credentials stay where they are: OS
keychain behind memguard enclaves (ADR-0003), env-var detection per provider.

The catalog is *data, refreshed at release time* — not a runtime fetch. A
synced external catalog feed (models.dev-style, with TTL cache and snapshot
fallback) is designed for a later release; when it lands it replaces the
embedded catalog's staleness, not its shape.

### 2. Protocol adapters are compiled in; no runtime code download

Wire protocols (openai-chat, anthropic-messages, gemini, ollama; openai-chat
also covers OpenRouter, DeepSeek, Groq, Mistral, GLM, LM Studio, and custom
OpenAI-compatible endpoints via `base_url`) are ordinary Go packages
implementing the existing `llm.Provider` interface. opencode downloads
unbundled provider adapters as npm packages at first use; PairAdmin does the
opposite and always will: a desktop app that fetches executable code at
runtime is a supply-chain exposure that contradicts this project's threat
model, and the protocol count that covers ~220 catalog providers is small
enough to compile in.

Custom OpenAI-compatible providers are configuration (id, name, base_url,
keychain-referenced key, optional headers/models), never code.

### 3. Downstream restriction hooks are seams, not logic

Where a future licensed build needs to override behavior, the open repo ships
**a single, documented choke point whose default honors the user's own
config, and nothing else**. The override itself lives in the downstream, not
here. Shipped example: `services/scanner/policy.go` — `scanAllowed` is a
package-level func var defaulting to the user's `ScannerEnabled` setting; the
comment states that a downstream may replace the var and that force-disable
logic must not be added to the default. The same pattern will apply to any
provider/model restriction points in the registry: the open build evaluates
the user's own (self-imposed, freely editable) preferences; a downstream may
install a higher-priority evaluator through the seam.

Corollary (documentation rule): any mechanisms belonging to a downstream
licensed build are not described, hinted at, or stubbed in this repo's code,
docs, issues, or PRs — this ADR states only that the seam exists and what the
default does. The public-facing line for questions about organizational
control is MESSAGING.md §8.

## Consequences

**Positive:**
- Provider breadth is a catalog-data change, not a code change; the picker
  gives users model capabilities/costs/context before they choose.
- The `llm.Provider` interface and the filter pipeline (`services/llm/filter/`)
  are untouched — adapters slot in underneath, so redaction-before-transmission
  (MESSAGING §2.5) applies to every current and future provider uniformly.
- The seam pattern keeps the open-core boundary structural: nothing to
  remove or fork when a downstream exists, nothing downstream to leak here.
- Compiled-in adapters keep the supply-chain surface at build time where CI
  gates it (govulncheck, gitleaks, reproducible builds).

**Negative / costs:**
- The static catalog goes stale between releases (new models, price changes);
  accepted until the synced-catalog release lands.
- Curated top-11 means some providers users want aren't in the picker yet;
  custom OpenAI-compatible entries cover most of that gap.
- Seam vars are package-level mutable state — powerful for downstreams, but
  the open repo's tests must pin them (`t.Cleanup`) and reviewers must treat
  new seams as API surface, not convenience globals.

## Alternatives considered

- **Keep the switch, add providers as needed.** Rejected: every new provider
  is a code change plus UI churn, and the free-text model field's failure
  modes (typos, no capability info) scale with provider count.
- **Runtime-fetched catalog (models.dev-style) in v1.** Deferred, not
  rejected: it adds a network dependency, a trust surface (community feeds
  inject endpoint URLs), and cache-invalidation work that the static catalog
  avoids for now. The schema is catalog-compatible so the sync layer can be
  added without reshaping the registry.
- **Runtime-downloaded protocol adapters (opencode's npm model).** Rejected
  outright: executing downloaded code at runtime is incompatible with the
  product's security posture (Decision 2).
- **Carrying a user-visible "policy" concept in the open build.** Rejected:
  an editable-by-the-user restriction feature is self-restriction (which the
  config already supports via provider/model choice) and creates a false
  impression of enforceability. The open build stays honest: the user's
  preferences are theirs to change; downstream enforcement, if it ever ships,
  arrives through the seam and is documented there, not here.

## References

- PRs #60 (static catalog), #61 (Gemini adapter), #62 (registry), #63 (picker)
- `services/scanner/policy.go` (seam exemplar, PR #51)
- ADR-0003 (memguard enclaves — credential handling unchanged), ADR-0005
  (config priority — catalog/registry resolution sits above it, user config
  still wins over env)
- docs/MESSAGING.md §2.8 (explicit model selection), §8 (the paid tier)
- docs/security/threat-model.md (the "no execution interface" guarantee that
  Decision 2 protects)
