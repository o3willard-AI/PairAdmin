import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";

export interface ChatMessage {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  isStreaming: boolean;
  tokenCount?: number;
  isError?: boolean;
}

interface ChatState {
  messagesByTab: Record<string, ChatMessage[]>;
  addUserMessage: (tabId: string, text: string) => string;
  addAssistantMessage: (tabId: string, content: string) => string;
  addSystemMessage: (tabId: string, text: string) => void;
  clearTab: (tabId: string) => void;
  startStreamingMessage: (tabId: string) => string;
  appendChunk: (tabId: string, msgId: string, text: string) => void;
  finalizeMessage: (tabId: string, msgId: string, tokenCount?: number) => void;
  setStreamError: (tabId: string, msgId: string | null, errorText: string) => void;
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
      addSystemMessage: (tabId, text) => {
        set((state) => {
          if (!state.messagesByTab[tabId]) state.messagesByTab[tabId] = [];
          state.messagesByTab[tabId].push({
            id: crypto.randomUUID(),
            role: "system",
            content: text,
            isStreaming: false,
          });
        });
      },
      clearTab: (tabId) => {
        set((state) => {
          state.messagesByTab[tabId] = [];
        });
      },
      startStreamingMessage: (tabId) => {
        const id = crypto.randomUUID();
        set((state) => {
          if (!state.messagesByTab[tabId]) state.messagesByTab[tabId] = [];
          state.messagesByTab[tabId].push({ id, role: "assistant", content: "", isStreaming: true });
        });
        return id;
      },
      appendChunk: (tabId, msgId, text) => {
        set((state) => {
          const messages = state.messagesByTab[tabId];
          if (!messages) return;
          const msg = messages.find((m) => m.id === msgId);
          if (!msg) return;
          // Replace trailing cursor before appending, then re-add cursor
          msg.content = msg.content.replace(/▋$/, "") + text + "▋";
        });
      },
      finalizeMessage: (tabId, msgId, tokenCount) => {
        set((state) => {
          const messages = state.messagesByTab[tabId];
          if (!messages) return;
          const msg = messages.find((m) => m.id === msgId);
          if (!msg) return;
          msg.content = msg.content.replace(/▋$/, "");
          msg.isStreaming = false;
          if (tokenCount !== undefined) {
            msg.tokenCount = tokenCount;
          }
        });
      },
      setStreamError: (tabId, msgId, errorText) => {
        set((state) => {
          if (!state.messagesByTab[tabId]) state.messagesByTab[tabId] = [];
          if (msgId !== null) {
            // Stream was interrupted mid-response — find existing message
            const msg = state.messagesByTab[tabId].find((m) => m.id === msgId);
            if (msg) {
              msg.content = msg.content.replace(/▋$/, "") + "\n\n(stream interrupted)";
              msg.isError = true;
              msg.isStreaming = false;
            }
          } else {
            // Error before any tokens — create a new error message
            const id = crypto.randomUUID();
            state.messagesByTab[tabId].push({
              id,
              role: "assistant",
              content: errorText,
              isStreaming: false,
              isError: true,
            });
          }
        });
      },
    })),
    { name: "chat-store" }
  )
);
