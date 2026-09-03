import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { CommandCard } from "@/components/sidebar/CommandCard";
import { useCommandStore, type Command } from "@/stores/commandStore";
import { useQuickSelectStore, setQuickSelectBadges } from "@/stores/quickSelectStore";

vi.mock("../../../wailsjs/go/services/PTYService", () => ({
  WriteInput: vi.fn(() => Promise.resolve()),
}));

const command: Command = {
  id: "cmd-1",
  command: "kubectl get pods",
  originalQuestion: "",
  timestamp: 0,
  tabId: "seed",
  pinned: true,
};

function renderCard() {
  return render(
    <CommandCard command={command} onCopy={vi.fn()} onExecute={vi.fn()} />
  );
}

beforeEach(() => {
  useCommandStore.setState({ commands: [command] });
  useQuickSelectStore.setState({
    visible: false,
    commandFkeys: {},
    terminalFkeys: {},
  });
});

describe("CommandCard — quick-select F-key badge", () => {
  it("shows no F-key badge while quick-select is inactive", () => {
    renderCard();
    expect(screen.queryByText("F1")).not.toBeInTheDocument();
  });

  it("renders its OWN F-key badge on the row when its id is assigned", () => {
    setQuickSelectBadges(true, { "cmd-1": "F3" }, {});
    renderCard();

    const badge = screen.getByText("F3");
    expect(badge).toBeInTheDocument();
    expect(badge.getAttribute("aria-hidden")).toBe("true");
  });

  it("badge is signage only: aria-hidden and pointer-events-none", () => {
    setQuickSelectBadges(true, { "cmd-1": "F2" }, {});
    renderCard();

    const badge = screen.getByText("F2");
    expect(badge.getAttribute("aria-hidden")).toBe("true");
    expect(badge.className).toContain("pointer-events-none");
  });

  it("shows no badge when quick-select is visible but this card has no F-key", () => {
    // e.g. the 13th item beyond the F1..F12 cap, or an unpinned card
    setQuickSelectBadges(true, { "other-cmd": "F1" }, {});
    renderCard();

    expect(screen.queryByText("F1")).not.toBeInTheDocument();
  });

  it("card root is position:relative so the absolute badge anchors to the card", () => {
    setQuickSelectBadges(true, { "cmd-1": "F1" }, {});
    renderCard();

    const card = screen.getByTestId("command-card");
    expect(card.className).toContain("relative");
  });
});
