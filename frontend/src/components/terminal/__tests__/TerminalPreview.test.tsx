import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { TerminalPreview } from "@/components/terminal/TerminalPreview";

// xterm.js uses DOM APIs not available in jsdom — mock the whole module.
// vi.mock factories are hoisted above the rest of the file, so the shared
// "last instance" holder must be created via vi.hoisted to be safely
// referenced from inside the factory.
const { terminalInstances, fitAddonInstances, callOrder } = vi.hoisted(() => ({
  terminalInstances: [] as unknown[],
  fitAddonInstances: [] as { fit: (...args: unknown[]) => void }[],
  // Tracks the order "onResize registered" vs "fit called" happen in, so the
  // regression test below can assert the listener is attached before the
  // first fit — not just that both eventually happen.
  callOrder: [] as string[],
}));

vi.mock("@xterm/xterm", () => {
  class FakeTerminal {
    loadAddon = vi.fn();
    open = vi.fn();
    writeln = vi.fn();
    write = vi.fn();
    dispose = vi.fn();
    onData = vi.fn(() => ({ dispose: vi.fn() }));
    onResize = vi.fn(() => {
      callOrder.push("onResize-registered");
      return { dispose: vi.fn() };
    });
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
    fit = vi.fn(() => {
      callOrder.push("fit-called");
    });
    constructor() {
      fitAddonInstances.push(this);
    }
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

  describe("terminal fit timing", () => {
    beforeEach(() => {
      fitAddonInstances.length = 0;
      callOrder.length = 0;
    });

    // Regression test for the reported bug: tmux opened at roughly half the
    // terminal's real width, only correcting itself after an unrelated later
    // resize (toggling Hide/Show PairAdmin) nudged it. Root cause was
    // registering term.onResize() *after* the first fitAddon.fit() call —
    // xterm only fires "resize" when the size actually changes, and the very
    // first fit (going from xterm's default size to the real container size)
    // is exactly that one-time change. Registering the listener after that
    // fit meant the remote PTY never learned the terminal's real size, and
    // stayed at RequestPty's placeholder until some later, unrelated resize
    // happened to fire onResize again. This asserts the listener is
    // registered before the first fit, not just that both eventually happen.
    it("registers the resize listener before the first fit, so the initial resize isn't missed", () => {
      render(<TerminalPreview tabId="real-tab" />);

      expect(callOrder.indexOf("onResize-registered")).toBeGreaterThanOrEqual(0);
      expect(callOrder.indexOf("fit-called")).toBeGreaterThanOrEqual(0);
      expect(callOrder.indexOf("onResize-registered")).toBeLessThan(callOrder.indexOf("fit-called"));
    });

    // Regression test: FitAddon measures character-cell pixel width using
    // whatever font is *currently* resolved. If that measurement runs before
    // the browser finishes resolving the fontFamily stack, it undercounts
    // columns using a wider fallback font instead of the intended one. This
    // was reported as tmux opening at roughly half width, only
    // self-correcting after an unrelated later resize (e.g. toggling
    // Hide/Show PairAdmin). Re-fitting once document.fonts.ready resolves
    // corrects it without user interaction.
    it("fits once immediately on mount and again once the font has resolved", async () => {
      render(<TerminalPreview tabId="real-tab" />);

      const fitAddon = fitAddonInstances[fitAddonInstances.length - 1];
      expect(fitAddon.fit).toHaveBeenCalledTimes(1);

      await vi.waitFor(() => {
        expect(fitAddon.fit).toHaveBeenCalledTimes(2);
      });
    });
  });
});
