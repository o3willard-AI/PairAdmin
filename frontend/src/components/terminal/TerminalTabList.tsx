import { useTerminalStore } from "@/stores/terminalStore";
import { TerminalTab } from "./TerminalTab";
import { NewTerminalDialog } from "./NewTerminalDialog";

export function TerminalTabList() {
  const tabs = useTerminalStore((state) => state.tabs);
  const activeTabId = useTerminalStore((state) => state.activeTabId);
  const dialogOpen = useTerminalStore((state) => state.newTerminalDialogOpen);
  const setDialogOpen = useTerminalStore((state) => state.setNewTerminalDialogOpen);

  return (
    <div className="flex flex-col h-full">
      <div className="px-3 py-2 text-xs font-semibold text-surface-text-muted uppercase tracking-wider">
        Terminals
      </div>
      <div className="flex-1 overflow-y-auto">
        {tabs.map((tab) => (
          <TerminalTab
            key={tab.id}
            tab={tab}
            isActive={tab.id === activeTabId}
            onClick={() => useTerminalStore.getState().setActiveTab(tab.id)}
          />
        ))}
      </div>
      <button
        onClick={() => setDialogOpen(true)}
        className="w-full px-3 py-1.5 text-xs text-surface-text-muted hover:text-surface-text transition-colors"
      >
        + Connect
      </button>
      <NewTerminalDialog
        open={dialogOpen}
        onClose={() => {
          setDialogOpen(false);
          // base-ui's Dialog returns focus to its trigger ("+ Connect") on
          // close — without this, the very next keystroke (e.g. the user
          // typing into their freshly-connected terminal) would re-trigger
          // "+ Connect" instead, opening an unwanted duplicate dialog. Deferring
          // to the next frame lets that built-in restoration finish first,
          // then wins the race back to the terminal (same pattern as
          // CommandCard.tsx/TerminalTab.tsx's rename-input focus race).
          requestAnimationFrame(() => {
            const { activeTabId, getTermRef } = useTerminalStore.getState();
            getTermRef(activeTabId)?.focus();
          });
        }}
      />
    </div>
  );
}
