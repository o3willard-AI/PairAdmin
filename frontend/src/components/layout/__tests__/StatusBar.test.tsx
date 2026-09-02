import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { StatusBar } from "@/components/layout/StatusBar";
import { useSettingsStore } from "@/stores/settingsStore";

describe("StatusBar connection status", () => {
  beforeEach(() => {
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "checking",
    });
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
