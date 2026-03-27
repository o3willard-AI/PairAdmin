import { useRef, useState } from "react";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ChatInputProps {
  onSend: (text: string) => void;
}

export function ChatInput({ onSend }: ChatInputProps) {
  const [value, setValue] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const handleSend = () => {
    const trimmed = value.trim();
    if (!trimmed) return;
    onSend(trimmed);
    setValue("");
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setValue(e.target.value);
    e.target.style.height = "auto";
    e.target.style.height = Math.min(e.target.scrollHeight, 200) + "px";
  };

  return (
    <div className="border-t border-zinc-800 p-3 flex gap-2 items-end">
      <textarea
        ref={textareaRef}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder="Ask about the terminal output... (Enter to send, Shift+Enter for newline)"
        rows={1}
        className="flex-1 resize-none bg-zinc-900 text-zinc-100 rounded-md px-3 py-2 text-sm placeholder-zinc-500 focus:outline-none focus:ring-1 focus:ring-zinc-600 min-h-[40px] max-h-[200px]"
      />
      <Button
        size="sm"
        variant="ghost"
        onClick={handleSend}
        disabled={!value.trim()}
        aria-label="Send message"
      >
        <Send size={16} />
      </Button>
    </div>
  );
}
