import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom";
import { CodeBlock } from "../chat/CodeBlock";
import { useCommandStore } from "@/stores/commandStore";

// Mock react-shiki
vi.mock("react-shiki", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <pre data-testid="code-highlight">{children}</pre>
  ),
}));

// Mock terminalStore
vi.mock("@/stores/terminalStore", () => ({
  useTerminalStore: (selector: (s: { activeTabId: string }) => unknown) =>
    selector({ activeTabId: "bash-1" }),
}));

// Mock the PTY-write utility so we're testing CodeBlock's own wiring, not
// the Wails binding it ultimately calls.
const mockSendToTerminal = vi.fn();
vi.mock("@/utils/sendToTerminal", () => ({
  sendToTerminal: (...args: unknown[]) => mockSendToTerminal(...args),
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
    mockSendToTerminal.mockClear();
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

  it("clicking Copy to Terminal sends the code to the terminal without executing or logging it as a command", () => {
    render(<CodeBlock code="echo hello" language="bash" isStreaming={false} />);
    fireEvent.click(screen.getByRole("button", { name: /copy to terminal/i }));
    expect(mockSendToTerminal).toHaveBeenCalledWith("bash-1", "echo hello", false);
    expect(mockAddCommand).not.toHaveBeenCalled();
  });

  it("clicking Execute in Terminal sends and executes the code, without auto-logging it as a command", () => {
    render(<CodeBlock code="echo hello" language="bash" isStreaming={false} />);
    fireEvent.click(screen.getByRole("button", { name: /execute in terminal/i }));
    expect(mockSendToTerminal).toHaveBeenCalledWith("bash-1", "echo hello", true);
    // Execute and "Save to Commands" are independent now — auto-saving on
    // execute used to produce a duplicate entry when the user also saved
    // explicitly.
    expect(mockAddCommand).not.toHaveBeenCalled();
  });

  it("clicking Save to Commands logs the command without sending anything to the terminal", () => {
    render(<CodeBlock code="echo hello" language="bash" isStreaming={false} />);
    fireEvent.click(screen.getByRole("button", { name: /save to commands/i }));
    expect(mockAddCommand).toHaveBeenCalledWith(
      "bash-1",
      expect.objectContaining({ command: "echo hello" })
    );
    expect(mockSendToTerminal).not.toHaveBeenCalled();
  });

  it("language prop is passed to CodeHighlighter as-is", () => {
    render(<CodeBlock code="print('hi')" language="python" isStreaming={false} />);
    // Language label is shown in header
    expect(screen.getByText("python")).toBeInTheDocument();
  });

  it("does not show any action buttons for a 'text' language block", () => {
    render(<CodeBlock code="dir" language="text" isStreaming={false} />);
    expect(screen.queryByRole("button", { name: /copy to terminal/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /execute in terminal/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /save to commands/i })).not.toBeInTheDocument();
  });

  it("does not show any action buttons when no language is specified", () => {
    render(<CodeBlock code="dir" isStreaming={false} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("shows action buttons for an actionable language like bash", () => {
    render(<CodeBlock code="dir" language="bash" isStreaming={false} />);
    expect(screen.getByRole("button", { name: /copy to terminal/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /execute in terminal/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /save to commands/i })).toBeInTheDocument();
  });
});
