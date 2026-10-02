import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useCopyModeHotkey, DEFAULT_COPY_MODE_HOTKEY } from "@/hooks/useCopyModeHotkey";
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

describe("useCopyModeHotkey", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    useTerminalStore.setState({
      tabs: [{ id: "tab-1", name: "main" }],
      activeTabId: "tab-1",
      copyModeOpen: false,
    });
    getSettings.mockResolvedValue({});
  });

  afterEach(() => {
    document.body.innerHTML = "";
  });

  // Mutation check: deleting the setCopyModeOpen(true) call inside
  // useCopyModeHotkey leaves copyModeOpen false here — this asserts STORE STATE,
  // not that some function was called, per AGENTS.md §5.
  it("sets copyModeOpen true on the default combo with the terminal focused", async () => {
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useCopyModeHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "C", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();

    expect(useTerminalStore.getState().copyModeOpen).toBe(true);
    unmount();
  });

  // Mutation check: hard-coding the combo literal in useConfiguredHotkey's
  // configKey lookup (i.e. ignoring the user's configured value) breaks this.
  it("uses a hotkey combo loaded from settings instead of the default", async () => {
    getSettings.mockResolvedValue({ HotkeyCopyMode: "Ctrl+Alt+G" });
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useCopyModeHotkey());
    await flushMicrotasks();

    // The default combo must stop firing once a custom one is loaded.
    dispatchKeydown({ key: "C", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();
    expect(useTerminalStore.getState().copyModeOpen).toBe(false);

    dispatchKeydown({ key: "g", ctrlKey: true, altKey: true });
    await flushMicrotasks();
    expect(useTerminalStore.getState().copyModeOpen).toBe(true);
    unmount();
  });

  // Mutation check: dropping the isForeignTextEntry guard (or exempting the
  // terminal textarea wrongly) opens Copy Mode while the user is typing into the
  // chat composer.
  it("does not fire while focus is in an unrelated text input", async () => {
    fakeTermWithTextarea();
    const input = document.createElement("input");
    input.type = "text";
    document.body.appendChild(input);
    input.focus();

    const { unmount } = renderHook(() => useCopyModeHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "C", ctrlKey: true, shiftKey: true });
    await flushMicrotasks();

    expect(useTerminalStore.getState().copyModeOpen).toBe(false);
    unmount();
  });

  // The whole point of the binding: plain Ctrl+C must keep reaching the shell as
  // SIGINT. If the hook ever claimed bare Ctrl+C, this would open the overlay.
  it("does not claim plain Ctrl+C, which must stay SIGINT", async () => {
    const { textarea } = fakeTermWithTextarea();
    useTerminalStore.getState().setTermRef("tab-1", { textarea } as unknown as Terminal);
    textarea.focus();

    const { unmount } = renderHook(() => useCopyModeHotkey());
    await flushMicrotasks();

    dispatchKeydown({ key: "c", ctrlKey: true });
    await flushMicrotasks();

    expect(useTerminalStore.getState().copyModeOpen).toBe(false);
    unmount();
  });

  // Mutation check: changing the const (e.g. to Ctrl+C) fails this. It pins the
  // documented default AND the mirror of DefaultHotkeyCopyMode in config.go.
  it("exports the documented default combo", () => {
    expect(DEFAULT_COPY_MODE_HOTKEY).toBe("Ctrl+Shift+C");
  });
});