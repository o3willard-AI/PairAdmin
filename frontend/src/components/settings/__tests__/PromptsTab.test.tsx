import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { PromptsTab } from "@/components/settings/PromptsTab";

const getSettings = vi.fn();
const saveSettings = vi.fn();
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  SaveSettings: (...args: unknown[]) => saveSettings(...args),
}));

beforeEach(() => {
  getSettings.mockReset().mockResolvedValue({});
  saveSettings.mockReset().mockResolvedValue(undefined);
});

describe("PromptsTab — custom prompt editing", () => {
  it("shows the built-in system prompt as read-only text", async () => {
    render(<PromptsTab />);
    expect(
      await screen.findByText(/You are PairAdmin, an AI assistant/)
    ).toBeInTheDocument();
  });

  it("pre-fills the custom prompt from stored settings when one is present", async () => {
    getSettings.mockResolvedValue({ CustomPrompt: "Always use --no-color" });
    render(<PromptsTab />);

    expect(
      await screen.findByDisplayValue("Always use --no-color")
    ).toBeInTheDocument();
  });

  it("edits the custom prompt textarea", async () => {
    const user = userEvent.setup();
    render(<PromptsTab />);
    const textarea = await screen.findByPlaceholderText(
      "Add custom instructions to extend the system prompt..."
    );

    await user.type(textarea, "Be concise and verify with safe commands.");

    expect(
      screen.getByDisplayValue("Be concise and verify with safe commands.")
    ).toBeInTheDocument();
  });

  it("saves the custom prompt on Save and surfaces 'Saved!'", async () => {
    const user = userEvent.setup();
    render(<PromptsTab />);
    const textarea = await screen.findByPlaceholderText(
      "Add custom instructions to extend the system prompt..."
    );

    await user.type(textarea, "Prefer idempotent commands.");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ CustomPrompt: "Prefer idempotent commands." })
    );
    expect(await screen.findByText("Saved!")).toBeInTheDocument();
  });

  it("shows 'Save failed' when persisting throws", async () => {
    const user = userEvent.setup();
    saveSettings.mockRejectedValue(new Error("disk error"));
    render(<PromptsTab />);
    await screen.findByPlaceholderText(
      "Add custom instructions to extend the system prompt..."
    );

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(await screen.findByText("Save failed")).toBeInTheDocument();
  });
});