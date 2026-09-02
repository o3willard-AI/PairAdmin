import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import "@testing-library/jest-dom";
import { QuickSelectOverlay } from "@/components/QuickSelectOverlay";
import { useQuickSelect, type QuickSelectItem } from "@/hooks/useQuickSelect";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";

// The overlay renders the hook's state; the hook's chord/F-key routing logic
// is E1's territory and covered by useQuickSelect.test.ts. Here the hook is
// mocked so tests drive `visible`/`items` directly and assert only what the
// overlay itself does with them.
const mockUseQuickSelect = vi.mocked(useQuickSelect);
// Capture the REAL implementation inside the factory (vi.mock replaces every
// import binding of the specifier, so the factory is the only place the
// original is still reachable). vi.hoisted runs before the hoisted vi.mock,
// avoiding the TDZ the factory would otherwise hit on a plain `let`.
const captured = vi.hoisted(() => ({ real: null as unknown as typeof useQuickSelect }));
vi.mock("@/hooks/useQuickSelect", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/hooks/useQuickSelect")>();
  captured.real = actual.useQuickSelect;
  return {
    ...actual,
    useQuickSelect: vi.fn(() => ({ visible: false, items: [] as QuickSelectItem[] })),
  };
});

function setState(visible: boolean, items: QuickSelectItem[]) {
  mockUseQuickSelect.mockReturnValue({ visible, items });
}

beforeEach(() => {
  mockUseQuickSelect.mockReset();
  mockUseQuickSelect.mockReturnValue({ visible: false, items: [] });
});

describe("QuickSelectOverlay", () => {
  it("renders one F-key label per item", () => {
    setState(true, [
      { label: "F1", kind: "command", id: "c1" },
      { label: "F2", kind: "command", id: "c2" },
      { label: "F3", kind: "terminal", id: "t1" },
    ]);
    render(<QuickSelectOverlay />);

    expect(screen.getByText("F1")).toBeInTheDocument();
    expect(screen.getByText("F2")).toBeInTheDocument();
    expect(screen.getByText("F3")).toBeInTheDocument();
  });

  it("is hidden (not visible) while the chord is not held", () => {
    setState(false, [
      { label: "F1", kind: "command", id: "c1" },
      { label: "F2", kind: "terminal", id: "t1" },
    ]);
    const { container } = render(<QuickSelectOverlay />);

    // Keep-mounted fade-out: labels stay in the DOM but the layer is
    // transparent and click-through.
    const layer = container.firstElementChild as HTMLElement;
    expect(layer.className).toContain("opacity-0");
    expect(layer.className).toContain("pointer-events-none");
    expect(screen.getByText("F1")).toBeInTheDocument(); // still mounted
  });

  it("fades in when visible (opacity transition on, not display:none)", () => {
    setState(true, [{ label: "F1", kind: "command", id: "c1" }]);
    const { container } = render(<QuickSelectOverlay />);

    const layer = container.firstElementChild as HTMLElement;
    expect(layer.className).toContain("opacity-100");
    expect(layer.className).not.toContain("opacity-0");
    // "Keep mounted, toggle visibility" — never display:none, so the CSS
    // opacity transition can actually animate both ways.
    expect(layer.className).not.toContain("hidden");
    expect(layer.className).toContain("transition-opacity");
  });

  it("fades out on release while keeping the labels mounted", () => {
    setState(true, [{ label: "F1", kind: "command", id: "c1" }]);
    const { container, rerender } = render(<QuickSelectOverlay />);

    expect((container.firstElementChild as HTMLElement).className).toContain("opacity-100");

    setState(false, [{ label: "F1", kind: "command", id: "c1" }]);
    rerender(<QuickSelectOverlay />);

    const layer = container.firstElementChild as HTMLElement;
    expect(layer.className).toContain("opacity-0");
    expect(screen.getByText("F1")).toBeInTheDocument();
  });

  it("aligns terminal items to the left edge and command items to the right edge", () => {
    setState(true, [
      { label: "F1", kind: "command", id: "c1" },
      { label: "F2", kind: "terminal", id: "t1" },
      { label: "F3", kind: "terminal", id: "t2" },
    ]);
    render(<QuickSelectOverlay />);

    // Group containers: one per kind, positioned to their sidebar's edge.
    const leftGroup = screen.getByTestId("quick-select-terminals");
    const rightGroup = screen.getByTestId("quick-select-commands");
    expect(leftGroup.className).toMatch(/left-/);
    expect(rightGroup.className).toMatch(/right-/);

    // Labels live under the right group for commands...
    expect(rightGroup).toHaveTextContent("F1");
    expect(withinGroup(leftGroup, "F1")).toBe(false);
    // ...and under the left group for terminals.
    expect(leftGroup).toHaveTextContent("F2");
    expect(leftGroup).toHaveTextContent("F3");
  });

  it("never intercepts pointer events, even while visible", () => {
    setState(true, [{ label: "F1", kind: "command", id: "c1" }]);
    const { container } = render(<QuickSelectOverlay />);

    expect((container.firstElementChild as HTMLElement).className).toContain("pointer-events-none");
  });

  it("renders nothing but the (empty) layer when there are no items", () => {
    setState(true, []);
    const { container } = render(<QuickSelectOverlay />);

    expect(container.firstElementChild).toBeInTheDocument();
    expect(screen.queryByText(/^F\d+$/)).not.toBeInTheDocument();
  });

  it("live-updates from the hook while mounted (hook drives re-render)", () => {
    // Integration-ish guard for the E2 mount in ThreeColumnLayout: the
    // overlay must reflect hook state changes without a remount. Uses the
    // REAL hook (captured in the mock factory above), driving it through
    // document key events — the same path the GUI takes.
    vi.mocked(useQuickSelect).mockImplementation(captured.real);

    useCommandStore.setState({
      commands: [
        { id: "c1", command: "echo pinned", originalQuestion: "", timestamp: 0, tabId: "seed", pinned: true },
      ],
    });
    useTerminalStore.setState({ tabs: [{ id: "t1", name: "main" }], activeTabId: "t1" });

    const dispatchKey = (type: "keydown" | "keyup", init: Partial<KeyboardEventInit> & { key: string }) =>
      document.dispatchEvent(new KeyboardEvent(type, { bubbles: true, cancelable: true, ...init }));

    const { container } = render(<QuickSelectOverlay />);

    act(() => {
      dispatchKey("keydown", { key: "Control", ctrlKey: true, altKey: true });
    });
    expect(container.firstElementChild!.className).toContain("opacity-100");
    expect(screen.getByText("F1")).toBeInTheDocument();
    expect(screen.getByText("F2")).toBeInTheDocument();

    act(() => {
      dispatchKey("keyup", { key: "Control", ctrlKey: false });
    });
    expect(container.firstElementChild!.className).toContain("opacity-0");
    // Keep-mounted: labels remain in the DOM after fade-out.
    expect(screen.getByText("F1")).toBeInTheDocument();

    // Restore the mock for subsequent tests in this file.
    vi.mocked(useQuickSelect).mockImplementation(() => ({ visible: false, items: [] }));
  });
});

function withinGroup(group: HTMLElement, text: string): boolean {
  return group.textContent?.includes(text) ?? false;
}
