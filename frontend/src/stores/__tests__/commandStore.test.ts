import { describe, it, expect, beforeEach } from "vitest";
import { useCommandStore } from "@/stores/commandStore";

describe("commandStore", () => {
  beforeEach(() => {
    useCommandStore.setState({ commandsByTab: {} });
  });

  it("addCommand adds to the specified tab", () => {
    useCommandStore.getState().addCommand("tab-1", {
      command: "ls -la",
      originalQuestion: "list files",
    });
    const cmds = useCommandStore.getState().commandsByTab["tab-1"];
    expect(cmds).toHaveLength(1);
    expect(cmds[0].command).toBe("ls -la");
    expect(cmds[0].originalQuestion).toBe("list files");
  });

  it("commands have id, command, originalQuestion, timestamp, tabId fields", () => {
    useCommandStore.getState().addCommand("tab-1", {
      command: "pwd",
      originalQuestion: "where am I?",
    });
    const cmd = useCommandStore.getState().commandsByTab["tab-1"][0];
    expect(cmd).toHaveProperty("id");
    expect(cmd).toHaveProperty("command");
    expect(cmd).toHaveProperty("originalQuestion");
    expect(cmd).toHaveProperty("timestamp");
    expect(cmd).toHaveProperty("tabId");
  });

  it("getCommandsForTab returns commands newest first", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "first", originalQuestion: "q1" });
    // small delay to ensure different timestamps
    const before = Date.now();
    while (Date.now() === before) {} // busy wait 1ms
    useCommandStore.getState().addCommand("tab-1", { command: "newest", originalQuestion: "q2" });
    const cmds = useCommandStore.getState().getCommandsForTab("tab-1");
    expect(cmds[0].command).toBe("newest");
    expect(cmds[1].command).toBe("first");
  });

  it("clearTab empties only the specified tab", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    useCommandStore.getState().addCommand("tab-2", { command: "cmd2", originalQuestion: "q2" });
    useCommandStore.getState().clearTab("tab-1");
    expect(useCommandStore.getState().commandsByTab["tab-1"]).toHaveLength(0);
    expect(useCommandStore.getState().commandsByTab["tab-2"]).toHaveLength(1);
  });
});
