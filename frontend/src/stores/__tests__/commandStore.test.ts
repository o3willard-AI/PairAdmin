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

  it("clearAll empties unpinned commands but keeps pinned ones", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "cmd1", originalQuestion: "q1" });
    useCommandStore.getState().addCommand("tab-2", { command: "cmd2", originalQuestion: "q2" });
    const pinnedId = useCommandStore.getState().commands[0].id;
    useCommandStore.getState().togglePin(pinnedId);

    useCommandStore.getState().clearAll();

    const remaining = useCommandStore.getState().commands;
    expect(remaining).toHaveLength(1);
    expect(remaining[0].command).toBe("cmd1");
    expect(remaining[0].pinned).toBe(true);
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

  it("addPinnedCommand adds a command that is already pinned", () => {
    useCommandStore.getState().addPinnedCommand("ssh:tab-1", {
      command: "tmux set -g mouse on",
      originalQuestion: "Enables mouse-wheel scrolling inside tmux (auto-added)",
    });
    const cmds = useCommandStore.getState().commands;
    expect(cmds).toHaveLength(1);
    expect(cmds[0].command).toBe("tmux set -g mouse on");
    expect(cmds[0].pinned).toBe(true);
  });

  it("addPinnedCommand does not duplicate an existing pinned command with the same text", () => {
    useCommandStore.getState().addPinnedCommand("ssh:tab-1", {
      command: "tmux set -g mouse on",
      originalQuestion: "first",
    });
    // e.g. reconnecting to the same tmux host again
    useCommandStore.getState().addPinnedCommand("ssh:tab-2", {
      command: "tmux set -g mouse on",
      originalQuestion: "second",
    });
    expect(useCommandStore.getState().commands).toHaveLength(1);
  });

  it("addPinnedCommand adds a new entry if an unpinned command happens to share the same text", () => {
    useCommandStore.getState().addCommand("tab-1", { command: "tmux set -g mouse on", originalQuestion: "q" });
    useCommandStore.getState().addPinnedCommand("tab-1", {
      command: "tmux set -g mouse on",
      originalQuestion: "auto-added",
    });
    const cmds = useCommandStore.getState().commands;
    expect(cmds).toHaveLength(2);
    expect(cmds.filter((c) => c.pinned)).toHaveLength(1);
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
