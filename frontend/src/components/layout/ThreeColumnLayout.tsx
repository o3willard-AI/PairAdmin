import { useState, useEffect, useCallback, type ReactNode } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useSettingsStore } from "@/stores/settingsStore";
import { useTerminalCapture } from "@/hooks/useTerminalCapture";
import { useDefaultTerminalFocus } from "@/hooks/useDefaultTerminalFocus";
import { useAddClipboardCommandHotkey } from "@/hooks/useAddClipboardCommandHotkey";
import { TerminalTabList } from "@/components/terminal/TerminalTabList";
import { TerminalPreview } from "@/components/terminal/TerminalPreview";
import { StatusBar } from "./StatusBar";
import { SettingsDialog } from "@/components/settings/SettingsDialog";

interface AdapterStatusInfo {
  name: string;
  status: string;
  message: string;
}

interface ThreeColumnLayoutProps {
  children?: ReactNode;
  sidebar?: ReactNode;
}

// Persisted across restarts (mirrors theme-provider.tsx's localStorage pattern)
// so a sysadmin who mostly wants a terminal doesn't have to re-hide it every launch.
const CHAT_VISIBLE_STORAGE_KEY = "pairadmin-chat-visible";

export function ThreeColumnLayout({ children, sidebar }: ThreeColumnLayoutProps) {
  useTerminalCapture(); // Subscribe to terminal events from Go service
  useDefaultTerminalFocus(); // Keep keyboard focus on the terminal by default
  useAddClipboardCommandHotkey(); // Ctrl+Shift+A (configurable): clipboard -> new sidebar command

  const activeTabId = useTerminalStore((state) => state.activeTabId);
  const tabs = useTerminalStore((state) => state.tabs);
  const settingsOpen = useSettingsStore((s) => s.settingsOpen);
  const setSettingsOpen = useSettingsStore((s) => s.setSettingsOpen);
  const setActiveModel = useSettingsStore((s) => s.setActiveModel);
  const setConnectionStatus = useSettingsStore((s) => s.setConnectionStatus);
  const [adapterStatus, setAdapterStatus] = useState<AdapterStatusInfo[]>([]);
  // Lets a user reclaim the chat pane's vertical space for a bigger terminal
  // when they don't need the assistant — the chat area (and its useLLMStream
  // subscription) stays mounted underneath, just visually collapsed, so an
  // in-flight response keeps streaming and is there when shown again.
  const [chatVisible, setChatVisible] = useState(
    () => localStorage.getItem(CHAT_VISIBLE_STORAGE_KEY) !== "false"
  );
  const toggleChatVisible = useCallback(() => {
    setChatVisible((prev) => {
      const next = !prev;
      localStorage.setItem(CHAT_VISIBLE_STORAGE_KEY, String(next));
      return next;
    });
  }, []);
  const handleCloseSettings = useCallback(() => {
    setSettingsOpen(false);
    // base-ui's Dialog returns focus to its trigger (the gear icon) on
    // close — deferring to the next frame lets that finish first, then wins
    // the race back to the terminal. Same pattern already used in
    // CommandCard.tsx/TerminalTab.tsx for the identical class of race.
    requestAnimationFrame(() => {
      const { activeTabId, getTermRef } = useTerminalStore.getState();
      getTermRef(activeTabId)?.focus();
    });
  }, [setSettingsOpen]);

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/capture/CaptureManager")
      .then(({ GetAdapterStatus }) => GetAdapterStatus())
      .then(setAdapterStatus)
      .catch(() => {}); // Wails runtime unavailable in test/dev environments
  }, []);

  // The status bar previously showed "No model" / "Disconnected" forever —
  // both were hardcoded/never-populated. Fetch the actually configured
  // provider+model on startup and verify connectivity against it.
  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(async ({ GetSettings, TestConnection }) => {
        const cfg = await GetSettings();
        const provider = cfg?.Provider;
        const model = cfg?.Model;
        if (provider && model) {
          setActiveModel(`${provider}:${model}`);
        }
        if (!provider) {
          setConnectionStatus("disconnected");
          return;
        }
        try {
          await TestConnection(provider, model ?? "", "");
          setConnectionStatus("connected");
        } catch {
          setConnectionStatus("disconnected");
        }
      })
      .catch(() => {}); // Wails runtime unavailable in test/dev environments
  }, [setActiveModel, setConnectionStatus]);

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden">
      <div className="flex flex-1 overflow-hidden bg-surface-0 text-surface-text">
        {/* Left column: terminal tab list */}
        <aside className="w-40 flex-none border-r border-surface-border overflow-y-auto">
          <TerminalTabList />
        </aside>

        {/* Center column: terminal preview + chat area, top to bottom.
            The two are flex-basis-0 so they split the available height evenly
            (when chat is visible) — the chat input's own fixed height comes
            out of the chat section's share, so terminal and chat message area
            end up approximately equal rather than exactly equal. When chat is
            hidden, the terminal section takes all remaining space instead. */}
        <main className="flex flex-1 flex-col overflow-hidden">
          {/* Upper: xterm.js terminal preview. Each tab keeps its own persistently
              mounted TerminalPreview so switching tabs doesn't recreate (and lose
              the scrollback/session of) the underlying xterm instance. */}
          <div
            key="terminal-section"
            className={`border-b border-surface-border relative ${
              chatVisible ? "flex-1 basis-0" : "flex-1"
            }`}
          >
            {tabs.length === 0 && (
              <TerminalPreview tabId="" adapterStatus={adapterStatus} />
            )}
            {tabs.map((tab) => (
              <div
                key={tab.id}
                className="absolute inset-0"
                style={{ display: tab.id === activeTabId ? "block" : "none" }}
              >
                <TerminalPreview tabId={tab.id} adapterStatus={adapterStatus} />
              </div>
            ))}
          </div>

          {/* Lower: chat area (chat message list + input box), collapsible.
              `children` (ChatPane) stays mounted at all times — only its
              wrapper's visibility toggles — so useLLMStream's subscription
              keeps receiving events and an in-flight response isn't lost
              while collapsed. */}
          <div
            key="chat-section"
            className={`flex flex-col overflow-hidden ${chatVisible ? "flex-1 basis-0" : "flex-none"}`}
          >
            <div className="flex-none flex items-center justify-center border-b border-surface-border bg-surface-0">
              <button
                onClick={toggleChatVisible}
                className="px-3 py-1 text-xs text-surface-text-muted hover:text-surface-text transition-colors"
              >
                {chatVisible ? "Hide PairAdmin" : "Show PairAdmin"}
              </button>
            </div>
            <div className={chatVisible ? "flex flex-1 flex-col overflow-hidden" : "hidden"}>
              {children}
            </div>
          </div>
        </main>

        {/* Right column: command sidebar */}
        <aside className="w-[220px] flex-none border-l border-surface-border overflow-y-auto">
          {sidebar}
        </aside>
      </div>

      <StatusBar />
      <SettingsDialog open={settingsOpen} onClose={handleCloseSettings} />
    </div>
  );
}
