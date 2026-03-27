import { useTerminalStore } from "@/stores/terminalStore";
import { useChatStore } from "@/stores/chatStore";
import { ChatMessageList } from "./ChatMessageList";
import { ChatInput } from "./ChatInput";

export function ChatPane() {
  const activeTabId = useTerminalStore((state) => state.activeTabId);

  const handleSend = (text: string) => {
    useChatStore.getState().addUserMessage(activeTabId, text);

    if (text.trim() === "/clear") {
      useChatStore.getState().clearTab(activeTabId);
      return;
    }

    setTimeout(() => {
      useChatStore.getState().addAssistantMessage(activeTabId, "Echo: " + text);
    }, 200);
  };

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <ChatMessageList />
      <ChatInput onSend={handleSend} />
    </div>
  );
}
