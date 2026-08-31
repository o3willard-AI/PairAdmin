import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";
import type { Terminal } from "@xterm/xterm";

export interface TerminalTab {
  id: string;
  name: string;
  degraded?: boolean;
  degradedMsg?: string;
  /** Connection kind, purely for tab-list decoration (icon/badge) — the "ssh:"/"winrm:"
   * tabId prefix is what actually drives backend/event routing, not this field. */
  kind?: "local" | "ssh" | "winrm";
  /** ID of the config.RemoteHost this tab was opened from (reconnect, or a fresh
   * connection that was saved), if any. When set, renaming this tab also persists
   * the new name onto that saved host record via RemoteService.RenameRemoteHost,
   * so future reconnects use the friendly name instead of reverting to
   * "username@host". Absent for local tabs and unsaved remote connections. */
  savedHostId?: string;
}

interface TerminalState {
  tabs: TerminalTab[];
  activeTabId: string;
  nextTabNumber: number;
  /** "+ New" terminal dialog open state — lifted out of TerminalTabList's own
   * local state so useNewTerminalHotkey.ts can open it from anywhere. */
  newTerminalDialogOpen: boolean;
  setNewTerminalDialogOpen: (open: boolean) => void;
  setActiveTab: (tabId: string) => void;
  addTab: (
    id: string,
    name: string,
    degraded?: boolean,
    degradedMsg?: string,
    kind?: "local" | "ssh" | "winrm",
    savedHostId?: string
  ) => void;
  removeTab: (id: string) => void;
  renameTab: (id: string, name: string) => void;
  clearTabs: () => void;
  setTermRef: (tabId: string, term: Terminal | null) => void;
  getTermRef: (tabId: string) => Terminal | undefined;
  /** Returns a number guaranteed unused by any current or past tab in this
   * session, for use in auto-generated tab names like "Terminal N". Unlike
   * tabs.length, this never gets reused after a tab is closed. */
  takeNextTabNumber: () => number;
}

// Outside the store — NOT in Zustand state (xterm objects are not serializable)
const termRefsMap = new Map<string, Terminal>();

export const useTerminalStore = create<TerminalState>()(
  devtools(
    immer((set) => ({
      tabs: [],
      activeTabId: "",
      nextTabNumber: 1,
      newTerminalDialogOpen: false,
      setNewTerminalDialogOpen: (open) => {
        set((state) => {
          state.newTerminalDialogOpen = open;
        });
      },
      setActiveTab: (tabId) => {
        set((state) => {
          state.activeTabId = tabId;
        });
      },
      addTab: (id, name, degraded, degradedMsg, kind, savedHostId) => {
        set((state) => {
          if (state.tabs.some((t) => t.id === id)) return; // duplicate guard
          state.tabs.push({ id, name, degraded, degradedMsg, kind, savedHostId });
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
      renameTab: (id, name) => {
        set((state) => {
          const tab = state.tabs.find((t) => t.id === id);
          if (tab) tab.name = name;
        });
      },
      clearTabs: () => {
        set((state) => {
          state.tabs = [];
          state.activeTabId = "";
        });
        termRefsMap.clear();
      },
      setTermRef: (tabId, term) => {
        if (term) {
          termRefsMap.set(tabId, term);
        } else {
          termRefsMap.delete(tabId);
        }
      },
      getTermRef: (tabId) => {
        return termRefsMap.get(tabId);
      },
      takeNextTabNumber: () => {
        let n = 0;
        set((state) => {
          n = state.nextTabNumber;
          state.nextTabNumber += 1;
        });
        return n;
      },
    })),
    { name: "terminal-store" }
  )
);
