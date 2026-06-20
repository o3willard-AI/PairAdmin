import { useState, useEffect, useCallback, type ReactNode } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useSettingsStore } from "@/stores/settingsStore";
import { useTerminalCapture } from "@/hooks/useTerminalCapture";
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

export function ThreeColumnLayout({ children, sidebar }: ThreeColumnLayoutProps) {
  useTerminalCapture(); // Subscribe to terminal events from Go service

  const activeTabId = useTerminalStore((state) => state.activeTabId);
  const tabs = useTerminalStore((state) => state.tabs);
  const settingsOpen = useSettingsStore((s) => s.settingsOpen);
  const setSettingsOpen = useSettingsStore((s) => s.setSettingsOpen);
  const [adapterStatus, setAdapterStatus] = useState<AdapterStatusInfo[]>([]);
  const handleCloseSettings = useCallback(() => setSettingsOpen(false), [setSettingsOpen]);

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/capture/CaptureManager")
      .then(({ GetAdapterStatus }) => GetAdapterStatus())
      .then(setAdapterStatus)
      .catch(() => {}); // Wails runtime unavailable in test/dev environments
  }, []);

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden">
      <div className="flex flex-1 overflow-hidden bg-zinc-950 text-zinc-100">
        {/* Left column: terminal tab list */}
        <aside className="w-40 flex-none border-r border-zinc-800 overflow-y-auto">
          <TerminalTabList />
        </aside>

        {/* Center column: chat area + terminal preview */}
        <main className="flex flex-1 flex-col overflow-hidden">
          {/* Upper: chat area */}
          <div className="flex flex-1 flex-col overflow-hidden">{children}</div>

          {/* Lower: xterm.js terminal preview. Each tab keeps its own persistently
              mounted TerminalPreview so switching tabs doesn't recreate (and lose
              the scrollback/session of) the underlying xterm instance. */}
          <div className="h-[30%] border-t border-zinc-800 relative">
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
        </main>

        {/* Right column: command sidebar */}
        <aside className="w-[220px] flex-none border-l border-zinc-800 overflow-y-auto">
          {sidebar}
        </aside>
      </div>

      <StatusBar />
      <SettingsDialog open={settingsOpen} onClose={handleCloseSettings} />
    </div>
  );
}
