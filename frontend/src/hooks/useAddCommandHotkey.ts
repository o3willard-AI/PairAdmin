import { useCommandStore } from "@/stores/commandStore";
import { useConfiguredHotkey } from "./useConfiguredHotkey";

// Mirrors DefaultHotkeyAddCommand in services/config/config.go.
export const DEFAULT_ADD_COMMAND_HOTKEY = "Ctrl+Shift+P";

/**
 * In-app hotkey that opens the "Add Command" dialog in the Commands sidebar
 * — for a user who already knows the interface, saving a command shouldn't
 * require a mouse trip to the bottom of the sidebar.
 */
export function useAddCommandHotkey() {
  useConfiguredHotkey(DEFAULT_ADD_COMMAND_HOTKEY, "HotkeyAddCommand", () => {
    useCommandStore.getState().setAddCommandDialogOpen(true);
  });
}
