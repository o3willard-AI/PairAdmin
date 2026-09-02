import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useQuickSelect, type QuickSelectItem } from "@/hooks/useQuickSelect";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { sendToTerminal } from "@/utils/sendToTerminal";

// sendToTerminal writes into a real PTY via the Wails binding and focuses the
// terminal — both outside the hook's unit scope. Spy on the module so tests
// assert the exact (tabId, text, execute) triple the hook forwards.
vi.mock("@/utils/sendToTerminal", () => ({
  sendToTerminal: vi.fn(),
}));

const mockedSendToTerminal = vi.mocked(sendToTerminal);

// From frontend/src/hooks/__tests__/ the relative path to
// frontend/wailsjs/go/services/PTYService is ../../../wailsjs/... —
// sendToTerminal dynamically imports it, so it must be mocked to keep the
// tests hermetic (no Wails runtime).
vi.mock("../../../wailsjs/go/services/PTYService", () => ({
  WriteInput: vi.fn(() => Promise.resolve()),
}));

function dispatchKey(
  type: "keydown" | "keyup",
  init: Partial<KeyboardEventInit> & { key: string }
): KeyboardEvent {
  const event = new KeyboardEvent(type, { bubbles: true, cancelable: true, ...init });
  document.dispatchEvent(event);
  return event;
}

function seedState(opts?: {
  commands?: Array<{ id: string; command: string; pinned: boolean }>;
  tabs?: Array<{ id: string; name: string }>;
  activeTabId?: string;
}) {
  useCommandStore.setState({
    commands: (opts?.commands ?? []).map((c) => ({
      id: c.id,
      command: c.command,
      originalQuestion: "",
      timestamp: 0,
      tabId: "seed",
      pinned: c.pinned,
    })),
  });
  useTerminalStore.setState({
    tabs: opts?.tabs ?? [],
    activeTabId: opts?.activeTabId ?? "",
  });
}

// Convenience: activate the chord, then release it.
function chordDown(extra?: { metaKey?: boolean; altKey?: boolean }) {
  return dispatchKey("keydown", {
    key: "Control",
    ctrlKey: true,
    altKey: extra?.altKey ?? true,
    metaKey: extra?.metaKey ?? false,
  });
}

describe("useQuickSelect", () => {
  beforeEach(() => {
    mockedSendToTerminal.mockClear();
    seedState();
  });

  it("exposes QuickSelectItem shapes with labels F1..Fn", async () => {
    seedState({
      commands: [
        { id: "c1", command: "echo one", pinned: true },
        { id: "c2", command: "echo two", pinned: true },
      ],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    const { result } = renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });

    expect(result.current.visible).toBe(true);
    const expected: QuickSelectItem[] = [
      { label: "F1", kind: "command", id: "c1" },
      { label: "F2", kind: "command", id: "c2" },
      { label: "F3", kind: "terminal", id: "t1" },
    ];
    expect(result.current.items).toEqual(expected);
  });

  it("chord requires Ctrl+Alt or Ctrl+Meta — plain Ctrl does not activate", () => {
    const { result } = renderHook(() => useQuickSelect());

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true });
    });
    expect(result.current.visible).toBe(false);

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, metaKey: true });
    });
    expect(result.current.visible).toBe(true);
  });

  it("an F-key over a command inserts its consumed text into the active terminal without executing", () => {
    seedState({
      commands: [
        { id: "c1", command: "kubectl get pods", pinned: true },
        { id: "unpinned", command: "rm -rf /", pinned: false },
      ],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });
    act(() => {
      dispatchKey("keydown", { key: "F1", ctrlKey: true, altKey: true });
    });

    // Insertion (execute=false), on the active tab, with the consumed text
    expect(mockedSendToTerminal).toHaveBeenCalledTimes(1);
    expect(mockedSendToTerminal).toHaveBeenCalledWith("t1", "kubectl get pods", false);
  });

  it("an F-key over a terminal switches to that tab", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [
        { id: "t1", name: "main" },
        { id: "t2", name: "build" },
      ],
      activeTabId: "t1",
    });
    renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });
    // F1 → command c1; F2 → terminal t1; F3 → terminal t2
    act(() => {
      dispatchKey("keydown", { key: "F3", ctrlKey: true, altKey: true });
    });

    expect(useTerminalStore.getState().activeTabId).toBe("t2");
    expect(mockedSendToTerminal).not.toHaveBeenCalled();
  });

  it("caps the item list at 12 entries", () => {
    seedState({
      commands: Array.from({ length: 10 }, (_, i) => ({
        id: `c${i}`,
        command: `cmd ${i}`,
        pinned: true,
      })),
      tabs: Array.from({ length: 10 }, (_, i) => ({ id: `t${i}`, name: `tab ${i}` })),
      activeTabId: "t0",
    });
    const { result } = renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });

    expect(result.current.items).toHaveLength(12);
    expect(result.current.items[11].label).toBe("F12");
  });

  it("an F-key beyond the item count does nothing (but is still preventDefault'ed)", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [],
    });
    renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });
    let event: KeyboardEvent;
    act(() => {
      event = dispatchKey("keydown", { key: "F5", ctrlKey: true, altKey: true });
    });

    expect(mockedSendToTerminal).not.toHaveBeenCalled();
    expect(useTerminalStore.getState().activeTabId).toBe("");
    expect(event!.defaultPrevented).toBe(true);
  });

  it("prevents the default action for F-keys while the chord is active", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });
    const event = dispatchKey("keydown", { key: "F11", ctrlKey: true, altKey: true });
    expect(event.defaultPrevented).toBe(true);
  });

  it("does not preventDefault F-keys when the chord is NOT active", () => {
    renderHook(() => useQuickSelect());
    const event = dispatchKey("keydown", { key: "F5" });
    expect(event.defaultPrevented).toBe(false);
  });

  it("releasing Ctrl ends quick-select (visible false)", () => {
    const { result } = renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });
    expect(result.current.visible).toBe(true);

    act(() => {
      dispatchKey("keyup", { key: "Control", ctrlKey: false, altKey: true });
    });
    expect(result.current.visible).toBe(false);
  });

  it("releasing both Alt and Meta (Ctrl still held) ends quick-select", () => {
    const { result } = renderHook(() => useQuickSelect());

    // Activate with Ctrl+Meta chord
    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, metaKey: true });
    });
    expect(result.current.visible).toBe(true);

    act(() => {
      dispatchKey("keyup", { key: "Meta", ctrlKey: true, metaKey: false });
    });
    // Meta released, Alt was never held → chord broken
    expect(result.current.visible).toBe(false);
  });

  it("releasing only Alt while Meta is still held keeps quick-select active", () => {
    const { result } = renderHook(() => useQuickSelect());

    act(() => {
      dispatchKey("keydown", {
        key: "Control",
        ctrlKey: true,
        altKey: true,
        metaKey: true,
      });
    });
    expect(result.current.visible).toBe(true);

    act(() => {
      dispatchKey("keyup", { key: "Alt", ctrlKey: true, metaKey: true });
    });
    expect(result.current.visible).toBe(true);
  });

  it("removes document listeners on unmount (no stale activation or insertion)", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    const addSpy = vi.spyOn(document, "addEventListener");
    const removeSpy = vi.spyOn(document, "removeEventListener");
    const { unmount } = renderHook(() => useQuickSelect());

    expect(addSpy).toHaveBeenCalledWith("keydown", expect.any(Function));
    expect(addSpy).toHaveBeenCalledWith("keyup", expect.any(Function));

    unmount();

    expect(removeSpy).toHaveBeenCalledWith("keydown", expect.any(Function));
    expect(removeSpy).toHaveBeenCalledWith("keyup", expect.any(Function));

    // Events after unmount must be ignored entirely
    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, altKey: true });
    });
    expect(mockedSendToTerminal).not.toHaveBeenCalled();
    addSpy.mockRestore();
    removeSpy.mockRestore();
  });

  it("recomputes the item list on each activation (fresh store state)", () => {
    seedState({
      commands: [{ id: "c1", command: "echo first", pinned: true }],
      tabs: [],
    });
    const { result } = renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });
    expect(result.current.items).toEqual([{ label: "F1", kind: "command", id: "c1" }]);

    act(() => {
      dispatchKey("keyup", { key: "Control", ctrlKey: false });
    });

    seedState({
      commands: [{ id: "c9", command: "echo second", pinned: true }],
      tabs: [{ id: "t7", name: "new" }],
    });
    act(() => {
      chordDown();
    });
    expect(result.current.items).toEqual([
      { label: "F1", kind: "command", id: "c9" },
      { label: "F2", kind: "terminal", id: "t7" },
    ]);
  });
});
