import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

interface SettingsState {
  activeModel: string; // "provider:model" display string
  settingsOpen: boolean; // modal open state
  setActiveModel: (model: string) => void;
  setSettingsOpen: (open: boolean) => void;
}

export const useSettingsStore = create<SettingsState>()(
  devtools(
    immer((set) => ({
      activeModel: "",
      settingsOpen: false,
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
    })),
    { name: "settings-store" }
  )
);
