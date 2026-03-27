import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

export interface TerminalTab {
  id: string;
  name: string;
}

interface TerminalState {
  tabs: TerminalTab[];
  activeTabId: string;
  setActiveTab: (tabId: string) => void;
}

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
    })),
    { name: "terminal-store" }
  )
);
