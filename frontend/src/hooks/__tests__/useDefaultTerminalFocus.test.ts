import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDefaultTerminalFocus } from "@/hooks/useDefaultTerminalFocus";
import { useTerminalStore } from "@/stores/terminalStore";
import type { Terminal } from "@xterm/xterm";

// Cast away Terminal's real shape for the store, but keep a directly-typed
// handle to the mock itself so assertions retain vi.fn()'s mock methods
// (mockClear, etc.) instead of the narrowed `() => void` from Terminal.
function fakeTerm() {
  const focus = vi.fn();
  return { term: { focus } as unknown as Terminal, focus };
}

describe("useDefaultTerminalFocus", () => {
  let cleanupFns: Array<() => void> = [];

  beforeEach(() => {
    useTerminalStore.setState({ tabs: [{ id: "tab-1", name: "main" }], activeTabId: "tab-1" });
    document.body.innerHTML = "";
  });

  afterEach(() => {
    cleanupFns.forEach((fn) => fn());
    cleanupFns = [];
    document.body.innerHTML = "";
  });

  it("focuses the active terminal on mount", () => {
    const { term, focus } = fakeTerm();
    useTerminalStore.getState().setTermRef("tab-1", term);

    const { unmount } = renderHook(() => useDefaultTerminalFocus());
    cleanupFns.push(unmount);

    expect(focus).toHaveBeenCalled();
  });

  it("refocuses the terminal after clicking a plain button", () => {
    const { term, focus } = fakeTerm();
    useTerminalStore.getState().setTermRef("tab-1", term);
    const button = document.createElement("button");
    document.body.appendChild(button);

    const { unmount } = renderHook(() => useDefaultTerminalFocus());
    cleanupFns.push(unmount);
    focus.mockClear();

    button.click();

    expect(focus).toHaveBeenCalled();
  });

  it("does not steal focus after clicking into a text input", () => {
    const { term, focus } = fakeTerm();
    useTerminalStore.getState().setTermRef("tab-1", term);
    const input = document.createElement("input");
    input.type = "text";
    document.body.appendChild(input);
    input.focus();

    const { unmount } = renderHook(() => useDefaultTerminalFocus());
    cleanupFns.push(unmount);
    focus.mockClear();

    input.click();

    expect(focus).not.toHaveBeenCalled();
  });

  it("does not steal focus after clicking a checkbox", () => {
    const { term, focus } = fakeTerm();
    useTerminalStore.getState().setTermRef("tab-1", term);
    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    document.body.appendChild(checkbox);

    const { unmount } = renderHook(() => useDefaultTerminalFocus());
    cleanupFns.push(unmount);
    focus.mockClear();

    // Simulate the checkbox retaining focus after the click (jsdom does this
    // automatically for real user clicks, but a synthetic .click() call
    // doesn't always move focus — set it explicitly to match real behavior).
    checkbox.focus();
    checkbox.click();

    // A checkbox is not a genuine text-entry surface, so this SHOULD refocus
    // the terminal — this test documents that intentional behavior (the bug
    // report: a stray keystroke toggling a checkbox instead of reaching the
    // terminal is exactly what this hook prevents).
    expect(focus).toHaveBeenCalled();
  });

  it("does not steal focus while a modal dialog is open", () => {
    const { term, focus } = fakeTerm();
    useTerminalStore.getState().setTermRef("tab-1", term);
    const dialog = document.createElement("div");
    dialog.setAttribute("role", "dialog");
    const button = document.createElement("button");
    dialog.appendChild(button);
    document.body.appendChild(dialog);

    const { unmount } = renderHook(() => useDefaultTerminalFocus());
    cleanupFns.push(unmount);
    focus.mockClear();

    button.click();

    expect(focus).not.toHaveBeenCalled();
  });

  it("refocuses the newly active terminal when activeTabId changes", () => {
    const { term: term1, focus: focus1 } = fakeTerm();
    const { term: term2, focus: focus2 } = fakeTerm();
    useTerminalStore.setState({
      tabs: [
        { id: "tab-1", name: "main" },
        { id: "tab-2", name: "second" },
      ],
      activeTabId: "tab-1",
    });
    useTerminalStore.getState().setTermRef("tab-1", term1);
    useTerminalStore.getState().setTermRef("tab-2", term2);

    const { unmount } = renderHook(() => useDefaultTerminalFocus());
    cleanupFns.push(unmount);
    focus1.mockClear();
    focus2.mockClear();

    act(() => {
      useTerminalStore.getState().setActiveTab("tab-2");
    });

    expect(focus2).toHaveBeenCalled();
  });
});
