import { useState } from "react";
import { useTheme } from "@/theme/theme-provider";

export function AppearanceTab() {
  const { theme, setTheme } = useTheme();
  const [fontSize, setFontSize] = useState(14);
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      const { SaveSettings } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await SaveSettings({ theme, fontSize });
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  return (
    <div className="space-y-4 p-6">
      <div className="space-y-2">
        <label className="text-xs text-zinc-400">Theme</label>
        <div className="flex gap-2">
          <button
            onClick={() => setTheme("dark")}
            className={`text-xs px-4 py-1.5 rounded border transition-colors ${
              theme === "dark"
                ? "bg-zinc-600 border-zinc-500 text-zinc-100"
                : "bg-zinc-800 border-zinc-700 text-zinc-400 hover:text-zinc-300"
            }`}
          >
            Dark
          </button>
          <button
            onClick={() => setTheme("light")}
            className={`text-xs px-4 py-1.5 rounded border transition-colors ${
              theme === "light"
                ? "bg-zinc-600 border-zinc-500 text-zinc-100"
                : "bg-zinc-800 border-zinc-700 text-zinc-400 hover:text-zinc-300"
            }`}
          >
            Light
          </button>
        </div>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-zinc-400">Font Size (px)</label>
        <input
          type="number"
          value={fontSize}
          onChange={(e) => setFontSize(Math.max(10, Math.min(24, Number(e.target.value))))}
          min={10}
          max={24}
          className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
        />
        <p className="text-xs text-zinc-600">Min: 10px, Max: 24px. Default: 14px.</p>
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
