import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { ChatInput } from "@/components/chat/ChatInput";

describe("ChatInput", () => {
  it("renders with placeholder text", () => {
    render(<ChatInput onSend={vi.fn()} />);
    expect(
      screen.getByPlaceholderText(
        /Ask about the terminal output.*Enter to send/
      )
    ).toBeInTheDocument();
  });

  it("calls onSend with trimmed text when Enter is pressed", async () => {
    const onSend = vi.fn();
    const user = userEvent.setup();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    await user.click(textarea);
    await user.type(textarea, "  hello world  ");
    await user.keyboard("{Enter}");

    expect(onSend).toHaveBeenCalledWith("hello world");
    expect(onSend).toHaveBeenCalledTimes(1);
  });

  it("does NOT call onSend when Shift+Enter is pressed", async () => {
    const onSend = vi.fn();
    const user = userEvent.setup();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    await user.click(textarea);
    await user.type(textarea, "hello");
    await user.keyboard("{Shift>}{Enter}{/Shift}");

    expect(onSend).not.toHaveBeenCalled();
  });

  it("does NOT call onSend when Enter is pressed with empty input", async () => {
    const onSend = vi.fn();
    const user = userEvent.setup();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox");
    await user.click(textarea);
    await user.keyboard("{Enter}");

    expect(onSend).not.toHaveBeenCalled();
  });

  it("clears the textarea value after sending", async () => {
    const onSend = vi.fn();
    const user = userEvent.setup();
    render(<ChatInput onSend={onSend} />);

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    await user.click(textarea);
    await user.type(textarea, "hello");
    await user.keyboard("{Enter}");

    expect(textarea.value).toBe("");
  });
});
