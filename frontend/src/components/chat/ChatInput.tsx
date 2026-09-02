import { useRef, useState } from "react";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSettingsStore } from "@/stores/settingsStore";

interface ChatInputProps {
  onSend: (text: string) => void;
}

export function ChatInput({ onSend }: ChatInputProps) {
  const [value, setValue] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  // Same source of truth the StatusBar reads: the "disabled" connection
  // status set by the startup probe (ThreeColumnLayout) when
  // Provider === "disabled". Deriving it here (rather than adding a second
  // disabled flag on the store) keeps the two surfaces consistent by
  // construction — there's exactly one state to keep in sync.
  const llmDisabled = useSettingsStore((s) => s.connectionStatus === "disabled");

  const handleSend = () => {
    const trimmed = value.trim();
    if (!trimmed || llmDisabled) return;
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
    <div className="border-t border-surface-border p-3 flex gap-2 items-end">
      <textarea
        ref={textareaRef}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        disabled={llmDisabled}
        placeholder={
          llmDisabled
            ? "Pair LLM disabled in Settings"
            : "Ask about the terminal output... (Enter to send, Shift+Enter for newline)"
        }
        rows={1}
        className="flex-1 resize-none bg-surface-1 text-surface-text rounded-md px-3 py-2 text-sm placeholder-surface-text-muted focus:outline-none focus:ring-1 focus:ring-surface-border-strong min-h-[40px] max-h-[200px] disabled:opacity-50 disabled:cursor-not-allowed"
      />
      <Button
        size="sm"
        variant="ghost"
        onClick={handleSend}
        disabled={!value.trim() || llmDisabled}
        aria-label="Send message"
      >
        <Send size={16} />
      </Button>
    </div>
  );
}
