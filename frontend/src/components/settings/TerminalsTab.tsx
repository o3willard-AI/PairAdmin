import { useState } from "react";

export function TerminalsTab() {
  const [atspiPollingMs, setAtspiPollingMs] = useState(500);
  const [clipboardClearSecs, setClipboardClearSecs] = useState(60);
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      const { SaveSettings } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await SaveSettings({ ATSPIPollingMs: atspiPollingMs, ClipboardClearSecs: clipboardClearSecs } as import("../../../wailsjs/go/models").config.AppConfig);
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  return (
    <div className="space-y-4 p-6">
      <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wider">
        Capture Settings
      </h3>

      <div className="space-y-1">
        <label className="text-xs text-zinc-400">AT-SPI2 Polling Interval (ms)</label>
        <input
          type="number"
          value={atspiPollingMs}
          onChange={(e) => setAtspiPollingMs(Math.max(100, Math.min(5000, Number(e.target.value))))}
          min={100}
          max={5000}
          className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
        />
        <p className="text-xs text-zinc-600">Min: 100ms, Max: 5000ms. Default: 500ms.</p>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-zinc-400">Clipboard Auto-Clear Interval (seconds)</label>
        <input
          type="number"
          value={clipboardClearSecs}
          onChange={(e) =>
            setClipboardClearSecs(Math.max(0, Math.min(600, Number(e.target.value))))
          }
          min={0}
          max={600}
          className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none"
        />
        <p className="text-xs text-zinc-600">0 = disabled. Min: 0, Max: 600s. Default: 60s.</p>
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
