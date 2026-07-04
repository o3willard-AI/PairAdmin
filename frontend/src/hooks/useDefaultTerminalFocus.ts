import { useEffect } from "react";
import { useTerminalStore } from "@/stores/terminalStore";

const TEXT_ENTRY_INPUT_TYPES = new Set([
  "text",
  "password",
  "number",
  "search",
  "email",
  "url",
  "tel",
  "date",
  "datetime-local",
  "month",
  "time",
  "week",
]);

// Genuine typing surfaces the user deliberately clicked into — leave focus
// alone. Everything else (buttons, checkboxes, tab list items, command
// cards, the document body) is fair game to reclaim for the terminal.
// xterm.js's own hidden input is itself a <textarea>, so focus already
// correctly on the terminal is a no-op here, not something this excludes.
function isTextEntryElement(el: Element | null): boolean {
  if (!el) return false;
  if (el.tagName === "TEXTAREA") return true;
  if ((el as HTMLElement).isContentEditable) return true;
  if (el.tagName === "INPUT") {
    return TEXT_ENTRY_INPUT_TYPES.has((el as HTMLInputElement).type.toLowerCase());
  }
  return false;
}

// base-ui's Dialog (via Floating UI's useRole) renders role="dialog" on its
// Popup only while open — used to avoid fighting a modal's own focus
// management (e.g. Tab-navigating its fields) while one is open. The
// asynchronous-close race (dialog closes after an awaited network call,
// restoring focus to its trigger button) isn't covered here — that needs an
// explicit refocus at each dialog's onClose call site instead, since by the
// time it closes there's no new click event for this listener to observe.
function isDialogOpen(): boolean {
  return document.querySelector('[role="dialog"]') !== null;
}

function refocusTerminal() {
  if (isDialogOpen()) return;
  if (isTextEntryElement(document.activeElement)) return;
  const { activeTabId, getTermRef } = useTerminalStore.getState();
  getTermRef(activeTabId)?.focus();
}

/**
 * The terminal is the app's primary surface — the user is almost always
 * about to type a command there. Left to the browser's default behavior,
 * keyboard focus stays wherever the last click landed (a Pin button, a tab
 * list item, "+ New"), so the next keystroke silently re-triggers that
 * element (toggling a checkbox, re-opening a dialog) instead of reaching the
 * terminal. This restores focus to the active terminal on a fresh session,
 * whenever the active tab changes, and after any click that isn't on a
 * genuine text-entry control and isn't inside an open modal dialog.
 */
export function useDefaultTerminalFocus() {
  const activeTabId = useTerminalStore((state) => state.activeTabId);

  useEffect(() => {
    refocusTerminal();
  }, [activeTabId]);

  useEffect(() => {
    // Bubble phase: let the clicked element's own handler (rename, pin,
    // reconnect, etc.) run first, then reclaim focus once it's done.
    document.addEventListener("click", refocusTerminal);
    return () => document.removeEventListener("click", refocusTerminal);
  }, []);
}
