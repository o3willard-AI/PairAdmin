import ReactMarkdown from "react-markdown";
import { CodeBlock } from "./CodeBlock";
import { useChatStore } from "@/stores/chatStore";
import { useTerminalStore } from "@/stores/terminalStore";
import { useEffect, useRef } from "react";

interface ChatMessageListProps {
  onRetry?: () => void;
}

export function ChatMessageList({ onRetry }: ChatMessageListProps) {
  const activeTabId = useTerminalStore((s) => s.activeTabId);
  const messages = useChatStore((s) => s.messagesByTab[activeTabId] ?? []);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    if (distFromBottom <= 100) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages]);

  return (
    <div ref={containerRef} className="flex-1 overflow-y-auto p-4 space-y-4">
      {messages.length === 0 ? (
        <div className="flex items-center justify-center h-full py-8">
          <p className="text-zinc-600 text-sm">
            Ask a question about the terminal output...
          </p>
        </div>
      ) : (
        messages.map((msg) => (
          <div
            key={msg.id}
            className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
          >
            <div
              className={[
                "max-w-[80%] rounded-lg px-4 py-2 text-sm",
                msg.role === "user"
                  ? "bg-primary text-primary-foreground"
                  : msg.isError
                    ? "bg-amber-950/50 border border-amber-600/50 text-amber-200"
                    : "bg-muted text-foreground",
              ].join(" ")}
            >
              {msg.isError && <span className="mr-1">⚠</span>}
              <ReactMarkdown
                components={{
                  code({ children, className, node, ...props }) {
                    const match = /language-(\w+)/.exec(className ?? "");
                    // Determine if this is a block (fenced) code vs inline code
                    // react-markdown passes inline=true for backtick-inline code
                    const isInline = (props as { inline?: boolean }).inline === true;
                    const codeStr = String(children).replace(/\n$/, "");
                    if (match && !isInline) {
                      return (
                        <CodeBlock
                          code={codeStr}
                          language={match[1]}
                          isStreaming={msg.isStreaming}
                        />
                      );
                    }
                    return (
                      <code className="bg-muted-foreground/20 px-1 rounded text-xs">
                        {children}
                      </code>
                    );
                  },
                }}
              >
                {msg.content}
              </ReactMarkdown>
              {msg.isError && msg.content.includes("Rate limit") && onRetry && (
                <button
                  onClick={onRetry}
                  className="mt-2 text-xs underline hover:no-underline block"
                >
                  Retry
                </button>
              )}
            </div>
          </div>
        ))
      )}
    </div>
  );
}
