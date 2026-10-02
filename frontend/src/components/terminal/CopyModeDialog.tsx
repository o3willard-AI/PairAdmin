import { useEffect, useRef, useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
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

export interface CopyModeDialogProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Copy Mode — a hotkey-triggered, copy-only mirror of the active terminal's
 * content.
 *
 * WHY THIS EXISTS: a full-screen TUI (Hermes, opencode, vim, htop) switches
 * xterm.js to the ALTERNATE screen buffer, which has no scrollback, and enables
 * mouse tracking so a click-drag is forwarded to the app rather than used for
 * selection. Both are standard terminal behavior, not a PairAdmin defect. This
 * overlay gives the text back by rendering it as a plain read-only textarea, so
 * the browser's own selection and copy handling apply.
 *
 * LIFECYCLE — one state machine, deliberately:
 *   - snapshot is taken ONCE on open, and again only when Refresh is clicked;
 *   - while open it is FROZEN — no interval, no `pty:output` subscription, no
 *     re-render. Text must not move under a cursor that is selecting it;
 *   - close (Escape, click-outside, or the Close button) discards the snapshot,
 *     so the next open takes a fresh one.
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
  const [copied, setCopied] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  // Tracks the timer so an unmount mid-feedback window can't setState after the
  // dialog is gone (and so a second click restarts the window rather than racing).
  const copiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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

  // Snapshot on open. `open` toggling is the only trigger — this effect must NOT
  // depend on the terminal or on a refresh tick, or the text would move while
  // the user is selecting it.
  useEffect(() => {
    if (!open) return;
    setSnapshot(takeSnapshot());
    setCopied(false);
  }, [open]);

  // Autofocus + select all on open: a user who wants everything gets it with the
  // hotkey and then Ctrl+C, zero further clicks. Re-runs on open because the
  // textarea mounts with the dialog each time.
  useEffect(() => {
    if (!open) return;
    const el = textareaRef.current;
    if (!el) return;
    el.focus();
    el.select();
  }, [open, snapshot]);

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
    setCopied(false);
  }, [open]);

  const handleRefresh = () => {
    setSnapshot(takeSnapshot());
    setCopied(false);
  };

  // Writes to the OS clipboard directly rather than relying on the textarea's
  // selection (which the browser only copies on the user's own Ctrl+C). The
  // catch() mirrors TerminalPreview.tsx: a clipboard write can be refused on
  // permissions, and that must not throw out of a click handler.
  const handleCopyAll = () => {
    navigator.clipboard
      .writeText(snapshot)
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
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[min(900px,90vw)] h-[70vh] max-h-[70vh] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-1 border border-surface-border-strong shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-surface-text border-b border-surface-border flex items-center justify-between gap-4">
            <span>Copy Mode</span>
            <span className="text-xs font-normal text-surface-text-muted">
              Frozen snapshot — use Copy All, or select and press Ctrl+C
            </span>
          </Dialog.Title>

          {/* min-h-0 is load-bearing: without it a flex child refuses to shrink
              below its content, so a 2000-line snapshot would grow the dialog
              off-screen instead of scrolling inside it. */}
          <div className="flex-1 min-h-0 p-4">
            <textarea
              readOnly
              value={snapshot}
              aria-label="Terminal output (copy mode)"
              ref={textareaRef}
              spellCheck={false}
              className="w-full h-full resize-none bg-surface-0 text-surface-text text-sm p-3 outline-none"
              style={{ fontFamily: TERMINAL_FONT_STACK }}
            />
          </div>

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
              className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-surface-1"
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