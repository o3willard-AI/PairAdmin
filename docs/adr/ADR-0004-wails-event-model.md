# ADR-0004: Wails event model — `component:action` naming with an injectable emitter

- **Status:** Accepted
- **Date:** 2026-09-05
- **Deciders:** PairAdmin maintainers
- **Technical area:** Frontend/backend contract / Wails runtime events

## Context

PairAdmin's backend services (Go) need to push asynchronous data to the
frontend (TypeScript/React) — LLM response tokens arriving over seconds,
terminal output from PTY read loops, capture-manager pane updates, settings
changes. Wails v2 provides two mechanisms:

1. **Bound methods** (frontend calls backend): services listed in `main.go`'s
   `Bind: []interface{}{...}` become callable from JS. Request/response only
   — the frontend pulls.
2. **Runtime events** (`runtime.EventsEmit` on the Go side, `EventsOn` in
   JS): the backend pushes, the frontend subscribes. This is what a
   streaming/asynchronous UI needs.

The push surface is substantial. Events currently emitted, all from backend
services:

| Event                    | Emitter                       | Payload                  |
|--------------------------|-------------------------------|--------------------------|
| `llm:chunk`              | LLMService.SendMessage        | ChatTokenEvent (seq, text) |
| `llm:error`              | LLMService.SendMessage        | ChatTokenEvent (error)   |
| `llm:done`               | LLMService.SendMessage        | ChatTokenEvent (done)    |
| `terminal:tabs`          | CaptureManager.tick           | TerminalTabsEvent        |
| `terminal:update`        | CaptureManager.tick           | TerminalUpdateEvent      |
| `settings:changed`       | SettingsService.SaveSettings  | AppConfig                |
| `settings:model-changed` | SettingsService.SetModel      | provider:model string    |
| `terminal:rename`        | SettingsService.RenameTab     | tabId + label            |
| `pty:output`             | PTYService (all terminal kinds) | PTYOutputEvent         |
| `pty:closed`             | PTYService (all terminal kinds) | {tabId}                |

The alternative to events for any of these is frontend polling (setInterval
calling a bound method) — which is exactly what the CaptureManager's
500ms-tick push model replaced for terminal content.

## Decision

**Backend-to-frontend asynchronous communication uses Wails runtime events,
named `component:action` (colon-separated, lowercase component noun, verb
suffix), and every service holds the emitter behind an injectable `emitFn`
field defaulting to `runtime.EventsEmit`.**

Concrete rules, as implemented:

1. **Naming: `component:action`.** The component noun comes first so related
   events group together and are grep-able per component: `llm:*` (the
   assistant), `pty:*` (terminal sessions), `terminal:*` (capture-side pane
   state), `settings:*` (settings changes). The action verb comes after the
   colon. No bare verbs (`"chunk"`), no dots, no camelCase components.
2. **Injectable emitter seam.** Each service holds
   `emitFn func(ctx context.Context, event string, optionalData ...interface{})`
   — `LLMService`, `PTYService`, `RemoteService`, `SettingsService`,
   `CaptureManager` (passed as a constructor arg). Production wiring sets it
   to `runtime.EventsEmit`; tests substitute a recording closure
   (`newTestPTYService`, `newTestRemoteService`), so event-emission behavior
   is unit-testable without a Wails runtime (which panics/fatals outside the
   app context).
3. **Payloads are typed structs with JSON tags.** `ChatTokenEvent{Seq, Text,
   Done, Error}`, `PTYOutputEvent{TabID, Data}`, `TerminalTabsEvent`,
   `TerminalUpdateEvent` — not loose maps, so the frontend consumes a stable
   shape (`frontend/wailsjs/go/models.ts` mirrors them).
4. **Subscriptions are explicit and cleaned up.** The frontend subscribes
   with `runtime.EventsOn(event, handler)` inside `useEffect` hooks
   (`useLLMStream.ts` subscribes to `llm:chunk`/`llm:done`/`llm:error`;
   `useTerminalCapture.ts` to the `terminal:*`/`pty:*` events) and calls the
   returned unsubscribe function on unmount — no leaked listeners across tab
   mounts.
5. **Per-tab routing is the payload's job, not the event bus's.** Wails
   events are global (no per-window/topic channels used); payloads carry the
   routing key (`TabID`, `PaneID`) and the frontend routes to per-tab state
   (e.g. `messagesByTab` keyed by tabId). `ChatTokenEvent` deliberately
   carries no tabId because a stream is global — see the `llmRequest` flag
   comment in `chatStore.ts`.

## Consequences

### Positive
- **Streaming works.** LLM tokens, PTY output, and capture updates push to
  the UI the moment they arrive — no polling, no artificial latency, and the
  500ms capture tick coalesces naturally into one `terminal:update` per
  changed pane per tick.
- **Testability without a runtime.** The `emitFn` seam means every emitter
  site (`SendMessage`'s chunk/done/error emission, the PTY read loop's
  output/closed events, `SaveSettings`' `settings:changed`) is asserted in
  unit tests by swapping one field. Without it, none of this code could be
  tested outside a running app.
- **Grep-able contract.** The `component:action` rule means
  `grep -rn '"pty:' services/` finds every emitter, and the frontend's
  `EventsOn` calls mirror it — the event contract is auditable from both
  sides with simple text search.
- **Stable, typed payload shapes.** JSON-tagged structs + the generated
  `models.ts` mirror keep both sides honest about field names.

### Negative / accepted trade-offs
- **Event names are stringly-typed on both sides.** A typo
  (`"llm:chunks"`) compiles and silently breaks the feature; only the
  frontend's tests that assert specific `EventsOn`/emit strings catch it.
  A constants file was considered (see Alternatives) and deferred.
- **Global broadcast, no built-in scoping.** Every subscriber receives every
  event; per-tab routing is hand-rolled in consumers. Fine at PairAdmin's
  event volume; would need revisiting if per-window isolation were required.
- **Ordering guarantee is only per-emitter.** `pty:output` ordering is
  preserved because one read loop emits sequentially, but there is no
  cross-event ordering contract (e.g. between `settings:changed` and
  `pty:output`) — consumers must not depend on one.
- **`optionalData ...interface{}` is untyped at the emitter.** Wails'
  signature forces it; the typed-struct rule (3) is the discipline that
  compensates, and code review enforces it.

## Alternatives considered

1. **Frontend polling via bound methods.**
   *Rejected for streaming surfaces.* LLM tokens would land in 500ms-sized
   bursts (or require a tighter poll burning CPU), terminal output would need
   a content-diff endpoint, and battery/CPU cost scales with open tabs.
   Polling remains acceptable only where latency is irrelevant — nothing in
   the current event set qualifies.

2. **A single `app:event` channel with a discriminated-union payload.**
   *Rejected.* Collapses the grep-able `component:action` namespace into one
   switch statement in every consumer, and makes partial subscription
   impossible — every listener pays deserialization cost for every event.

3. **A shared constants file for event names (exported Go consts + TS
   mirror).**
   *Deferred.* Genuinely nice, but the `component:action` rule plus the
   frontend tests asserting literal names have kept drift at zero so far;
   adding a codegen/mirror step now is ceremony without a demonstrated bug.
   Revisit if the event count grows much further.

4. **Callbacks instead of events (backend holds a frontend-registered
   function).**
   *Rejected.* Wails supports JS callbacks into Go, but a *backend→frontend*
   callback requires holding a JS function reference from Go, which couples
   backend services to the frontend's lifetime and breaks the
   test-without-runtime property the `emitFn` seam provides.

5. **Per-tab event channels (topic-style).**
   *Rejected.* Wails events are app-global; emulating topics means
   per-tab event-name suffixes (`llm:chunk:<tabId>`) and dynamic
   subscription churn on tab switch. Carrying the tabId in the payload
   (rule 5) achieves the routing with static subscriptions.

## References

- `services/llm_service.go` — `ChatTokenEvent`, `emitFn` field, the
  `llm:*` emissions in `SendMessage`.
- `services/pty_service.go` — `PTYOutputEvent`, `pumpPTYOutput` (`pty:output`
  / `pty:closed`), `emitFn` field.
- `services/capture/manager.go` — `TerminalTabsEvent`,
  `TerminalUpdateEvent`, `CaptureManager.emitFn` (constructor-injected).
- `services/settings_service.go` — `settings:changed`,
  `settings:model-changed`, `terminal:rename`.
- `main.go` — `Bind: []interface{}{...}` (bound-method surface), production
  `runtime.EventsEmit` wiring.
- `frontend/src/hooks/useLLMStream.ts`, `useTerminalCapture.ts` — the
  `EventsOn` subscription + unsubscribe pattern.
- `frontend/wailsjs/go/models.ts` — the TypeScript mirror of the payload
  structs.
