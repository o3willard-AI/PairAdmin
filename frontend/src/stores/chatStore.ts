import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  isStreaming: boolean;
}

interface ChatState {
  messagesByTab: Record<string, ChatMessage[]>;
  addUserMessage: (tabId: string, text: string) => string;
  addAssistantMessage: (tabId: string, content: string) => string;
  clearTab: (tabId: string) => void;
}

export const useChatStore = create<ChatState>()(
  devtools(
    immer((set) => ({
      messagesByTab: {},
      addUserMessage: (tabId, text) => {
        const id = crypto.randomUUID();
        set((state) => {
          if (!state.messagesByTab[tabId]) state.messagesByTab[tabId] = [];
          state.messagesByTab[tabId].push({ id, role: "user", content: text, isStreaming: false });
        });
        return id;
      },
      addAssistantMessage: (tabId, content) => {
        const id = crypto.randomUUID();
        set((state) => {
          if (!state.messagesByTab[tabId]) state.messagesByTab[tabId] = [];
          state.messagesByTab[tabId].push({ id, role: "assistant", content, isStreaming: false });
        });
        return id;
      },
      clearTab: (tabId) => {
        set((state) => {
          state.messagesByTab[tabId] = [];
        });
      },
    })),
    { name: "chat-store" }
  )
);
