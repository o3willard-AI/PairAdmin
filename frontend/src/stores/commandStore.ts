import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

export interface Command {
  id: string;
  command: string;
  originalQuestion: string;
  timestamp: number;
  // The terminal tab the command originated from. Kept for reference only —
  // commands are shared across all terminal sessions (see addCommand), not
  // scoped to the tab that generated them.
  tabId: string;
  // Pinned commands are shown above the scrolling list and can be manually
  // reordered via drag-and-drop; unpinned commands stay in insertion order.
  pinned: boolean;
  // Set by editForNextUse — overrides `command` for exactly one
  // copy/execute, then is cleared automatically (see consumeCommandText).
  tempOverride?: string;
  // An optional user-assigned name that, when set, replaces the command text
  // in the sidebar display. The full command text is still shown on hover.
  name?: string;
}

interface CommandState {
  commands: Command[];
  /** Add Command dialog open state — lifted into the store so
   * useAddCommandHotkey can toggle it in sync with the Add Command button. */
  addCommandDialogOpen: boolean;
  setAddCommandDialogOpen: (open: boolean) => void;
  addCommand: (tabId: string, cmd: { command: string; originalQuestion: string; name?: string }) => void;
  /**
   * Adds a command already pinned — used for auto-added helper commands
   * (e.g. tmux mouse-mode toggles) rather than AI-suggested ones. A no-op if
   * a pinned command with the same text already exists, so calling this
   * repeatedly (e.g. on every reconnect to the same tmux host) doesn't stack
   * duplicate cards.
   */
  addPinnedCommand: (tabId: string, cmd: { command: string; originalQuestion: string; name?: string }) => void;
  clearAll: () => void;
  togglePin: (id: string) => void;
  removeCommand: (id: string) => void;
  /** Permanent edit — all future uses see the new text. */
  editCommand: (id: string, newText: string) => void;
  /** One-time edit — the next copy/execute uses newText, then reverts. */
  editForNextUse: (id: string, newText: string) => void;
  /**
   * Returns the text that should actually be sent to the terminal for this
   * command (the one-time override if set, otherwise the regular command
   * text), and clears the one-time override so it only applies once.
   */
  consumeCommandText: (id: string) => string;
  /** Reorders a pinned command to sit immediately before the target command. */
  reorderPinned: (draggedId: string, targetId: string) => void;
  /**
   * Assigns a custom name to a command. When set, the name is used as the
   * display text in the sidebar instead of the command text. Pass an empty
   * string to clear the name.
   */
  renameCommand: (id: string, name: string) => void;
}

export const useCommandStore = create<CommandState>()(
  devtools(
    immer((set, get) => ({
      commands: [],
      addCommandDialogOpen: false,
      setAddCommandDialogOpen: (open) => {
        set((state) => {
          state.addCommandDialogOpen = open;
        });
      },
      addCommand: (tabId, cmd) => {
        set((state) => {
          const entry: Command = {
            id: crypto.randomUUID(),
            command: cmd.command,
            originalQuestion: cmd.originalQuestion,
            timestamp: Date.now(),
            tabId,
            pinned: false,
          };
          if (cmd.name) entry.name = cmd.name;
          state.commands.push(entry);
        });
      },
      addPinnedCommand: (tabId, cmd) => {
        set((state) => {
          if (state.commands.some((c) => c.pinned && c.command === cmd.command)) return;
          const entry: Command = {
            id: crypto.randomUUID(),
            command: cmd.command,
            originalQuestion: cmd.originalQuestion,
            timestamp: Date.now(),
            tabId,
            pinned: true,
          };
          if (cmd.name) entry.name = cmd.name;
          state.commands.push(entry);
        });
      },
      clearAll: () => {
        set((state) => {
          // Pinned commands are deliberately kept around — clearing history
          // shouldn't remove the ones the user explicitly chose to keep.
          state.commands = state.commands.filter((c) => c.pinned);
        });
      },
      togglePin: (id) => {
        set((state) => {
          const cmd = state.commands.find((c) => c.id === id);
          if (cmd) cmd.pinned = !cmd.pinned;
        });
      },
      removeCommand: (id) => {
        set((state) => {
          state.commands = state.commands.filter((c) => c.id !== id);
        });
      },
      editCommand: (id, newText) => {
        set((state) => {
          const cmd = state.commands.find((c) => c.id === id);
          if (cmd) {
            cmd.command = newText;
            cmd.tempOverride = undefined;
          }
        });
      },
      editForNextUse: (id, newText) => {
        set((state) => {
          const cmd = state.commands.find((c) => c.id === id);
          if (cmd) cmd.tempOverride = newText;
        });
      },
      consumeCommandText: (id) => {
        const cmd = get().commands.find((c) => c.id === id);
        if (!cmd) return "";
        const text = cmd.tempOverride ?? cmd.command;
        if (cmd.tempOverride !== undefined) {
          set((state) => {
            const c = state.commands.find((c) => c.id === id);
            if (c) c.tempOverride = undefined;
          });
        }
        return text;
      },
      reorderPinned: (draggedId, targetId) => {
        set((state) => {
          if (draggedId === targetId) return;
          const fromIndex = state.commands.findIndex((c) => c.id === draggedId);
          const toIndex = state.commands.findIndex((c) => c.id === targetId);
          if (fromIndex === -1 || toIndex === -1) return;
          const [moved] = state.commands.splice(fromIndex, 1);
          const insertAt = state.commands.findIndex((c) => c.id === targetId);
          state.commands.splice(insertAt, 0, moved);
        });
      },
      renameCommand: (id, name) => {
        set((state) => {
          const cmd = state.commands.find((c) => c.id === id);
          if (cmd) cmd.name = name || undefined;
        });
      },
    })),
    { name: "command-store" }
  )
);
