import { useEffect, useRef, useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
import { Tabs } from "@base-ui/react/tabs";
import { useTerminalStore } from "@/stores/terminalStore";
import { readTerminalLines } from "@/utils/terminalContext";

// Mirrors the fontFamily stack TerminalPreview.tsx passes to `new Terminal({...})`.
// Reused rather than restated so the snapshot cannot drift out of visual sync with
// the live terminal it is mirroring — a second hardcoded stack is exactly the kind
// of duplication that makes column alignment look subtly wrong on a later font change.
const TERMINAL_FONT_STACK =
  "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', monospace";

// How long the "Copied!" label stays on the Copy All button.
const COPIED_FEEDBACK_MS = 1500;

// Matches ChatPane's fallback and SettingsService.SetContextLines' behavior when
// AppConfig.ContextLines is unset. Deliberately the SAME literal as ChatPane's:
// the filtered view claims to be "what the LLM would receive", so a different
// default here would make that claim false the moment the two drift apart.
const DEFAULT_CONTEXT_LINES = 200;

/**
 * The two things a user can copy out of Copy Mode.
 *
 * "complete" is the raw snapshot — everything the terminal still retains.
 * "filtered" is the same terminal through the LLM's own pipeline: capped to the
 * configured context window and run through the credential/ANSI filters, i.e.
 * exactly the bytes the model would be handed.
 */
type CopyModeView = "complete" | "filtered";

const VIEW_LABELS: Record<CopyModeView, string> = {
  complete: "Complete buffer",
  filtered: "Filtered LLM buffer",
};

const VIEW_ORDER: CopyModeView[] = ["complete", "filtered"];

export interface CopyModeDialogProps {
  open: boolean;
  onClose: () => void;
}

/** Honest text for a rejection that arrived as something other than an Error. */
function filterErrorMessage(err: unknown): string {
  if (typeof err === "string" && err.trim()) return err.trim();
  if (err instanceof Error && err.message.trim()) return err.message.trim();
  return "Filtering failed";
}

/**
 * Copy Mode — a hotkey-triggered, copy-only mirror of the active terminal's
 * content, in two views.
 *
 * WHY THIS EXISTS: a full-screen TUI (Hermes, opencode, vim, htop) switches
 * xterm.js to the ALTERNATE screen buffer, which has no scrollback, and enables
 * mouse tracking so a click-drag is forwarded to the app rather than used for
 * selection. Both are standard terminal behavior, not a PairAdmin defect. This
 * overlay gives the text back by rendering it as a plain read-only textarea, so
 * the browser's own selection and copy handling apply.
 *
 * WHY TWO TABS (UAT): the single view showed the whole buffer, which is what you
 * want for "give me everything", and is useless for answering "what exactly did
 * the assistant see?". The second tab answers that question with the backend's
 * own pipeline (LLMService.FilterText over the same capped window the context
 * builder uses) instead of a frontend reimplementation that could drift.
 *
 * LIFECYCLE — one state machine, deliberately:
 *   - snapshot is taken ONCE on open, and again only when Refresh is clicked;
 *   - while open it is FROZEN — no interval, no `pty:output` subscription, no
 *     re-render. Text must not move under a cursor that is selecting it;
 *   - the filtered view is computed on demand (first switch to it, or Refresh
 *     while it is showing), never on open — an unused tab must not pay for a
 *     backend round trip;
 *   - close (Escape, click-outside, or the Close button) discards both views, so
 *     the next open takes a fresh snapshot.
 * The freeze is the whole point: a live-updating mirror is useless for copying.
 *
 * KNOWN LIMITATION: while a TUI holds the alternate screen, `term.buffer.active`
 * IS the alternate buffer, so this shows the current screen plus whatever real
 * scrollback exists in the NORMAL buffer — not content the TUI has already
 * scrolled out of its own view. Continuously capturing alt-screen redraws into a
 * rolling per-tab buffer is a real v2 feature; deliberately not attempted here.
 */
export function CopyModeDialog({ open, onClose }: CopyModeDialogProps) {
  const [snapshot, setSnapshot] = useState("");
  // `null` means "not computed yet" — distinct from "" (a genuinely empty
  // filtered buffer), which is what lets the loading state be honest.
  const [filteredText, setFilteredText] = useState<string | null>(null);
  const [filtering, setFiltering] = useState(false);
  const [filterError, setFilterError] = useState<string | null>(null);
  const [view, setView] = useState<CopyModeView>("complete");
  const [copied, setCopied] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  // Tracks the timer so an unmount mid-feedback window can't setState after the
  // dialog is gone (and so a second click restarts the window rather than racing).
  const copiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // How many trailing lines the LLM context builder would send. Loaded once from
  // the persisted setting on mount, exactly as ChatPane does.
  const contextLinesRef = useRef(DEFAULT_CONTEXT_LINES);
  // Monotonic id of the newest filter request. A slower earlier request must not
  // overwrite a newer one's result (Refresh while a filter is still in flight).
  const filterRequestRef = useRef(0);

  // Read the CURRENT active terminal. Deliberately a function, not a value read
  // at render time: the active tab (and therefore the term ref) can change while
  // the dialog is open, and a stale closure would snapshot the wrong terminal.
  const takeSnapshot = () => {
    const { activeTabId, getTermRef } = useTerminalStore.getState();
    // No second argument: the whole active buffer, NOT the 200-line default. A
    // truncation here would silently drop output, which is the one failure mode
    // a copy tool cannot have.
    return readTerminalLines(getTermRef(activeTabId));
  };

  // The LLM's own window: the capped tail, read the same way the context builder
  // reads it, so the filtered view and a real send see the same lines.
  const takeFilteredSource = () => {
    const { activeTabId, getTermRef } = useTerminalStore.getState();
    return readTerminalLines(getTermRef(activeTabId), contextLinesRef.current);
  };

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => {
        if (cfg?.ContextLines) contextLinesRef.current = cfg.ContextLines;
      })
      .catch(() => {});
  }, []);

  // Snapshot on open. `open` toggling is the only trigger — this effect must NOT
  // depend on the terminal or on a refresh tick, or the text would move while
  // the user is selecting it.
  useEffect(() => {
    if (!open) return;
    setSnapshot(takeSnapshot());
    setView("complete");
    setFilteredText(null);
    setFiltering(false);
    setFilterError(null);
    setCopied(false);
  }, [open]);

  // Autofocus on open, and again when the visible view changes, so the textarea
  // the user is looking at is the one that gets Ctrl+C.
  //
  // NO el.select() HERE. Auto-select-all on open was the UAT complaint: it makes
  // the first drag a fight against a pre-existing selection, and a stray Ctrl+C
  // then dumps the entire buffer onto the clipboard. Focus without selection is
  // the whole contract; "Copy All" is the deliberate way to take everything.
  //
  // The dialog's own focus manager (initialFocus on the Popup) is the primary
  // focus path and lands a tick after this effect, which collapses any selection
  // made here — so a re-added el.select() would not be visible in the caret
  // state alone. The test therefore also spies on
  // HTMLTextAreaElement.prototype.select and requires it never to be called.
  useEffect(() => {
    if (!open) return;
    const el = textareaRef.current;
    if (!el) return;
    el.focus();
    // `filtering` is a dependency so that the textarea which APPEARS when the
    // filtered text arrives is the one that receives focus.
  }, [open, view, filtering]);

  // Clear the pending "Copied!" timer on unmount.
  useEffect(
    () => () => {
      if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current);
    },
    []
  );

  // Discards the snapshot on close so no stale text is retained between sessions.
  //
  // MEASURED EQUIVALENT (mutation M11, hand-verified): removing this effect
  // changes no observable behavior today, because the open-effect above
  // unconditionally re-snapshots on every open and both effects run in the same
  // commit — so the stale value is overwritten before any render can display it.
  // Verified directly: reopening against a missing terminal yields "" with AND
  // without this effect.
  //
  // Kept deliberately as defense-in-depth: it makes "no stale text survives a
  // close" a property of the CLOSE transition itself rather than an emergent
  // consequence of two effects happening to share a dependency. If the open-effect
  // is ever optimized to skip re-reading (say, restoring scroll position instead),
  // this is what stops the previous session's output from reappearing. It is
  // untestable today, and that is recorded here rather than dressed up with a test
  // that would only assert the current implementation order.
  useEffect(() => {
    if (open) return;
    setSnapshot("");
    setFilteredText(null);
    setFilterError(null);
    setCopied(false);
  }, [open]);

  /**
   * Computes the filtered view from the current terminal.
   *
   * The redaction itself is the BACKEND's (LLMService.FilterText — the same ANSI
   * + credential + custom-pattern pipeline a real send uses). Reimplementing it
   * here would mean two pipelines that can disagree, and the one the user is
   * shown is the one they would trust.
   */
  const runFilter = async () => {
    const request = ++filterRequestRef.current;
    setFiltering(true);
    setFilterError(null);
    try {
      const source = takeFilteredSource();
      const { FilterText } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/LLMService"
      );
      const result = await FilterText(source);
      if (filterRequestRef.current !== request) return; // superseded
      setFilteredText(result);
    } catch (err) {
      if (filterRequestRef.current !== request) return;
      setFilteredText(null);
      setFilterError(filterErrorMessage(err));
    } finally {
      if (filterRequestRef.current === request) setFiltering(false);
    }
  };

  const handleViewChange = (next: CopyModeView) => {
    setView(next);
    setCopied(false);
    // First switch to the filtered view computes it; switching back and forth
    // does not re-run the backend call (Refresh is the explicit way to re-read).
    if (next === "filtered" && filteredText === null && !filtering) {
      void runFilter();
    }
  };

  const handleRefresh = () => {
    setCopied(false);
    if (view === "filtered") {
      void runFilter();
      return;
    }
    setSnapshot(takeSnapshot());
  };

  // What "Copy All" copies: the ACTIVE view, never a mix of the two.
  const activeText = view === "complete" ? snapshot : (filteredText ?? "");
  // Nothing to copy in the filtered view until the backend has answered — an
  // empty write that still reported "Copied!" would be a false success claim.
  const copyDisabled = view === "filtered" && filteredText === null;

  // Writes to the OS clipboard directly rather than relying on the textarea's
  // selection (which the browser only copies on the user's own Ctrl+C). The
  // catch() mirrors TerminalPreview.tsx: a clipboard write can be refused on
  // permissions, and that must not throw out of a click handler.
  const handleCopyAll = () => {
    if (copyDisabled) return;
    navigator.clipboard
      .writeText(activeText)
      .then(() => {
        setCopied(true);
        if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current);
        copiedTimerRef.current = setTimeout(() => setCopied(false), COPIED_FEEDBACK_MS);
      })
      .catch(() => {});
  };

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        // Escape and click-outside both land here — one dismiss path, no
        // hand-rolled second mechanism.
        if (!o) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Popup
          // The dialog's own focus manager runs after mount, and now that the
          // tablist is the first tabbable element inside the popup it would
          // otherwise land on the "Complete buffer" tab instead of the text.
          // Point it at the textarea: the textarea is what Ctrl+C acts on.
          initialFocus={textareaRef}
          className="fixed left-1/2 top-1/2 z-50 w-[min(900px,90vw)] h-[70vh] max-h-[70vh] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-1 border border-surface-border-strong shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-surface-text border-b border-surface-border flex items-center justify-between gap-4">
            <span>Copy Mode</span>
            <span className="text-xs font-normal text-surface-text-muted">
              Frozen snapshot — use Copy All, or select and press Ctrl+C
            </span>
          </Dialog.Title>

          <Tabs.Root
            value={view}
            onValueChange={(value) => handleViewChange(value as CopyModeView)}
            className="flex-1 min-h-0 flex flex-col"
          >
            <Tabs.List className="px-6 flex items-center gap-1 border-b border-surface-border">
              {VIEW_ORDER.map((value) => (
                <Tabs.Tab
                  key={value}
                  value={value}
                  className="px-3 py-2 text-xs text-surface-text-muted hover:text-surface-text cursor-pointer border-b-2 border-transparent aria-selected:text-surface-text aria-selected:border-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  {VIEW_LABELS[value]}
                </Tabs.Tab>
              ))}
            </Tabs.List>

            {/* min-h-0 is load-bearing: without it a flex child refuses to shrink
                below its content, so a 2000-line snapshot would grow the dialog
                off-screen instead of scrolling inside it. */}
            <Tabs.Panel value="complete" className="flex-1 min-h-0 p-4">
              <textarea
                readOnly
                value={snapshot}
                aria-label="Terminal output (copy mode)"
                ref={textareaRef}
                spellCheck={false}
                className="w-full h-full resize-none bg-surface-0 text-surface-text text-sm p-3 outline-none"
                style={{ fontFamily: TERMINAL_FONT_STACK }}
              />
            </Tabs.Panel>

            <Tabs.Panel value="filtered" className="flex-1 min-h-0 p-4 flex flex-col gap-2">
              {/* While the backend is answering (or has refused), the textarea
                  is replaced by the state, so an empty box is never mistaken for
                  "the model would receive nothing". */}
              {filtering ? (
                <p className="text-xs text-surface-text-muted">Filtering…</p>
              ) : filterError ? (
                <p role="alert" className="text-xs text-red-400">
                  {filterError}
                </p>
              ) : (
                <textarea
                  readOnly
                  value={filteredText ?? ""}
                  aria-label="Filtered LLM buffer (copy mode)"
                  ref={textareaRef}
                  spellCheck={false}
                  className="w-full flex-1 min-h-0 resize-none bg-surface-0 text-surface-text text-sm p-3 outline-none"
                  style={{ fontFamily: TERMINAL_FONT_STACK }}
                />
              )}
              <p className="text-xs text-surface-text-muted">
                Exactly what the model would receive: the last{" "}
                <span className="font-mono">{contextLinesRef.current}</span> lines of this
                terminal, with credentials and ANSI control sequences removed.
              </p>
            </Tabs.Panel>
          </Tabs.Root>

          {/* Zero-code discoverability tip: xterm.js already honors Shift+drag to
              force local selection even while a TUI has mouse tracking on. That
              half of the problem needs no code at all — surfacing it here costs
              nothing and helps immediately. */}
          <p className="px-6 pb-2 text-xs text-surface-text-muted">
            Tip: hold <span className="font-mono">Shift</span> while dragging in the terminal
            to select text even when a TUI app has taken over the mouse.
          </p>

          <div className="flex items-center gap-2 px-6 py-4 border-t border-surface-border">
            <button
              onClick={handleRefresh}
              className="bg-surface-2 hover:bg-surface-3 text-surface-text text-xs px-4 py-1.5 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-surface-1"
            >
              Refresh
            </button>
            {/* Deliberately does NOT close. The user may still want to select a
                smaller portion afterward; closing is the lifecycle's job alone. */}
            <button
              onClick={handleCopyAll}
              disabled={copyDisabled}
              className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-surface-1"
            >
              {copied ? "Copied!" : "Copy All"}
            </button>
            <button
              onClick={onClose}
              className="ml-auto bg-surface-2 hover:bg-surface-3 text-surface-text-muted text-xs px-4 py-1.5 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-surface-1"
            >
              Close
            </button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
