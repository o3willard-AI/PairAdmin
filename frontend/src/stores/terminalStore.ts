import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";
import type { Terminal } from "@xterm/xterm";

export interface TerminalTab {
  id: string;
  name: string;
  degraded?: boolean;
  degradedMsg?: string;
}

interface TerminalState {
  tabs: TerminalTab[];
  activeTabId: string;
  setActiveTab: (tabId: string) => void;
  addTab: (id: string, name: string, degraded?: boolean, degradedMsg?: string) => void;
  removeTab: (id: string) => void;
  clearTabs: () => void;
  setTermRef: (tabId: string, term: Terminal) => void;
  getTermRef: (tabId: string) => Terminal | undefined;
}

// Outside the store — NOT in Zustand state (xterm objects are not serializable)
const termRefsMap = new Map<string, Terminal>();

export const useTerminalStore = create<TerminalState>()(
  devtools(
    immer((set) => ({
      tabs: [],
      activeTabId: "",
      setActiveTab: (tabId) => {
        set((state) => {
          state.activeTabId = tabId;
        });
      },
      addTab: (id, name, degraded, degradedMsg) => {
        set((state) => {
          if (state.tabs.some((t) => t.id === id)) return; // duplicate guard
          state.tabs.push({ id, name, degraded, degradedMsg });
          if (state.tabs.length === 1 && !degraded) {
            state.activeTabId = id; // first non-degraded tab becomes active
          }
        });
      },
      removeTab: (id) => {
        set((state) => {
          state.tabs = state.tabs.filter((t) => t.id !== id);
          if (state.activeTabId === id) {
            state.activeTabId = state.tabs.length > 0 ? state.tabs[0].id : "";
          }
        });
        termRefsMap.delete(id); // cleanup xterm ref
      },
      clearTabs: () => {
        set((state) => {
          state.tabs = [];
          state.activeTabId = "";
        });
        termRefsMap.clear();
      },
      setTermRef: (tabId, term) => {
        termRefsMap.set(tabId, term);
      },
      getTermRef: (tabId) => {
        return termRefsMap.get(tabId);
      },
    })),
    { name: "terminal-store" }
  )
);
