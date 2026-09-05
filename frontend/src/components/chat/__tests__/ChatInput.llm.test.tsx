import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { ChatInput } from "@/components/chat/ChatInput";
import { useSettingsStore } from "@/stores/settingsStore";

describe("ChatInput — LLM disabled state", () => {
  beforeEach(() => {
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "checking",
    });
  });

  it("keeps the textarea enabled with the normal placeholder when the LLM is not disabled", () => {
    render(<ChatInput onSend={vi.fn()} />);

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(textarea.disabled).toBe(false);
    expect(textarea.placeholder).toMatch(/Ask about the terminal output/);
  });

  it("disables the textarea with a 'Pair LLM disabled' placeholder when connectionStatus is 'disabled'", () => {
    useSettingsStore.setState({ connectionStatus: "disabled" });
    render(<ChatInput onSend={vi.fn()} />);

    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(textarea.disabled).toBe(true);
    expect(textarea.placeholder).toMatch(/Pair LLM disabled in Settings/i);
  });
});
