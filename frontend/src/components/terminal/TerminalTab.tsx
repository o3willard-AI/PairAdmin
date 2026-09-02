import { useEffect, useRef, useState } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import type { TerminalTab } from "@/stores/terminalStore";
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
} from "@/components/ui/context-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Pencil, Save, Check } from "lucide-react";

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
      // Persist onto the saved host record (if this tab came from one) so a
      // future reconnect shows the friendly name instead of reverting to
      // "username@host". Best-effort: the local rename above already applies
      // regardless, and a failure here just means next reconnect falls back
      // to the computed name — not worth blocking or erroring the rename UI
      // over, but still logged so it's not entirely invisible when debugging.
      if (tab.savedHostId) {
        import(/* @vite-ignore */ "../../../wailsjs/go/services/RemoteService")
          .then(({ RenameRemoteHost }) => RenameRemoteHost(tab.savedHostId!, trimmed))
          .catch((err) => {
            console.error("Failed to persist renamed saved host:", err);
          });
      }
    }
    setRenaming(false);
  };

  const cancelRename = () => setRenaming(false);

  // Builds a config.RemoteHost from this tab's non-secret connection metadata
  // and persists it (metadata only — password and passphrase are passed empty,
  // so nothing reaches the keychain). On success the returned generated host ID
  // is written back onto the tab so its context menu flips from "Save Terminal"
  // to a disabled "Saved" item. Best-effort: failures are logged, and since the
  // connection is already open, there's no user-facing error to raise.
  const handleSaveTerminal = async () => {
    if (!tab.remote) return;
    try {
      const { SaveRemoteHost } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/RemoteService"
      );
      const record = {
        ID: "",
        Kind: tab.kind === "winrm" ? "winrm" : "ssh",
        Name: tab.name,
        Host: tab.remote.host,
        Port: tab.remote.port,
        Username: tab.remote.username,
        AuthType: tab.remote.authType,
        PrivateKeyPath: tab.remote.privateKeyPath,
        LastUsed: "",
        UseTmux: tab.remote.useTmux,
        TmuxSessionName: tab.remote.tmuxSessionName,
      };
      const saved = await SaveRemoteHost(record, "", "");
      useTerminalStore.getState().setSavedHostId(tab.id, saved.ID);
    } catch (err) {
      console.error("Failed to save terminal:", err);
    }
  };

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
      <div className="w-full px-3 py-2 bg-surface-1 border-l-2 border-blue-500">
        <input
          ref={renameInputRef}
          value={renameValue}
          onChange={(e) => setRenameValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") commitRename();
            if (e.key === "Escape") cancelRename();
          }}
          onBlur={commitRename}
          className="w-full bg-transparent text-sm text-surface-text outline-none"
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
                ? "group flex items-center w-full px-3 py-2 text-left text-sm bg-surface-2 text-surface-text border-l-2 border-blue-500 cursor-pointer"
                : "group flex items-center w-full px-3 py-2 text-left text-sm text-surface-text-muted hover:bg-surface-1 hover:text-surface-text border-l-2 border-transparent transition-colors cursor-pointer"
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
                : "bg-surface-text-muted"
          }`}
        />
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger render={<span className="truncate flex-1" />}>
              {tab.name}
            </TooltipTrigger>
            <TooltipContent side="right">
              <p className="text-xs break-words">{tab.name}</p>
              {tab.host && (
                <p className="text-xs break-words text-surface-text-muted">
                  {tab.host}:{tab.port}
                </p>
              )}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
        {tab.degraded && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger render={<span className="ml-1 text-amber-500 text-xs" />}>
                &#9888;
              </TooltipTrigger>
              <TooltipContent side="right" className="max-w-xs">
                {tab.degradedMsg || "Text extraction not available"}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
        <button
          onClick={handleClose}
          className="ml-2 px-1 text-surface-text-muted hover:text-surface-text opacity-0 group-hover:opacity-100 transition-opacity"
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
        {tab.savedHostId ? (
          <ContextMenuItem disabled>
            <Check size={12} />
            Saved
          </ContextMenuItem>
        ) : tab.host ? (
          <ContextMenuItem onClick={handleSaveTerminal}>
            <Save size={12} />
            Save Terminal
          </ContextMenuItem>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  );
}
