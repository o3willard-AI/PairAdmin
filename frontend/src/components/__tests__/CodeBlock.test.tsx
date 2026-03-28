import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom";
import { CodeBlock } from "../chat/CodeBlock";
import { useCommandStore } from "@/stores/commandStore";

// Mock react-shiki
vi.mock("react-shiki", () => ({
  default: ({ code }: { code: string }) => (
    <pre data-testid="code-highlight">{code}</pre>
  ),
}));

// Mock terminalStore
vi.mock("@/stores/terminalStore", () => ({
  useTerminalStore: (selector: (s: { activeTabId: string }) => unknown) =>
    selector({ activeTabId: "bash-1" }),
}));

// Mock commandStore
const mockAddCommand = vi.fn();
vi.mock("@/stores/commandStore", () => ({
  useCommandStore: {
    getState: vi.fn(() => ({ addCommand: mockAddCommand })),
  },
}));

describe("CodeBlock", () => {
  beforeEach(() => {
    mockAddCommand.mockClear();
  });

  it("renders syntax-highlighted code block (react-shiki present in DOM)", () => {
    render(<CodeBlock code="const x = 1" language="typescript" isStreaming={false} />);
    expect(screen.getByTestId("code-highlight")).toBeInTheDocument();
    expect(screen.getByTestId("code-highlight")).toHaveTextContent("const x = 1");
  });

  it("does NOT show Copy to Terminal button while isStreaming=true", () => {
    render(<CodeBlock code="ls -la" language="bash" isStreaming={true} />);
    expect(screen.queryByRole("button", { name: /copy to terminal/i })).not.toBeInTheDocument();
  });

  it("shows Copy to Terminal button when isStreaming=false", () => {
    render(<CodeBlock code="ls -la" language="bash" isStreaming={false} />);
    expect(screen.getByRole("button", { name: /copy to terminal/i })).toBeInTheDocument();
  });

  it("clicking Copy to Terminal calls commandStore.addCommand with the code content", () => {
    render(<CodeBlock code="echo hello" language="bash" isStreaming={false} />);
    fireEvent.click(screen.getByRole("button", { name: /copy to terminal/i }));
    expect(mockAddCommand).toHaveBeenCalledWith(
      "bash-1",
      expect.objectContaining({ command: "echo hello" })
    );
  });

  it("language prop is passed to CodeHighlighter as-is", () => {
    render(<CodeBlock code="print('hi')" language="python" isStreaming={false} />);
    // Language label is shown in header
    expect(screen.getByText("python")).toBeInTheDocument();
  });
});
