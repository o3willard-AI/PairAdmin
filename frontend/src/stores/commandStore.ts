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
}

interface CommandState {
  commands: Command[];
  addCommand: (tabId: string, cmd: { command: string; originalQuestion: string }) => void;
  clearAll: () => void;
}

export const useCommandStore = create<CommandState>()(
  devtools(
    immer((set) => ({
      commands: [],
      addCommand: (tabId, cmd) => {
        set((state) => {
          state.commands.push({
            id: crypto.randomUUID(),
            command: cmd.command,
            originalQuestion: cmd.originalQuestion,
            timestamp: Date.now(),
            tabId,
          });
        });
      },
      clearAll: () => {
        set((state) => {
          state.commands = [];
        });
      },
    })),
    { name: "command-store" }
  )
);
