import { useEffect } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { useWailsClipboard } from "@/hooks/useWailsClipboard";
import { ScrollArea } from "@/components/ui/scroll-area";
import { CommandCard } from "./CommandCard";
import { ClearHistoryButton } from "./ClearHistoryButton";

export function CommandSidebar() {
  const activeTabId = useTerminalStore((state) => state.activeTabId);
  const getCommandsForTab = useCommandStore((state) => state.getCommandsForTab);
  const commands = getCommandsForTab(activeTabId);
  const { copyToClipboard } = useWailsClipboard();

  useEffect(() => {
    useCommandStore.getState().initMockData();
  }, []);

  return (
    <div className="flex flex-col h-full">
      <div className="px-3 py-2 text-xs font-semibold text-zinc-500 uppercase tracking-wider">
        Commands
      </div>

      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 px-2">
          {commands.length === 0 ? (
            <p className="text-zinc-600 text-xs text-center py-4">
              No commands yet
            </p>
          ) : (
            commands.map((command) => (
              <CommandCard
                key={command.id}
                command={command}
                onCopy={copyToClipboard}
              />
            ))
          )}
        </div>
      </ScrollArea>

      <div className="p-2 border-t border-zinc-800">
        <ClearHistoryButton
          onClick={() => useCommandStore.getState().clearTab(activeTabId)}
        />
      </div>
    </div>
  );
}
