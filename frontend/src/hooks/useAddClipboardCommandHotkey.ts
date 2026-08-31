import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { useConfiguredHotkey } from "./useConfiguredHotkey";

// Mirrors DefaultHotkeyAddClipboardCommand in services/config/config.go.
export const DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY = "Ctrl+Shift+A";

/**
 * In-app hotkey that reads the current clipboard contents and adds them as a
 * new sidebar command — the fast path for saving a command the user derived
 * directly in a terminal, without opening the "Add Command" dialog.
 */
export function useAddClipboardCommandHotkey() {
  useConfiguredHotkey(DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY, "HotkeyAddClipboardCommand", () => {
    const { activeTabId } = useTerminalStore.getState();
    navigator.clipboard
      .readText()
      .then((text) => {
        const trimmed = text.trim();
        if (trimmed) {
          useCommandStore.getState().addCommand(activeTabId, {
            command: trimmed,
            originalQuestion: "",
          });
        }
      })
      .catch(() => {});
  });
}
