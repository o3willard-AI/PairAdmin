import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
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
vi.mock("../../../../wailsjs/go/services/capture/CaptureManager", () => ({
  GetAdapterStatus: vi.fn(() => Promise.resolve([])),
}));

// Mock the SettingsService Wails binding (LLMConfigTab fetches settings on mount)
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: vi.fn(() => Promise.resolve({})),
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
});

describe("ThreeColumnLayout", () => {
  it("renders three columns: left aside, center main, right aside", () => {
    const { container } = render(
      <ThreeColumnLayout sidebar={<div>Commands</div>}>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    // Left column has w-40 class
    const leftAside = container.querySelector(".w-40");
    expect(leftAside).toBeInTheDocument();

    // Right column has w-[220px] class
    const rightAside = container.querySelector(".w-\\[220px\\]");
    expect(rightAside).toBeInTheDocument();

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
