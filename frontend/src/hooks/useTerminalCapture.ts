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
          if (!term) return; // tab already removed — discard (per RESEARCH.md pitfall 2)
          term.clear();
          term.write(event.content);
        }) as (...args: unknown[]) => void
      );
    });

    return () => {
      unsubUpdate?.();
      unsubTabs?.();
    };
  }, []);
}
