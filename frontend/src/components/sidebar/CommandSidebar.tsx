import { useMemo, useRef, useState } from "react";
import { Plus, Pin } from "lucide-react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { sendToTerminal } from "@/utils/sendToTerminal";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Button } from "@/components/ui/button";
import { CommandCard } from "./CommandCard";
import { ClearHistoryButton } from "./ClearHistoryButton";
import { EditCommandDialog } from "./EditCommandDialog";

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
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [pinSaveStatus, setPinSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  const handleSavePinned = async () => {
    setPinSaveStatus("saving");
    try {
      const { SavePinnedCommands } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await SavePinnedCommands(
        pinnedCommands.map((c) => ({
          Command: c.command,
          OriginalQuestion: c.originalQuestion,
          Name: c.name ?? "",
        }))
      );
      setPinSaveStatus("saved");
      setTimeout(() => setPinSaveStatus("idle"), 2000);
    } catch {
      setPinSaveStatus("error");
      setTimeout(() => setPinSaveStatus("idle"), 3000);
    }
  };

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
      <div className="px-3 py-2 text-xs font-semibold text-surface-text-muted uppercase tracking-wider">
        Commands
      </div>

      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-1 px-2">
          {pinnedCommands.length === 0 && unpinnedCommands.length === 0 ? (
            <p className="text-surface-text-muted text-xs text-center py-4">
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
                <div className="border-t border-surface-border my-1" />
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

      <div className="p-2 border-t border-surface-border space-y-1">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setAddDialogOpen(true)}
          className="w-full text-xs text-surface-text-muted hover:text-surface-text"
        >
          <Plus size={14} />
          Add Command
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleSavePinned}
          disabled={pinSaveStatus === "saving" || pinnedCommands.length === 0}
          className="w-full text-xs text-surface-text-muted hover:text-surface-text"
        >
          <Pin size={14} />
          {pinSaveStatus === "saving"
            ? "Saving..."
            : pinSaveStatus === "saved"
              ? "Saved!"
              : pinSaveStatus === "error"
                ? "Save failed"
                : "Save Pinned"}
        </Button>
        <ClearHistoryButton
          onClick={() => useCommandStore.getState().clearAll()}
        />
      </div>

      <EditCommandDialog
        open={addDialogOpen}
        mode="add"
        initialValue=""
        onSave={(value, name) => {
          useCommandStore.getState().addCommand(activeTabId, {
            command: value,
            originalQuestion: "",
            name,
          });
          setAddDialogOpen(false);
        }}
        onClose={() => setAddDialogOpen(false)}
      />
    </div>
  );
}
