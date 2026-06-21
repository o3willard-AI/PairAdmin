import { describe, it, expect, beforeEach } from "vitest";
import { useCommandStore } from "@/stores/commandStore";

describe("commandStore", () => {
  beforeEach(() => {
    useCommandStore.setState({ commands: [] });
  });

  it("addCommand adds to the shared commands list", () => {
    useCommandStore.getState().addCommand("tab-1", {
      command: "ls -la",
      originalQuestion: "list files",
    });
    const cmds = useCommandStore.getState().commands;
    expect(cmds).toHaveLength(1);
    expect(cmds[0].command).toBe("ls -la");
    expect(cmds[0].originalQuestion).toBe("list files");
  });

  it("commands have id, command, originalQuestion, timestamp, tabId fields", () => {
    useCommandStore.getState().addCommand("tab-1", {
      command: "pwd",
      originalQuestion: "where am I?",
    });
    const cmd = useCommandStore.getState().commands[0];
    expect(cmd).toHaveProperty("id");
    expect(cmd).toHaveProperty("command");
    expect(cmd).toHaveProperty("originalQuestion");
    expect(cmd).toHaveProperty("timestamp");
    expect(cmd).toHaveProperty("tabId");
  });

  it("commands added from different tabs all appear in the same shared list", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    useCommandStore.getState().addCommand("tab-2", { command: "cmd2", originalQuestion: "q2" });
    const cmds = useCommandStore.getState().commands;
    expect(cmds).toHaveLength(2);
    expect(cmds.map((c) => c.command)).toEqual(["cmd1", "cmd2"]);
  });

  it("clearAll empties the entire shared list", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    useCommandStore.getState().addCommand("tab-2", { command: "cmd2", originalQuestion: "q2" });
    useCommandStore.getState().clearAll();
    expect(useCommandStore.getState().commands).toHaveLength(0);
  });
});
