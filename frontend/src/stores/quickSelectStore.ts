import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

// Per-row quick-select signage, published by useQuickSelect on chord
// activation and cleared on release. CommandCard / TerminalTab read their own
// id's label here and render the "F1".."F12" badge inline on the row — so a
// badge sits ON its card, not bunched in a fixed corner column.
interface QuickSelectState {
  visible: boolean;
  // command id -> "F1".."F12" (pinned commands, display order)
  commandFkeys: Record<string, string>;
  // terminal tab id -> "F1".."F12" (tabs, display order)
  terminalFkeys: Record<string, string>;
  setBadges: (
    visible: boolean,
    commandFkeys: Record<string, string>,
    terminalFkeys: Record<string, string>
  ) => void;
  clear: () => void;
}

export const useQuickSelectStore = create<QuickSelectState>()(
  devtools(
    immer((set) => ({
      visible: false,
      commandFkeys: {},
      terminalFkeys: {},
      setBadges: (visible, commandFkeys, terminalFkeys) => {
        set((state) => {
          state.visible = visible;
          state.commandFkeys = commandFkeys;
          state.terminalFkeys = terminalFkeys;
        });
      },
      clear: () => {
        set((state) => {
          state.visible = false;
          state.commandFkeys = {};
          state.terminalFkeys = {};
        });
      },
    })),
    { name: "quick-select-store" }
  )
);

// Imperative helpers for useQuickSelect's event-handler context (no hook
// subscriptions there — getState()-style access, same as the other stores).
export function setQuickSelectBadges(
  visible: boolean,
  commandFkeys: Record<string, string>,
  terminalFkeys: Record<string, string>
) {
  useQuickSelectStore.getState().setBadges(visible, commandFkeys, terminalFkeys);
}

export function clearQuickSelectBadges() {
  useQuickSelectStore.getState().clear();
}
