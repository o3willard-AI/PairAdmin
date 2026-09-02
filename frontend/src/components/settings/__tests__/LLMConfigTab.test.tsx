import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { LLMConfigTab } from "@/components/settings/LLMConfigTab";
import { useSettingsStore } from "@/stores/settingsStore";

const getSettings = vi.fn();
const saveSettings = vi.fn();
const saveAPIKey = vi.fn();
const setModel = vi.fn();

// Resolves (from frontend/src/components/settings/) to
// frontend/wailsjs/go/services/SettingsService. From this test file
// (frontend/src/components/settings/__tests__/) that is
// ../../../../wailsjs/go/services/SettingsService — the same module
// LLMConfigTab.tsx and utils/settingsSync.ts dynamically import.
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  SaveSettings: (...args: unknown[]) => saveSettings(...args),
  GetAPIKeyStatus: vi.fn(() => Promise.resolve("")),
  SaveAPIKey: (...args: unknown[]) => saveAPIKey(...args),
  TestConnection: vi.fn(() => Promise.resolve("Connected")),
  SetModel: (...args: unknown[]) => setModel(...args),
}));

describe("LLMConfigTab — Disable Pair LLM", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to openai:gpt-4");
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "checking",
    });
  });

  it("offers a 'Disable Pair LLM' option with value 'disabled' in the provider dropdown", () => {
    render(<LLMConfigTab onClose={vi.fn()} />);

    const option = screen.getByRole("option", { name: "Disable Pair LLM" });
    expect(option).toHaveValue("disabled");
  });

  it("hides the Model, Server URL, API Key, and Test Connection fields when 'Disable Pair LLM' is selected", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "disabled");

    expect(screen.queryByText("Model")).not.toBeInTheDocument();
    expect(screen.queryByText("Server URL")).not.toBeInTheDocument();
    expect(screen.queryByText("API Key")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /test connection/i })
    ).not.toBeInTheDocument();
    // Save must stay available so the disabled choice can actually be persisted
    expect(screen.getByRole("button", { name: /^save$/i })).toBeInTheDocument();
  });

  it("saving while disabled persists Provider 'disabled', sets the active model to 'disabled', and skips SetModel", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<LLMConfigTab onClose={onClose} />);

    await user.selectOptions(screen.getByRole("combobox"), "disabled");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ Provider: "disabled" })
    );
    // "disabled" is model-less — the provider:model SetModel format doesn't fit
    expect(setModel).not.toHaveBeenCalled();
    expect(useSettingsStore.getState().activeModel).toBe("disabled");
    // Surface the disabled state immediately (no app restart needed)
    expect(useSettingsStore.getState().connectionStatus).toBe("disabled");
    expect(onClose).toHaveBeenCalled();
  });

  it("saving a real provider after being disabled calls SetModel and releases the disabled status gate", async () => {
    const user = userEvent.setup();
    // Simulate the app currently in the disabled state
    useSettingsStore.setState({ connectionStatus: "disabled" });
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    // Default provider state is "openai" with an empty model
    expect(setModel).toHaveBeenCalledWith("openai:");
    // "disabled" must be released so stream done/error events drive the
    // status bar again (the startup probe only runs on mount)
    expect(useSettingsStore.getState().connectionStatus).toBe("disconnected");
  });
});
