import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { TerminalPreview } from "@/components/terminal/TerminalPreview";

// xterm.js uses DOM APIs not available in jsdom — mock the whole module.
// vi.mock factories are hoisted above the rest of the file, so the shared
// "last instance" holder must be created via vi.hoisted to be safely
// referenced from inside the factory.
const { terminalInstances } = vi.hoisted(() => ({ terminalInstances: [] as unknown[] }));

vi.mock("@xterm/xterm", () => {
  class FakeTerminal {
    loadAddon = vi.fn();
    open = vi.fn();
    writeln = vi.fn();
    write = vi.fn();
    dispose = vi.fn();
    onData = vi.fn(() => ({ dispose: vi.fn() }));
    onResize = vi.fn(() => ({ dispose: vi.fn() }));
    getSelection = vi.fn(() => "");
    attachCustomKeyEventHandler = vi.fn();

    constructor() {
      terminalInstances.push(this);
    }
  }
  return { Terminal: FakeTerminal };
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

const mockWriteInput = vi.fn((..._args: unknown[]) => Promise.resolve());
vi.mock("../../../../wailsjs/go/services/PTYService", () => ({
  GetWindowsContent: vi.fn(() => Promise.resolve("")),
  WriteInput: (...args: unknown[]) => mockWriteInput(...args),
  ResizeTerminal: vi.fn(() => Promise.resolve()),
}));
vi.mock("../../../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(() => () => {}),
}));

interface FakeTerminalHandle {
  getSelection: ReturnType<typeof vi.fn>;
  attachCustomKeyEventHandler: ReturnType<typeof vi.fn>;
}

const latestTerminal = () =>
  terminalInstances[terminalInstances.length - 1] as FakeTerminalHandle;

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
      screen.getByText(/Enable accessibility for GUI terminals/)
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

  describe("system clipboard handling", () => {
    beforeEach(() => {
      terminalInstances.length = 0;
      mockWriteInput.mockClear();
      Object.assign(navigator, {
        clipboard: {
          writeText: vi.fn(() => Promise.resolve()),
          readText: vi.fn(() => Promise.resolve("pasted text")),
        },
      });
    });

    const pressKey = (key: string) => {
      const handler = latestTerminal().attachCustomKeyEventHandler.mock.calls[0][0];
      return handler({ type: "keydown", key, ctrlKey: true, shiftKey: false, altKey: false, metaKey: false });
    };

    it("registers a custom key handler on mount", () => {
      render(<TerminalPreview tabId="real-tab" />);
      expect(latestTerminal().attachCustomKeyEventHandler).toHaveBeenCalled();
    });

    it("Ctrl+C copies the selection to the clipboard and suppresses sending it to the shell", () => {
      render(<TerminalPreview tabId="real-tab" />);
      latestTerminal().getSelection.mockReturnValue("selected text");

      const handled = pressKey("c");

      expect(navigator.clipboard.writeText).toHaveBeenCalledWith("selected text");
      expect(handled).toBe(false);
    });

    it("Ctrl+C with no selection lets the SIGINT pass through normally", () => {
      render(<TerminalPreview tabId="real-tab" />);
      latestTerminal().getSelection.mockReturnValue("");

      const handled = pressKey("c");

      expect(navigator.clipboard.writeText).not.toHaveBeenCalled();
      expect(handled).toBe(true);
    });

    it("Ctrl+V pastes clipboard text into the terminal instead of sending the literal byte", async () => {
      render(<TerminalPreview tabId="real-tab" />);

      const handled = pressKey("v");
      expect(handled).toBe(false);

      await vi.waitFor(() => {
        expect(mockWriteInput).toHaveBeenCalledWith("real-tab", "pasted text");
      });
    });
  });
});
