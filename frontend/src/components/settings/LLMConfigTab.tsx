import { useState, useEffect } from "react";
import { useSettingsStore } from "@/stores/settingsStore";
import { mergeAndSaveSettings } from "@/utils/settingsSync";
import { wailsErrorMessage } from "@/utils/wailsError";
import { probeLLMConnection } from "@/utils/connectionProbe";

// CatalogProvider/CatalogModel mirror the camelCase views SettingsService
// exposes via GetLLMCatalog (services.CatalogProviderView) — a provider/model
// picker needs id/name/adapter and per-model id/name/context/reasoning/
// tool_call; EnvKey and costs never leave the backend.
export interface CatalogModel {
  id: string;
  name: string;
  context: number;
  reasoning: boolean;
  toolCall: boolean;
}
export interface CatalogProvider {
  id: string;
  name: string;
  adapter: string;
  models: CatalogModel[];
}

// NO_KEY_PROVIDERS: local providers that get no API key — the provider->key
// logic keyed off the id is preserved from the original hardcoded set.
const NO_KEY_PROVIDERS: string[] = ["ollama", "lmstudio"];

// modelBadges returns the capability badges shown on a catalog-model
// suggestion: Reasoning, Tool-call, and the context window when known.
function modelBadges(m: CatalogModel): string[] {
  const b: string[] = [];
  if (m.reasoning) b.push("Reasoning");
  if (m.toolCall) b.push("Tool-call");
  if (m.context && m.context > 0) b.push(`${Math.max(1, Math.round(m.context / 1024))}k context`);
  return b;
}

// isLoopbackHost reports whether an Ollama/LM Studio server URL points at
// this machine: localhost, a 127.x.y.z address, or [::1] / ::1. Used to
// decide whether the "terminal output leaves the machine" warning shows for
// remote Ollama hosts.
export function isLoopbackHost(host: string): boolean {
  const trimmed = host.trim().toLowerCase();
  if (trimmed === "") return true; // empty = default localhost:11434
  let hostname: string;
  try {
    const u = new URL(trimmed);
    hostname = u.hostname;
  } catch {
    // Not a parseable URL — maybe a bare "localhost:11434". Try prefixing a
    // scheme so URL parsing still yields a hostname.
    try {
      hostname = new URL("http://" + trimmed).hostname;
    } catch {
      // Unparseable — fail closed toward "remote" (caller shows the warning).
      return false;
    }
  }
  if (hostname === "localhost") return true;
  if (hostname === "::1" || hostname === "[::1]") return true;
  if (hostname.startsWith("127.")) return true;
  return false;
}

interface LLMConfigTabProps {
  onClose: () => void;
}

export function LLMConfigTab({ onClose }: LLMConfigTabProps) {
  const setActiveModel = useSettingsStore((s) => s.setActiveModel);
  const setConnectionStatus = useSettingsStore((s) => s.setConnectionStatus);

  const [providers, setProviders] = useState<CatalogProvider[]>([]);
  const [modelOpen, setModelOpen] = useState(false);
  const [provider, setProvider] = useState<string>("openai");
  const [model, setModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [keyPlaceholder, setKeyPlaceholder] = useState("");
  const [ollamaHost, setOllamaHost] = useState("");
  const [lmstudioHost, setLmstudioHost] = useState("");
  const [testStatus, setTestStatus] = useState<"idle" | "testing" | "ok" | "error">("idle");
  const [testMessage, setTestMessage] = useState("");
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings, GetAPIKeyStatus, GetLLMCatalog }) => {
        GetSettings().then((cfg) => {
          if (cfg.Provider) setProvider(cfg.Provider);
          if (cfg.Model) setModel(cfg.Model as string);
          if (cfg.OllamaHost) setOllamaHost(cfg.OllamaHost);
          if (cfg.LMStudioHost) setLmstudioHost(cfg.LMStudioHost);
        });
        GetAPIKeyStatus(provider).then((status: string) => {
          setKeyPlaceholder(status ? "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (stored)" : "");
        });
        // Populate the provider/model picker from the backend catalog. If the
        // catalog binding is unavailable the tab degrades (provider select
        // falls back to just the "disabled" opt-out) rather than crashing.
        GetLLMCatalog().then((catalog: CatalogProvider[]) => setProviders(catalog)).catch(() => {});
      })
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Refresh key placeholder when provider changes
  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetAPIKeyStatus }) => {
        GetAPIKeyStatus(provider).then((status: string) => {
          setKeyPlaceholder(status ? "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (stored)" : "");
          setApiKey(""); // clear field when switching providers
        });
      })
      .catch(() => {});
  }, [provider]);

  const handleTestConnection = async () => {
    if (model.includes("\\")) {
      setTestStatus("error");
      setTestMessage('Model ID contains a backslash — use a forward slash (e.g. google/gemma-3-27b-it)');
      return;
    }
    // A real chat completion needs a real model, and the connection test now
    // goes through that path for every `openai`-adapter provider (PA-01). An
    // empty model would otherwise reach the API as an empty `model` field and
    // come back as an opaque 400 about a parameter the user never filled in, so
    // reject it up front where the message can be specific.
    //
    // Ollama is exempt: its test probes GET {host}/api/tags to see whether the
    // server is reachable and never looks at the model, so an empty field is a
    // legitimate state there and blocking it would break a working flow.
    if (model.trim() === "" && provider !== "ollama") {
      setTestStatus("error");
      setTestMessage("Enter or select a model before testing the connection");
      return;
    }
    setTestStatus("testing");
    setTestMessage("");
    // Whether a typed key was tested decides which message the user gets, and
    // only the frontend knows it. The backend's "Connected" string stays bare
    // so existing callers and tests are unaffected.
    const usedTypedKey = apiKey.trim() !== "";
    try {
      const { TestConnection } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      const hostURL = provider === "ollama" ? ollamaHost : provider === "lmstudio" ? lmstudioHost : "";
      // The typed key is forwarded so the test exercises what the user is
      // looking at, not just what is already saved — testing the stored key
      // while a different one sits in the field is how a good key gets
      // reported as unauthorized (and a dead one as working). It is passed
      // for this call only: never saved here, never cached, never written
      // into an enclave. Persisting stays Save's job, on an explicit click.
      const result = await TestConnection(provider, model, hostURL, apiKey);
      setTestStatus("ok");
      // An unsaved candidate key must NOT move the global indicator — see
      // handleSave, which re-probes the actually-saved configuration. This
      // only reports on the connection the user just asked about.
      const base = result || "Connected";
      setTestMessage(usedTypedKey ? `${base} (using the key you entered)` : `${base} (using the saved key)`);
    } catch (err) {
      setTestStatus("error");
      setTestMessage(wailsErrorMessage(err, "Connection failed"));
    }
  };

  const handleSave = async () => {
    // "Disable Pair LLM": persist Provider="disabled" and set the active
    // model to the bare string "disabled" — the provider:model format
    // SetModel expects doesn't fit a model-less disabled state, so that call
    // is skipped. The status is set to "disabled" immediately (the startup
    // probe in ThreeColumnLayout only runs on mount, so without this the
    // status bar / chat input wouldn't reflect the opt-out until restart).
    if (provider === "disabled") {
      setSaveStatus("saving");
      try {
        await mergeAndSaveSettings({ Provider: "disabled" });
        setActiveModel("disabled");
        setConnectionStatus("disabled");
        setSaveStatus("saved");
        setTimeout(() => setSaveStatus("idle"), 2000);
        onClose();
      } catch {
        setSaveStatus("error");
        setTimeout(() => setSaveStatus("idle"), 3000);
      }
      return;
    }
    if (model.includes("\\")) {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
      return;
    }
    setSaveStatus("saving");
    try {
      const { SaveAPIKey, SetModel } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await mergeAndSaveSettings({
        Provider: provider,
        Model: model,
        OllamaHost: ollamaHost,
        LMStudioHost: lmstudioHost,
      });
      if (apiKey) {
        await SaveAPIKey(provider, apiKey);
      }
      const activeModelStr = await SetModel(`${provider}:${model}`);
      setActiveModel(activeModelStr || `${provider}:${model}`);
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
      onClose();
      // The saved configuration is the active one now, so re-probe it and set
      // the Connected/Disconnected indicator from a real result. Without this
      // the indicator kept showing the last chat failure (or the old
      // "disconnected" baseline), so a freshly-saved working key still looked
      // broken until the user happened to send a message.
      //
      // This is the same probe the app runs at mount — one shared helper, not a
      // second copy that can drift. It also subsumes the old
      // "disabled → disconnected" release here: the probe claims "checking"
      // first, so a save that re-enables the LLM no longer dead-ends the chat
      // input on the stale opt-out.
      //
      // Deliberately not awaited: the probe performs a real network round trip
      // (a live chat completion), and the dialog must not stay open on it. The
      // store is global, so the result lands even though this dialog unmounts.
      void probeLLMConnection();
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  const requiresApiKey = !NO_KEY_PROVIDERS.includes(provider);
  // "Disable Pair LLM" is model-less: no Model, no Server URL, no API Key,
  // nothing to test — only the Provider dropdown and Save remain.
  const isDisabledProvider = provider === "disabled";
  // Remote-Ollama privacy warning: terminal output leaves the machine when
  // the host isn't loopback. Localhost, 127.* and ::1 are the loopback forms;
  // anything else (team GPU box, LAN IP, a public hostname) triggers the
  // amber warning. Non-empty is treated as remote by design — an unparseable
  // or bare-host value should fail closed toward the warning, not hide it.
  const isRemoteOllama =
    provider === "ollama" && ollamaHost.trim() !== "" && !isLoopbackHost(ollamaHost);

  // Local Ollama needs no key, but a REMOTE Ollama commonly requires one
  // (Authorization: Bearer). The field shows only for ollama; empty = no
  // key persisted (SaveAPIKey with an empty key removes the entry, so we
  // only call it when the user actually typed something).
  const showOllamaKeyField = provider === "ollama";

  // Catalog-driven picker state. A provider with no catalog models
  // (ollama/lmstudio discover models at runtime) yields no suggestions, so
  // the model combobox degrades to a plain free-text input.
  const activeProvider = providers.find((p) => p.id === provider);
  const catalogModels = activeProvider?.models ?? [];
  const modelSuggestions = modelOpen
    ? catalogModels.filter((m) => m.id.toLowerCase().includes(model.trim().toLowerCase()))
    : [];

  return (
    <div className="space-y-4 p-6">
      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Provider</label>
        <div className="relative">
          <select
            value={provider}
            onChange={(e) => {
              setProvider(e.target.value);
              setModelOpen(false);
            }}
            className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
          >
            {providers.map((p) => (
              <option key={p.id} value={p.id} className="bg-surface-2 text-surface-text">
                {p.name}
              </option>
            ))}
            <option value="disabled" className="bg-surface-2 text-surface-text">
              Disable Pair LLM
            </option>
          </select>
        </div>
      </div>

      {!isDisabledProvider && (
        <div className="space-y-1">
          <label className="text-xs text-surface-text-muted">Model</label>
          <div className="relative">
            <input
              type="text"
              value={model}
              onChange={(e) => {
                setModel(e.target.value);
                setModelOpen(true);
              }}
              onFocus={() => setModelOpen(true)}
              onBlur={() => setTimeout(() => setModelOpen(false), 120)}
              placeholder="e.g. gpt-4o, claude-3-5-sonnet-20241022"
              aria-label="Model"
              className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
            />
            {modelSuggestions.length > 0 && (
              <ul
                role="listbox"
                className="absolute z-20 mt-1 w-full max-h-56 overflow-auto bg-surface-2 border border-surface-border-strong rounded text-sm text-surface-text shadow-lg"
              >
                {modelSuggestions.map((m) => (
                  <li key={m.id}>
                    <button
                      type="button"
                      role="option"
                      onMouseDown={(e) => {
                        e.preventDefault(); // keep the input focused; select below
                        setModel(m.id);
                        setModelOpen(false);
                      }}
                      className="w-full text-left px-2 py-1.5 hover:bg-surface-3/50 text-xs text-surface-text flex items-center justify-between gap-1.5"
                    >
                      <span className="font-mono truncate">{m.id}</span>
                      <span className="flex items-center gap-1 text-[10px] text-surface-text-muted">
                        {modelBadges(m).map((badge) => (
                          <span
                            key={badge}
                            className="rounded bg-surface-3 px-1.5 py-0.5 uppercase"
                          >
                            {badge}
                          </span>
                        ))}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          {catalogModels.length > 0 && (
            <p className="text-xs text-surface-text-muted">
              Suggestions from the curated catalog — you can still type any model id.
            </p>
          )}
        </div>
      )}

      {!isDisabledProvider && (provider === "ollama") ? (
        <div className="space-y-1">
          <label className="text-xs text-surface-text-muted" htmlFor="ollama-server-url">
            Server URL
          </label>
          <input
            id="ollama-server-url"
            type="text"
            value={ollamaHost}
            onChange={(e) => setOllamaHost(e.target.value)}
            placeholder="http://localhost:11434"
            className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
          />
          {isRemoteOllama && (
            <p
              role="alert"
              className="text-xs text-amber-500 mt-1"
            >
              ⚠ Terminal output will be sent to a remote Ollama server — only
              use a host you control.
            </p>
          )}
        </div>
      ) : (provider === "lmstudio") ? (
        <div className="space-y-1">
          <label className="text-xs text-surface-text-muted" htmlFor="lmstudio-server-url">
            Server URL
          </label>
          <input
            id="lmstudio-server-url"
            type="text"
            value={lmstudioHost}
            onChange={(e) => setLmstudioHost(e.target.value)}
            placeholder="http://localhost:1234/v1"
            className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
          />
          <p className="text-xs text-surface-text-muted">
            Works with any OpenAI-compatible server, not just LM Studio — point
            this at vLLM, llama.cpp, text-generation-webui, or similar by
            changing the URL.
          </p>
        </div>
      ) : null}

      {!isDisabledProvider && requiresApiKey ? (
        <div className="space-y-1">
          <label className="text-xs text-surface-text-muted">API Key</label>
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={keyPlaceholder || "Enter API key"}
            className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
          />
        </div>
      ) : showOllamaKeyField ? (
        <div className="space-y-1">
          <label className="text-xs text-surface-text-muted" htmlFor="ollama-api-key">
            Ollama API key
          </label>
          <input
            id="ollama-api-key"
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={keyPlaceholder || "Optional — only for authenticated remote servers"}
            className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
          />
          <p className="text-xs text-surface-text-muted">
            Leave empty for a local Ollama. Required by some remote servers —
            sent as an Authorization: Bearer header.
          </p>
        </div>
      ) : !isDisabledProvider ? (
        <div className="space-y-1">
          <label className="text-xs text-surface-text-muted">API Key</label>
          <p className="text-xs text-surface-text-muted">No API key required for {provider}</p>
        </div>
      ) : null}

      {!isDisabledProvider && (
        <div className="space-y-1">
          <button
            onClick={handleTestConnection}
            disabled={testStatus === "testing"}
            className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {testStatus === "testing" ? "Testing..." : "Test Connection"}
          </button>
          {testStatus === "ok" && (
            <p className="text-xs text-green-400 mt-1">&#x2713; {testMessage}</p>
          )}
          {testStatus === "error" && (
            <p className="text-xs text-red-400 mt-1">&#x2717; {testMessage}</p>
          )}
        </div>
      )}

      <div className="pt-2 flex items-center gap-3">
        <button
          onClick={handleSave}
          disabled={saveStatus === "saving"}
          className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded disabled:opacity-50"
        >
          {saveStatus === "saving" ? "Saving..." : saveStatus === "saved" ? "Saved!" : "Save"}
        </button>
        {saveStatus === "error" && <span className="text-xs text-red-400">Save failed</span>}
      </div>
    </div>
  );
}
