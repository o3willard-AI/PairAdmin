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
  initMockData: () => void;
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
      initMockData: () => {
        if (Object.keys(get().commandsByTab).length === 0) {
          set((state) => {
            state.commandsByTab["bash-1"] = [
              {
                id: crypto.randomUUID(),
                command: "sudo systemctl restart nginx",
                originalQuestion: "How do I restart nginx?",
                timestamp: Date.now() - 2000,
                tabId: "bash-1",
              },
              {
                id: crypto.randomUUID(),
                command: "tail -f /var/log/syslog | grep error",
                originalQuestion: "Show me recent errors",
                timestamp: Date.now() - 1000,
                tabId: "bash-1",
              },
              {
                id: crypto.randomUUID(),
                command: "df -h --output=source,pcent,target",
                originalQuestion: "Check disk usage",
                timestamp: Date.now(),
                tabId: "bash-1",
              },
            ];
          });
        }
      },
    })),
    { name: "command-store" }
  )
);
