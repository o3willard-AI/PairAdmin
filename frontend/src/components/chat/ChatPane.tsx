import { useRef } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useChatStore } from "@/stores/chatStore";
import { useLLMStream } from "@/hooks/useLLMStream";
import { readTerminalLines } from "@/utils/terminalContext";
import { ChatMessageList } from "./ChatMessageList";
import { ChatInput } from "./ChatInput";

export function ChatPane() {
  const activeTabId = useTerminalStore((state) => state.activeTabId);
  const lastSentRef = useRef<{ text: string; terminalContext: string } | null>(null);

  // Subscribe to Wails streaming events for this tab
  useLLMStream(activeTabId);

  const handleSend = async (text: string) => {
    useChatStore.getState().addUserMessage(activeTabId, text);

    if (text.trim() === "/clear") {
      useChatStore.getState().clearTab(activeTabId);
      return;
    }

    // Get terminal context (empty string if terminal not yet initialized)
    const term = useTerminalStore.getState().getTermRef(activeTabId);
    const terminalContext = readTerminalLines(term, 200);

    // Store last sent for retry button (rate limit recovery)
    lastSentRef.current = { text, terminalContext };

    // Call LLMService via Wails binding (dynamic import — generated at wails dev runtime)
    import(/* @vite-ignore */ "../../wailsjs/go/services/LLMService").then(({ SendMessage }) => {
      SendMessage(activeTabId, text, terminalContext).catch((err: Error) => {
        // Surface immediate errors (provider not configured, etc.)
        useChatStore.getState().setStreamError(activeTabId, null, err?.message ?? "Failed to send message");
      });
    }).catch(() => {
      // Wails runtime not available (e.g., running in browser dev mode)
      useChatStore.getState().addAssistantMessage(activeTabId, "[Dev mode: Wails runtime unavailable]");
    });
  };

  const handleRetry = () => {
    if (lastSentRef.current) {
      handleSend(lastSentRef.current.text);
    }
  };

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <ChatMessageList onRetry={handleRetry} />
      <ChatInput onSend={handleSend} />
    </div>
  );
}
