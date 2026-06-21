import { useState, useEffect, useRef } from "react";
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
  const [editValue, setEditValue] = useState("");
  const editInputRef = useRef<HTMLInputElement>(null);
  const displayText = command.tempOverride ?? command.command;

  const startEdit = (mode: "permanent" | "temporary") => {
    setEditValue(displayText);
    setEditing(mode);
  };

  // The context menu closes when "Edit" is clicked and returns focus to its
  // trigger asynchronously for accessibility — but entering edit mode
  // replaces that trigger entirely with this input. autoFocus alone loses
  // that race intermittently: the menu's focus-return steals focus right
  // back, firing onBlur (which commits) before the user can type anything,
  // making edits appear to silently do nothing. Deferring to the next frame
  // ensures our focus call runs after the menu's own settles.
  useEffect(() => {
    if (!editing) return;
    const id = requestAnimationFrame(() => editInputRef.current?.focus());
    return () => cancelAnimationFrame(id);
  }, [editing]);

  const commitEdit = () => {
    if (editing === "permanent") {
      useCommandStore.getState().editCommand(command.id, editValue);
    } else if (editing === "temporary") {
      useCommandStore.getState().editForNextUse(command.id, editValue);
    }
    setEditing(null);
  };

  const cancelEdit = () => setEditing(null);

  if (editing) {
    return (
      <div className="w-full px-3 py-2 bg-zinc-900 rounded border border-zinc-700">
        <input
          ref={editInputRef}
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") commitEdit();
            if (e.key === "Escape") cancelEdit();
          }}
          onBlur={commitEdit}
          className="w-full bg-transparent text-xs font-mono text-zinc-100 outline-none"
        />
        <p className="text-[10px] text-zinc-500 mt-1">
          {editing === "permanent"
            ? "Enter to save, Esc to cancel"
            : "Applies once on next use — Enter to save, Esc to cancel"}
        </p>
      </div>
    );
  }

  return (
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
                className="group w-full text-left px-3 py-2 text-xs font-mono bg-zinc-900 hover:bg-zinc-800 rounded border border-zinc-800 hover:border-zinc-700 transition-colors flex items-center gap-1"
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
              className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-zinc-400 hover:text-zinc-200"
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
              className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-zinc-400 hover:text-zinc-200"
            >
              <RotateCw size={12} />
            </button>
          </ContextMenuTrigger>
          <TooltipContent side="left" className="max-w-[280px]">
            <p className="text-xs font-mono break-all">{displayText}</p>
            {command.originalQuestion && (
              <>
                <p className="text-xs text-zinc-400 mt-1.5 mb-0.5">Generated from:</p>
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
  );
}
