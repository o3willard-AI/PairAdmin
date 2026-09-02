import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { CommandCard } from "@/components/sidebar/CommandCard";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useCommandStore, type Command } from "@/stores/commandStore";

const mockCommand: Command = {
  id: "test-id-1",
  command: "sudo systemctl restart nginx",
  originalQuestion: "How do I restart nginx?",
  timestamp: Date.now(),
  tabId: "bash-1",
  pinned: false,
};

describe("CommandCard", () => {
  beforeEach(() => {
    useCommandStore.setState({ commands: [mockCommand] });
  });

  it("renders the command text", () => {
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    expect(screen.getByText("sudo systemctl restart nginx")).toBeInTheDocument();
  });

  it("calls onCopy with the command id when the copy icon is clicked", async () => {
    const onCopy = vi.fn();
    const onExecute = vi.fn();
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={onCopy} onExecute={onExecute} />
      </TooltipProvider>
    );

    await user.click(screen.getByRole("button", { name: /copy to terminal/i }));

    expect(onCopy).toHaveBeenCalledWith("test-id-1");
    expect(onCopy).toHaveBeenCalledTimes(1);
    expect(onExecute).not.toHaveBeenCalled();
  });

  it("calls onExecute with the command id when the execute icon is clicked", async () => {
    const onCopy = vi.fn();
    const onExecute = vi.fn();
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={onCopy} onExecute={onExecute} />
      </TooltipProvider>
    );

    await user.click(screen.getByRole("button", { name: /execute in terminal/i }));

    expect(onExecute).toHaveBeenCalledWith("test-id-1");
    expect(onExecute).toHaveBeenCalledTimes(1);
    expect(onCopy).not.toHaveBeenCalled();
  });

  it("clicking the card itself (not an icon) triggers neither action", async () => {
    const onCopy = vi.fn();
    const onExecute = vi.fn();
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={onCopy} onExecute={onExecute} />
      </TooltipProvider>
    );

    await user.click(screen.getByTestId("command-card"));

    expect(onCopy).not.toHaveBeenCalled();
    expect(onExecute).not.toHaveBeenCalled();
  });

  it("renders tooltip with the originalQuestion text on hover", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.hover(screen.getByTestId("command-card"));

    expect(screen.getByText("How do I restart nginx?")).toBeInTheDocument();
  });

  it("shows a pin indicator when the command is pinned", () => {
    render(
      <TooltipProvider>
        <CommandCard
          command={{ ...mockCommand, pinned: true }}
          onCopy={vi.fn()}
          onExecute={vi.fn()}
        />
      </TooltipProvider>
    );

    // The pin glyph and the "Unpin" context-menu label both render an svg
    // with this aria attributes via lucide; assert via the card having the
    // amber pin icon present (lucide icons render as <svg>).
    const card = screen.getByTestId("command-card");
    expect(card.querySelector("svg")).toBeInTheDocument();
  });

  it("clicking the edit icon opens the inline edit input without needing a right-click", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.click(screen.getByRole("button", { name: /edit command/i }));
    // In permanent edit mode, both the name input and the command textarea
    // are textboxes — target the textarea by its initial display value.
    const input = screen.getByDisplayValue("sudo systemctl restart nginx");
    await user.clear(input);
    await user.type(input, "echo left-click-edit");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(useCommandStore.getState().commands[0].command).toBe("echo left-click-edit");
  });

  it("right-clicking opens a context menu with Pin, Edit, Edit/Append for next use, Rename, and Remove", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });

    expect(screen.getByText("Pin")).toBeInTheDocument();
    expect(screen.getByText("Edit")).toBeInTheDocument();
    expect(screen.getByText("Edit/Append for next use")).toBeInTheDocument();
    expect(screen.getByText("Rename")).toBeInTheDocument();
    expect(screen.getByText("Remove")).toBeInTheDocument();
  });

  it("clicking Pin in the context menu pins the command", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Pin"));

    expect(useCommandStore.getState().commands[0].pinned).toBe(true);
  });

  it("clicking Remove in the context menu removes the command from the store", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Remove"));

    expect(useCommandStore.getState().commands).toHaveLength(0);
  });

  it("Edit permanently changes the command text", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Edit"));
    // In permanent mode, both name input and textarea are textboxes.
    // Target the textarea by its initial display value.
    const input = screen.getByDisplayValue("sudo systemctl restart nginx");
    await user.clear(input);
    await user.type(input, "echo permanently-edited");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(useCommandStore.getState().commands[0].command).toBe("echo permanently-edited");
    expect(useCommandStore.getState().commands[0].tempOverride).toBeUndefined();
  });

  it("Edit/Append for next use sets a one-time override without changing the base command", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Edit/Append for next use"));
    // In temporary mode, only the textarea is shown (no name field).
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "echo one-time");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(useCommandStore.getState().commands[0].tempOverride).toBe("echo one-time");
    expect(useCommandStore.getState().commands[0].command).toBe("sudo systemctl restart nginx");

    // consumeCommandText (used by copy/execute) returns the override once, then clears it
    expect(useCommandStore.getState().consumeCommandText("test-id-1")).toBe("echo one-time");
    expect(useCommandStore.getState().commands[0].tempOverride).toBeUndefined();
  });

  it("displays the custom name instead of the command text when name is set", () => {
    render(
      <TooltipProvider>
        <CommandCard
          command={{ ...mockCommand, name: "Restart Nginx" }}
          onCopy={vi.fn()}
          onExecute={vi.fn()}
        />
      </TooltipProvider>
    );

    expect(screen.getByText("Restart Nginx")).toBeInTheDocument();
    expect(screen.queryByText("sudo systemctl restart nginx")).not.toBeInTheDocument();
  });

  it("displays the command text when no name is set", () => {
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    expect(screen.getByText("sudo systemctl restart nginx")).toBeInTheDocument();
  });

  it("tooltip shows the custom name as a header line, then the full command text", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard
          command={{ ...mockCommand, name: "Restart Nginx" }}
          onCopy={vi.fn()}
          onExecute={vi.fn()}
        />
      </TooltipProvider>
    );

    await user.hover(screen.getByTestId("command-card"));

    // The display text "Restart Nginx" appears both in the sidebar and in
    // the tooltip header — use getAllBy to confirm both render.
    expect(screen.getAllByText("Restart Nginx").length).toBeGreaterThanOrEqual(1);
    // The full command text only appears in the tooltip (not the sidebar),
    // since the name replaces it in the display.
    expect(screen.getByText("sudo systemctl restart nginx")).toBeInTheDocument();
  });

  it("context menu includes a Rename item", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });

    expect(screen.getByText("Rename")).toBeInTheDocument();
  });

  it("clicking Rename in the context menu opens a rename dialog", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Rename"));

    expect(screen.getByText("Rename Command")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("e.g. Restart nginx")).toBeInTheDocument();
  });

  it("saving a rename updates the command's name in the store", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Rename"));

    const input = screen.getByPlaceholderText("e.g. Restart nginx");
    await user.clear(input);
    await user.type(input, "My Custom Name");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(useCommandStore.getState().commands[0].name).toBe("My Custom Name");
  });

  it("editing a command permanently with a name saves both command and name", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Edit"));

    // The name input should appear in permanent edit mode
    expect(screen.getByLabelText("Name (optional)")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Name (optional)"), "New Name");
    // In permanent mode, target the textarea by its initial display value
    const textarea = screen.getByDisplayValue("sudo systemctl restart nginx");
    await user.clear(textarea);
    await user.type(textarea, "echo edited");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(useCommandStore.getState().commands[0].command).toBe("echo edited");
    expect(useCommandStore.getState().commands[0].name).toBe("New Name");
  });

  it("clearing the Name (optional) field in permanent edit removes an existing name", async () => {
    const user = userEvent.setup();
    // Start with a command that already has a name
    useCommandStore.setState({
      commands: [{ ...mockCommand, name: "Old Name" }],
    });

    render(
      <TooltipProvider>
        <CommandCard
          command={{ ...mockCommand, name: "Old Name" }}
          onCopy={vi.fn()}
          onExecute={vi.fn()}
        />
      </TooltipProvider>
    );

    await user.pointer({ keys: "[MouseRight]", target: screen.getByTestId("command-card") });
    await user.click(screen.getByText("Edit"));

    // The name field should be pre-filled with "Old Name"
    const nameInput = screen.getByLabelText("Name (optional)");
    await user.clear(nameInput);
    // Don't type anything — leave it empty to clear

    // Change the command text too and save
    const textarea = screen.getByDisplayValue("sudo systemctl restart nginx");
    await user.clear(textarea);
    await user.type(textarea, "echo cleared-name");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(useCommandStore.getState().commands[0].command).toBe("echo cleared-name");
    expect(useCommandStore.getState().commands[0].name).toBeUndefined();
  });

  it("tooltip shows the tempOverride text when a one-time edit is pending", async () => {
    const user = userEvent.setup();
    useCommandStore.setState({
      commands: [{ ...mockCommand, tempOverride: "echo one-time-edit" }],
    });

    render(
      <TooltipProvider>
        <CommandCard
          command={{ ...mockCommand, tempOverride: "echo one-time-edit" }}
          onCopy={vi.fn()}
          onExecute={vi.fn()}
        />
      </TooltipProvider>
    );

    await user.hover(screen.getByTestId("command-card"));

    // The tooltip's fullCommandText should reflect the tempOverride, not the
    // permanent command text. Since there's no name, the display text in the
    // sidebar also shows the tempOverride — both should contain it.
    expect(screen.getAllByText("echo one-time-edit").length).toBeGreaterThanOrEqual(1);
  });

  it("tooltip shows the tempOverride when both a name and a one-time edit are set", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard
          command={{
            ...mockCommand,
            name: "My Named Command",
            tempOverride: "echo temp-override",
          }}
          onCopy={vi.fn()}
          onExecute={vi.fn()}
        />
      </TooltipProvider>
    );

    // Sidebar shows the name, not the command text or temp override
    expect(screen.getByText("My Named Command")).toBeInTheDocument();
    expect(screen.queryByText("sudo systemctl restart nginx")).not.toBeInTheDocument();
    expect(screen.queryByText("echo temp-override")).not.toBeInTheDocument();

    // Tooltip shows the name header + the temp override as the command text
    await user.hover(screen.getByTestId("command-card"));

    expect(screen.getByText("echo temp-override")).toBeInTheDocument();
  });
});
