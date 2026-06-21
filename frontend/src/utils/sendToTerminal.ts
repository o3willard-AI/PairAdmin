import { useTerminalStore } from "@/stores/terminalStore";

// Writes text directly into a terminal's PTY (xterm-equivalent of "paste"),
// rather than going through the OS clipboard — avoids platform-specific
// clipboard mechanisms entirely (e.g. on Windows, OS clipboard access shells
// out to powershell/clip.exe, which can flash a visible console window).
export function sendToTerminal(tabId: string, text: string, execute: boolean) {
  // Terminals submit a line on carriage return ("\r"), not "\n" — writing
  // "\n" alone just inserts a newline character without triggering execution.
  import(/* @vite-ignore */ "../../wailsjs/go/services/PTYService")
    .then(({ WriteInput }) => WriteInput(tabId, execute ? text + "\r" : text))
    .catch(() => {});
  // Move focus to the terminal so a subsequent Enter keypress is sent to
  // the shell instead of re-triggering whatever previously had DOM focus.
  useTerminalStore.getState().getTermRef(tabId)?.focus();
}
