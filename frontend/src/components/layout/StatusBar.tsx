import { Settings } from "lucide-react";
import { useChatStore } from "@/stores/chatStore";
import { useTerminalStore } from "@/stores/terminalStore";
import { useSettingsStore } from "@/stores/settingsStore";

const CONNECTION_LABEL: Record<string, string> = {
  checking: "Checking…",
  connected: "Connected",
  disconnected: "Disconnected",
};

const CONNECTION_DOT: Record<string, string> = {
  checking: "bg-zinc-600",
  connected: "bg-green-500",
  disconnected: "bg-red-500",
};

export function StatusBar() {
  const activeTabId = useTerminalStore((s) => s.activeTabId);
  const activeModel = useSettingsStore((s) => s.activeModel);
  const connectionStatus = useSettingsStore((s) => s.connectionStatus);
  const setSettingsOpen = useSettingsStore((s) => s.setSettingsOpen);
  const lastTokenCount = useChatStore((s) => {
    const msgs = s.messagesByTab[activeTabId];
    if (!msgs || msgs.length === 0) return undefined;
    for (let i = msgs.length - 1; i >= 0; i--) {
      if (msgs[i].tokenCount != null) return msgs[i].tokenCount;
    }
    return undefined;
  });

  return (
    <div className="h-7 flex-none flex items-center px-3 text-xs text-zinc-500 bg-zinc-900 border-t border-zinc-800 gap-4">
      {/* Left: model indicator */}
      <div className="flex items-center gap-1.5">
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-zinc-600" />
        <span>{activeModel || "No model"}</span>
      </div>

      {/* Center: connection status */}
      <div className="flex-1 text-center flex items-center justify-center gap-1.5">
        <span className={`inline-block w-1.5 h-1.5 rounded-full ${CONNECTION_DOT[connectionStatus]}`} />
        <span>{CONNECTION_LABEL[connectionStatus]}</span>
      </div>

      {/* Right: token meter */}
      <div className="flex items-center gap-3">
        <span>{lastTokenCount != null ? `Tokens: ${lastTokenCount}` : "Tokens: —"}</span>
        <button
          className="hover:text-zinc-300 transition-colors"
          onClick={() => setSettingsOpen(true)}
          aria-label="Settings"
        >
          <Settings size={14} />
        </button>
      </div>
    </div>
  );
}
