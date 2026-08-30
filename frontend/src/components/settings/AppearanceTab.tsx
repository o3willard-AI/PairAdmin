import { useEffect, useState } from "react";
import { useTheme } from "@/theme/theme-provider";
import { mergeAndSaveSettings } from "@/utils/settingsSync";

export function AppearanceTab() {
  const { theme, setTheme } = useTheme();
  const [fontSize, setFontSize] = useState(14);
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => {
        if (cfg.FontSize) setFontSize(cfg.FontSize);
      })
      .catch(() => {});
  }, []);

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      await mergeAndSaveSettings({ Theme: theme, FontSize: fontSize });
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
        <label className="text-xs text-surface-text-muted">Theme</label>
        <div className="flex gap-2">
          <button
            onClick={() => setTheme("dark")}
            className={`text-xs px-4 py-1.5 rounded border transition-colors ${
              theme === "dark"
                ? "bg-surface-3 border-surface-border-strong text-surface-text"
                : "bg-surface-2 border-surface-border-strong text-surface-text-muted hover:text-surface-text"
            }`}
          >
            Dark
          </button>
          <button
            onClick={() => setTheme("light")}
            className={`text-xs px-4 py-1.5 rounded border transition-colors ${
              theme === "light"
                ? "bg-surface-3 border-surface-border-strong text-surface-text"
                : "bg-surface-2 border-surface-border-strong text-surface-text-muted hover:text-surface-text"
            }`}
          >
            Light
          </button>
        </div>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Font Size (px)</label>
        <input
          type="number"
          value={fontSize}
          onChange={(e) => setFontSize(Math.max(10, Math.min(24, Number(e.target.value))))}
          min={10}
          max={24}
          className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
        />
        <p className="text-xs text-surface-text-muted">Min: 10px, Max: 24px. Default: 14px.</p>
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
