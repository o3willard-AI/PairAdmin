import { describe, it, expect, beforeEach, vi } from "vitest";
import type { Terminal } from "@xterm/xterm";
import { useTerminalStore } from "@/stores/terminalStore";

describe("terminalStore", () => {
  beforeEach(() => {
    useTerminalStore.setState({ tabs: [], activeTabId: "" });
  });

  // --- Initial state ---
  it("initial state has empty tabs array", () => {
    const { tabs } = useTerminalStore.getState();
    expect(tabs).toHaveLength(0);
  });

  it("initial activeTabId is empty string", () => {
    expect(useTerminalStore.getState().activeTabId).toBe("");
  });

  // --- setActiveTab ---
  it("setActiveTab changes activeTabId", () => {
    useTerminalStore.getState().setActiveTab("some-id");
    expect(useTerminalStore.getState().activeTabId).toBe("some-id");
  });

  // --- addTab ---
  it("addTab adds a tab to the tabs array", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    const { tabs } = useTerminalStore.getState();
    expect(tabs).toHaveLength(1);
    expect(tabs[0]).toEqual({ id: "%0", name: "main:0.0" });
  });

  it("addTab called twice creates two tabs", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    expect(useTerminalStore.getState().tabs).toHaveLength(2);
  });

  it("addTab with duplicate ID does not create second entry", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%0", "main:0.0");
    expect(useTerminalStore.getState().tabs).toHaveLength(1);
  });

  it("addTab sets activeTabId when tabs were empty (first tab becomes active)", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    expect(useTerminalStore.getState().activeTabId).toBe("%0");
  });

  it("addTab does NOT change activeTabId when tabs already exist", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    expect(useTerminalStore.getState().activeTabId).toBe("%0");
  });

  // --- addTab with remote metadata ---
  it("addTab stores host, port, and remote metadata for a remote tab", () => {
    const remote = {
      host: "10.0.1.5",
      port: 22,
      username: "ubuntu",
      authType: "privatekey" as const,
      privateKeyPath: "/home/user/.ssh/id_ed25519",
      useTmux: true,
      tmuxSessionName: "work",
    };
    useTerminalStore.getState().addTab(
      "ssh:1",
      "ubuntu@10.0.1.5",
      false,
      undefined,
      "ssh",
      "host-id-1",
      "10.0.1.5",
      22,
      remote
    );
    const tab = useTerminalStore.getState().tabs[0];
    expect(tab.host).toBe("10.0.1.5");
    expect(tab.port).toBe(22);
    expect(tab.remote).toEqual(remote);
  });

  it("addTab for a local tab leaves host, port, and remote undefined", () => {
    useTerminalStore.getState().addTab("%0", "Terminal 1");
    const tab = useTerminalStore.getState().tabs[0];
    expect(tab.host).toBeUndefined();
    expect(tab.port).toBeUndefined();
    expect(tab.remote).toBeUndefined();
  });

  // --- setSavedHostId ---
  it("setSavedHostId updates savedHostId on the targeted tab only", () => {
    useTerminalStore.getState().addTab("ssh:1", "m[a]", false, undefined, "ssh");
    useTerminalStore.getState().addTab("ssh:2", "m[b]", false, undefined, "ssh", "old-id");
    useTerminalStore.getState().setSavedHostId("ssh:1", "new-host-id");
    const { tabs } = useTerminalStore.getState();
    expect(tabs.find((t) => t.id === "ssh:1")?.savedHostId).toBe("new-host-id");
    // The other tab's savedHostId is untouched.
    expect(tabs.find((t) => t.id === "ssh:2")?.savedHostId).toBe("old-id");
  });

  it("setSavedHostId on an unknown id is a no-op", () => {
    useTerminalStore.getState().addTab("ssh:1", "m[a]", false, undefined, "ssh", "orig");
    useTerminalStore.getState().setSavedHostId("does-not-exist", "ignored");
    expect(useTerminalStore.getState().tabs[0].savedHostId).toBe("orig");
  });

  // --- removeTab ---
  it("removeTab removes a tab from the array", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    useTerminalStore.getState().removeTab("%0");
    const { tabs } = useTerminalStore.getState();
    expect(tabs).toHaveLength(1);
    expect(tabs[0].id).toBe("%1");
  });

  it("removeTab on active tab auto-switches activeTabId to first remaining tab", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    // %0 is active (first tab)
    useTerminalStore.getState().removeTab("%0");
    expect(useTerminalStore.getState().activeTabId).toBe("%1");
  });

  it("removeTab on last tab sets activeTabId to empty string", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().removeTab("%0");
    expect(useTerminalStore.getState().activeTabId).toBe("");
    expect(useTerminalStore.getState().tabs).toHaveLength(0);
  });

  it("removeTab on non-active tab does not change activeTabId", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    // %0 is active
    useTerminalStore.getState().removeTab("%1");
    expect(useTerminalStore.getState().activeTabId).toBe("%0");
  });

  // --- clearTabs ---
  it("clearTabs resets tabs to empty array and activeTabId to empty string", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    useTerminalStore.getState().clearTabs();
    expect(useTerminalStore.getState().tabs).toHaveLength(0);
    expect(useTerminalStore.getState().activeTabId).toBe("");
  });

  // --- termRefsMap cleanup ---
  it("removeTab also removes term ref from termRefsMap", () => {
    const mockTerm = { dispose: vi.fn() } as unknown as Terminal;
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().setTermRef("%0", mockTerm);
    // Confirm ref was stored
    expect(useTerminalStore.getState().getTermRef("%0")).toBe(mockTerm);
    // Remove the tab
    useTerminalStore.getState().removeTab("%0");
    // Term ref should be gone
    expect(useTerminalStore.getState().getTermRef("%0")).toBeUndefined();
  });

  // --- renameTab ---
  it("renameTab changes the name of the targeted tab only", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    useTerminalStore.getState().renameTab("%0", "My Custom Name");
    const { tabs } = useTerminalStore.getState();
    expect(tabs.find((t) => t.id === "%0")?.name).toBe("My Custom Name");
    expect(tabs.find((t) => t.id === "%1")?.name).toBe("main:0.1");
  });

  it("renameTab on an unknown id is a no-op", () => {
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().renameTab("does-not-exist", "ignored");
    expect(useTerminalStore.getState().tabs).toEqual([
      expect.objectContaining({ id: "%0", name: "main:0.0" }),
    ]);
  });

  it("clearTabs removes all term refs from termRefsMap", () => {
    const mockTerm0 = { dispose: vi.fn() } as unknown as Terminal;
    const mockTerm1 = { dispose: vi.fn() } as unknown as Terminal;
    useTerminalStore.getState().addTab("%0", "main:0.0");
    useTerminalStore.getState().addTab("%1", "main:0.1");
    useTerminalStore.getState().setTermRef("%0", mockTerm0);
    useTerminalStore.getState().setTermRef("%1", mockTerm1);
    useTerminalStore.getState().clearTabs();
    expect(useTerminalStore.getState().getTermRef("%0")).toBeUndefined();
    expect(useTerminalStore.getState().getTermRef("%1")).toBeUndefined();
  });
});
