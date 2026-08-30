import { useEffect, useRef } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";

// Mirrors DefaultHotkeyAddClipboardCommand in services/config/config.go.
export const DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY = "Ctrl+Shift+A";

interface ParsedHotkey {
  ctrl: boolean;
  shift: boolean;
  alt: boolean;
  meta: boolean;
  key: string;
}

// Mirrors the combo string format produced by HotkeysTab.tsx's buildKeyCombo:
// modifiers in Ctrl/Shift/Alt/Meta order, joined with "+", then the key.
function parseHotkey(combo: string): ParsedHotkey | null {
  if (!combo) return null;
  const parts = combo.split("+");
  const key = parts[parts.length - 1];
  if (!key) return null;
  const mods = new Set(parts.slice(0, -1));
  return {
    ctrl: mods.has("Ctrl"),
    shift: mods.has("Shift"),
    alt: mods.has("Alt"),
    meta: mods.has("Meta"),
    key: key.toLowerCase(),
  };
}

function matchesHotkey(event: KeyboardEvent, parsed: ParsedHotkey): boolean {
  return (
    event.ctrlKey === parsed.ctrl &&
    event.shiftKey === parsed.shift &&
    event.altKey === parsed.alt &&
    event.metaKey === parsed.meta &&
    event.key.toLowerCase() === parsed.key
  );
}

// Elements the user is deliberately typing into elsewhere in the app (chat
// box, an open dialog's input/textarea, a settings field) — the hotkey must
// not fire there, since the configured combo may be a meaningful keystroke in
// that context. The active terminal's own hidden xterm.js textarea is
// deliberately excluded from this check: pressing the hotkey while sitting in
// the terminal — the primary use case, right after selecting/copying a
// command you just ran — must still work.
function isForeignTextEntry(el: Element | null, terminalTextarea: HTMLTextAreaElement | undefined): boolean {
  if (!el) return false;
  if (el === terminalTextarea) return false;
  if (el.tagName === "TEXTAREA" || el.tagName === "INPUT") return true;
  if ((el as HTMLElement).isContentEditable) return true;
  return false;
}

/**
 * In-app hotkey (window must be focused; not a global OS hotkey) that reads
 * the current clipboard contents and adds them as a new sidebar command —
 * the fast path for saving a command the user derived directly in a
 * terminal, without opening the "Add Command" dialog. Reads the configured
 * combo once on mount; changing it in Settings takes effect on next launch.
 */
export function useAddClipboardCommandHotkey() {
  const comboRef = useRef(DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY);

  useEffect(() => {
    import(/* @vite-ignore */ "../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => {
        if (cfg?.HotkeyAddClipboardCommand) comboRef.current = cfg.HotkeyAddClipboardCommand;
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const parsed = parseHotkey(comboRef.current);
      if (!parsed || !matchesHotkey(event, parsed)) return;

      const { activeTabId, getTermRef } = useTerminalStore.getState();
      const term = getTermRef(activeTabId);
      const terminalTextarea = (term as unknown as { textarea?: HTMLTextAreaElement } | null)
        ?.textarea;
      if (isForeignTextEntry(document.activeElement, terminalTextarea)) return;

      event.preventDefault();
      event.stopPropagation();

      navigator.clipboard
        .readText()
        .then((text) => {
          const trimmed = text.trim();
          if (trimmed) {
            useCommandStore.getState().addCommand(activeTabId, {
              command: trimmed,
              originalQuestion: "",
            });
          }
        })
        .catch(() => {});
    };

    // Capture phase: run before xterm's own attachCustomKeyEventHandler (which
    // listens on its textarea, not the document) so the hotkey wins even
    // while the terminal has focus, instead of the shell also seeing it.
    document.addEventListener("keydown", handler, { capture: true });
    return () => document.removeEventListener("keydown", handler, { capture: true });
  }, []);
}
