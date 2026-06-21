import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { CommandCard } from "@/components/sidebar/CommandCard";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { Command } from "@/stores/commandStore";

const mockCommand: Command = {
  id: "test-id-1",
  command: "sudo systemctl restart nginx",
  originalQuestion: "How do I restart nginx?",
  timestamp: Date.now(),
  tabId: "bash-1",
};

describe("CommandCard", () => {
  it("renders the command text", () => {
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} onExecute={vi.fn()} />
      </TooltipProvider>
    );

    expect(screen.getByText("sudo systemctl restart nginx")).toBeInTheDocument();
  });

  it("calls onCopy with the command string when the copy icon is clicked", async () => {
    const onCopy = vi.fn();
    const onExecute = vi.fn();
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={onCopy} onExecute={onExecute} />
      </TooltipProvider>
    );

    await user.click(screen.getByRole("button", { name: /copy to terminal/i }));

    expect(onCopy).toHaveBeenCalledWith("sudo systemctl restart nginx");
    expect(onCopy).toHaveBeenCalledTimes(1);
    expect(onExecute).not.toHaveBeenCalled();
  });

  it("calls onExecute with the command string when the execute icon is clicked", async () => {
    const onCopy = vi.fn();
    const onExecute = vi.fn();
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={onCopy} onExecute={onExecute} />
      </TooltipProvider>
    );

    await user.click(screen.getByRole("button", { name: /execute in terminal/i }));

    expect(onExecute).toHaveBeenCalledWith("sudo systemctl restart nginx");
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
});
