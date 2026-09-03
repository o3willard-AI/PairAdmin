import { useTerminalStore } from "@/stores/terminalStore";
import { useConfiguredHotkey } from "./useConfiguredHotkey";

// Mirrors DefaultHotkeyNewTerminal in services/config/config.go.
export const DEFAULT_NEW_TERMINAL_HOTKEY = "Ctrl+Shift+N";

/**
 * In-app hotkey that opens the "+ Connect" terminal dialog — for a user who
 * already knows the interface, opening a new session shouldn't require a
 * mouse trip all the way to the bottom of the terminal list.
 */
export function useNewTerminalHotkey() {
  useConfiguredHotkey(DEFAULT_NEW_TERMINAL_HOTKEY, "HotkeyNewTerminal", () => {
    useTerminalStore.getState().setNewTerminalDialogOpen(true);
  });
}
