import type { ChatMessage } from "@/stores/chatStore";

interface ChatBubbleProps {
  message: ChatMessage;
}

export function ChatBubble({ message }: ChatBubbleProps) {
  const isUser = message.role === "user";

  return (
    <div className={`flex w-full ${isUser ? "justify-end" : "justify-start"}`}>
      <div
        className={
          isUser
            ? "ml-auto max-w-[80%] rounded-lg bg-blue-600 px-3 py-2 text-sm text-white"
            : "mr-auto max-w-[80%] rounded-lg bg-zinc-800 px-3 py-2 text-sm text-zinc-100"
        }
      >
        {message.content}
      </div>
    </div>
  );
}
