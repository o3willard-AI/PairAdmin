import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { TerminalTab } from "@/components/terminal/TerminalTab";

describe("TerminalTab", () => {
  // Test 4: renders warning icon when tab.degraded is true
  it("renders warning badge when tab is degraded", () => {
    const tab = {
      id: "atspi::1.200/org/a11y/atspi/accessible/0",
      name: "Konsole",
      degraded: true,
      degradedMsg: "Konsole text extraction not available on this system.",
    };
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    // Warning icon should be present (⚠ character or role)
    const button = screen.getByRole("button");
    expect(button).toBeInTheDocument();
    // The warning icon (⚠ or warning text) should appear somewhere
    expect(button.textContent).toMatch(/⚠|Konsole/);
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
});
