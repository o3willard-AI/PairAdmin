import { useState, useEffect } from "react";
import { useSettingsStore } from "@/stores/settingsStore";

const PROVIDERS = ["openai", "anthropic", "ollama", "openrouter", "lmstudio"] as const;
type Provider = (typeof PROVIDERS)[number];

const NO_KEY_PROVIDERS: Provider[] = ["ollama", "lmstudio"];

interface LLMConfigTabProps {
  onClose: () => void;
}

export function LLMConfigTab({ onClose }: LLMConfigTabProps) {
  const setActiveModel = useSettingsStore((s) => s.setActiveModel);

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
      setTestMessage(err instanceof Error ? err.message : "Connection failed");
    }
  };

  const handleSave = async () => {
    if (model.includes("\\")) {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
      return;
    }
    setSaveStatus("saving");
    try {
      const { SaveSettings, SaveAPIKey, SetModel } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await SaveSettings({ Provider: provider, Model: model, OllamaHost: ollamaHost, LMStudioHost: lmstudioHost } as import("../../../wailsjs/go/models").config.AppConfig);
      if (apiKey) {
        await SaveAPIKey(provider, apiKey);
      }
      const activeModelStr = await SetModel(`${provider}:${model}`);
      setActiveModel(activeModelStr || `${provider}:${model}`);
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
      onClose();
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  const requiresApiKey = !NO_KEY_PROVIDERS.includes(provider);

  return (
    <div className="space-y-4 p-6">
      <div className="space-y-1">
        <label className="text-xs text-zinc-400">Provider</label>
        <div className="relative">
          <select
            value={provider}
            onChange={(e) => setProvider(e.target.value as Provider)}
            className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
          >
            {PROVIDERS.map((p) => (
              <option key={p} value={p} className="bg-zinc-800 text-zinc-100">
                {p}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-zinc-400">Model</label>
        <input
          type="text"
          value={model}
          onChange={(e) => setModel(e.target.value)}
          placeholder="e.g. gpt-4o, claude-3-5-sonnet-20241022"
          className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
        />
      </div>

      {(provider === "ollama") ? (
        <div className="space-y-1">
          <label className="text-xs text-zinc-400">Server URL</label>
          <input
            type="text"
            value={ollamaHost}
            onChange={(e) => setOllamaHost(e.target.value)}
            placeholder="http://localhost:11434"
            className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
          />
        </div>
      ) : (provider === "lmstudio") ? (
        <div className="space-y-1">
          <label className="text-xs text-zinc-400">Server URL</label>
          <input
            type="text"
            value={lmstudioHost}
            onChange={(e) => setLmstudioHost(e.target.value)}
            placeholder="http://localhost:1234/v1"
            className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
          />
        </div>
      ) : null}

      {requiresApiKey ? (
        <div className="space-y-1">
          <label className="text-xs text-zinc-400">API Key</label>
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={keyPlaceholder || "Enter API key"}
            className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
          />
        </div>
      ) : (
        <div className="space-y-1">
          <label className="text-xs text-zinc-400">API Key</label>
          <p className="text-xs text-zinc-500">No API key required for {provider}</p>
        </div>
      )}

      <div className="space-y-1">
        <button
          onClick={handleTestConnection}
          disabled={testStatus === "testing"}
          className="bg-zinc-700 hover:bg-zinc-600 text-zinc-100 text-xs px-4 py-1.5 rounded disabled:opacity-50 disabled:cursor-not-allowed"
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

      <div className="pt-2 flex items-center gap-3">
        <button
          onClick={handleSave}
          disabled={saveStatus === "saving"}
          className="bg-zinc-700 hover:bg-zinc-600 text-zinc-100 text-xs px-4 py-1.5 rounded disabled:opacity-50"
        >
          {saveStatus === "saving" ? "Saving..." : saveStatus === "saved" ? "Saved!" : "Save"}
        </button>
        {saveStatus === "error" && <span className="text-xs text-red-400">Save failed</span>}
      </div>
    </div>
  );
}
