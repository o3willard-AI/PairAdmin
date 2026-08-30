import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { CommandSidebar } from "@/components/sidebar/CommandSidebar";
import { useCommandStore } from "@/stores/commandStore";
import { useTerminalStore } from "@/stores/terminalStore";

describe("CommandSidebar — Add Command", () => {
  beforeEach(() => {
    useCommandStore.setState({ commands: [] });
    useTerminalStore.setState({
      tabs: [{ id: "tab-1", name: "main" }],
      activeTabId: "tab-1",
    });
  });

  it("opens an empty Add Command dialog and adds a freehand-typed command to the active tab", async () => {
    const user = userEvent.setup();
    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /add command/i }));

    const input = screen.getByRole("textbox");
    expect(input).toHaveValue("");

    await user.type(input, "echo hand-typed");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    const commands = useCommandStore.getState().commands;
    expect(commands).toHaveLength(1);
    expect(commands[0].command).toBe("echo hand-typed");
    expect(commands[0].tabId).toBe("tab-1");
    expect(commands[0].pinned).toBe(false);
  });

  it("does not add a command when Cancel is clicked", async () => {
    const user = userEvent.setup();
    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /add command/i }));
    await user.type(screen.getByRole("textbox"), "echo should-not-save");
    await user.click(screen.getByRole("button", { name: /cancel/i }));

    expect(useCommandStore.getState().commands).toHaveLength(0);
  });

  it("disables Save while the field is empty", async () => {
    const user = userEvent.setup();
    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /add command/i }));

    expect(screen.getByRole("button", { name: /^save$/i })).toBeDisabled();
  });
});
