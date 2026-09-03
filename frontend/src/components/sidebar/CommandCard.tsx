import { useState } from "react";
import { Copy, RotateCw, Pin, PinOff, Trash2, Pencil, History, Edit3 } from "lucide-react";
import { useCommandStore, type Command } from "@/stores/commandStore";
import { useQuickSelectStore } from "@/stores/quickSelectStore";
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
import { RenameDialog } from "./RenameDialog";

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
  const [renaming, setRenaming] = useState(false);
  const displayText = command.name ?? (command.tempOverride ?? command.command);
  const fullCommandText = command.tempOverride ?? command.command;

  const startEdit = (mode: "permanent" | "temporary") => setEditing(mode);
  const cancelEdit = () => setEditing(null);

  const saveEdit = (value: string, name?: string) => {
    if (editing === "permanent") {
      useCommandStore.getState().editCommand(command.id, value);
      // Always sync the name in permanent edit mode — passing an empty
      // string (or undefined coerced to "") clears any previously-set name,
      // matching the RenameDialog behavior where clearing the field removes
      // the name. Only applies in permanent mode; temporary edits don't
      // touch the name.
      useCommandStore.getState().renameCommand(command.id, name ?? "");
    } else if (editing === "temporary") {
      useCommandStore.getState().editForNextUse(command.id, value);
    }
    setEditing(null);
  };

  const cancelRename = () => setRenaming(false);

  const saveRename = (name: string) => {
    useCommandStore.getState().renameCommand(command.id, name);
    setRenaming(false);
  };

  // Quick-select signage: this card's own F-key while the chord is held.
  // Subscribed per-card so only affected rows re-render on chord press/
  // release. Signage only — see the badge's aria-hidden/pointer-events-none.
  const fkey = useQuickSelectStore(
    (s) => (s.visible ? s.commandFkeys[command.id] : undefined)
  );

  return (
    <>
      <EditCommandDialog
        open={editing !== null}
        mode={editing ?? "permanent"}
        initialValue={command.tempOverride ?? command.command}
        initialName={command.name}
        onSave={saveEdit}
        onClose={cancelEdit}
      />
      <RenameDialog
        open={renaming}
        initialValue={command.name ?? ""}
        onClose={cancelRename}
        onSave={saveRename}
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
                className="group relative w-full text-left px-3 py-2 text-xs font-mono bg-surface-1 hover:bg-surface-2 rounded border border-surface-border hover:border-surface-border-strong transition-colors flex items-center gap-1"
              />
            }
          >
            {command.pinned && (
              <Pin size={10} className="flex-none text-amber-500" />
            )}
            <span className="truncate flex-1">{displayText}</span>
            {fkey && (
              <span
                aria-hidden="true"
                className="absolute right-1 top-1/2 -translate-y-1/2 px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold leading-none bg-surface-3/95 text-surface-text border border-surface-border-strong shadow-sm pointer-events-none"
              >
                {fkey}
              </span>
            )}
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
            {command.name && (
              <p className="text-xs font-semibold text-surface-text mb-1">
                {command.name}
              </p>
            )}
            <p className="text-xs font-mono break-all">{fullCommandText}</p>
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
          <ContextMenuItem onClick={() => setRenaming(true)}>
            <Edit3 size={12} />
            Rename
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
