import ReactMarkdown from "react-markdown";
import { CodeBlock } from "./CodeBlock";
import { useChatStore, type ChatMessage } from "@/stores/chatStore";
import { useTerminalStore } from "@/stores/terminalStore";
import { useEffect, useRef } from "react";

const EMPTY_MESSAGES: ChatMessage[] = [];

interface ChatMessageListProps {
  onRetry?: () => void;
}

export function ChatMessageList({ onRetry }: ChatMessageListProps) {
  const activeTabId = useTerminalStore((s) => s.activeTabId);
  const messages = useChatStore((s) => s.messagesByTab[activeTabId] ?? EMPTY_MESSAGES);
  const containerRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const isPinnedRef = useRef(true);
  const prevMsgCountRef = useRef(messages.length);
  // scrollIntoView fires its own scroll event, asynchronously, after the
  // browser finishes the scroll. Without this guard, that event reaches
  // handleScroll while the scroll position hasn't fully settled yet, which
  // could miscompute distance-from-bottom and spuriously unpin us right when
  // we most need to stay locked (e.g. the final auto-scroll once streaming
  // completes).
  const ignoreScrollRef = useRef(false);

  const handleScroll = () => {
    if (ignoreScrollRef.current) return;
    const el = containerRef.current;
    if (!el) return;
    const dist = el.scrollHeight - el.scrollTop - el.clientHeight;
    // Consider "pinned to bottom" if within 50px
    isPinnedRef.current = dist <= 50;
  };

  const scrollToBottom = () => {
    ignoreScrollRef.current = true;
    bottomRef.current?.scrollIntoView({ block: "end" });
    // The resulting scroll event can land a frame or two later than the
    // scroll itself, so wait a couple of frames before re-arming handleScroll.
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        ignoreScrollRef.current = false;
      });
    });
  };

  // Re-pin to the bottom whenever a new message bubble appears.
  useEffect(() => {
    const isNewMessage = messages.length > prevMsgCountRef.current;
    prevMsgCountRef.current = messages.length;
    if (isNewMessage) {
      isPinnedRef.current = true;
    }
  }, [messages]);

  // Drive auto-scroll off actual layout growth rather than the messages
  // array changing. Streaming text, and async syntax highlighting in code
  // blocks that lands after the message content settles, both change the
  // content's height without necessarily changing `messages` at that exact
  // moment — reacting to height directly keeps the view pinned through both,
  // with a single instant (non-animated) scroll instead of competing
  // throttled-jump and smooth-scroll calls that fought each other.
  useEffect(() => {
    const content = contentRef.current;
    if (!content) return;
    const observer = new ResizeObserver(() => {
      if (isPinnedRef.current) {
        scrollToBottom();
      }
    });
    observer.observe(content);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      className="app-scrollbar flex-1 overflow-y-scroll p-4"
    >
      <div ref={contentRef} className="space-y-4">
      {messages.length === 0 ? (
        <div className="flex items-center justify-center h-full py-8">
          <p className="text-zinc-600 text-sm">
            Ask a question about the terminal output...
          </p>
        </div>
      ) : (
        messages.map((msg) => {
          if (msg.role === "system") {
            return (
              <div key={msg.id} className="flex justify-start px-4 py-1">
                <div className="text-zinc-500 italic text-sm whitespace-pre-wrap">
                  {msg.content}
                </div>
              </div>
            );
          }
          return (
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
                      const isInline = (props as { inline?: boolean }).inline === true;
                      const codeStr = String(children).replace(/\n$/, "");
                      if (!isInline) {
                        return (
                          <CodeBlock
                            code={codeStr}
                            language={match ? match[1] : "text"}
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
          );
        })
      )}
      <div ref={bottomRef} />
      </div>
    </div>
  );
}
