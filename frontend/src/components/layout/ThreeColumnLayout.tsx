import { useState, useEffect, useCallback, type ReactNode } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useSettingsStore } from "@/stores/settingsStore";
import { useCommandStore } from "@/stores/commandStore";
import { useTerminalCapture } from "@/hooks/useTerminalCapture";
import { useDefaultTerminalFocus } from "@/hooks/useDefaultTerminalFocus";
import { useAddClipboardCommandHotkey } from "@/hooks/useAddClipboardCommandHotkey";
import { useNewTerminalHotkey } from "@/hooks/useNewTerminalHotkey";
import { useAddCommandHotkey } from "@/hooks/useAddCommandHotkey";
import { TerminalTabList } from "@/components/terminal/TerminalTabList";
import { TerminalPreview } from "@/components/terminal/TerminalPreview";
import { QuickSelectOverlay } from "@/components/QuickSelectOverlay";
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
  useNewTerminalHotkey(); // Ctrl+Shift+N (configurable): opens the "+ Connect" terminal dialog
  useAddCommandHotkey(); // Ctrl+Shift+P (configurable): opens the "Add Command" dialog

  const activeTabId = useTerminalStore((state) => state.activeTabId);
  const tabs = useTerminalStore((state) => state.tabs);
  const settingsOpen = useSettingsStore((s) => s.settingsOpen);
  const setSettingsOpen = useSettingsStore((s) => s.setSettingsOpen);
  const setActiveModel = useSettingsStore((s) => s.setActiveModel);
  const setConnectionStatus = useSettingsStore((s) => s.setConnectionStatus);
  const [adapterStatus, setAdapterStatus] = useState<AdapterStatusInfo[]>([]);
  // Sidebar widths are deliberately not a live drag-resizable UI (an extra
  // source of layout bugs) — configured in Settings → Terminals instead, in
  // `ch` units, and applied once here at mount. Taking effect only after a
  // restart (rather than reacting live to a settings change) keeps this to
  // a single GetSettings() read instead of needing a subscription.
  const [terminalsSidebarWidthCh, setTerminalsSidebarWidthCh] = useState(20);
  const [commandsSidebarWidthCh, setCommandsSidebarWidthCh] = useState(30);
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
        if (cfg?.TerminalsSidebarWidthCh) setTerminalsSidebarWidthCh(cfg.TerminalsSidebarWidthCh);
        if (cfg?.CommandsSidebarWidthCh) setCommandsSidebarWidthCh(cfg.CommandsSidebarWidthCh);
        // Restore commands the user explicitly saved via "Save Pinned" so
        // they're back in the sidebar immediately, without waiting on any
        // terminal tab to exist yet — tabId is just a provenance hint on the
        // Command record, not something these need to be scoped to.
        if (cfg?.PinnedCommands?.length) {
          const { addPinnedCommand } = useCommandStore.getState();
          for (const pc of cfg.PinnedCommands) {
            addPinnedCommand("", {
              command: pc.Command,
              originalQuestion: pc.OriginalQuestion,
              name: pc.Name,
            });
          }
        }
        if (!provider) {
          setConnectionStatus("disconnected");
          return;
        }
        // "Disable Pair LLM" (Settings → LLM Config) is an explicit opt-out:
        // never probe, never show Connected/Disconnected — show Disabled and
        // surface it in the chat input too.
        if (provider === "disabled") {
          setConnectionStatus("disabled");
          return;
        }
        try {
          await TestConnection(provider, model ?? "", "");
          // The probe must only fill in the answer while it's still the
          // newest authority: if the status has moved on while this network
          // call was in flight — most importantly the user saving "Disable
          // Pair LLM" mid-probe — their choice wins and this stale result is
          // discarded rather than flipping "disabled" back to connected.
          if (useSettingsStore.getState().connectionStatus === "checking") {
            setConnectionStatus("connected");
          }
        } catch {
          if (useSettingsStore.getState().connectionStatus === "checking") {
            setConnectionStatus("disconnected");
          }
        }
      })
      .catch(() => {}); // Wails runtime unavailable in test/dev environments
  }, [setActiveModel, setConnectionStatus]);

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden">
      <div className="flex flex-1 overflow-hidden bg-surface-0 text-surface-text">
        {/* Left column: terminal tab list */}
        <aside
          className="flex-none border-r border-surface-border overflow-y-auto"
          style={{ width: `${terminalsSidebarWidthCh}ch` }}
        >
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
            <button
              onClick={toggleChatVisible}
              className="flex-none w-full py-1 text-center text-xs text-surface-text-muted hover:text-surface-text hover:bg-surface-1 border-b border-surface-border bg-surface-0 transition-colors"
            >
              {chatVisible ? "Hide PairAdmin" : "Show PairAdmin"}
            </button>
            <div className={chatVisible ? "flex flex-1 flex-col overflow-hidden" : "hidden"}>
              {children}
            </div>
          </div>
        </main>

        {/* Right column: command sidebar */}
        <aside
          className="flex-none border-l border-surface-border overflow-y-auto"
          style={{ width: `${commandsSidebarWidthCh}ch` }}
        >
          {sidebar}
        </aside>
      </div>

      {/* Quick-select F-key labels — fixed overlay above both sidebars
          (terminal tabs left, pinned commands right). pointer-events-none,
          so it can't intercept clicks or steal focus; purely visual. */}
      <QuickSelectOverlay />

      <StatusBar />
      <SettingsDialog open={settingsOpen} onClose={handleCloseSettings} />
    </div>
  );
}
