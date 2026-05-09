import { useEffect } from "react";
import { useTerminalStore } from "@/stores/terminalStore";

interface TerminalUpdatePayload {
  paneId: string;
  content: string;
}

interface TabInfo {
  id: string;
  name: string;
}

interface TerminalTabsPayload {
  tabs: TabInfo[];
}

export function useTerminalCapture() {
  useEffect(() => {
    let unsubUpdate: (() => void) | null = null;
    let unsubTabs: (() => void) | null = null;
    let unsubPtyClosed: (() => void) | null = null;

    import(/* @vite-ignore */ "../../wailsjs/runtime/runtime").then((rt) => {
      unsubTabs = rt.EventsOn(
        "terminal:tabs",
        ((event: TerminalTabsPayload) => {
          const store = useTerminalStore.getState();
          const currentIds = new Set(store.tabs.map((t) => t.id));
          const newIds = new Set(event.tabs.map((t) => t.id));

          // Remove tabs no longer present
          for (const id of currentIds) {
            if (!newIds.has(id)) {
              store.removeTab(id);
            }
          }

          // Add new tabs
          for (const tab of event.tabs) {
            if (!currentIds.has(tab.id)) {
              store.addTab(tab.id, tab.name);
            }
          }
        }) as (...args: unknown[]) => void
      );

      unsubUpdate = rt.EventsOn(
        "terminal:update",
        ((event: TerminalUpdatePayload) => {
          const term = useTerminalStore.getState().getTermRef(event.paneId);
          if (!term) return; // tab already removed
          // Use ANSI escape sequences for double buffering to prevent flickering.
          // \x1b[?7l disables line wrapping so long lines don't push the terminal down.
          // \x1b[H moves cursor to top left.
          // \x1b[K clears from cursor to end of line.
          // \x1b[J clears from cursor to bottom of screen.
          const formatted = event.content.trimEnd().split('\n').join('\x1b[K\r\n');
          term.write("\x1b[?7l\x1b[H" + formatted + "\x1b[K\x1b[J");
        }) as (...args: unknown[]) => void
      );

      unsubPtyClosed = rt.EventsOn(
        "pty:closed",
        ((event: { tabId: string }) => {
          useTerminalStore.getState().removeTab(event.tabId);
        }) as (...args: unknown[]) => void
      );
    });

    return () => {
      unsubUpdate?.();
      unsubTabs?.();
      unsubPtyClosed?.();
    };
  }, []);
}
