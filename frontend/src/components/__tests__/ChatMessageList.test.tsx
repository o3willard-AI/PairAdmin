import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { ChatMessageList } from "../chat/ChatMessageList";

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

// Mock commandStore for CodeBlock dependency
vi.mock("@/stores/commandStore", () => ({
  useCommandStore: {
    getState: vi.fn(() => ({ addCommand: vi.fn() })),
  },
}));

// Helper to set up chatStore messages
import { useChatStore } from "@/stores/chatStore";

function setupMessages(messages: Parameters<typeof useChatStore.setState>[0]) {
  useChatStore.setState(messages);
}

describe("ChatMessageList", () => {
  beforeEach(() => {
    // Reset store state
    useChatStore.setState({
      messagesByTab: {
        "bash-1": [],
      },
    });
  });

  it("renders user messages on the right side (justify-end)", () => {
    useChatStore.setState({
      messagesByTab: {
        "bash-1": [
          { id: "1", role: "user", content: "Hello AI", isStreaming: false },
        ],
      },
    });

    const { container } = render(<ChatMessageList />);
    const msgWrapper = container.querySelector(".justify-end");
    expect(msgWrapper).toBeInTheDocument();
    expect(msgWrapper).toHaveTextContent("Hello AI");
  });

  it("renders assistant messages on the left side (justify-start)", () => {
    useChatStore.setState({
      messagesByTab: {
        "bash-1": [
          { id: "1", role: "assistant", content: "Hello user", isStreaming: false },
        ],
      },
    });

    const { container } = render(<ChatMessageList />);
    const msgWrapper = container.querySelector(".justify-start");
    expect(msgWrapper).toBeInTheDocument();
    expect(msgWrapper).toHaveTextContent("Hello user");
  });

  it("shows ▋ in content for messages with isStreaming=true", () => {
    useChatStore.setState({
      messagesByTab: {
        "bash-1": [
          { id: "1", role: "assistant", content: "Streaming text▋", isStreaming: true },
        ],
      },
    });

    render(<ChatMessageList />);
    expect(screen.getByText(/▋/)).toBeInTheDocument();
  });

  it("applies error styling for messages with isError=true", () => {
    useChatStore.setState({
      messagesByTab: {
        "bash-1": [
          { id: "1", role: "assistant", content: "Something went wrong", isStreaming: false, isError: true },
        ],
      },
    });

    const { container } = render(<ChatMessageList />);
    // Error styling: amber background
    const errorBubble = container.querySelector(".bg-amber-950\\/50");
    expect(errorBubble).toBeInTheDocument();
  });

  it("renders CodeBlock component for fenced code blocks (not raw text)", () => {
    useChatStore.setState({
      messagesByTab: {
        "bash-1": [
          {
            id: "1",
            role: "assistant",
            content: "```typescript\nconst x = 1\n```",
            isStreaming: false,
          },
        ],
      },
    });

    render(<ChatMessageList />);
    // react-shiki mock renders as pre[data-testid="code-highlight"]
    expect(screen.getByTestId("code-highlight")).toBeInTheDocument();
    expect(screen.getByTestId("code-highlight")).toHaveTextContent("const x = 1");
  });

  it("renders bold text (**bold**) as <strong> element", () => {
    useChatStore.setState({
      messagesByTab: {
        "bash-1": [
          { id: "1", role: "assistant", content: "This is **bold** text", isStreaming: false },
        ],
      },
    });

    const { container } = render(<ChatMessageList />);
    const strong = container.querySelector("strong");
    expect(strong).toBeInTheDocument();
    expect(strong).toHaveTextContent("bold");
  });
});
