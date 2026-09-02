import { useEffect, useState, useCallback, useRef } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { sendToTerminal } from "@/utils/sendToTerminal";
import { isForeignTextEntry } from "@/utils/hotkey";

export interface QuickSelectItem {
  label: string; // "F1" .. "F12"
  kind: "command" | "terminal";
  id: string; // command id when kind==="command"; terminal tab id when kind==="terminal"
}

// F1..F12 only — the chord must not swallow F13+ (media/server keys).
const FKEY_RE = /^F([1-9]|1[0-2])$/;

const MAX_ITEMS = 12;

export function useQuickSelect(): { visible: boolean; items: QuickSelectItem[] } {
  const [visible, setVisible] = useState(false);
  const [items, setItems] = useState<QuickSelectItem[]>([]);
  // Mirror of `items` for the (bound-once) document keydown listener — lets
  // the handler route an F-key press without the effect re-subscribing every
  // time the item list is recomputed.
  const itemsRef = useRef<QuickSelectItem[]>([]);

  // Rebuilt on every activation so the overlay always reflects the current
  // pinned-command / terminal state. The hook deliberately subscribes to
  // neither store — a live subscription would re-render the app's layout on
  // every sidebar or tab change just to keep an unshown list warm.
  const activate = useCallback(() => {
    const { commands } = useCommandStore.getState();
    const { tabs } = useTerminalStore.getState();

    // Pinned commands first (sidebar display order, top-to-bottom), then
    // terminals (tab-list order, top-to-bottom), capped at 12 (F1..F12).
    const next: QuickSelectItem[] = [
      ...commands.filter((c) => c.pinned).map((c) => ({ label: "", kind: "command" as const, id: c.id })),
      ...tabs.map((t) => ({ label: "", kind: "terminal" as const, id: t.id })),
    ]
      .slice(0, MAX_ITEMS)
      .map((item, index) => ({ ...item, label: `F${index + 1}` }));

    itemsRef.current = next;
    setItems(next);
    setVisible(true);
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // Chord held = Ctrl + (Alt | Meta). Everything below only applies while
      // it's down; outside the chord, F-keys keep their normal meanings.
      const chordHeld = event.ctrlKey && (event.altKey || event.metaKey);
      if (!chordHeld) return;

      // Don't activate or route F-keys while the user is typing in a
      // non-terminal text entry (chat box, dialog inputs, settings fields) —
      // same isForeignTextEntry check useConfiguredHotkey applies; the active
      // terminal's own hidden textarea is exempted, so the primary use case
      // (terminal focused) is unaffected. The keyup release handler below is
      // deliberately NOT guarded: releasing the chord must always hide the
      // overlay, whatever the focus landed in meanwhile.
      const { activeTabId, getTermRef } = useTerminalStore.getState();
      const term = getTermRef(activeTabId);
      const terminalTextarea = (term as unknown as { textarea?: HTMLTextAreaElement } | null)
        ?.textarea;
      if (isForeignTextEntry(document.activeElement, terminalTextarea)) return;

      if (FKEY_RE.test(event.key)) {
        // Suppress the F-key's default meaning for every chorded press —
        // F5 must not refresh, F11 must not fullscreen, F1 must not open
        // help — even when the slot is beyond the item count.
        event.preventDefault();
        // Chromium reserves some F-keys at the browser level (F12 devtools —
        // inert in the Wails WebView, but stay conservative); stopPropagation
        // also keeps any other document-level handler from double-handling.
        event.stopPropagation();

        const item = itemsRef.current[Number(event.key.slice(1)) - 1];
        if (!item) return;

        if (item.kind === "command") {
          // Insertion only — never execute on the user's behalf. The active
          // tab is read at press time so a terminal switch moments earlier
          // doesn't misroute the insertion.
          const text = useCommandStore.getState().consumeCommandText(item.id);
          const { activeTabId } = useTerminalStore.getState();
          sendToTerminal(activeTabId, text, false);
        } else {
          useTerminalStore.getState().setActiveTab(item.id);
        }
        return;
      }

      // Any other chorded keydown (including Ctrl auto-repeat) (re-)arms the
      // overlay with a freshly computed list — cheap, and keeps it correct
      // if the sidebar or tab list changed while the chord was held.
      activate();
    };

    const handleKeyUp = (event: KeyboardEvent) => {
      // Chord broken when Ctrl goes up, or when the last of Alt/Meta goes up
      // (the other of the two may still be physically held). Note: no
      // preventDefault/stopPropagation on keyup — releasing a modifier has
      // no default action worth suppressing, and blocking it system-wide
      // could confuse other listeners (e.g. xterm's own key handling).
      if (!event.ctrlKey || !(event.altKey || event.metaKey)) {
        setVisible(false);
      }
    };

    // Capture phase on both listeners: run before xterm's own key handlers
    // (which listen on its textarea, not the document) so (a) the chorded
    // F-key routing wins while the terminal has focus instead of the shell
    // also seeing it, and (b) the chord-release keyup reliably hides the
    // overlay even if xterm swallows a keyup. Mirrors useConfiguredHotkey.
    // removeEventListener MUST carry the same capture flag or the cleanup
    // won't actually detach.
    document.addEventListener("keydown", handleKeyDown, { capture: true });
    document.addEventListener("keyup", handleKeyUp, { capture: true });
    return () => {
      document.removeEventListener("keydown", handleKeyDown, { capture: true });
      document.removeEventListener("keyup", handleKeyUp, { capture: true });
    };
  }, [activate]);

  return { visible, items };
}
