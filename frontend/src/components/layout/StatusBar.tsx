import { useEffect, useState } from "react";
import { Loader2, Settings } from "lucide-react";
import { useChatStore } from "@/stores/chatStore";
import { useTerminalStore } from "@/stores/terminalStore";
import { useSettingsStore } from "@/stores/settingsStore";

const CONNECTION_LABEL: Record<string, string> = {
  checking: "Checking…",
  connected: "Connected",
  disconnected: "Disconnected",
  disabled: "Disabled",
};

// "disabled" uses an amber dot (deliberately chosen, not an error): the LLM
// isn't unreachable, the user turned it off in Settings → LLM Config.
const CONNECTION_DOT: Record<string, string> = {
  checking: "bg-surface-text-muted",
  connected: "bg-green-500",
  disconnected: "bg-red-500",
  disabled: "bg-amber-500",
};

export function StatusBar() {
  const activeTabId = useTerminalStore((s) => s.activeTabId);
  const activeModel = useSettingsStore((s) => s.activeModel);
  const connectionStatus = useSettingsStore((s) => s.connectionStatus);
  const setSettingsOpen = useSettingsStore((s) => s.setSettingsOpen);
  const llmRequest = useChatStore((s) => s.llmRequest);
  const lastTokenCount = useChatStore((s) => {
    const msgs = s.messagesByTab[activeTabId];
    if (!msgs || msgs.length === 0) return undefined;
    for (let i = msgs.length - 1; i >= 0; i--) {
      if (msgs[i].tokenCount != null) return msgs[i].tokenCount;
    }
    return undefined;
  });

  // Elapsed-seconds ticker for the LLM activity indicator. Only mounts the
  // interval while a request is in flight; Date.now() is read on each tick so
  // the label re-derives from the request's startedAt.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!llmRequest) return;
    setNow(Date.now());
    const id = setInterval(() => setNow(Date.now()), 100);
    return () => clearInterval(id);
  }, [llmRequest]);

  return (
    <div className="h-7 flex-none flex items-center px-3 text-xs text-surface-text-muted bg-surface-1 border-t border-surface-border gap-4">
      {/* Left: model indicator */}
      <div className="flex items-center gap-1.5">
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-surface-text-muted" />
        <span>{activeModel || "No model"}</span>
      </div>

      {/* Center: connection status */}
      <div className="flex-1 text-center flex items-center justify-center gap-1.5">
        <span className={`inline-block w-1.5 h-1.5 rounded-full ${CONNECTION_DOT[connectionStatus]}`} />
        <span>{CONNECTION_LABEL[connectionStatus]}</span>
      </div>

      {/* Right: LLM activity + token meter */}
      <div className="flex items-center gap-3">
        {llmRequest && (
          <div
            role="status"
            aria-live="polite"
            className="flex items-center gap-1.5 text-surface-text"
          >
            <Loader2 size={12} className="animate-spin" aria-hidden="true" />
            {/* Stable label ("Working…") keeps the live region calm; only the
                elapsed figure changes as the interval ticks. */}
            <span>Working…</span>
            <span className="tabular-nums">
              {((now - llmRequest.startedAt) / 1000).toFixed(1)}s
            </span>
          </div>
        )}
        <span>{lastTokenCount != null ? `Tokens: ${lastTokenCount}` : "Tokens: —"}</span>
        <button
          className="hover:text-surface-text transition-colors"
          onClick={() => setSettingsOpen(true)}
          aria-label="Settings"
        >
          <Settings size={14} />
        </button>
      </div>
    </div>
  );
}
