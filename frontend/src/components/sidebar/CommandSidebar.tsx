import { useMemo } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { sendToTerminal } from "@/utils/sendToTerminal";
import { ScrollArea } from "@/components/ui/scroll-area";
import { CommandCard } from "./CommandCard";
import { ClearHistoryButton } from "./ClearHistoryButton";

export function CommandSidebar() {
  const activeTabId = useTerminalStore((state) => state.activeTabId);
  // Subscribe to the actual per-tab command array (not the getCommandsForTab
  // function, whose reference never changes) so Zustand re-renders this
  // panel whenever a command is added.
  const tabCommands = useCommandStore((state) => state.commandsByTab[activeTabId]);
  const commands = useMemo(
    () => [...(tabCommands ?? [])].sort((a, b) => b.timestamp - a.timestamp),
    [tabCommands]
  );

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
                onCopy={(text) => sendToTerminal(activeTabId, text, false)}
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
