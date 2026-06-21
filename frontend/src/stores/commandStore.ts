import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

export interface Command {
  id: string;
  command: string;
  originalQuestion: string;
  timestamp: number;
  tabId: string;
}

interface CommandState {
  commandsByTab: Record<string, Command[]>;
  addCommand: (tabId: string, cmd: { command: string; originalQuestion: string }) => void;
  getCommandsForTab: (tabId: string) => Command[];
  clearTab: (tabId: string) => void;
}

export const useCommandStore = create<CommandState>()(
  devtools(
    immer((set, get) => ({
      commandsByTab: {},
      addCommand: (tabId, cmd) => {
        set((state) => {
          if (!state.commandsByTab[tabId]) state.commandsByTab[tabId] = [];
          state.commandsByTab[tabId].push({
            id: crypto.randomUUID(),
            command: cmd.command,
            originalQuestion: cmd.originalQuestion,
            timestamp: Date.now(),
            tabId,
          });
        });
      },
      getCommandsForTab: (tabId) => {
        const cmds = get().commandsByTab[tabId] || [];
        return [...cmds].sort((a, b) => b.timestamp - a.timestamp);
      },
      clearTab: (tabId) => {
        set((state) => {
          state.commandsByTab[tabId] = [];
        });
      },
    })),
    { name: "command-store" }
  )
);
