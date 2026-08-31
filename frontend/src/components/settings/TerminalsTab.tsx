import { useEffect, useState } from "react";
import { mergeAndSaveSettings } from "@/utils/settingsSync";

// Matches DefaultTerminalsSidebarWidthCh/DefaultCommandsSidebarWidthCh in
// services/config/config.go.
const DEFAULT_TERMINALS_SIDEBAR_WIDTH_CH = 20;
const DEFAULT_COMMANDS_SIDEBAR_WIDTH_CH = 30;

export function TerminalsTab() {
  const [atspiPollingMs, setAtspiPollingMs] = useState(500);
  const [clipboardClearSecs, setClipboardClearSecs] = useState(60);
  const [terminalsSidebarWidthCh, setTerminalsSidebarWidthCh] = useState(
    DEFAULT_TERMINALS_SIDEBAR_WIDTH_CH
  );
  const [commandsSidebarWidthCh, setCommandsSidebarWidthCh] = useState(
    DEFAULT_COMMANDS_SIDEBAR_WIDTH_CH
  );
  const [promptNewHostKeys, setPromptNewHostKeys] = useState(false);
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => {
        if (cfg.ATSPIPollingMs) setAtspiPollingMs(cfg.ATSPIPollingMs);
        if (cfg.ClipboardClearSecs !== undefined && cfg.ClipboardClearSecs !== null) {
          setClipboardClearSecs(cfg.ClipboardClearSecs);
        }
        if (cfg.TerminalsSidebarWidthCh) setTerminalsSidebarWidthCh(cfg.TerminalsSidebarWidthCh);
        if (cfg.CommandsSidebarWidthCh) setCommandsSidebarWidthCh(cfg.CommandsSidebarWidthCh);
        setPromptNewHostKeys(!!cfg.PromptNewHostKeys);
      })
      .catch(() => {});
  }, []);

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      await mergeAndSaveSettings({
        ATSPIPollingMs: atspiPollingMs,
        ClipboardClearSecs: clipboardClearSecs,
        TerminalsSidebarWidthCh: terminalsSidebarWidthCh,
        CommandsSidebarWidthCh: commandsSidebarWidthCh,
        PromptNewHostKeys: promptNewHostKeys,
      });
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  return (
    <div className="space-y-4 p-6">
      <h3 className="text-xs font-semibold text-surface-text-muted uppercase tracking-wider">
        Capture Settings
      </h3>

      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">AT-SPI2 Polling Interval (ms)</label>
        <input
          type="number"
          value={atspiPollingMs}
          onChange={(e) => setAtspiPollingMs(Math.max(100, Math.min(5000, Number(e.target.value))))}
          min={100}
          max={5000}
          className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
        />
        <p className="text-xs text-surface-text-muted">Min: 100ms, Max: 5000ms. Default: 500ms.</p>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Clipboard Auto-Clear Interval (seconds)</label>
        <input
          type="number"
          value={clipboardClearSecs}
          onChange={(e) =>
            setClipboardClearSecs(Math.max(0, Math.min(600, Number(e.target.value))))
          }
          min={0}
          max={600}
          className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
        />
        <p className="text-xs text-surface-text-muted">0 = disabled. Min: 0, Max: 600s. Default: 60s.</p>
      </div>

      <h3 className="text-xs font-semibold text-surface-text-muted uppercase tracking-wider pt-2">
        Layout
      </h3>
      <p className="text-xs text-surface-text-muted -mt-2">
        The Terminals and Commands sidebars are a fixed width rather than a
        live drag-resizable one, to avoid an extra source of layout bugs.
        Set your preferred width here instead — takes effect after
        restarting the app.
      </p>

      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Terminals List Width (characters)</label>
        <input
          type="number"
          value={terminalsSidebarWidthCh}
          onChange={(e) =>
            setTerminalsSidebarWidthCh(Math.max(10, Math.min(80, Number(e.target.value))))
          }
          min={10}
          max={80}
          className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
        />
        <p className="text-xs text-surface-text-muted">Min: 10, Max: 80. Default: 20.</p>
      </div>

      <div className="space-y-1">
        <label className="text-xs text-surface-text-muted">Commands List Width (characters)</label>
        <input
          type="number"
          value={commandsSidebarWidthCh}
          onChange={(e) =>
            setCommandsSidebarWidthCh(Math.max(10, Math.min(80, Number(e.target.value))))
          }
          min={10}
          max={80}
          className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
        />
        <p className="text-xs text-surface-text-muted">Min: 10, Max: 80. Default: 30.</p>
      </div>

      <h3 className="text-xs font-semibold text-surface-text-muted uppercase tracking-wider pt-2">
        Remote Connections
      </h3>

      <label className="flex items-start gap-2 text-xs text-surface-text-muted">
        <input
          type="checkbox"
          className="mt-0.5"
          checked={promptNewHostKeys}
          onChange={(e) => setPromptNewHostKeys(e.target.checked)}
        />
        <span>
          Prompt to accept new SSH host keys
          <span className="block text-surface-text-muted/80">
            Off by default: the first connection to a new host silently trusts and
            remembers its key (most target hosts don't give you an independently
            verifiable fingerprint to check anyway), and any later connection
            presenting a different key is always refused, regardless of this
            setting. Turn this on if your security team wants to review and accept
            each new host's fingerprint explicitly.
          </span>
        </span>
      </label>

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
