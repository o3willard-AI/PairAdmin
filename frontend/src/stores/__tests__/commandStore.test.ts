import { describe, it, expect, beforeEach } from "vitest";
import { useCommandStore } from "@/stores/commandStore";

describe("commandStore", () => {
  beforeEach(() => {
    useCommandStore.setState({ commands: [] });
  });

  it("addCommand adds to the shared commands list, unpinned by default", () => {
    useCommandStore.getState().addCommand("tab-1", {
      command: "ls -la",
      originalQuestion: "list files",
    });
    const cmds = useCommandStore.getState().commands;
    expect(cmds).toHaveLength(1);
    expect(cmds[0].command).toBe("ls -la");
    expect(cmds[0].originalQuestion).toBe("list files");
    expect(cmds[0].pinned).toBe(false);
  });

  it("commands have id, command, originalQuestion, timestamp, tabId, pinned fields", () => {
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
    expect(cmd).toHaveProperty("pinned");
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

  it("togglePin flips a command's pinned state", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    const id = useCommandStore.getState().commands[0].id;
    useCommandStore.getState().togglePin(id);
    expect(useCommandStore.getState().commands[0].pinned).toBe(true);
    useCommandStore.getState().togglePin(id);
    expect(useCommandStore.getState().commands[0].pinned).toBe(false);
  });

  it("removeCommand removes only the targeted command", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    useCommandStore.getState().addCommand("tab-1", { command: "cmd2", originalQuestion: "q2" });
    const idToRemove = useCommandStore.getState().commands[0].id;
    useCommandStore.getState().removeCommand(idToRemove);
    const remaining = useCommandStore.getState().commands;
    expect(remaining).toHaveLength(1);
    expect(remaining[0].command).toBe("cmd2");
  });

  it("editCommand permanently changes the command text", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    const id = useCommandStore.getState().commands[0].id;
    useCommandStore.getState().editCommand(id, "cmd1-edited");
    expect(useCommandStore.getState().commands[0].command).toBe("cmd1-edited");
  });

  it("editForNextUse sets a one-time override that consumeCommandText returns and clears", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    const id = useCommandStore.getState().commands[0].id;
    useCommandStore.getState().editForNextUse(id, "cmd1-temp");

    expect(useCommandStore.getState().commands[0].command).toBe("cmd1");
    expect(useCommandStore.getState().commands[0].tempOverride).toBe("cmd1-temp");

    expect(useCommandStore.getState().consumeCommandText(id)).toBe("cmd1-temp");
    expect(useCommandStore.getState().commands[0].tempOverride).toBeUndefined();

    // Second use falls back to the original, unedited command.
    expect(useCommandStore.getState().consumeCommandText(id)).toBe("cmd1");
  });

  it("reorderPinned moves a command to sit immediately before the target", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "a", originalQuestion: "" });
    useCommandStore.getState().addCommand("tab-1", { command: "b", originalQuestion: "" });
    useCommandStore.getState().addCommand("tab-1", { command: "c", originalQuestion: "" });
    const [a, b, c] = useCommandStore.getState().commands;

    // Move "c" to just before "a": expected order becomes c, a, b
    useCommandStore.getState().reorderPinned(c.id, a.id);
    expect(useCommandStore.getState().commands.map((cmd) => cmd.command)).toEqual(["c", "a", "b"]);
  });
});
