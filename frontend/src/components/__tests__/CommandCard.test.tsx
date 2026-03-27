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
        <CommandCard command={mockCommand} onCopy={vi.fn()} />
      </TooltipProvider>
    );

    expect(screen.getByText("sudo systemctl restart nginx")).toBeInTheDocument();
  });

  it("calls onCopy with the command string when clicked", async () => {
    const onCopy = vi.fn();
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={onCopy} />
      </TooltipProvider>
    );

    await user.click(screen.getAllByRole("button")[0]);

    expect(onCopy).toHaveBeenCalledWith("sudo systemctl restart nginx");
    expect(onCopy).toHaveBeenCalledTimes(1);
  });

  it("renders tooltip with the originalQuestion text", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <CommandCard command={mockCommand} onCopy={vi.fn()} />
      </TooltipProvider>
    );

    await user.hover(screen.getAllByRole("button")[0]);

    expect(screen.getByText("How do I restart nginx?")).toBeInTheDocument();
  });
});
