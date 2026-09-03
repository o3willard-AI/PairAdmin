import { describe, it, expect, beforeEach } from "vitest";
import {
  useQuickSelectStore,
  setQuickSelectBadges,
  clearQuickSelectBadges,
} from "@/stores/quickSelectStore";

describe("quickSelectStore", () => {
  beforeEach(() => {
    useQuickSelectStore.setState({
      visible: false,
      commandFkeys: {},
      terminalFkeys: {},
    });
  });

  it("starts hidden with no badge assignments", () => {
    const s = useQuickSelectStore.getState();
    expect(s.visible).toBe(false);
    expect(s.commandFkeys).toEqual({});
    expect(s.terminalFkeys).toEqual({});
  });

  it("setQuickSelectBadges writes visible plus per-id F-key maps", () => {
    setQuickSelectBadges(
      true,
      { "cmd-1": "F1", "cmd-2": "F2" },
      { "tab-1": "F3" }
    );

    const s = useQuickSelectStore.getState();
    expect(s.visible).toBe(true);
    expect(s.commandFkeys).toEqual({ "cmd-1": "F1", "cmd-2": "F2" });
    expect(s.terminalFkeys).toEqual({ "tab-1": "F3" });
  });

  it("clearQuickSelectBadges hides and empties both maps", () => {
    setQuickSelectBadges(true, { "cmd-1": "F1" }, { "tab-1": "F2" });
    clearQuickSelectBadges();

    const s = useQuickSelectStore.getState();
    expect(s.visible).toBe(false);
    expect(s.commandFkeys).toEqual({});
    expect(s.terminalFkeys).toEqual({});
  });
});
