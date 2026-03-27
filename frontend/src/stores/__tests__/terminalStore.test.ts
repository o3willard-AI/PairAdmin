import { describe, it, expect, beforeEach } from "vitest";
import { useTerminalStore } from "@/stores/terminalStore";

describe("terminalStore", () => {
  beforeEach(() => {
    useTerminalStore.setState({
      tabs: [
        { id: "bash-1", name: "bash:1" },
        { id: "bash-2", name: "bash:2" },
      ],
      activeTabId: "bash-1",
    });
  });

  it("initial state has 2 tabs with ids bash-1 and bash-2", () => {
    const { tabs } = useTerminalStore.getState();
    expect(tabs).toHaveLength(2);
    expect(tabs[0].id).toBe("bash-1");
    expect(tabs[1].id).toBe("bash-2");
  });

  it("initial activeTabId is bash-1", () => {
    expect(useTerminalStore.getState().activeTabId).toBe("bash-1");
  });

  it("setActiveTab changes activeTabId", () => {
    useTerminalStore.getState().setActiveTab("bash-2");
    expect(useTerminalStore.getState().activeTabId).toBe("bash-2");
  });
});
