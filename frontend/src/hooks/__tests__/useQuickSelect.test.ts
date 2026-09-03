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

const getSettings = vi.fn();
// useQuickSelect reads the configured chord via GetSettings (same pattern as
// useConfiguredHotkey), so the SettingsService binding must be mocked too.
vi.mock("../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
}));

// The hook's settings load chains dynamic import() -> GetSettings(). Vite's
// dynamic import resolves via a macrotask in this environment (see
// useAddClipboardCommandHotkey.test.ts), so a real setTimeout flush is needed.
async function flushMicrotasks(times = 4) {
  for (let i = 0; i < times; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

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
    getSettings.mockReset().mockResolvedValue({});
    // Remove any textareas a previous test left behind AND blur whatever
    // holds focus — isForeignTextEntry consults document.activeElement, so a
    // stray focused input from an earlier test would silently disable
    // activation in this one (the guard working as designed, but leaking
    // across tests).
    document.body.innerHTML = "";
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
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

  it("chord is exactly the configured modifier set: default Ctrl+Alt — plain Ctrl and Ctrl+Meta do not activate", () => {
    const { result } = renderHook(() => useQuickSelect());

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true });
    });
    expect(result.current.visible).toBe(false);

    // With the configurable chord, the hold condition is an EXACT modifier-set
    // match: the default is Ctrl+Alt, so Ctrl+Meta (no Alt) is not the chord.
    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, metaKey: true });
    });
    expect(result.current.visible).toBe(false);

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, altKey: true });
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

  it("releasing the chord modifier (Alt) while other non-chord modifiers are held ends quick-select", () => {
    const { result } = renderHook(() => useQuickSelect());

    // Activate with the default Ctrl+Alt chord
    act(() => {
      chordDown();
    });
    expect(result.current.visible).toBe(true);

    act(() => {
      dispatchKey("keyup", { key: "Alt", ctrlKey: true, altKey: false, metaKey: true });
    });
    // Alt released → Ctrl+Alt chord broken (Meta being held is irrelevant —
    // it's not part of the configured chord)
    expect(result.current.visible).toBe(false);
  });

  it("skips activation while focus is in a non-terminal text input (isForeignTextEntry guard)", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    const { result } = renderHook(() => useQuickSelect());

    // Focus in the chat input (a foreign text entry — not the terminal's textarea)
    const chatInput = document.createElement("textarea");
    document.body.appendChild(chatInput);
    chatInput.focus();
    expect(document.activeElement).toBe(chatInput);

    act(() => {
      chordDown();
    });
    expect(result.current.visible).toBe(false);

    act(() => {
      dispatchKey("keydown", { key: "F1", ctrlKey: true, altKey: true });
    });
    expect(mockedSendToTerminal).not.toHaveBeenCalled();
  });

  it("still activates while the terminal's own hidden textarea has focus (guard exemption)", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    const { result } = renderHook(() => useQuickSelect());

    // Mimic xterm: a hidden textarea stored as the active tab's term ref
    const termTextarea = document.createElement("textarea");
    document.body.appendChild(termTextarea);
    useTerminalStore.getState().setTermRef("t1", {
      textarea: termTextarea,
    } as unknown as import("@xterm/xterm").Terminal);
    termTextarea.focus();
    expect(document.activeElement).toBe(termTextarea);

    act(() => {
      chordDown();
    });
    expect(result.current.visible).toBe(true);
    expect(result.current.items).toEqual([
      { label: "F1", kind: "command", id: "c1" },
      { label: "F2", kind: "terminal", id: "t1" },
    ]);
  });

  it("registers document listeners in the CAPTURE phase and detaches with the same flag on unmount", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    const addSpy = vi.spyOn(document, "addEventListener");
    const removeSpy = vi.spyOn(document, "removeEventListener");
    const { unmount } = renderHook(() => useQuickSelect());

    // Capture phase on both listeners: run before xterm's own key handlers
    // (which listen on its textarea, not the document) so (a) the chorded
    // F-key routing wins while the terminal has focus instead of the shell
    // also seeing it, and (b) the chord-release keyup reliably hides the
    // overlay even if xterm swallows a keyup. Mirrors useConfiguredHotkey.
    expect(addSpy).toHaveBeenCalledWith("keydown", expect.any(Function), { capture: true });
    expect(addSpy).toHaveBeenCalledWith("keyup", expect.any(Function), { capture: true });

    unmount();

    // removeEventListener MUST carry the same capture flag or the cleanup
    // won't actually detach the listener.
    expect(removeSpy).toHaveBeenCalledWith("keydown", expect.any(Function), { capture: true });
    expect(removeSpy).toHaveBeenCalledWith("keyup", expect.any(Function), { capture: true });

    addSpy.mockRestore();
    removeSpy.mockRestore();
  });

  it("still routes keys normally after unmount+remount (cleanup genuinely detaches capture listeners)", () => {
    seedState({
      commands: [{ id: "c1", command: "echo pinned", pinned: true }],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    const first = renderHook(() => useQuickSelect());
    first.unmount();

    // Second mount's activation must work — proves the first mount's capture
    // listeners were really removed, not silently left attached (a botched
    // removeEventListener with a mismatched capture flag would leave the
    // first handler consuming the chord).
    const second = renderHook(() => useQuickSelect());

    act(() => {
      chordDown();
    });
    expect(second.result.current.visible).toBe(true);
    second.unmount();
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

    expect(addSpy).toHaveBeenCalledWith("keydown", expect.any(Function), { capture: true });
    expect(addSpy).toHaveBeenCalledWith("keyup", expect.any(Function), { capture: true });

    unmount();

    // Same capture flag on removal — a flagless removeEventListener would
    // silently no-op against a capture-phase registration.
    expect(removeSpy).toHaveBeenCalledWith("keydown", expect.any(Function), { capture: true });
    expect(removeSpy).toHaveBeenCalledWith("keyup", expect.any(Function), { capture: true });

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

  // --- Configurable chord (HotkeyQuickSelect) ---
  //
  // The chord combo is loaded via GetSettings (same pattern as
  // useConfiguredHotkey) and parsed with parseHotkey. The tests mock the
  // SettingsService binding and flush the dynamic-import chain like
  // useAddClipboardCommandHotkey.test.ts does.

  it("still activates with the DEFAULT chord (Ctrl+Alt) before settings load", async () => {
    seedState({ commands: [{ id: "c1", command: "echo x", pinned: true }] });
    const { result } = renderHook(() => useQuickSelect());
    await flushMicrotasks();

    act(() => {
      chordDown();
    });
    expect(result.current.visible).toBe(true);
  });

  it("uses the configured chord from settings (Ctrl+Shift works, plain Ctrl+Alt does not)", async () => {
    getSettings.mockResolvedValue({ HotkeyQuickSelect: "Ctrl+Shift" });
    seedState({ commands: [{ id: "c1", command: "echo x", pinned: true }] });
    const { result } = renderHook(() => useQuickSelect());
    await flushMicrotasks();

    // Old default chord must no longer activate
    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, altKey: true });
    });
    expect(result.current.visible).toBe(false);

    // Configured chord activates
    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, shiftKey: true });
    });
    expect(result.current.visible).toBe(true);
  });

  it("routes F-key insertion through the configured chord (Ctrl+Meta)", async () => {
    getSettings.mockResolvedValue({ HotkeyQuickSelect: "Ctrl+Meta" });
    seedState({
      commands: [{ id: "c1", command: "echo meta", pinned: true }],
      tabs: [{ id: "t1", name: "main" }],
      activeTabId: "t1",
    });
    renderHook(() => useQuickSelect());
    await flushMicrotasks();

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, metaKey: true });
    });
    act(() => {
      dispatchKey("keydown", { key: "F1", ctrlKey: true, metaKey: true });
    });

    expect(mockedSendToTerminal).toHaveBeenCalledWith("t1", "echo meta", false);
  });

  it("release semantics follow the configured chord (Ctrl+Shift releases on Shift or Ctrl up)", async () => {
    getSettings.mockResolvedValue({ HotkeyQuickSelect: "Ctrl+Shift" });
    const { result } = renderHook(() => useQuickSelect());
    await flushMicrotasks();

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, shiftKey: true });
    });
    expect(result.current.visible).toBe(true);

    // Shift up (Ctrl still held) breaks the Ctrl+Shift chord
    act(() => {
      dispatchKey("keyup", { key: "Shift", ctrlKey: true, shiftKey: false });
    });
    expect(result.current.visible).toBe(false);
  });

  it("ignores a configured combo that includes a non-modifier key (falls back to default)", async () => {
    getSettings.mockResolvedValue({ HotkeyQuickSelect: "Ctrl+Alt+K" });
    const { result } = renderHook(() => useQuickSelect());
    await flushMicrotasks();

    // Default Ctrl+Alt still works because the invalid combo was rejected
    act(() => {
      chordDown();
    });
    expect(result.current.visible).toBe(true);
  });
});
