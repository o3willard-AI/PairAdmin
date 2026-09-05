import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { LLMConfigTab, isLoopbackHost } from "@/components/settings/LLMConfigTab";
import { useSettingsStore } from "@/stores/settingsStore";

const getSettings = vi.fn();
const saveSettings = vi.fn();
const saveAPIKey = vi.fn();
const setModel = vi.fn();
const testConnection = vi.fn();
const getApiKeyStatus = vi.fn();

// Resolves (from frontend/src/components/settings/) to
// frontend/wailsjs/go/services/SettingsService. From this test file
// (frontend/src/components/settings/__tests__/) that is
// ../../../../wailsjs/go/services/SettingsService — the same module
// LLMConfigTab.tsx and utils/settingsSync.ts dynamically import.
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  SaveSettings: (...args: unknown[]) => saveSettings(...args),
  GetAPIKeyStatus: (...args: unknown[]) => getApiKeyStatus(...args),
  SaveAPIKey: (...args: unknown[]) => saveAPIKey(...args),
  TestConnection: (...args: unknown[]) => testConnection(...args),
  SetModel: (...args: unknown[]) => setModel(...args),
}));

describe("LLMConfigTab — Disable Pair LLM", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to openai:gpt-4");
    getApiKeyStatus.mockReset().mockResolvedValue("");
    testConnection.mockReset().mockResolvedValue("Connected");
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

describe("LLMConfigTab — Ollama API key (remote servers)", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to ollama:llama3");
    getApiKeyStatus.mockReset().mockResolvedValue("");
    testConnection.mockReset().mockResolvedValue("Connected");
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "connected",
    });
  });

  it("shows an 'Ollama API key' field when the ollama provider is selected", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "ollama");

    expect(await screen.findByText("Ollama API key")).toBeInTheDocument();
  });

  it("persists the Ollama key to the keychain on save", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "ollama");
    await user.type(
      screen.getByLabelText("Ollama API key"),
      "sk-remote-ollama-key"
    );
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveAPIKey).toHaveBeenCalledWith("ollama", "sk-remote-ollama-key");
  });

  it("does not call SaveAPIKey when the Ollama key field is left empty (local instance)", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "ollama");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveAPIKey).not.toHaveBeenCalled();
  });

  it("updates the model field when the user types into it", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.type(screen.getByPlaceholderText(/e\.g\. gpt-4o/), "gpt-4o-mini");

    expect(screen.getByDisplayValue("gpt-4o-mini")).toBeInTheDocument();
  });

  it("updates the API key field when the user types into it", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.type(screen.getByPlaceholderText("Enter API key"), "sk-test-123");

    expect(screen.getByDisplayValue("sk-test-123")).toBeInTheDocument();
  });
});

describe("LLMConfigTab — remote Ollama privacy warning", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to ollama:llama3");
    getApiKeyStatus.mockReset().mockResolvedValue("");
    testConnection.mockReset().mockResolvedValue("Connected");
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "connected",
    });
  });

  it("warns when the Ollama host is remote (terminal output leaves the machine)", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "ollama");
    await user.type(
      screen.getByLabelText("Server URL"),
      "http://team-gpu-box.lan:11434"
    );

    expect(
      screen.getByText(/terminal output will be sent to a remote ollama server/i)
    ).toBeInTheDocument();
  });

  it("does NOT warn for localhost or 127.0.0.1 hosts", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "ollama");
    // Placeholder default (localhost) — leave the field untouched.
    expect(
      screen.queryByText(/terminal output will be sent to a remote ollama server/i)
    ).not.toBeInTheDocument();

    await user.type(screen.getByLabelText("Server URL"), "http://127.0.0.1:11434");
    expect(
      screen.queryByText(/terminal output will be sent to a remote ollama server/i)
    ).not.toBeInTheDocument();
  });

  it("does not warn for ::1 loopback", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "ollama");
    // userEvent.type chokes on ':' key-descriptor parsing, so set ::1 via a
    // paste-style change instead of typing it character by character.
    const input = screen.getByLabelText("Server URL");
    await user.click(input);
    await user.paste("http://[::1]:11434");

    expect(
      screen.queryByText(/terminal output will be sent to a remote ollama server/i)
    ).not.toBeInTheDocument();
  });

  it("does not warn for other providers (lmstudio)", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.selectOptions(screen.getByRole("combobox"), "lmstudio");
    await user.type(
      screen.getByLabelText("Server URL"),
      "http://some-lmstudio-box:1234/v1"
    );

    expect(
      screen.queryByText(/terminal output will be sent to a remote ollama server/i)
    ).not.toBeInTheDocument();
  });
});

describe("LLMConfigTab — isLoopbackHost", () => {
  it("treats empty host as loopback (default localhost)", () => {
    expect(isLoopbackHost("")).toBe(true);
    expect(isLoopbackHost("   ")).toBe(true);
  });

  it("treats localhost and 127.x addresses as loopback", () => {
    expect(isLoopbackHost("http://localhost:11434")).toBe(true);
    expect(isLoopbackHost("http://127.0.0.1:11434")).toBe(true);
    expect(isLoopbackHost("http://127.8.8.8:11434")).toBe(true);
  });

  it("treats IPv6 loopback (bracketed URL) as loopback", () => {
    expect(isLoopbackHost("http://[::1]:11434")).toBe(true);
  });

  it("treats a bare IPv6 literal as non-loopback (URL parsing fails closed)", () => {
    // A bare "::1" is not a parseable URL, so isLoopbackHost falls through to
    // the fail-closed path (returns false → the remote warning shows). This
    // documents the actual behavior, not the ideal one.
    expect(isLoopbackHost("::1")).toBe(false);
  });

  it("treats remote and unparseable hosts as non-loopback (fail closed)", () => {
    expect(isLoopbackHost("http://team-gpu-box.lan:11434")).toBe(false);
    expect(isLoopbackHost("not a url at all")).toBe(false);
  });
});

describe("LLMConfigTab — Test Connection", () => {
  it("shows a success message when the connection test resolves", async () => {
    const user = userEvent.setup();
    testConnection.mockResolvedValue("Connected to Ollama on http://localhost:11434");
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /test connection/i }));

    expect(
      await screen.findByText(/✓ Connected to Ollama/i)
    ).toBeInTheDocument();
  });

  it("shows a failure message when the connection test rejects", async () => {
    const user = userEvent.setup();
    testConnection.mockRejectedValue(new Error("connection refused"));
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /test connection/i }));

    // wailsErrorMessage surfaces the backend Error's own message verbatim.
    expect(await screen.findByText(/✗ connection refused/i)).toBeInTheDocument();
  });
});
