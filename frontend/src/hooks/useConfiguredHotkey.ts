import { useEffect, useRef } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { parseHotkey, matchesHotkey, isForeignTextEntry } from "@/utils/hotkey";
import type { config } from "../../wailsjs/go/models";

/**
 * Shared plumbing for every in-app hotkey (window must be focused; not a
 * global OS hotkey) that's configurable in Settings → Hotkeys. Reads the
 * configured combo once on mount (falling back to defaultCombo if unset);
 * changing it in Settings takes effect on next launch. Fires onMatch()
 * unless focus is in a genuine text-entry element elsewhere in the app (the
 * active terminal's own hidden textarea is exempted — see isForeignTextEntry).
 *
 * onMatch is read from a ref internally, so callers don't need to memoize it
 * with useCallback — the keydown listener itself is only attached once.
 */
export function useConfiguredHotkey(
  defaultCombo: string,
  configKey: keyof config.AppConfig,
  onMatch: (event: KeyboardEvent) => void
) {
  const comboRef = useRef(defaultCombo);
  const onMatchRef = useRef(onMatch);
  onMatchRef.current = onMatch;

  useEffect(() => {
    import(/* @vite-ignore */ "../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => {
        const value = cfg?.[configKey];
        if (typeof value === "string" && value) comboRef.current = value;
      })
      .catch(() => {});
  }, [configKey]);

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
      onMatchRef.current(event);
    };

    // Capture phase: run before xterm's own attachCustomKeyEventHandler (which
    // listens on its textarea, not the document) so the hotkey wins even
    // while the terminal has focus, instead of the shell also seeing it.
    document.addEventListener("keydown", handler, { capture: true });
    return () => document.removeEventListener("keydown", handler, { capture: true });
  }, []);
}
