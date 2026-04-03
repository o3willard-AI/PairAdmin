import { useState } from "react";

const DEFAULT_SYSTEM_PROMPT = `You are PairAdmin, an AI assistant that helps sysadmins work in the terminal.
You can see the user's terminal output and help them understand what's happening, diagnose issues, and suggest commands.
Always provide clear, concise explanations and practical command suggestions.
When suggesting commands, explain what they do and any potential risks.`;

export function PromptsTab() {
  const [customPrompt, setCustomPrompt] = useState("");
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      const { SaveSettings } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await SaveSettings({ CustomPrompt: customPrompt } as import("../../../wailsjs/go/models").config.AppConfig);
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
        <label className="text-xs text-zinc-400">Built-in System Prompt (read-only)</label>
        <div className="bg-zinc-800 border border-zinc-700 rounded px-3 py-2 text-xs text-zinc-500 whitespace-pre-wrap">
          {DEFAULT_SYSTEM_PROMPT}
        </div>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-zinc-400">Custom Prompt Extension</label>
        <textarea
          value={customPrompt}
          onChange={(e) => setCustomPrompt(e.target.value)}
          placeholder="Add custom instructions to extend the system prompt..."
          rows={6}
          className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none resize-none"
        />
      </div>

      <div className="flex items-center gap-3">
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
