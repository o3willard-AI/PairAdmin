import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";
import type { Terminal } from "@xterm/xterm";

export interface TerminalTab {
  id: string;
  name: string;
}

interface TerminalState {
  tabs: TerminalTab[];
  activeTabId: string;
  setActiveTab: (tabId: string) => void;
  setTermRef: (tabId: string, term: Terminal) => void;
  getTermRef: (tabId: string) => Terminal | undefined;
}

// Outside the store — NOT in Zustand state (xterm objects are not serializable)
const termRefsMap = new Map<string, Terminal>();

export const useTerminalStore = create<TerminalState>()(
  devtools(
    immer((set) => ({
      tabs: [
        { id: "bash-1", name: "bash:1" },
        { id: "bash-2", name: "bash:2" },
      ],
      activeTabId: "bash-1",
      setActiveTab: (tabId) => {
        set((state) => {
          state.activeTabId = tabId;
        });
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
