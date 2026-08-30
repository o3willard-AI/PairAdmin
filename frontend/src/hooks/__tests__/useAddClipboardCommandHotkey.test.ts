import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import {
  useAddClipboardCommandHotkey,
  DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY,
} from "@/hooks/useAddClipboardCommandHotkey";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
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

// The mount effect chains a dynamic import() -> GetSettings() -> comboRef
// assignment, and the hotkey handler itself chains a clipboard.readText()
// promise. Vite's dynamic import() resolves via a macrotask in this test
// environment, not a plain microtask, so draining only Promise.resolve()
// ticks never catches up — a real setTimeout flush is needed instead.
async function flushMicrotasks(times = 4) {
  for (let i = 0; i < times; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

describe("useAddClipboardCommandHotkey", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    useTerminalStore.setState({ tabs: [{ id: "tab-1", name: "main" }], activeTabId: "tab-1" });
    useCommandStore.setState({ commands: [] });
    getSettings.mockResolvedValue({});
    Object.assign(navigator, {
      clipboard: { readText: vi.fn().mockResolvedValue("echo from-clipboard") },
    });
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("adds the clipboard contents as a new command when the default combo fires with the terminal focused", async () => {
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useAddClipboardCommandHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "A", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();

    expect(useCommandStore.getState().commands).toHaveLength(1);
    expect(useCommandStore.getState().commands[0].command).toBe("echo from-clipboard");
    unmount();
  });

  it("uses a hotkey combo loaded from settings instead of the default", async () => {
    getSettings.mockResolvedValue({ HotkeyAddClipboardCommand: "Ctrl+Alt+X" });
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useAddClipboardCommandHotkey());
    await flushMicrotasks();

    // The default combo must no longer fire once a custom one is loaded.
    dispatchKeydown({ key: "A", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();
    expect(useCommandStore.getState().commands).toHaveLength(0);

    dispatchKeydown({ key: "x", ctrlKey: true, altKey: true });
    await flushMicrotasks();
    expect(useCommandStore.getState().commands).toHaveLength(1);
    unmount();
  });

  it("does not fire while focus is in an unrelated text input", async () => {
    fakeTermWithTextarea();
    const input = document.createElement("input");
    input.type = "text";
    document.body.appendChild(input);
    input.focus();

    const { unmount } = renderHook(() => useAddClipboardCommandHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "A", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();

    expect(useCommandStore.getState().commands).toHaveLength(0);
    unmount();
  });

  it("does not add a command when the clipboard is empty", async () => {
    Object.assign(navigator, { clipboard: { readText: vi.fn().mockResolvedValue("   ") } });
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useAddClipboardCommandHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "A", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();

    expect(useCommandStore.getState().commands).toHaveLength(0);
    unmount();
  });

  it("exports the documented default combo", () => {
    expect(DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY).toBe("Ctrl+Shift+A");
  });
});
