import { useEffect, useRef, useState } from "react";
import { Tooltip } from "@base-ui/react/tooltip";
import { useTerminalStore } from "@/stores/terminalStore";
import type { TerminalTab } from "@/stores/terminalStore";
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
} from "@/components/ui/context-menu";
import { Pencil } from "lucide-react";

interface TerminalTabProps {
  tab: TerminalTab;
  isActive: boolean;
  onClick: () => void;
}

export function TerminalTab({ tab, isActive, onClick }: TerminalTabProps) {
  const [renaming, setRenaming] = useState(false);
  const [renameValue, setRenameValue] = useState("");
  const renameInputRef = useRef<HTMLInputElement>(null);

  const handleClose = (e: React.MouseEvent) => {
    e.stopPropagation();
    const store = useTerminalStore.getState();
    import(/* @vite-ignore */ "../../../wailsjs/go/services/PTYService")
      .then(({ CloseTerminal }) => {
        CloseTerminal(tab.id).finally(() => {
          store.removeTab(tab.id);
        });
      })
      .catch(() => {
        store.removeTab(tab.id);
      });
  };

  const startRename = () => {
    setRenameValue(tab.name);
    setRenaming(true);
  };

  const commitRename = () => {
    const trimmed = renameValue.trim();
    if (trimmed) {
      useTerminalStore.getState().renameTab(tab.id, trimmed);
    }
    setRenaming(false);
  };

  const cancelRename = () => setRenaming(false);

  // The context menu closes when "Rename" is clicked and returns focus to
  // its trigger asynchronously for accessibility — but entering rename mode
  // replaces that trigger entirely with this input. Deferring focus to the
  // next frame avoids a race where the menu's focus-return steals focus
  // back immediately, firing onBlur (which commits) before the user can
  // type anything.
  useEffect(() => {
    if (!renaming) return;
    const id = requestAnimationFrame(() => renameInputRef.current?.focus());
    return () => cancelAnimationFrame(id);
  }, [renaming]);

  if (renaming) {
    return (
      <div className="w-full px-3 py-2 bg-zinc-900 border-l-2 border-blue-500">
        <input
          ref={renameInputRef}
          value={renameValue}
          onChange={(e) => setRenameValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") commitRename();
            if (e.key === "Escape") cancelRename();
          }}
          onBlur={commitRename}
          className="w-full bg-transparent text-sm text-zinc-100 outline-none"
        />
      </div>
    );
  }

  return (
    <ContextMenu>
      <ContextMenuTrigger
        render={
          <div
            className={
              isActive
                ? "group flex items-center w-full px-3 py-2 text-left text-sm bg-zinc-800 text-zinc-100 border-l-2 border-blue-500 cursor-pointer"
                : "group flex items-center w-full px-3 py-2 text-left text-sm text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200 border-l-2 border-transparent transition-colors cursor-pointer"
            }
            onClick={onClick}
          />
        }
      >
        <span
          className={`inline-block w-1.5 h-1.5 rounded-full mr-2 shrink-0 ${
            tab.degraded
              ? "bg-amber-500"
              : isActive
                ? "bg-green-500"
                : "bg-zinc-600"
          }`}
        />
        <span className="truncate flex-1">
          {tab.name}
        </span>
        {tab.degraded && (
          <Tooltip.Provider>
            <Tooltip.Root>
              <Tooltip.Trigger className="ml-1 text-amber-500 text-xs">
                &#9888;
              </Tooltip.Trigger>
              <Tooltip.Portal>
                <Tooltip.Positioner>
                  <Tooltip.Popup className="bg-zinc-800 text-zinc-200 text-xs px-2 py-1 rounded shadow-lg max-w-xs">
                    {tab.degradedMsg || "Text extraction not available"}
                  </Tooltip.Popup>
                </Tooltip.Positioner>
              </Tooltip.Portal>
            </Tooltip.Root>
          </Tooltip.Provider>
        )}
        <button
          onClick={handleClose}
          className="ml-2 px-1 text-zinc-600 hover:text-zinc-200 opacity-0 group-hover:opacity-100 transition-opacity"
          aria-label="Close terminal"
        >
          &times;
        </button>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem onClick={startRename}>
          <Pencil size={12} />
          Rename
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}
