import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

export type ConnectionStatus = "checking" | "connected" | "disconnected";

interface SettingsState {
  activeModel: string; // "provider:model" display string
  settingsOpen: boolean; // modal open state
  connectionStatus: ConnectionStatus;
  setActiveModel: (model: string) => void;
  setSettingsOpen: (open: boolean) => void;
  setConnectionStatus: (status: ConnectionStatus) => void;
}

export const useSettingsStore = create<SettingsState>()(
  devtools(
    immer((set) => ({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "checking",
      setActiveModel: (model) => {
        set((state) => {
          state.activeModel = model;
        });
      },
      setSettingsOpen: (open) => {
        set((state) => {
          state.settingsOpen = open;
        });
      },
      setConnectionStatus: (status) => {
        set((state) => {
          state.connectionStatus = status;
        });
      },
    })),
    { name: "settings-store" }
  )
);
