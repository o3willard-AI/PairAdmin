import { useState } from "react";
import { Copy, RotateCw, Pin, PinOff, Trash2, Pencil, History } from "lucide-react";
import { useCommandStore, type Command } from "@/stores/commandStore";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
} from "@/components/ui/context-menu";
import { EditCommandDialog } from "./EditCommandDialog";

interface CommandCardProps {
  command: Command;
  onCopy: (id: string) => void;
  onExecute: (id: string) => void;
  draggable?: boolean;
  onDragStartId?: (id: string) => void;
  onDropOnId?: (id: string) => void;
}

export function CommandCard({
  command,
  onCopy,
  onExecute,
  draggable = false,
  onDragStartId,
  onDropOnId,
}: CommandCardProps) {
  const [editing, setEditing] = useState<"permanent" | "temporary" | null>(null);
  const displayText = command.tempOverride ?? command.command;

  const startEdit = (mode: "permanent" | "temporary") => setEditing(mode);

  const saveEdit = (value: string) => {
    if (editing === "permanent") {
      useCommandStore.getState().editCommand(command.id, value);
    } else if (editing === "temporary") {
      useCommandStore.getState().editForNextUse(command.id, value);
    }
    setEditing(null);
  };

  const cancelEdit = () => setEditing(null);

  return (
    <>
      <EditCommandDialog
        open={editing !== null}
        mode={editing ?? "permanent"}
        initialValue={displayText}
        onSave={saveEdit}
        onClose={cancelEdit}
      />
      <TooltipProvider>
      <ContextMenu>
        <Tooltip>
          <ContextMenuTrigger
            render={
              <TooltipTrigger
                render={<div />}
                data-testid="command-card"
                draggable={draggable}
                onDragStart={() => onDragStartId?.(command.id)}
                onDragOver={(e) => draggable && e.preventDefault()}
                onDrop={() => onDropOnId?.(command.id)}
                className="group w-full text-left px-3 py-2 text-xs font-mono bg-surface-1 hover:bg-surface-2 rounded border border-surface-border hover:border-surface-border-strong transition-colors flex items-center gap-1"
              />
            }
          >
            {command.pinned && (
              <Pin size={10} className="flex-none text-amber-500" />
            )}
            <span className="truncate flex-1">{displayText}</span>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onCopy(command.id);
              }}
              aria-label="Copy to Terminal"
              className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-surface-text-muted hover:text-surface-text"
            >
              <Copy size={12} />
            </button>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onExecute(command.id);
              }}
              aria-label="Execute in Terminal"
              className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-surface-text-muted hover:text-surface-text"
            >
              <RotateCw size={12} />
            </button>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                startEdit("permanent");
              }}
              aria-label="Edit Command"
              className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-surface-text-muted hover:text-surface-text"
            >
              <Pencil size={12} />
            </button>
          </ContextMenuTrigger>
          <TooltipContent side="left" className="max-w-[280px]">
            <p className="text-xs font-mono break-all">{displayText}</p>
            {command.originalQuestion && (
              <>
                <p className="text-xs text-surface-text-muted mt-1.5 mb-0.5">Generated from:</p>
                <p className="text-xs">{command.originalQuestion}</p>
              </>
            )}
          </TooltipContent>
        </Tooltip>
        <ContextMenuContent>
          <ContextMenuItem
            onClick={() => useCommandStore.getState().togglePin(command.id)}
          >
            {command.pinned ? <PinOff size={12} /> : <Pin size={12} />}
            {command.pinned ? "Unpin" : "Pin"}
          </ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuItem onClick={() => startEdit("permanent")}>
            <Pencil size={12} />
            Edit
          </ContextMenuItem>
          <ContextMenuItem onClick={() => startEdit("temporary")}>
            <History size={12} />
            Edit/Append for next use
          </ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuItem
            onClick={() => useCommandStore.getState().removeCommand(command.id)}
          >
            <Trash2 size={12} />
            Remove
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      </TooltipProvider>
    </>
  );
}
