import { useTerminalStore } from "@/stores/terminalStore";
import { useConfiguredHotkey } from "./useConfiguredHotkey";

// Mirrors DefaultHotkeyCopyMode in services/config/config.go.
export const DEFAULT_COPY_MODE_HOTKEY = "Ctrl+Shift+C";

/**
 * In-app hotkey that opens the Copy Mode overlay — a frozen, selectable mirror
 * of the active terminal's buffer.
 *
 * Why this exists: a full-screen TUI (Hermes, opencode, vim, htop) switches
 * xterm.js to the alternate screen buffer, which has no scrollback, and turns
 * on mouse tracking so click-drag is forwarded to the app instead of selecting
 * text. That is standard terminal behavior, not a PairAdmin bug — but it leaves
 * the user with no way to get the text out. Copy Mode is the workaround: freeze
 * what xterm currently retains and let the browser's own selection + copy
 * handle it.
 *
 * Ctrl+Shift+C rather than Ctrl+C because plain Ctrl+C is SIGINT and the shell
 * must keep owning it.
 */
export function useCopyModeHotkey() {
  useConfiguredHotkey(DEFAULT_COPY_MODE_HOTKEY, "HotkeyCopyMode", () => {
    useTerminalStore.getState().setCopyModeOpen(true);
  });
}
