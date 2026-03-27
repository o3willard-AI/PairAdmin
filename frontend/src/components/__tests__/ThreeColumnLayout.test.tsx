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

  it("renders bash:1 and bash:2 tabs in the tab list", () => {
    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    expect(screen.getByText("bash:1")).toBeInTheDocument();
    expect(screen.getByText("bash:2")).toBeInTheDocument();
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
});
