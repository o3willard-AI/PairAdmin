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
  // LLM activity flag: set when a message is dispatched to the backend,
  // cleared on llm:done / llm:error / synchronous send failure. Deliberately
  // a SINGLE GLOBAL value, not per-tab: the streaming events are not
  // tab-scoped (ChatTokenEvent in services/llm_service.go carries no tabId —
  // the Go side streams to whoever subscribed), so a per-tab flag would go
  // STALE if the user switched terminal tabs mid-stream: the tab where the
  // request started would keep showing an indicator for a response that is
  // actually finishing elsewhere, and the "end" event would have no way to
  // attribute itself. One global flag = one truth: something is in flight
  // or it isn't, and every terminal path (done/error/sync failure) clears
  // the same value.
  llmRequest: { startedAt: number } | null;
  startLLMRequest: () => void;
  endLLMRequest: () => void;
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
      llmRequest: null,
      startLLMRequest: () => {
        set((state) => {
          state.llmRequest = { startedAt: Date.now() };
        });
      },
      endLLMRequest: () => {
        set((state) => {
          state.llmRequest = null;
        });
      },
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
          // The backend doesn't report real usage figures yet, so estimate
          // from content length (~4 chars/token, the same rule of thumb
          // services/llm/context.go uses) rather than showing nothing.
          msg.tokenCount = tokenCount ?? Math.ceil(msg.content.length / 4);
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
