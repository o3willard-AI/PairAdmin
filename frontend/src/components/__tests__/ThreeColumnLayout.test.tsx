import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { ThreeColumnLayout } from "@/components/layout/ThreeColumnLayout";

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

// Mock the CSS import
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

// Mock the Wails runtime so useTerminalCapture (mounted in ThreeColumnLayout) doesn't
// call window.runtime.EventsOnMultiple in jsdom.
// Path resolves from frontend/src/components/__tests__/ → frontend/wailsjs/runtime/runtime
vi.mock("../../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(() => vi.fn()),
}));

// Mock the CaptureManager Wails binding (ThreeColumnLayout fetches adapter status on mount)
// Path resolves from frontend/src/components/__tests__/ → frontend/wailsjs/go/services/capture/CaptureManager
// (previously had one extra "../" — harmless before now since ThreeColumnLayout.tsx's
// own .catch(() => {}) swallowed the resulting failed dynamic import either way, but
// it meant this mock never actually intercepted anything).
vi.mock("../../../wailsjs/go/services/capture/CaptureManager", () => ({
  GetAdapterStatus: vi.fn(() => Promise.resolve([])),
}));

// Mock the SettingsService Wails binding (LLMConfigTab fetches settings on mount).
// GetSettings is delegated to a module-level `getSettings` mock (rather than a
// vi.fn() defined directly in this factory) so individual tests can hold a
// stable reference to override its resolved value with mockResolvedValueOnce —
// a fresh `await import(...)` from inside a test body isn't guaranteed to
// yield the same mock instance ThreeColumnLayout.tsx's own internal dynamic
// import resolves to.
const getSettings = vi.fn();
vi.mock("../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  GetAPIKeyStatus: vi.fn(() => Promise.resolve("")),
  SaveSettings: vi.fn(() => Promise.resolve(undefined)),
  SaveAPIKey: vi.fn(() => Promise.resolve(undefined)),
  TestConnection: vi.fn(() => Promise.resolve("Connected")),
  SetModel: vi.fn(() => Promise.resolve("")),
}));

// Mock useTheme for AppearanceTab rendered inside SettingsDialog
vi.mock("@/theme/theme-provider", () => ({
  useTheme: () => ({ theme: "dark", setTheme: vi.fn() }),
}));

beforeEach(() => {
  // ResizeObserver is not available in jsdom — must use a class
  class ResizeObserverMock {
    observe = vi.fn();
    disconnect = vi.fn();
    unobserve = vi.fn();
  }
  global.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;
  localStorage.clear();
  getSettings.mockReset().mockResolvedValue({});
});

describe("ThreeColumnLayout", () => {
  it("renders three columns: left aside, center main, right aside", () => {
    const { container } = render(
      <ThreeColumnLayout sidebar={<div>Commands</div>}>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    // Left and right columns are <aside> elements sized via an inline `ch`
    // width (configurable in Settings → Terminals) rather than a fixed
    // Tailwind class.
    const asides = container.querySelectorAll("aside");
    expect(asides).toHaveLength(2);
    expect(asides[0].style.width).toBe("20ch"); // default Terminals width
    expect(asides[1].style.width).toBe("30ch"); // default Commands width

    // Center column (main element)
    const centerMain = container.querySelector("main");
    expect(centerMain).toBeInTheDocument();
  });

  it("renders Terminals header in left column", () => {
    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    expect(screen.getByText("Terminals")).toBeInTheDocument();
  });

  it("renders status bar with No model text", () => {
    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    expect(screen.getByText("No model")).toBeInTheDocument();
  });

  it("renders empty tab list when store has no tabs (initial empty state)", () => {
    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    // Initial store state is now empty (tabs added dynamically via Wails events)
    expect(screen.queryByText("bash:1")).not.toBeInTheDocument();
    expect(screen.queryByText("bash:2")).not.toBeInTheDocument();
  });

  it("passes children to the center column", () => {
    render(
      <ThreeColumnLayout>
        <div>Chat area content</div>
      </ThreeColumnLayout>
    );

    expect(screen.getByText("Chat area content")).toBeInTheDocument();
  });

  it("passes sidebar prop to the right column", () => {
    render(
      <ThreeColumnLayout sidebar={<div>Commands sidebar</div>}>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    expect(screen.getByText("Commands sidebar")).toBeInTheDocument();
  });

  it("shows a 'Hide PairAdmin' button by default, with chat content visible", () => {
    render(
      <ThreeColumnLayout>
        <div>Chat area content</div>
      </ThreeColumnLayout>
    );

    expect(screen.getByText("Hide PairAdmin")).toBeInTheDocument();
    expect(screen.getByText("Chat area content")).toBeInTheDocument();
  });

  it("clicking 'Hide PairAdmin' collapses the chat area and shows 'Show PairAdmin' instead", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <ThreeColumnLayout>
        <div>Chat area content</div>
      </ThreeColumnLayout>
    );

    await user.click(screen.getByText("Hide PairAdmin"));

    expect(screen.getByText("Show PairAdmin")).toBeInTheDocument();
    expect(screen.queryByText("Hide PairAdmin")).not.toBeInTheDocument();
    // Content stays mounted (so useLLMStream keeps working in the background)
    // — just visually hidden via the `hidden` utility class, not unmounted.
    const chatContent = screen.getByText("Chat area content");
    expect(chatContent.closest(".hidden")).not.toBeNull();
  });

  it("clicking 'Show PairAdmin' restores the chat area", async () => {
    const user = userEvent.setup();
    render(
      <ThreeColumnLayout>
        <div>Chat area content</div>
      </ThreeColumnLayout>
    );

    await user.click(screen.getByText("Hide PairAdmin"));
    await user.click(screen.getByText("Show PairAdmin"));

    expect(screen.getByText("Hide PairAdmin")).toBeInTheDocument();
    const chatContent = screen.getByText("Chat area content");
    expect(chatContent.closest(".hidden")).toBeNull();
  });

  it("persists the collapsed state across remounts via localStorage", async () => {
    const user = userEvent.setup();
    const { unmount } = render(
      <ThreeColumnLayout>
        <div>Chat area content</div>
      </ThreeColumnLayout>
    );
    await user.click(screen.getByText("Hide PairAdmin"));
    unmount();

    render(
      <ThreeColumnLayout>
        <div>Chat area content</div>
      </ThreeColumnLayout>
    );

    expect(screen.getByText("Show PairAdmin")).toBeInTheDocument();
  });

  it("the Hide/Show PairAdmin control spans the full sidebar width, not just its text", () => {
    // Regression guard: this used to be a small <button> centered inside a
    // full-width wrapper div, so only the text itself was clickable — the
    // fix makes the bar itself the button.
    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    const toggle = screen.getByText("Hide PairAdmin");
    expect(toggle.tagName).toBe("BUTTON");
    expect(toggle.className).toContain("w-full");
  });

  it("applies sidebar widths loaded from settings, in ch units", async () => {
    // mockResolvedValue (not Once): SettingsDialog's own tabs (HotkeysTab,
    // TerminalsTab, etc.) each call GetSettings() on their own mount too —
    // even while the dialog is closed, since base-ui may keep tab panels
    // mounted — so more than one call can race here, in no guaranteed order.
    getSettings.mockResolvedValue({
      TerminalsSidebarWidthCh: 25,
      CommandsSidebarWidthCh: 40,
    });

    const { container } = render(
      <ThreeColumnLayout sidebar={<div>Commands</div>}>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    await waitFor(() => {
      const asides = container.querySelectorAll("aside");
      expect(asides[0].style.width).toBe("25ch");
      expect(asides[1].style.width).toBe("40ch");
    });
  });

  it("renders SettingsDialog component (closed by default)", () => {
    const { container } = render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    // SettingsDialog is mounted but closed by default — dialog popup is not in DOM
    // The dialog root itself doesn't have visible content when closed
    // Verify SettingsDialog doesn't show Settings title when closed
    expect(screen.queryByText("Settings")).not.toBeInTheDocument();
    // But the container should still render without errors
    expect(container).toBeInTheDocument();
  });
});
