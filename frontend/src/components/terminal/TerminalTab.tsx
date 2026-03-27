import type { TerminalTab } from "@/stores/terminalStore";

interface TerminalTabProps {
  tab: TerminalTab;
  isActive: boolean;
  onClick: () => void;
}

export function TerminalTab({ tab, isActive, onClick }: TerminalTabProps) {
  return (
    <button
      className={
        isActive
          ? "w-full px-3 py-2 text-left text-sm bg-zinc-800 text-zinc-100 border-l-2 border-blue-500"
          : "w-full px-3 py-2 text-left text-sm text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200 border-l-2 border-transparent transition-colors"
      }
      onClick={onClick}
    >
      <span
        className={`inline-block w-1.5 h-1.5 rounded-full mr-2 ${isActive ? "bg-green-500" : "bg-zinc-600"}`}
      />
      {tab.name}
    </button>
  );
}
