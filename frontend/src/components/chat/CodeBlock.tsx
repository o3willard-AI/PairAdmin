import CodeHighlighter from "react-shiki";
import { useCommandStore } from "@/stores/commandStore";
import { useTerminalStore } from "@/stores/terminalStore";
import { sendToTerminal } from "@/utils/sendToTerminal";

interface CodeBlockProps {
  code: string;
  language?: string;
  isStreaming: boolean;
}

const ACTION_BUTTON_CLASS =
  "rounded px-1.5 py-0.5 hover:bg-muted-foreground/30 hover:text-foreground transition-colors";

// Blocks tagged with one of these languages are illustrative/non-actionable
// (per the system prompt, the model is told to use these — or inline code —
// for examples it isn't actually recommending the user run). Rendering
// action buttons on them anyway is how the same snippet could show up
// twice: once as a real, actionable command block, and once again as an
// inert "text" block with its own Copy/Execute/Save buttons.
const NON_ACTIONABLE_LANGUAGES = new Set(["text", "plaintext", "plain", ""]);

export function CodeBlock({ code, language = "text", isStreaming }: CodeBlockProps) {
  const activeTabId = useTerminalStore((s) => s.activeTabId);
  const isActionable = !NON_ACTIONABLE_LANGUAGES.has(language.toLowerCase());

  const handleSaveToCommands = () => {
    useCommandStore.getState().addCommand(activeTabId, {
      command: code,
      originalQuestion: "",
    });
  };

  return (
    <div className="relative my-2 rounded-md overflow-hidden border border-border">
      <div className="flex items-center justify-between gap-12 px-3 py-1 bg-muted text-xs text-muted-foreground">
        <span className="truncate">{language}</span>
        {!isStreaming && isActionable && (
          <div className="flex-none flex items-center gap-1">
            <button
              onClick={handleSaveToCommands}
              className={ACTION_BUTTON_CLASS}
              aria-label="Save to Commands"
            >
              Save to Commands
            </button>
            <span aria-hidden="true" className="text-muted-foreground/50">|</span>
            <button
              onClick={() => sendToTerminal(activeTabId, code, false)}
              className={ACTION_BUTTON_CLASS}
              aria-label="Copy to Terminal"
            >
              Copy to Terminal
            </button>
            <span aria-hidden="true" className="text-muted-foreground/50">|</span>
            <button
              onClick={() => sendToTerminal(activeTabId, code, true)}
              className={ACTION_BUTTON_CLASS}
              aria-label="Execute in Terminal"
            >
              Execute in Terminal
            </button>
          </div>
        )}
      </div>
      <CodeHighlighter language={language} theme="github-dark" delay={50}>{code}</CodeHighlighter>
    </div>
  );
}
