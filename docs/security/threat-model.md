# PairAdmin — Threat Model

This document states, precisely and verifiably, what PairAdmin sends to a model,
what it never sends, what runs locally, and where the boundaries are. Every claim
cites the code that makes it true; where a claim is a hard architectural
guarantee, the verification command is printed so you can check it yourself.

---

## 1. What PairAdmin sends, exactly

A request to a model contains three things and nothing else:

- the system prompt (fixed instructions),
- the last N lines of the terminal you are looking at,
- what you typed.

No conversation history, no other tabs, no filesystem, no saved connections.
`services/llm/context.go` — `BuildMessages()` returns exactly `[]Message{system, user}`.

## 2. What it never sends

- Conversation history — for any tab.
- Other sessions or tabs.
- Your filesystem.
- Saved connection details or credentials.
- Terminal content before you press send — the terminal is read only inside
  `handleSend` (`frontend/src/components/chat/ChatPane.tsx`). No ambient
  streaming, no background capture.

## 3. What runs locally

- Credential redaction — in-process, before the provider call.
- The audit log — a local rotating JSONL file.
- Key storage — the OS keychain or an encrypted file, plus memguard enclaves in memory.
- Local inference — Ollama (`localhost:11434`) and LM Studio (`localhost:1234`)
  default to loopback.

## 4. The execution boundary

The assistant has no path to the shell. There is no tool-calling,
function-calling, or agent-loop code anywhere in the tree — not a disabled
permission, an absence. Verify it yourself:

```
git grep -niE '"tools"|tool_choice|function_call|tool_use|ToolCall' origin/master -- services frontend/src
```

This returns zero. The only writes into a terminal are `sendToTerminal()`
(`frontend/src/utils/sendToTerminal.ts`), called from button `onClick` handlers
(`CodeBlock.tsx`, `CommandSidebar.tsx`), and your own typing (`TerminalPreview.tsx`).

## 5. Redaction pattern list

Fifteen patterns, scrubbed in-process before transmission:

`aws-access-key-id` · `github-token` · `gitlab-personal-access-token` ·
`openai-api-key` · `anthropic-api-key` · `slack-token` · `google-api-key` ·
`google-service-account` · `azure-account-key` · `bearer-token` · `jwt` ·
`pem-private-key` · `password-assignment` · `generic-api-key` ·
`connection-string-credentials`

Plus any regexes you add with `/filter`.
`services/llm/filter/credential.go`; applied in `LLMService.SendMessage` before `BuildMessages`.

## 6. Audit log contents

Every prompt and every response is written to a local rotating JSONL log
(`services/audit/audit.go`; events `user_message` and `ai_response` in
`services/llm_service.go`). The log records the operator's typed message and the
model's reply. It does **not** record the terminal context that was transmitted.

## 7. Key storage

API keys and the file-backend master password are held in the OS keychain
(macOS Keychain / Windows Credential Manager / Secret Service) or an encrypted
file backend, and in memguard enclaves while in memory (ADR-0003).

## 8. SSH host-key pinning

Remote SSH sessions use trust-on-first-use host-key pinning with a per-user
trust store (ADR-0002). No blanket skip-verify.

## 9. Known limitations

- The audit log excludes the transmitted terminal context.
- Redaction is regex-based; it is not a commercial secret-scanning engine.
- Remote Ollama is supported by design (defaults to loopback; Settings warns
  while a non-loopback host is configured).
- There is no central/fleet lockdown in the open build — the posture is set per
  operator.
- *On Linux, the WebKit content-process sandbox is disabled* (ADR-0001: the
  sandbox's bubblewrap/user-namespace requirement can fall back to a blank
  window, so it is disabled; only bundled, trusted UI assets load).

## 10. How to run fully air-gapped

Set the provider to **Disable Pair LLM** — no model is constructed and no
endpoint is contacted, and the status bar reads **Disabled** (amber) for the
whole session. Or point PairAdmin at a local Ollama/LM Studio on loopback. The
app has no account and no telemetry; nothing phones home.
