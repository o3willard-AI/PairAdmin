import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import "@testing-library/jest-dom";
import { QuickSelectOverlay } from "@/components/QuickSelectOverlay";
import { useQuickSelect } from "@/hooks/useQuickSelect";
import { useQuickSelectStore } from "@/stores/quickSelectStore";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";

// The overlay no longer renders badges itself — cards do (CommandCard /
// TerminalTab read quickSelectStore). The overlay component is now just the
// mount point that keeps useQuickSelect's capture-phase key listeners alive.
// Here the hook is mocked for the mount-shape tests; the live-update test
// swaps in the real hook (captured in the factory below) and drives it
// through document key events, asserting the STORE the badges read.
const mockUseQuickSelect = vi.mocked(useQuickSelect);
const captured = vi.hoisted(() => ({ real: null as unknown as typeof useQuickSelect }));
vi.mock("@/hooks/useQuickSelect", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/hooks/useQuickSelect")>();
  captured.real = actual.useQuickSelect;
  return {
    ...actual,
    useQuickSelect: vi.fn(() => ({ visible: false, items: [] as QuickSelectItem[] })),
  };
});

import type { QuickSelectItem } from "@/hooks/useQuickSelect";

function setState(visible: boolean, items: QuickSelectItem[]) {
  mockUseQuickSelect.mockReturnValue({ visible, items });
}

beforeEach(() => {
  mockUseQuickSelect.mockReset();
  mockUseQuickSelect.mockReturnValue({ visible: false, items: [] });
  useQuickSelectStore.setState({
    visible: false,
    commandFkeys: {},
    terminalFkeys: {},
  });
});

describe("QuickSelectOverlay (hook mount; badges render in cards)", () => {
  it("renders nothing — badges moved inline into CommandCard/TerminalTab", () => {
    setState(true, [
      { label: "F1", kind: "command", id: "c1" },
      { label: "F2", kind: "terminal", id: "t1" },
    ]);
    const { container } = render(<QuickSelectOverlay />);

    // Regression guard for the follow-up that moved badges inline: no fixed
    // column, no label text, no layer div.
    expect(screen.queryByText(/^F\d+$/)).not.toBeInTheDocument();
    expect(screen.queryByTestId("quick-select-terminals")).not.toBeInTheDocument();
    expect(screen.queryByTestId("quick-select-commands")).not.toBeInTheDocument();
    expect(container.firstElementChild).toBeNull();
  });

  it("still mounts the hook so its capture-phase key listeners stay registered", () => {
    // useQuickSelect must be called by the mount — otherwise the chord and
    // F-key routing would silently die when the badge rendering moved into
    // the cards.
    render(<QuickSelectOverlay />);
    expect(mockUseQuickSelect).toHaveBeenCalled();
  });

  it("live-updates: real hook drives quickSelectStore through document key events", () => {
    // End-to-end-ish through the exact GUI path: chord down → hook publishes
    // per-id F-key maps + visible to the store the badges read; release →
    // cleared. (Badge rendering itself is covered by the CommandCard /
    // TerminalTab badge tests.)
    vi.mocked(useQuickSelect).mockImplementation(captured.real);

    useCommandStore.setState({
      commands: [
        { id: "c1", command: "echo pinned", originalQuestion: "", timestamp: 0, tabId: "seed", pinned: true },
        { id: "c2", command: "echo second", originalQuestion: "", timestamp: 0, tabId: "seed", pinned: true },
      ],
    });
    useTerminalStore.setState({ tabs: [{ id: "t1", name: "main" }], activeTabId: "t1" });

    const dispatchKey = (type: "keydown" | "keyup", init: Partial<KeyboardEventInit> & { key: string }) =>
      document.dispatchEvent(new KeyboardEvent(type, { bubbles: true, cancelable: true, ...init }));

    render(<QuickSelectOverlay />);

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, altKey: true });
    });

    const s = useQuickSelectStore.getState();
    expect(s.visible).toBe(true);
    // Pinned commands first, terminals second — per-id F-key labels
    expect(s.commandFkeys).toEqual({ c1: "F1", c2: "F2" });
    expect(s.terminalFkeys).toEqual({ t1: "F3" });

    act(() => {
      dispatchKey("keyup", { key: "Control", ctrlKey: false });
    });

    const after = useQuickSelectStore.getState();
    expect(after.visible).toBe(false);
    expect(after.commandFkeys).toEqual({});
    expect(after.terminalFkeys).toEqual({});

    // Restore the mock for subsequent tests in this file.
    vi.mocked(useQuickSelect).mockImplementation(() => ({ visible: false, items: [] }));
  });
});
