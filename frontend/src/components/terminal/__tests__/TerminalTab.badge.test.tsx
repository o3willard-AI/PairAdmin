import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { TerminalTab } from "@/components/terminal/TerminalTab";
import { useTerminalStore } from "@/stores/terminalStore";
import { useQuickSelectStore, setQuickSelectBadges } from "@/stores/quickSelectStore";

vi.mock("../../../wailsjs/go/services/PTYService", () => ({
  WriteInput: vi.fn(() => Promise.resolve()),
  CloseTerminal: vi.fn(() => Promise.resolve()),
}));
vi.mock("../../../wailsjs/go/services/RemoteService", () => ({
  RenameRemoteHost: vi.fn(() => Promise.resolve("")),
  SaveRemoteHost: vi.fn(() => Promise.resolve({ ID: "" })),
}));

const tab = { id: "tab-1", name: "main" };

function renderTab(active = false) {
  return render(<TerminalTab tab={tab} isActive={active} onClick={vi.fn()} />);
}

beforeEach(() => {
  useTerminalStore.setState({ tabs: [tab], activeTabId: tab.id });
  useQuickSelectStore.setState({
    visible: false,
    commandFkeys: {},
    terminalFkeys: {},
  });
});

describe("TerminalTab — quick-select F-key badge", () => {
  it("shows no F-key badge while quick-select is inactive", () => {
    renderTab();
    expect(screen.queryByText("F1")).not.toBeInTheDocument();
  });

  it("renders its OWN F-key badge on the tab row when its id is assigned", () => {
    setQuickSelectBadges(true, {}, { "tab-1": "F2" });
    renderTab();

    const badge = screen.getByText("F2");
    expect(badge).toBeInTheDocument();
    expect(badge.getAttribute("aria-hidden")).toBe("true");
    expect(badge.className).toContain("pointer-events-none");
  });

  it("shows no badge when visible but this tab has no F-key", () => {
    setQuickSelectBadges(true, {}, { "other-tab": "F1" });
    renderTab();

    expect(screen.queryByText("F1")).not.toBeInTheDocument();
  });

  it("tab row is position:relative so the absolute badge anchors to the row", () => {
    setQuickSelectBadges(true, {}, { "tab-1": "F1" });
    const { container } = renderTab();

    const row = container.firstElementChild as HTMLElement;
    expect(row.className).toContain("relative");
  });
});
