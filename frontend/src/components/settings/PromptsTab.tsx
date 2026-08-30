import { useEffect, useState } from "react";
import { mergeAndSaveSettings } from "@/utils/settingsSync";

const DEFAULT_SYSTEM_PROMPT = `You are PairAdmin, an AI assistant that helps sysadmins work in the terminal.
You can see the user's terminal output and help them understand what's happening, diagnose issues, and suggest commands.
Always provide clear, concise explanations and practical command suggestions.
When suggesting commands, explain what they do and any potential risks.`;

export function PromptsTab() {
  const [customPrompt, setCustomPrompt] = useState("");
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => {
        if (cfg.CustomPrompt) setCustomPrompt(cfg.CustomPrompt);
      })
      .catch(() => {});
  }, []);

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      await mergeAndSaveSettings({ CustomPrompt: customPrompt });
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  return (
    <div className="space-y-4 p-6">
      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Built-in System Prompt (read-only)</label>
        <div className="bg-surface-2 border border-surface-border-strong rounded px-3 py-2 text-xs text-surface-text-muted whitespace-pre-wrap">
          {DEFAULT_SYSTEM_PROMPT}
        </div>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Custom Prompt Extension</label>
        <textarea
          value={customPrompt}
          onChange={(e) => setCustomPrompt(e.target.value)}
          placeholder="Add custom instructions to extend the system prompt..."
          rows={6}
          className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none resize-none"
        />
      </div>

      <div className="flex items-center gap-3">
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
