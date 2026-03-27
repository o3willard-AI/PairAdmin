import { useTerminalStore } from "@/stores/terminalStore";
import { TerminalTab } from "./TerminalTab";

export function TerminalTabList() {
  const tabs = useTerminalStore((state) => state.tabs);
  const activeTabId = useTerminalStore((state) => state.activeTabId);

  return (
    <div className="flex flex-col h-full">
      <div className="px-3 py-2 text-xs font-semibold text-zinc-500 uppercase tracking-wider">
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
        disabled
        className="w-full px-3 py-1.5 text-xs text-zinc-600 hover:text-zinc-400 transition-colors"
      >
        + New
      </button>
    </div>
  );
}
