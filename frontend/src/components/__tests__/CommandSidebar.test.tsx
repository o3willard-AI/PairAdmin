import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { CommandSidebar } from "@/components/sidebar/CommandSidebar";
import { useCommandStore } from "@/stores/commandStore";
import { useTerminalStore } from "@/stores/terminalStore";

// Mock the Wails SettingsService binding so handleSavePinned can call
// SavePinnedCommands without a real Wails runtime.
const savePinnedCommandsMock = vi.fn();
vi.mock("../../../wailsjs/go/services/SettingsService", () => ({
  SavePinnedCommands: (...args: unknown[]) => savePinnedCommandsMock(...args),
}));

// Mock sendToTerminal so copy/execute don't try to load Wails PTY bindings.
vi.mock("@/utils/sendToTerminal", () => ({
  sendToTerminal: vi.fn(),
}));

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

    // In "add" mode, both the name input and the command textarea are textboxes.
    // Target the textarea by its placeholder (it starts empty).
    const inputs = screen.getAllByRole("textbox");
    expect(inputs).toHaveLength(2);

    // Find the textarea (rows is not set on input, textarea has rows={12})
    const textarea = inputs.find((el) => el.tagName.toLowerCase() === "textarea");
    expect(textarea).toBeDefined();
    expect(textarea).toHaveValue("");

    await user.type(textarea!, "echo hand-typed");
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
    const textarea = screen.getAllByRole("textbox").find((el) => el.tagName.toLowerCase() === "textarea");
    await user.type(textarea!, "echo should-not-save");
    await user.click(screen.getByRole("button", { name: /cancel/i }));

    expect(useCommandStore.getState().commands).toHaveLength(0);
  });

  it("disables Save while the field is empty", async () => {
    const user = userEvent.setup();
    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /add command/i }));

    expect(screen.getByRole("button", { name: /^save$/i })).toBeDisabled();
  });

  it("Add Command dialog includes a Name (optional) field that is saved with the command", async () => {
    const user = userEvent.setup();
    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /add command/i }));

    // The name input should be visible in "add" mode
    const nameInput = screen.getByLabelText("Name (optional)");
    expect(nameInput).toBeInTheDocument();

    await user.type(nameInput, "My Alias");
    const textarea = screen.getAllByRole("textbox").find((el) => el.tagName.toLowerCase() === "textarea");
    await user.type(textarea!, "echo named");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    const commands = useCommandStore.getState().commands;
    expect(commands).toHaveLength(1);
    expect(commands[0].command).toBe("echo named");
    expect(commands[0].name).toBe("My Alias");
  });
});

describe("CommandSidebar — Save Pinned", () => {
  beforeEach(() => {
    useCommandStore.setState({ commands: [] });
    useTerminalStore.setState({
      tabs: [{ id: "tab-1", name: "main" }],
      activeTabId: "tab-1",
    });
    savePinnedCommandsMock.mockReset();
    savePinnedCommandsMock.mockResolvedValue(undefined);
  });

  it("Save Pinned sends an array of pinned commands including Name", async () => {
    const user = userEvent.setup();

    // Pin a command with a name
    useCommandStore.getState().addPinnedCommand("tab-1", {
      command: "kubectl get pods",
      originalQuestion: "list pods",
      name: "List Pods",
    });

    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /save pinned/i }));

    await waitFor(() => {
      expect(savePinnedCommandsMock).toHaveBeenCalledTimes(1);
    });

    const args = savePinnedCommandsMock.mock.calls[0][0];
    expect(args).toHaveLength(1);
    expect(args[0]).toEqual({
      Command: "kubectl get pods",
      OriginalQuestion: "list pods",
      Name: "List Pods",
    });
  });

  it("Save Pinned sends Name as empty string when no custom name is set", async () => {
    const user = userEvent.setup();

    useCommandStore.getState().addPinnedCommand("tab-1", {
      command: "df -h",
      originalQuestion: "disk usage",
    });

    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /save pinned/i }));

    await waitFor(() => {
      expect(savePinnedCommandsMock).toHaveBeenCalledTimes(1);
    });

    const args = savePinnedCommandsMock.mock.calls[0][0];
    expect(args[0]).toEqual({
      Command: "df -h",
      OriginalQuestion: "disk usage",
      Name: "",
    });
  });

  it("Save Pinned shows Saved! status after successful save", async () => {
    const user = userEvent.setup();

    useCommandStore.getState().addPinnedCommand("tab-1", {
      command: "ls",
      originalQuestion: "",
      name: "List",
    });

    render(<CommandSidebar />);

    await user.click(screen.getByRole("button", { name: /save pinned/i }));

    await waitFor(() => {
      expect(screen.getByText("Saved!")).toBeInTheDocument();
    });
  });
});
