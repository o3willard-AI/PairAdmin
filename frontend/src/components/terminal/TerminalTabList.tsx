import { useTerminalStore } from "@/stores/terminalStore";
import { TerminalTab } from "./TerminalTab";

export function TerminalTabList() {
  const tabs = useTerminalStore((state) => state.tabs);
  const activeTabId = useTerminalStore((state) => state.activeTabId);

  // The ".+ Connect" button, the NewTerminalDialog, and the dialog's
  // close-focus-restore live in ThreeColumnLayout's left <aside>, BELOW the
  // NetworkPanel — this list is just the "Terminals" header + the scrollable
  // tab list. flex-1 min-h-0 makes it the sidebar's single flex-grow (scroll)
  // container; without min-h-0 a long tab list would force the aside to grow
  // instead of scrolling.
  return (
    <div className="flex flex-col flex-1 min-h-0">
      <div className="px-3 py-2 text-xs font-semibold text-surface-text-muted uppercase tracking-wider">
        Terminals
      </div>
      <div className="flex-1 overflow-y-auto min-h-0">
        {tabs.map((tab) => (
          <TerminalTab
            key={tab.id}
            tab={tab}
            isActive={tab.id === activeTabId}
            onClick={() => useTerminalStore.getState().setActiveTab(tab.id)}
          />
        ))}
      </div>
    </div>
  );
}