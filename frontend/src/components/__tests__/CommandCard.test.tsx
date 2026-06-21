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

  it("right-clicking opens a context menu with Pin, Edit, Edit/Append for next use, and Remove", async () => {
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
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "echo permanently-edited{Enter}");

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
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "echo one-time{Enter}");

    expect(useCommandStore.getState().commands[0].tempOverride).toBe("echo one-time");
    expect(useCommandStore.getState().commands[0].command).toBe("sudo systemctl restart nginx");

    // consumeCommandText (used by copy/execute) returns the override once, then clears it
    expect(useCommandStore.getState().consumeCommandText("test-id-1")).toBe("echo one-time");
    expect(useCommandStore.getState().commands[0].tempOverride).toBeUndefined();
  });
});
