import { useMemo, useRef } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { sendToTerminal } from "@/utils/sendToTerminal";
import { ScrollArea } from "@/components/ui/scroll-area";
import { CommandCard } from "./CommandCard";
import { ClearHistoryButton } from "./ClearHistoryButton";

export function CommandSidebar() {
  const activeTabId = useTerminalStore((state) => state.activeTabId);
  // Commands are shared across every terminal tab — switching tabs only
  // changes which terminal a click writes to, not which commands are shown.
  const allCommands = useCommandStore((state) => state.commands);
  // Pinned commands stay on top in a user-orderable list; unpinned commands
  // stay below in plain insertion order — neither group is re-sorted by
  // timestamp, so a drag reorder (pinned) or new addition (unpinned) is the
  // only thing that changes display order.
  const pinnedCommands = useMemo(
    () => allCommands.filter((c) => c.pinned),
    [allCommands]
  );
  const unpinnedCommands = useMemo(
    () => allCommands.filter((c) => !c.pinned),
    [allCommands]
  );
  const draggedIdRef = useRef<string | null>(null);

  const handleCopy = (id: string) => {
    const text = useCommandStore.getState().consumeCommandText(id);
    sendToTerminal(activeTabId, text, false);
  };
  const handleExecute = (id: string) => {
    const text = useCommandStore.getState().consumeCommandText(id);
    sendToTerminal(activeTabId, text, true);
  };

  return (
    <div className="flex flex-col h-full">
      <div className="px-3 py-2 text-xs font-semibold text-zinc-500 uppercase tracking-wider">
        Commands
      </div>

      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 px-2">
          {pinnedCommands.length === 0 && unpinnedCommands.length === 0 ? (
            <p className="text-zinc-600 text-xs text-center py-4">
              No commands yet
            </p>
          ) : (
            <>
              {pinnedCommands.map((command) => (
                <CommandCard
                  key={command.id}
                  command={command}
                  onCopy={handleCopy}
                  onExecute={handleExecute}
                  draggable
                  onDragStartId={(id) => {
                    draggedIdRef.current = id;
                  }}
                  onDropOnId={(targetId) => {
                    if (draggedIdRef.current) {
                      useCommandStore
                        .getState()
                        .reorderPinned(draggedIdRef.current, targetId);
                    }
                    draggedIdRef.current = null;
                  }}
                />
              ))}
              {pinnedCommands.length > 0 && unpinnedCommands.length > 0 && (
                <div className="border-t border-zinc-800 my-1" />
              )}
              {unpinnedCommands.map((command) => (
                <CommandCard
                  key={command.id}
                  command={command}
                  onCopy={handleCopy}
                  onExecute={handleExecute}
                />
              ))}
            </>
          )}
        </div>
      </ScrollArea>

      <div className="p-2 border-t border-zinc-800">
        <ClearHistoryButton
          onClick={() => useCommandStore.getState().clearAll()}
        />
      </div>
    </div>
  );
}
