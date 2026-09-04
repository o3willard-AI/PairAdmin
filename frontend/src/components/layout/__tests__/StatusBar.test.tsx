import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import "@testing-library/jest-dom";
import { StatusBar } from "@/components/layout/StatusBar";
import { useSettingsStore } from "@/stores/settingsStore";
import { useChatStore } from "@/stores/chatStore";

describe("StatusBar connection status", () => {
  beforeEach(() => {
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "checking",
    });
    useChatStore.setState({ messagesByTab: {}, llmRequest: null });
  });

  it("renders 'Disabled' with a neutral (non-green/non-red) dot when connectionStatus is 'disabled'", () => {
    useSettingsStore.setState({ connectionStatus: "disabled" });
    const { container } = render(<StatusBar />);

    expect(screen.getByText("Disabled")).toBeInTheDocument();
    // The disabled dot must not reuse connected-green or disconnected-red
    expect(container.innerHTML).toContain("bg-amber-500");
    expect(container.innerHTML).not.toContain("bg-green-500");
    expect(container.innerHTML).not.toContain("bg-red-500");
  });

  it("still renders the existing connected/disconnected labels", () => {
    useSettingsStore.setState({ connectionStatus: "connected" });
    const { unmount } = render(<StatusBar />);
    expect(screen.getByText("Connected")).toBeInTheDocument();
    unmount();

    useSettingsStore.setState({ connectionStatus: "disconnected" });
    render(<StatusBar />);
    expect(screen.getByText("Disconnected")).toBeInTheDocument();
  });
});

describe("StatusBar LLM activity indicator", () => {
  beforeEach(() => {
    useSettingsStore.setState({
      activeModel: "openai:gpt-4o",
      settingsOpen: false,
      connectionStatus: "connected",
    });
    useChatStore.setState({ messagesByTab: {}, llmRequest: null });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows no activity indicator when llmRequest is null (idle)", () => {
    render(<StatusBar />);

    expect(screen.queryByText("Working…")).not.toBeInTheDocument();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    // The token meter is still there.
    expect(screen.getByText("Tokens: —")).toBeInTheDocument();
  });

  it("shows the spinner + 'Working…' + elapsed seconds while llmRequest is set", () => {
    vi.useFakeTimers();

    useChatStore.setState({ llmRequest: { startedAt: Date.now() } });
    render(<StatusBar />);

    const status = screen.getByRole("status");
    expect(status).toBeInTheDocument();
    expect(status).toHaveAttribute("aria-live", "polite");
    expect(screen.getByText("Working…")).toBeInTheDocument();
    // 0.0s elapsed at t0
    expect(screen.getByText("0.0s")).toBeInTheDocument();
    // The spinner is a lucide Loader2 with animate-spin.
    expect(status.querySelector(".lucide-loader-circle, .lucide-loader-2")).not.toBeNull();
    expect(status.innerHTML).toContain("animate-spin");
  });

  it("elapsed seconds tick while busy (interval re-render)", () => {
    vi.useFakeTimers();

    // Fixed start timestamp; advancing the fake clock moves Date.now() forward
    // and fires the 100ms interval ticks, so the elapsed label follows.
    const startedAt = Date.now();
    useChatStore.setState({ llmRequest: { startedAt } });
    render(<StatusBar />);
    expect(screen.getByText("0.0s")).toBeInTheDocument();

    // 2.5s later the label reads 2.5s
    act(() => {
      vi.advanceTimersByTime(2500);
    });
    expect(screen.getByText("2.5s")).toBeInTheDocument();

    // and 10.0s at the 10s mark
    act(() => {
      vi.advanceTimersByTime(7500);
    });
    expect(screen.getByText("10.0s")).toBeInTheDocument();
  });

  it("clears when llmRequest returns to null", () => {
    vi.useFakeTimers();
    useChatStore.setState({ llmRequest: { startedAt: Date.now() } });
    const { unmount } = render(<StatusBar />);
    expect(screen.getByRole("status")).toBeInTheDocument();

    act(() => {
      useChatStore.setState({ llmRequest: null });
    });
    unmount();

    render(<StatusBar />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.queryByText("Working…")).not.toBeInTheDocument();
  });
});
