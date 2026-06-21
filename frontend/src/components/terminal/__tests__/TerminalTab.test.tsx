import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { TerminalTab } from "@/components/terminal/TerminalTab";
import { useTerminalStore } from "@/stores/terminalStore";

describe("TerminalTab", () => {
  beforeEach(() => {
    useTerminalStore.setState({ tabs: [], activeTabId: "" });
  });

  // Test 4: renders warning icon when tab.degraded is true
  it("renders warning badge when tab is degraded", () => {
    const tab = {
      id: "atspi::1.200/org/a11y/atspi/accessible/0",
      name: "Konsole",
      degraded: true,
      degradedMsg: "Konsole text extraction not available on this system.",
    };
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    // The warning icon button should be present (Tooltip.Trigger renders as a button)
    const warningButton = screen.getByText("⚠");
    expect(warningButton).toBeInTheDocument();
    // Tab name should still be visible
    expect(screen.getByText("Konsole")).toBeInTheDocument();
  });

  it("does NOT render warning badge for non-degraded tabs", () => {
    const tab = {
      id: "tmux:%0",
      name: "main:0.0",
      degraded: false,
    };
    render(<TerminalTab tab={tab} isActive={true} onClick={vi.fn()} />);

    const button = screen.getByRole("button");
    expect(button).toBeInTheDocument();
    // No warning icon
    expect(button.textContent).not.toMatch(/⚠/);
  });

  it("renders tab name in button", () => {
    const tab = { id: "tmux:%0", name: "main:0.0" };
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);
    expect(screen.getByText("main:0.0")).toBeInTheDocument();
  });

  it("right-click shows a Rename option", async () => {
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("main:0.0") });

    expect(screen.getByText("Rename")).toBeInTheDocument();
  });

  it("renaming via the context menu updates the tab name in terminalStore", async () => {
    useTerminalStore.getState().addTab("tmux:%0", "main:0.0");
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("main:0.0") });
    await user.click(screen.getByText("Rename"));
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "My Custom Name{Enter}");

    expect(useTerminalStore.getState().tabs[0].name).toBe("My Custom Name");
  });

  it("clicking the tab still triggers onClick (not blocked by the context menu wrapper)", async () => {
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const onClick = vi.fn();
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={onClick} />);

    await user.click(screen.getByText("main:0.0"));

    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
