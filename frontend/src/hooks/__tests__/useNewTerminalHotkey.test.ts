import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useNewTerminalHotkey, DEFAULT_NEW_TERMINAL_HOTKEY } from "@/hooks/useNewTerminalHotkey";
import { useTerminalStore } from "@/stores/terminalStore";
import type { Terminal } from "@xterm/xterm";

const getSettings = vi.fn();
// Resolves (from frontend/src/hooks/) to frontend/wailsjs/go/services/SettingsService.
// From this test file (frontend/src/hooks/__tests__/) that is ../../../wailsjs/...
vi.mock("../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
}));

function fakeTermWithTextarea() {
  const textarea = document.createElement("textarea");
  document.body.appendChild(textarea);
  return { term: { textarea } as unknown as Terminal, textarea };
}

function dispatchKeydown(init: Partial<KeyboardEventInit> & { key: string }) {
  const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  document.dispatchEvent(event);
  return event;
}

// See useAddClipboardCommandHotkey.test.ts for why a real setTimeout flush
// is needed instead of draining plain microtasks.
async function flushMicrotasks(times = 4) {
  for (let i = 0; i < times; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

describe("useNewTerminalHotkey", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    useTerminalStore.setState({
      tabs: [{ id: "tab-1", name: "main" }],
      activeTabId: "tab-1",
      newTerminalDialogOpen: false,
    });
    getSettings.mockResolvedValue({});
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("opens the new-terminal dialog when the default combo fires with the terminal focused", async () => {
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useNewTerminalHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "N", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();

    expect(useTerminalStore.getState().newTerminalDialogOpen).toBe(true);
    unmount();
  });

  it("uses a hotkey combo loaded from settings instead of the default", async () => {
    getSettings.mockResolvedValue({ HotkeyNewTerminal: "Ctrl+Alt+Z" });
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useNewTerminalHotkey());
    await flushMicrotasks();

    // The default combo must no longer fire once a custom one is loaded.
    dispatchKeydown({ key: "N", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();
    expect(useTerminalStore.getState().newTerminalDialogOpen).toBe(false);

    dispatchKeydown({ key: "z", ctrlKey: true, altKey: true });
    await flushMicrotasks();
    expect(useTerminalStore.getState().newTerminalDialogOpen).toBe(true);
    unmount();
  });

  it("does not fire while focus is in an unrelated text input", async () => {
    fakeTermWithTextarea();
    const input = document.createElement("input");
    input.type = "text";
    document.body.appendChild(input);
    input.focus();

    const { unmount } = renderHook(() => useNewTerminalHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "N", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();

    expect(useTerminalStore.getState().newTerminalDialogOpen).toBe(false);
    unmount();
  });

  it("exports the documented default combo", () => {
    expect(DEFAULT_NEW_TERMINAL_HOTKEY).toBe("Ctrl+Shift+N");
  });
});
