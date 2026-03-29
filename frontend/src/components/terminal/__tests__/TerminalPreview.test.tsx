import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { TerminalPreview } from "@/components/terminal/TerminalPreview";

// xterm.js uses DOM APIs not available in jsdom — mock the whole module
vi.mock("@xterm/xterm", () => {
  class Terminal {
    loadAddon = vi.fn();
    open = vi.fn();
    writeln = vi.fn();
    dispose = vi.fn();
  }
  return { Terminal };
});

vi.mock("@xterm/addon-fit", () => {
  class FitAddon {
    fit = vi.fn();
  }
  return { FitAddon };
});

vi.mock("@xterm/addon-canvas", () => {
  class CanvasAddon {}
  return { CanvasAddon };
});

vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

beforeEach(() => {
  class ResizeObserverMock {
    observe = vi.fn();
    disconnect = vi.fn();
    unobserve = vi.fn();
  }
  global.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;
});

describe("TerminalPreview", () => {
  // Test 5: shows AT-SPI2 onboarding when adapterStatus includes atspi with status "onboarding"
  it("shows AT-SPI2 onboarding instructions when atspi adapter has status onboarding", () => {
    const adapterStatus = [
      { name: "atspi", status: "onboarding", message: "Enable accessibility" },
    ];
    render(<TerminalPreview tabId="" adapterStatus={adapterStatus} />);

    expect(
      screen.getByText("No terminal sessions detected.")
    ).toBeInTheDocument();
    expect(
      screen.getByText(/toolkit-accessibility true/)
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Option 2: Enable accessibility/)
    ).toBeInTheDocument();
  });

  // Test 6: shows standard no-tabs message when no onboarding adapter
  it("shows standard no-sessions message without AT-SPI2 section when no onboarding status", () => {
    const adapterStatus = [
      { name: "atspi", status: "active", message: "" },
    ];
    render(<TerminalPreview tabId="" adapterStatus={adapterStatus} />);

    expect(
      screen.getByText("No terminal sessions detected.")
    ).toBeInTheDocument();
    // Should NOT show the AT-SPI2 onboarding section
    expect(
      screen.queryByText(/toolkit-accessibility true/)
    ).not.toBeInTheDocument();
  });

  it("shows tmux start command in both onboarding and non-onboarding empty states", () => {
    render(<TerminalPreview tabId="" />);

    expect(screen.getByText(/tmux new-session/)).toBeInTheDocument();
    expect(
      screen.getByText("No terminal sessions detected.")
    ).toBeInTheDocument();
  });
});
