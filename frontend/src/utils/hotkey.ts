export interface ParsedHotkey {
  ctrl: boolean;
  shift: boolean;
  alt: boolean;
  meta: boolean;
  key: string;
}

// Mirrors the combo string format produced by HotkeysTab.tsx's buildKeyCombo:
// modifiers in Ctrl/Shift/Alt/Meta order, joined with "+", then the key.
export function parseHotkey(combo: string): ParsedHotkey | null {
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

export function matchesHotkey(event: KeyboardEvent, parsed: ParsedHotkey): boolean {
  return (
    event.ctrlKey === parsed.ctrl &&
    event.shiftKey === parsed.shift &&
    event.altKey === parsed.alt &&
    event.metaKey === parsed.meta &&
    event.key.toLowerCase() === parsed.key
  );
}

// Elements the user is deliberately typing into elsewhere in the app (chat
// box, an open dialog's input/textarea, a settings field) — a configured
// hotkey must not fire there, since the combo may be a meaningful keystroke
// in that context. The active terminal's own hidden xterm.js textarea is
// deliberately excluded from this check: pressing a hotkey while sitting in
// the terminal — the primary use case for all of these — must still work.
export function isForeignTextEntry(
  el: Element | null,
  terminalTextarea: HTMLTextAreaElement | undefined
): boolean {
  if (!el) return false;
  if (el === terminalTextarea) return false;
  if (el.tagName === "TEXTAREA" || el.tagName === "INPUT") return true;
  if ((el as HTMLElement).isContentEditable) return true;
  return false;
}
