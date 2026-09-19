# PairAdmin — Messaging & Positioning Update

**Audience:** the GitHub orchestrator/maintainer agent, and the website SEO/marketing agent.
**Status:** authoritative. Supersedes all existing product messaging in the repo and on pairadmin.tech.
**Verified against:** `origin/master` @ `c17acc8` (2026-09-12) and pairadmin.tech v2.4.0.
**Last updated:** 2026-09-17.

---

## 0. How to use this document

| Section | Repo agent | Website agent |
| --- | --- | --- |
| 1 — The change | Read | Read |
| 2 — Approved claims | **Binding** | **Binding** |
| 3 — Prohibited claims | **Binding** | **Binding** |
| 4 — Vocabulary | **Binding** | **Binding** |
| 5 — Website tasks | Skip | Own |
| 6 — Repo tasks | Own | Skip |
| 7 — SEO & metadata | Skip | Own |
| 8 — Paid tier | Read | Read |
| 9 — Verification protocol | **Binding** | **Binding** |

Sections 2, 3, 4 and 9 are binding constraints, not suggestions. They exist because this is a security product: a single overstatement that a reader can disprove in one screenshot costs more credibility than all the correct claims earn. **When in doubt, claim less.**

---

## 1. The change in one paragraph

PairAdmin is currently marketed as a convenience tool — a nicer terminal with an AI sidebar that saves you from copy-pasting. That positioning puts it in a crowded category it does not win, and it omits the product's actual differentiator. **PairAdmin is a control boundary.** The assistant is architecturally incapable of executing anything, receives only what the operator explicitly sends, can be restricted to a model on the operator's own machine, and can be switched off entirely with the state shown on screen for the whole session. The new story leads with that. Convenience becomes the second reason to adopt, not the first.

**The core line:** the AI can advise, and only the human can act.

---

## 2. Approved claims

Every claim below is verified against `origin/master` @ `c17acc8`. Cite-by-symbol is given so any claim can be re-checked before reuse. Line numbers are accurate as of that commit — **grep for the symbol, do not trust the line number** after the tree moves.

### 2.1 The assistant cannot execute anything

> The AI has no path to the shell. Not a disabled one, not a permission it could be granted — there is no execution interface in the product to disable. Every character that has ever reached a PairAdmin shell came from a human keystroke, a human paste, or a human click on one of three labeled buttons.

**Evidence:** no tool-calling, function-calling, or agent-loop code exists anywhere in the tree. Verify with:
```
git grep -niE '"tools"|tool_choice|function_call|tool_use|ToolCall' origin/master -- services frontend/src
```
This must return **zero results**. If it ever returns a result, this claim is void and this document must be revised before any further publication.

The only writes into a PTY are `sendToTerminal()` (`frontend/src/utils/sendToTerminal.ts`), called exclusively from `onClick` handlers in `CodeBlock.tsx` and `CommandSidebar.tsx`, and direct user input in `TerminalPreview.tsx`.

### 2.2 The three buttons

Exact UI labels, in UI order — **use these strings verbatim, do not paraphrase:**

1. `Save to Commands`
2. `Copy to Terminal`
3. `Execute in Terminal`

> Every command the model recommends arrives as text with three buttons and no fourth option. Stage it, study it, or run it — each one a deliberate click on a command you have read.

**Evidence:** `frontend/src/components/chat/CodeBlock.tsx`.

Approved framing of the principle: **"There is no user in the loop. The user *is* the loop."**

### 2.3 The model is stateless; context is one tab, one question

> A request contains the system prompt, the last N lines of the terminal you are looking at, and what you typed. It does not contain your other sessions, earlier turns in this one, your filesystem, or your saved connections.

**Evidence:** `llm.BuildMessages()` in `services/llm/context.go` returns exactly `[]Message{system, user}`. No conversation history is transmitted for any tab. Called once, from `LLMService.SendMessage` in `services/llm_service.go`.

Because history is never transmitted for *any* tab, the following is also approved:

> Switch tabs and the assistant knows nothing about where you just were. Close a tab and there is nothing left to know.

**Note for agents:** local application state in the frontend store outlives a closed tab. This is irrelevant to the claim — no code path reads chat history into a provider request — but if asked, the honest answer is: *"No code path reads chat history into a request. `BuildMessages` takes the system prompt, the live buffer, and the question, and nothing else."* Do not claim the application purges frontend memory on tab close.

### 2.4 Nothing transmits until the operator sends

> Terminal content is read only when you press send. There is no ambient streaming, no background capture feeding a model.

**Evidence:** `readTerminalLines()` is called only inside `handleSend` in `frontend/src/components/chat/ChatPane.tsx`.

### 2.5 Redaction before transmission

Fifteen credential patterns, applied in-process **before** the provider call, plus user-supplied regexes via `/filter`:

`aws-access-key-id` · `github-token` · `gitlab-personal-access-token` · `openai-api-key` · `anthropic-api-key` · `slack-token` · `google-api-key` · `google-service-account` · `azure-account-key` · `bearer-token` · `jwt` · `pem-private-key` · `password-assignment` · `generic-api-key` · `connection-string-credentials`

**Evidence:** `services/llm/filter/credential.go`; pipeline applied in `LLMService.SendMessage` before `BuildMessages`.

Approved emphasis: the ordering is the claim. Scrubbing happens *before* transmission, not after receipt.

Also approved: API keys are held in encrypted, mlock'd memory (memguard) rather than plain variables. **Evidence:** `services/keychain/`, `SetAPIKeyEnclave` in `services/llm_service.go`, ADR-0003.

### 2.6 Local inference is available and is the default posture for local providers

> Point PairAdmin at Ollama or LM Studio and nothing crosses the network. Both default to loopback — Ollama to `http://localhost:11434`, LM Studio to `http://localhost:1234`.

**Evidence:** `defaultOllamaHost` in `services/llm/ollama.go`; LM Studio default in `buildProvider()` in `services/llm_service.go`.

**Required qualifier whenever remote inference is discussed:** remote Ollama hosts are permitted by design (team GPU box), authenticated with a bearer token from the OS keychain, and Settings displays a warning for as long as a non-loopback host is configured. See §3.3 — the unqualified version of this claim is prohibited.

### 2.7 The assistant can be switched off entirely

This is currently undocumented everywhere and is a priority addition.

> Set the provider to **Disable Pair LLM** and no model is constructed and no endpoint is contacted. The request is refused in the backend, not hidden in the interface. The status bar reads **Disabled** for the entire session — amber, not red: this is a chosen posture, not a fault.

**Evidence:**
- Backend refusal: `LLMService.SendMessage` returns before any provider is built when `cfg.Provider == "disabled"` (`services/llm_service.go`, ~line 167).
- Provider option and label `"Disable Pair LLM"`: `frontend/src/components/settings/LLMConfigTab.tsx`.
- Persistent indicator: `CONNECTION_LABEL.disabled = "Disabled"`, `CONNECTION_DOT.disabled = "bg-amber-500"` in `frontend/src/components/layout/StatusBar.tsx`.

Approved use case framing: opening a session against a system you will not reason about out loud — belt-and-suspenders on top of every other guarantee.

### 2.8 Explicit model selection

> The provider is always an explicit choice, never a default that drifts. Nothing auto-discovers an endpoint or silently falls back to a hosted model when a local one is unreachable.

Providers: OpenAI, Anthropic, Ollama, LM Studio, OpenRouter, and Disable Pair LLM. Switchable mid-session with `/model`.

**Phrase the internal-model story as capability, not restriction:** "can be entirely internal." See §3.6.

### 2.9 Audit log

> Every prompt and every response is written to a local rotating JSONL audit log.

**Evidence:** `services/audit/audit.go`; `Event: "user_message"` and `Event: "ai_response"` written in `services/llm_service.go`.

**Hard boundary — see §3.4.** The log records the operator's typed message and the model's response. It does **not** record the terminal context that was transmitted. Do not imply otherwise.

### 2.10 Registered Accountable Entity (RAE) display

> The Security tab displays the logged-in OS account as the Registered Accountable Entity, explicitly labeled at assurance level L0 (declared, unverified).

**Evidence:** `services/settings_service.go` (`GetCurrentUsername`); `frontend/src/components/settings/SecurityTab.tsx` (`RAE_L0_SUFFIX`).

**Hard boundary — see §3.11.** The free build displays an RAE at L0 only. It does not verify identity, does not register anyone, and is not "RAE-compliant." The spec lives at https://github.com/o3willard-AI/RAE; the claim is level-scoped.

---

## 3. Prohibited claims

Hard stops. Do not publish these in any form, in any channel, including social posts, release notes, issue replies, and meta descriptions.

### 3.1 "No chat window"

**Prohibited.** PairAdmin has a chat pane (`ChatPane.tsx`, `ChatInput.tsx`). Anyone who opens the app disproves this instantly, and a reader who catches one overstatement discounts every other claim on the page.

**Use instead:** the claim is about *destination*, not interface — the conversation never reaches a consumer chat service, never transits the clipboard, and never leaves the application.

### 3.2 "None of the risk" / any absolute

**Prohibited.** Absolutes are the first thing a security reviewer tests, and "no risk" is indefensible for any software.

**Use instead:** **"All of the capability. None of the handover."** This is the approved tagline variant. It keeps the original cadence and describes exactly what the architecture guarantees.

### 3.3 "Ollama is localhost-only" / "refuses remote hosts"

**Prohibited — this is factually false on current master.** Remote Ollama is supported deliberately.

**Use instead:** "defaults to loopback, and warns you in Settings whenever it isn't."

### 3.4 "The audit log tells you what we sent them"

**Prohibited as currently worded.** The log excludes `terminalContext`, which is the sensitive payload a security manager is actually asking about.

**Use instead:** "Every prompt and response is written to a local rotating JSONL audit log." Do not extend this to the terminal output. **Repo agent: the existing README sentence violates this — see §6.3.**

### 3.5 gitleaks / "managed secret scanning" / "enterprise-grade scanning"

**Prohibited.** Redaction is regex-based. gitleaks is not a dependency. The fifteen-pattern set is genuinely strong — describe it accurately and let it stand on its own.

### 3.6 Any claim that central/fleet policy enforcement exists today

**Prohibited.** Each operator sets their own provider and can change it mid-session with `/model`. There is no central lockdown in the open build, by design. See §8 for the approved way to answer this.

### 3.7 "Human in the loop"

**Prohibited** — it is the industry phrase for a human approving an agent's autonomous actions, which is the model PairAdmin exists to reject. The human is not a checkpoint in an automated pipeline; there is no pipeline.

**Use instead:** "There is no user in the loop. The user *is* the loop."

### 3.8 "Sandboxed AI" / "restricted AI" / "guardrails"

**Discouraged.** All three imply a capability being restrained, which misdescribes the architecture and invites "what happens when the guardrail fails?" The AI is not restrained from executing commands; it has no execution interface at all. Prefer language of *absence*, not *restriction*.

Exception: "per-terminal sandbox" is acceptable when describing context scoping, but §2.3's stateless framing is stronger and preferred.

### 3.9 "Agentic" as a description of PairAdmin

**Approved in one place only:** the headline "The terminal for the agentic age" — where "agentic age" names the *era* (the operator's environment), not the product. It stays because it is the strongest hook for the exact audience this product serves, and the control-boundary lines immediately correct the record.

**Still prohibited:** describing PairAdmin *itself* as "agentic" / "an agent" — the product has no agency (that is the point). "Agentic" may only modify "age", never the product.

### 3.10 Anything implying the free build is incomplete or insecure without the paid tier

**Prohibited.** See §8. Every guarantee in §2 is unconditional in the open-source build. No asterisk, no "upgrade for security."

### 3.11 "RAE-compliant" / "implements RAE" / "accountable by design" / any RAE term without its level

**Prohibited.** The free build displays an RAE at L0 (declared, unverified), which the RAE spec defines as "below the registration bar." It neither verifies identity nor registers anyone. Claiming "RAE-compliant," "implements RAE," or "accountable by design" — or using the term without its assurance level attached — overclaims the free build and invites the exact scrutiny the honest label exists to avoid.

**Use instead:** "displays an RAE at L0 (declared, unverified)."

---

## 4. Vocabulary

| Use | Avoid | Why |
| --- | --- | --- |
| advises / recommends | assists, helps you run | precision about capability |
| operator | user (in security contexts) | names the professional audience |
| the AI has no path to the shell | the AI is prevented from running commands | absence, not restraint |
| control boundary | guardrail, safety layer | not a mitigation of a risk being taken |
| posture | mode, setting | matches security vernacular |
| handover | exposure, leakage | names the specific thing avoided |
| transmitted / sent | shared | "shared" softens a security-relevant event |

**Tone:** flat and technical. Do not use exclamation marks, hype adjectives ("revolutionary", "game-changing"), or fear-based urgency about breaches. The alternatives section (§5.3) describes real tradeoffs neutrally — an ops manager recognizing their own workflow is more persuasive than alarm.

---

## 5. Website tasks

**Owner:** website SEO/marketing agent. Current site has **zero** security content across every section — hero, "Why PairAdmin," the four feature cards, download. This is the whole problem.

### 5.1 The hero

Keep the headline "The terminal for the agentic age" — approved per §3.9 (it names the era, the strongest hook). Pair it with the two control-boundary lines:

> # The terminal for the agentic age
>
> PairAdmin puts expert administration in front of a human operator without handing a model your credentials, your topology, or your shell. It reads the terminal you point it at. It recommends. You execute — or you don't.
>
> All of the capability. None of the handover.
>
> **Get PairAdmin ↓**  ·  **Read the threat model →**

### 5.2 Add the comparison table, high on the page

This does the positioning work four feature cards cannot, because it names the alternatives the buyer is actually choosing between. Rows: *Who executes · What the model receives · Where it goes · Usable context · Time to a correct answer · Record of what was sent · Blast radius of one bad suggestion · Can be switched off.* Columns: *Bare terminal · Terminal + copy/paste to a chatbot · Autonomous agent with credentials · PairAdmin.*

Full populated table: see the positioning review artifact. Keep the PairAdmin column visually distinct.

### 5.3 Add "the alternatives" section

> **Learn it the long way.** Books, forums, trial and error against production. Safe, and measured in years.
>
> **Paste it into a chatbot.** Fast, and every paste is a judgment call about what's in those lines — made in a hurry, against an endpoint whose retention and training defaults you don't set. The context is only as wide as what you selected.
>
> **Hand an agent the keys.** Genuinely effective, and it means giving a model your credentials, your internal endpoints, and a shell — then watching it act faster than you can read. Every review gate your organization wrote for human administrators becomes a suggestion.
>
> PairAdmin is the fourth option: the speed of the third, with the exposure profile of the first.

### 5.4 Add the three-buttons section

Per §2.2. Show the three labels as UI chips. Include the "no fourth option" line and the architectural point from §2.1 — that there is no execution interface to disable.

### 5.5 Add "what the model is allowed to know"

Per §2.3, §2.5, §2.6. Lead with "One tab. One question. Nothing else."

### 5.6 Add the off-switch section

Per §2.7. Include a visual of the amber `Disabled` status-bar state.

### 5.7 **Build a `/security` page — highest-priority new asset**

A written threat model. This is simultaneously the best trust asset and the best SEO asset, because it is the page that gets linked, cited, and pasted into vendor-review threads. Structure:

1. What PairAdmin sends, exactly — field by field
2. What it never sends
3. What runs locally
4. The execution boundary, with the `git grep` verification from §2.1 printed so a reader can check the claim themselves
5. Redaction pattern list
6. Audit log contents (accurate per §3.4)
7. Key storage (keychain, memguard, ADR-0003)
8. SSH host-key pinning (ADR-0002)
9. Known limitations — state them plainly; this page earns credibility by what it admits
10. How to run fully air-gapped

Publishing verifiable instructions for disproving your own claim is the strongest possible signal. Include it.

**Repo-first (decided 2026-09-17):** the threat model is authoritative in the repo at `docs/security/threat-model.md`; the `/security` page renders the same content (SEO-optimized) and links back to it. Draft once, use twice.

### 5.8 Update download-section framing

Keep "No account. No trial." — it supports the security story (nothing phones home to obtain the software). Add: Apache-2.0, checksum verification available via `install.sh --sha256`.

---

## 6. Repo tasks

**Owner:** GitHub orchestrator/maintainer agent. Work from `origin/master`; open a PR, do not push to master.

### 6.1 README tagline

Keep `**The terminal for the agentic age.**` (README.md line 7) — approved per §3.9. Add "All of the capability. None of the handover." as the supporting tagline immediately beneath it.

### 6.2 Reorder the Overview section

Current order argues convenience for four sections before reaching any security material, and never mentions the execution boundary or the off switch. Target order:

1. Tagline (new, per §6.1)
2. **New:** *The assistant cannot run anything* — §2.1 + §2.2, four sentences
3. **New:** *Or switch it off entirely* — §2.7
4. **Moved up:** *Your secrets stay yours* — currently 5th. Lead with the before-transmission ordering, then enumerate the fifteen patterns
5. **Moved up, retitled:** *One tab, one question* — §2.3. Currently folded into "Bring your own model" and missing the stateless fact entirely
6. A terminal first
7. Quick Commands
8. A genuine SSH client
9. tmux, minus the papercuts

### 6.3 Correct the audit-log sentence

In *Your secrets stay yours*, this text violates §3.4:

> …so you can answer "what did we send them?" with a file instead of a shrug.

Cut that clause. Replace with a plain statement of what the log contains.

### 6.4 Add the threat model + `SECURITY.md`

(a) Create `docs/security/threat-model.md` — the authoritative 10-section threat model (the §5.7 content, repo-first: the repo is the source of truth, the `/security` page renders it). (b) `SECURITY.md` = standard vulnerability-disclosure policy plus links to both `docs/security/threat-model.md` and the `/security` page. GitHub surfaces SECURITY.md in the Security tab.

### 6.5 Repository metadata

- **Description:** currently absent. Set it to a one-line version of the new positioning.
- **Topics:** `terminal`, `ssh`, `llm`, `local-llm`, `ollama`, `security`, `devops`, `sysadmin`, `privacy`, `terminal-emulator`, `wails`, `air-gapped`.
- Enable the Discussions tab if it isn't — vendor-review questions belong somewhere public and findable.

### 6.6 Optional cleanup (not messaging, but adjacent)

- `services/llm/filter/credential.go` carries a comment describing a gitleaks detector that is not a dependency. Remove it before a reviewer reads it as a claim (§3.5).
- Audit logging currently omits `terminalContext`. Logging line count, byte count, and which redaction patterns fired with how many matches would make §3.4's restriction unnecessary and would be the best available evidence that the scrubber works. Track as an issue; do not block messaging on it.

---

## 7. SEO & metadata

**Owner:** website agent.

### 7.1 Title and description

```
Title:       PairAdmin — The terminal for the agentic age
Description: An AI that reads your terminal and recommends — with no path to
             execute anything itself. Runs fully local. Open source, Apache-2.0.
```

Keep under 60 / 155 characters respectively. The title matches the hero headline (approved §5.1); the differentiator ("with no path to execute anything itself") lives in the description, not the title.

### 7.2 Intent clusters

Rank by commercial intent, not volume. The highest-intent visitors arrive already worried.

**Cluster A — risk-aware (highest intent, lowest competition).** These searchers have the problem and no solution in mind.
- is it safe to paste terminal output into ChatGPT
- AI agent ran destructive command production
- LLM agent with production server access risk
- sharing logs with AI data leak
- security risk of AI coding agents on infrastructure

**Cluster B — solution-seeking (local/private).**
- local LLM terminal assistant
- offline AI terminal assistant
- self-hosted AI sysadmin tool
- Ollama terminal integration
- air-gapped AI assistant
- AI terminal no cloud

**Cluster C — control-seeking.**
- AI suggests commands human approves
- AI terminal without execution
- read-only AI assistant terminal
- AI copilot that cannot run commands

**Cluster D — category-adjacent.**
- AI terminal alternative privacy
- private alternative to AI terminal assistants
- open source AI terminal

Cluster A maps to the `/security` page and the alternatives section. Clusters B and C map to the hero and the off-switch section. Serve A with substance, not a landing page — that audience detects and leaves marketing pages.

### 7.3 Structured data

Add `SoftwareApplication` JSON-LD: name, `applicationCategory: DeveloperApplication`, `operatingSystem: Windows, macOS, Linux`, `offers` priced 0, `license` Apache-2.0, `softwareVersion`. Keep `softwareVersion` in sync with the current release — it is currently v2.4.0.

### 7.4 Content that earns links

The `/security` page (§5.7) is the linkable asset. Secondary: a plainly written comparison of the four operator workflows, publishable as a standalone page. Both are referenceable in vendor reviews and procurement threads, which is where this product gets chosen.

**Do not** produce keyword-stuffed blog volume. This audience is small, technical, and hostile to content marketing. Two excellent pages beat twenty adequate ones.

---

## 8. The paid tier

**Both agents: read, do not publish specifics.**

Centralized configuration lockdown and auto-populated connections from a database or file are **deliberately reserved** for a future licensed build (name undecided; "Enterprise" is preferred over "Pro" because both features are bought by central IT rather than by an individual operator). The private repository does not yet exist. **Do not announce, name, date, or price this tier.**

### 8.1 The open-core line holds

Every guarantee in §2 is unconditional in the open-source build. The paid tier adds the ability to *guarantee the posture for people other than yourself* — a different product for a different buyer. Nothing behind the paywall is load-bearing for the free tier's security claim. State the free-tier guarantees flat, with no asterisk pointing at a pricing page (§3.10).

### 8.2 Approved answer to the fleet question

When asked "can I enforce this across four hundred engineers?" — in an issue, a Discussion, a review, or site copy — the approved response is:

> Central policy control is on the enterprise roadmap. In the open build, the posture is set per operator and shown on screen for the whole session.

True today, signals direction, promises no date. Silence is worse than this sentence: an evaluator who asks and gets nothing concludes the answer is *no, permanently*.

### 8.3 When the tier does ship

Position connection-import as a **security** feature, not a convenience one. It ends the text file of IP addresses, and means connection details come from a system of record rather than a hundred personal notes files. Internal topology stops living on laptops. Sold as bulk import it competes with a CSV parser; sold as controlled provenance for connection data it belongs in the same story as everything in §2.

---

## 9. Verification protocol

**Binding on both agents.**

1. **No new security claim ships without a `file:symbol` citation.** If you cannot point at the code that makes a claim true, do not publish the claim. Add it to §2 with its citation, or drop it.
2. **Re-run the §2.1 grep before any release that touches messaging.** If tool-calling ever appears in the tree, §2.1 is void and this document must be revised before publication.
3. **Treat §2 line numbers as stale.** Grep for the symbol.
4. **When the code and this document disagree, the code wins** — then fix this document.
5. **Escalate rather than improvise.** If a claim seems true but is not listed in §2, do not publish it on your own judgment. Security claims are the one category where an agent guessing correctly nine times out of ten is a net loss.
6. **Do not weaken §3 to fit a headline.** If a prohibited phrasing is the only one that fits a layout, change the layout.

---

## 10. Why this changed

Positioning review, 2026-09-17. Interviews with IT operations and security managers indicated the product was being evaluated as a convenience tool, against competitors it does not need to compete with, while its actual differentiator went unmentioned on every public surface.

Findings that drove this document:

- The website contained no security content of any kind.
- The README contained good security material, buried mid-document, and led with a tagline naming the category PairAdmin differentiates from.
- The execution boundary — the strongest and most defensible property in the product — was stated nowhere publicly.
- The Disable Pair LLM feature, backend-enforced with a persistent on-screen indicator, was shipped and documented nowhere.
- Two claims in circulation were unsupported by current code (Ollama localhost-only; audit log answering "what did we send them?"), and two were phrased in ways trivially disproved by opening the app ("no chat windows"; "none of the risk").

The through-line: **this product's claims are unusually strong and unusually verifiable, and it was selling none of them.**
