import { useEffect, useRef, useState } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import type { TerminalTab } from "@/stores/terminalStore";
import { useQuickSelectStore } from "@/stores/quickSelectStore";
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
  // Which rename flow is active, if any: a plain rename, or the local-tmux
  // "rename and save session" flow (rename + create-or-attach + persist).
  const [renaming, setRenaming] = useState<"rename" | "saveTmux" | null>(null);
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
    setRenaming("rename");
  };

  // Local-only flow: rename the tab, then start/attach the named tmux session
  // in that very terminal, and persist it as a Kind:"local" saved host so the
  // "+ Connect" dialog's Recent list can one-click back into it.
  const startRenameAndSaveTmux = () => {
    setRenameValue(tab.name);
    setRenaming("saveTmux");
  };

  const commitRename = () => {
    const mode = renaming;
    setRenaming(null);
    const trimmed = renameValue.trim();
    if (!trimmed) return;
    useTerminalStore.getState().renameTab(tab.id, trimmed);

    if (mode === "saveTmux") {
      // (b) create-or-attach the named tmux session in this terminal. Any
      // tmux-side failure (missing tmux, session already attached) is visible
      // in the terminal output itself — no blocking UI here.
      import(/* @vite-ignore */ "../../../wailsjs/go/services/PTYService")
        .then(({ WriteInput }) =>
          WriteInput(tab.id, `tmux new-session -A -s ${trimmed}\r`)
        )
        .catch((err) => {
          console.error("Failed to send tmux create-or-attach command:", err);
        });
      // (c) persist as a Kind:"local" saved host (no host coordinates, no
      // credentials — local has none). Upsert by the tab's existing ID when
      // it was saved before. Best-effort: a failure here doesn't undo the
      // rename or the tmux attach.
      import(/* @vite-ignore */ "../../../wailsjs/go/services/RemoteService")
        .then(({ SaveRemoteHost }) =>
          SaveRemoteHost(
            {
              ID: tab.savedHostId || "",
              Kind: "local",
              Name: trimmed,
              Host: "",
              Port: 0,
              Username: "",
              AuthType: "",
              PrivateKeyPath: "",
              LastUsed: "",
              UseTmux: false,
              TmuxSessionName: trimmed,
            },
            "",
            ""
          )
        )
        .then((saved) => {
          useTerminalStore.getState().setSavedHostId(tab.id, saved.ID);
        })
        .catch((err) => {
          console.error("Failed to save local tmux session:", err);
        });
      return;
    }

    // Plain rename: persist onto the saved host record (if this tab came from
    // one) so a future reconnect shows the friendly name instead of reverting
    // to "username@host". Best-effort: the local rename above already applies
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
  };

  const cancelRename = () => setRenaming(null);

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

  // Quick-select signage: this tab's own F-key while the chord is held.
  // Subscribed per-tab so only affected rows re-render on chord press/
  // release. Not shown in rename mode — the rename input replaces the whole
  // row and the badge would sit on top of the text field.
  const fkey = useQuickSelectStore(
    (s) => (s.visible ? s.terminalFkeys[tab.id] : undefined)
  );

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
                ? "group relative flex items-center w-full px-3 py-2 text-left text-sm bg-surface-2 text-surface-text border-l-2 border-blue-500 cursor-pointer"
                : "group relative flex items-center w-full px-3 py-2 text-left text-sm text-surface-text-muted hover:bg-surface-1 hover:text-surface-text border-l-2 border-transparent transition-colors cursor-pointer"
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
        {fkey && (
          <span
            aria-hidden="true"
            className="absolute right-1 top-1/2 -translate-y-1/2 px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold leading-none bg-surface-2/95 text-surface-text border border-surface-border shadow-sm pointer-events-none"
          >
            {fkey}
          </span>
        )}
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
        {/* Local-only: rename + start/attach the named tmux session in this
            terminal + persist it for the Recent list. Remote tabs attach tmux
            at connect time instead, so this flow doesn't apply to them. */}
        {tab.kind === "local" && !tab.host && (
          <ContextMenuItem onClick={startRenameAndSaveTmux}>
            <Save size={12} />
            Rename and Save tmux session
          </ContextMenuItem>
        )}
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
