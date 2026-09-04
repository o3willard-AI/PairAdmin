import { useState, useEffect } from "react";
import { useSettingsStore } from "@/stores/settingsStore";
import { mergeAndSaveSettings } from "@/utils/settingsSync";
import { wailsErrorMessage } from "@/utils/wailsError";

const PROVIDERS = ["openai", "anthropic", "ollama", "openrouter", "lmstudio", "disabled"] as const;
type Provider = (typeof PROVIDERS)[number];

// "disabled" renders as the user-facing "Disable Pair LLM" option; the other
// providers keep their bare IDs as labels.
const PROVIDER_LABELS: Record<Provider, string> = {
  openai: "openai",
  anthropic: "anthropic",
  ollama: "ollama",
  openrouter: "openrouter",
  lmstudio: "lmstudio",
  disabled: "Disable Pair LLM",
};

const NO_KEY_PROVIDERS: Provider[] = ["ollama", "lmstudio"];

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

  const [provider, setProvider] = useState<Provider>("openai");
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
      .then(({ GetSettings, GetAPIKeyStatus }) => {
        GetSettings().then((cfg) => {
          if (cfg.Provider) setProvider(cfg.Provider as Provider);
          if (cfg.Model) setModel(cfg.Model as string);
          if (cfg.OllamaHost) setOllamaHost(cfg.OllamaHost);
          if (cfg.LMStudioHost) setLmstudioHost(cfg.LMStudioHost);
        });
        GetAPIKeyStatus(provider).then((status: string) => {
          setKeyPlaceholder(status ? "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (stored)" : "");
        });
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
    setTestStatus("testing");
    setTestMessage("");
    try {
      const { TestConnection } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      const hostURL = provider === "ollama" ? ollamaHost : provider === "lmstudio" ? lmstudioHost : "";
      const result = await TestConnection(provider, model, hostURL);
      setTestStatus("ok");
      setTestMessage(result || "Connected");
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
      // Re-enable restore: release the "disabled" state so normal
      // connected/disconnected status updates resume (stream events drive
      // them from here on). "disconnected" is the honest baseline — nothing
      // re-probes on save — but it un-blocks the stream-event path, whereas
      // leaving "disabled" in place would dead-end the chat input until
      // restart.
      if (useSettingsStore.getState().connectionStatus === "disabled") {
        setConnectionStatus("disconnected");
      }
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
      onClose();
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

  return (
    <div className="space-y-4 p-6">
      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Provider</label>
        <div className="relative">
          <select
            value={provider}
            onChange={(e) => setProvider(e.target.value as Provider)}
            className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
          >
            {PROVIDERS.map((p) => (
              <option key={p} value={p} className="bg-surface-2 text-surface-text">
                {PROVIDER_LABELS[p]}
              </option>
            ))}
          </select>
        </div>
      </div>

      {!isDisabledProvider && (
        <div className="space-y-1">
          <label className="text-xs text-surface-text-muted">Model</label>
          <input
            type="text"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="e.g. gpt-4o, claude-3-5-sonnet-20241022"
            className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
          />
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
