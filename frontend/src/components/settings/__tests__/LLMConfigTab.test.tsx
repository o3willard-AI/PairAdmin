import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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
const getLLMCatalog = vi.fn();

// Default catalog returned by the mocked GetLLMCatalog. Mirrors the backend's
// curated top-11: names for display, per-provider model lists (local providers
// expose none — they discover models at runtime).
const defaultCatalog = () => [
  {
    id: "openai",
    name: "OpenAI",
    adapter: "openai",
    models: [
      { id: "gpt-4o", name: "gpt-4o", context: 0, reasoning: true, toolCall: true },
      { id: "gpt-5.6-luna", name: "gpt-5.6-luna", context: 400000, reasoning: true, toolCall: true },
    ],
  },
  { id: "anthropic", name: "Anthropic", adapter: "anthropic", models: [] },
  { id: "google", name: "Google Gemini", adapter: "gemini", models: [] },
  { id: "deepseek", name: "DeepSeek", adapter: "openai", models: [] },
  { id: "xai", name: "xAI (Grok)", adapter: "openai", models: [] },
  { id: "openrouter", name: "OpenRouter", adapter: "openai", models: [] },
  { id: "mistral", name: "Mistral", adapter: "openai", models: [] },
  { id: "groq", name: "Groq", adapter: "openai", models: [] },
  { id: "glm", name: "Z-AI (GLM)", adapter: "openai", models: [] },
  { id: "ollama", name: "Ollama", adapter: "ollama", models: [] },
  { id: "lmstudio", name: "LM Studio", adapter: "openai", models: [] },
];

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
  GetLLMCatalog: (...args: unknown[]) => getLLMCatalog(...args),
}));

beforeEach(() => {
  // Every describe renders LLMConfigTab, which loads the catalog on mount —
  // give it the same default catalog unless a test overrides it.
  getLLMCatalog.mockReset().mockResolvedValue(defaultCatalog());
});

// selectProvider waits for the catalog to populate the provider dropdown
// (findByRole resolves once the option appears), THEN selects by id. Catalog
// options render asynchronously from GetLLMCatalog, so a bare selectOptions
// right after render races the catalog and fails with "value not found".
const selectProvider = async (
  user: ReturnType<typeof userEvent.setup>,
  value: string
) => {
  const optionName =
    value === "ollama" ? "Ollama" : value === "lmstudio" ? "LM Studio" : value;
  await screen.findByRole("option", { name: optionName });
  await user.selectOptions(screen.getByRole("combobox"), value);
};

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

  it("saving a real provider after being disabled releases the disabled status gate", async () => {
    const user = userEvent.setup();
    // Simulate the app currently in the disabled state
    useSettingsStore.setState({ connectionStatus: "disabled" });
    render(<LLMConfigTab onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    // Default provider state is "openai" with an empty model
    expect(setModel).toHaveBeenCalledWith("openai:");
    // "disabled" must be released so the status bar is driven again rather than
    // dead-ended on the stale opt-out. Since PA-TC this happens through the
    // post-save probe, which claims "checking" before it calls out — so the
    // assertion is that the status has MOVED, not that it equals any one value.
    // (This fixture's GetSettings resolves {}, so the probe's honest no-provider
    // answer is "disconnected"; the connected case is covered below.)
    await waitFor(() =>
      expect(useSettingsStore.getState().connectionStatus).not.toBe("disabled")
    );
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

    await selectProvider(user, "ollama");

    expect(await screen.findByText("Ollama API key")).toBeInTheDocument();
  });

  it("persists the Ollama key to the keychain on save", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await selectProvider(user, "ollama");
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

    await selectProvider(user, "ollama");
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

    await selectProvider(user, "ollama");
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

    await selectProvider(user, "ollama");
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

    await selectProvider(user, "ollama");
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

    await selectProvider(user, "lmstudio");
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

// Types a model into the model field, because the connection test is gated on
// a non-empty model for every provider except Ollama (PA-01). The two tests
// below used to click Test Connection with the field empty, which was only
// possible while the probe hit the /models catalog; they now fill the model so
// they still exercise the success and failure RENDERING paths.
const typeModel = async (
  user: ReturnType<typeof userEvent.setup>,
  value: string
) => {
  await user.type(screen.getByLabelText("Model"), value);
};

describe("LLMConfigTab — Test Connection", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to openai:gpt-4o");
    getApiKeyStatus.mockReset().mockResolvedValue("");
    testConnection.mockReset().mockResolvedValue("Connected");
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "connected",
    });
  });

  it("shows a success message when the connection test resolves", async () => {
    const user = userEvent.setup();
    testConnection.mockResolvedValue("Connected to OpenRouter");
    render(<LLMConfigTab onClose={vi.fn()} />);

    await typeModel(user, "openai/gpt-4o");
    await user.click(screen.getByRole("button", { name: /test connection/i }));

    expect(
      await screen.findByText(/✓ Connected to OpenRouter/i)
    ).toBeInTheDocument();
  });

  it("shows a failure message when the connection test rejects", async () => {
    const user = userEvent.setup();
    testConnection.mockRejectedValue(new Error("connection refused"));
    render(<LLMConfigTab onClose={vi.fn()} />);

    await typeModel(user, "openai/gpt-4o");
    await user.click(screen.getByRole("button", { name: /test connection/i }));

    // wailsErrorMessage surfaces the backend Error's own message verbatim.
    expect(await screen.findByText(/✗ connection refused/i)).toBeInTheDocument();
  });
});

describe("LLMConfigTab — Test Connection empty-model guard (PA-01)", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to openai:gpt-4o");
    getApiKeyStatus.mockReset().mockResolvedValue("");
    testConnection.mockReset().mockResolvedValue("Connected");
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "connected",
    });
  });

  // Mutation check: deleting the `if (model.trim() === "" && provider !==
  // "ollama")` guard in handleTestConnection makes this test fail — the
  // backend TestConnection binding is called and the guard's specific message
  // never renders. Without the guard an empty `model` is sent to the chat
  // completion endpoint and comes back as an opaque parameter error.
  it("blocks the test and never calls the backend when the model is empty", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    // Model field is intentionally left empty.
    await user.click(screen.getByRole("button", { name: /test connection/i }));

    expect(
      await screen.findByText(/✗ Enter or select a model before testing the connection/i)
    ).toBeInTheDocument();
    expect(testConnection).not.toHaveBeenCalled();
  });

  // Whitespace-only must be treated as empty — a stray space is not a model,
  // and `model.trim() === ""` is what makes that true. Asserting the message
  // alone would also pass with a naive `model === ""` guard.
  it("treats a whitespace-only model as empty", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await typeModel(user, "   ");
    await user.click(screen.getByRole("button", { name: /test connection/i }));

    expect(
      await screen.findByText(/✗ Enter or select a model before testing the connection/i)
    ).toBeInTheDocument();
    expect(testConnection).not.toHaveBeenCalled();
  });

  // Mutation check: removing the `provider !== "ollama"` exemption from the
  // same guard makes this test fail — Ollama's test probes GET {host}/api/tags
  // and never looks at the model, so blocking it there would break a flow that
  // legitimately has no model entered yet.
  it("still allows the test with an empty model for Ollama", async () => {
    const user = userEvent.setup();
    testConnection.mockResolvedValue("Connected to Ollama on http://localhost:11434");
    render(<LLMConfigTab onClose={vi.fn()} />);

    await selectProvider(user, "ollama");
    await user.click(screen.getByRole("button", { name: /test connection/i }));

    expect(
      await screen.findByText(/✓ Connected to Ollama on http:\/\/localhost:11434/i)
    ).toBeInTheDocument();
    expect(testConnection).toHaveBeenCalled();
  });
});

describe("LLMConfigTab — catalog-driven provider/model picker", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to openai:gpt-4o");
    getApiKeyStatus.mockReset().mockResolvedValue("");
    testConnection.mockReset().mockResolvedValue("Connected");
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "connected",
    });
  });

  it("renders the 11 catalog providers by name plus the Disable Pair LLM opt-out", async () => {
    render(<LLMConfigTab onClose={vi.fn()} />);

    await screen.findByRole("option", { name: "OpenAI" });
    for (const name of ["OpenAI", "Anthropic", "Google Gemini", "OpenRouter", "Ollama", "LM Studio"]) {
      expect(screen.getByRole("option", { name })).toBeInTheDocument();
    }
    expect(screen.getByRole("option", { name: "Disable Pair LLM" })).toHaveValue("disabled");
    // 11 catalog providers + the disabled opt-out
    expect(screen.getAllByRole("option")).toHaveLength(12);
    // Mutation check: reverting to a hardcoded provider array (or failing to
    // surface the catalog) would omit e.g. "Google Gemini" — this fails.
  });

  it("selecting a provider with catalog models shows suggestions with capability badges that fill the model", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);
    await screen.findByRole("option", { name: "OpenAI" });
    await user.selectOptions(screen.getByRole("combobox"), "openai");

    // Focus the model input to open the suggestion listbox.
    await user.click(screen.getByLabelText("Model"));
    const suggestion = await screen.findByRole("option", { name: /gpt-5.6-luna/ });
    expect(suggestion).toBeInTheDocument();
    // Capability badges: Reasoning, Tool-call, and the context window.
    expect(screen.getAllByText("Reasoning").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Tool-call").length).toBeGreaterThan(0);
    expect(screen.getByText("391k context")).toBeInTheDocument();

    // Clicking the gpt-4o suggestion fills the model field.
    await user.click(screen.getByRole("option", { name: /gpt-4o/ }));
    expect(screen.getByDisplayValue("gpt-4o")).toBeInTheDocument();
    // Mutation check: dropping the suggestion list (or the onMouseDown fill)
    // would leave the model field empty after this click — this fails.
  });

  it("a provider with no catalog models (ollama) falls back to free-text with no suggestions", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);
    await screen.findByRole("option", { name: "OpenAI" });
    await selectProvider(user, "ollama");

    await user.click(screen.getByLabelText("Model"));
    // No suggestion listbox for a provider with no curated models.
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.queryByText(/Suggestions from the curated catalog/)).not.toBeInTheDocument();

    await user.type(screen.getByLabelText("Model"), "llama3");
    expect(screen.getByDisplayValue("llama3")).toBeInTheDocument();
    // Mutation check: if selecting ollama pulled in another provider's model
    // list (leaving suggestions up), this test fails on the listbox.
  });

  it("free-text model entry still saves (custom model id round-trips)", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);
    await screen.findByRole("option", { name: "OpenAI" });

    await user.type(screen.getByLabelText("Model"), "my-custom-123");
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ Model: "my-custom-123" })
    );
    // The provider:model SetModel format is unchanged by the combobox.
    expect(setModel).toHaveBeenCalledWith("openai:my-custom-123");
    // Mutation check: if the combobox only allowed catalog ids (rejecting
    // free text), the typed custom model would not reach SaveSettings — this
    // fails, since OpenRouter/Ollama/LM Studio have an open-ended model space.
  });
});

describe("LLMConfigTab — Test Connection tests the key you typed (PA-TC)", () => {
  beforeEach(() => {
    getSettings.mockReset().mockResolvedValue({});
    saveSettings.mockReset().mockResolvedValue(undefined);
    saveAPIKey.mockReset().mockResolvedValue(undefined);
    setModel.mockReset().mockResolvedValue("Model set to openai:gpt-4o");
    getApiKeyStatus.mockReset().mockResolvedValue("");
    testConnection.mockReset().mockResolvedValue("Connected");
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "connected",
    });
  });

  // The API Key input only shows a placeholder when nothing is stored, so this
  // is the field the user types a candidate key into.
  const typeApiKey = async (
    user: ReturnType<typeof userEvent.setup>,
    value: string
  ) => {
    await user.type(screen.getByPlaceholderText("Enter API key"), value);
  };

  // Mutation check: dropping the apiKey argument from the TestConnection call
  // (i.e. reverting to the 3-argument binding) makes the mock receive "" and
  // reject, so the success message never renders and this test fails. That is
  // defect #1: the test would silently have exercised the stored key instead.
  it("forwards the typed key to the backend and says it used it", async () => {
    const user = userEvent.setup();
    const typed = "sk-typed-candidate-key";
    testConnection.mockImplementation((async (
      _provider: string,
      _model: string,
      _host: string,
      key: string
    ) => {
      if (key !== typed) {
        throw new Error("the typed key was not forwarded");
      }
      return "Connected";
    }) as never);

    render(<LLMConfigTab onClose={vi.fn()} />);

    await typeModel(user, "openai/gpt-4o");
    await typeApiKey(user, typed);
    await user.click(screen.getByRole("button", { name: /test connection/i }));

    expect(
      await screen.findByText(/✓ Connected \(using the key you entered\)/i)
    ).toBeInTheDocument();
    expect(testConnection).toHaveBeenCalledWith("openai", "openai/gpt-4o", "", typed);
  });

  // The other half of the contract: an empty field keeps testing the saved key,
  // which is exactly what the app-mount probe relies on.
  //
  // Mutation check: always appending the "key you entered" suffix (or always
  // forwarding a non-empty key) makes this fail — the user must be told which
  // key was actually tested.
  it("says it used the saved key when the API Key field is empty", async () => {
    const user = userEvent.setup();
    render(<LLMConfigTab onClose={vi.fn()} />);

    await typeModel(user, "openai/gpt-4o");
    await user.click(screen.getByRole("button", { name: /test connection/i }));

    expect(
      await screen.findByText(/✓ Connected \(using the saved key\)/i)
    ).toBeInTheDocument();
    expect(testConnection).toHaveBeenCalledWith("openai", "openai/gpt-4o", "", "");
  });

  // Security/correctness property: a typed-but-unsaved key is a CANDIDATE, not
  // the active configuration. Flipping the global indicator to Connected because
  // a candidate worked would be a new lie (and Save is what makes it active).
  //
  // Mutation check: making handleTestConnection write connectionStatus makes
  // this fail.
  it("a successful unsaved Test must not move the global connection indicator", async () => {
    const user = userEvent.setup();
    useSettingsStore.setState({ connectionStatus: "disconnected" });
    render(<LLMConfigTab onClose={vi.fn()} />);

    await typeModel(user, "openai/gpt-4o");
    await typeApiKey(user, "sk-typed-candidate-key");
    await user.click(screen.getByRole("button", { name: /test connection/i }));
    await screen.findByText(/using the key you entered/i);

    expect(useSettingsStore.getState().connectionStatus).toBe("disconnected");
  });

  // A failing unsaved Test must not move it either — the indicator describes the
  // saved configuration, not the last thing the user poked at.
  it("a failed unsaved Test must not move the global connection indicator", async () => {
    const user = userEvent.setup();
    useSettingsStore.setState({ connectionStatus: "connected" });
    testConnection.mockRejectedValue(new Error("401 unauthorized"));
    render(<LLMConfigTab onClose={vi.fn()} />);

    await typeModel(user, "openai/gpt-4o");
    await typeApiKey(user, "sk-bad-candidate-key");
    await user.click(screen.getByRole("button", { name: /test connection/i }));
    await screen.findByText(/✗ 401 unauthorized/i);

    expect(useSettingsStore.getState().connectionStatus).toBe("connected");
  });

  // Defect #3: after saving a working key the indicator stayed Disconnected, so
  // the key looked broken until the user happened to send a chat message.
  //
  // Mutation check: removing the probeLLMConnection() call from handleSave
  // leaves the status at "disconnected" and this test fails.
  it("re-probes after a successful Save and flips Disconnected to Connected", async () => {
    const user = userEvent.setup();
    useSettingsStore.setState({ connectionStatus: "disconnected" });
    // Mount reads a config with no provider; the post-save probe reads the
    // just-saved one. (The probe is the only later caller of GetSettings here.)
    getSettings
      .mockResolvedValueOnce({})
      .mockResolvedValue({ Provider: "openai", Model: "gpt-4o" });
    testConnection.mockResolvedValue("Connected");

    render(<LLMConfigTab onClose={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() =>
      expect(useSettingsStore.getState().connectionStatus).toBe("connected")
    );
    expect(useSettingsStore.getState().activeModel).toBe("openai:gpt-4o");
  });

  // The probe reports on the SAVED configuration, so a save that still cannot
  // connect must land on "disconnected" rather than leaving "checking" behind.
  it("re-probes after a successful Save and lands on Disconnected when it fails", async () => {
    const user = userEvent.setup();
    useSettingsStore.setState({ connectionStatus: "connected" });
    getSettings
      .mockResolvedValueOnce({})
      .mockResolvedValue({ Provider: "openai", Model: "gpt-4o" });
    testConnection.mockRejectedValue(new Error("connection refused"));

    render(<LLMConfigTab onClose={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() =>
      expect(useSettingsStore.getState().connectionStatus).toBe("disconnected")
    );
  });
});
