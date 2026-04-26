import CodeHighlighter from "react-shiki";
import { useCommandStore } from "@/stores/commandStore";
import { useTerminalStore } from "@/stores/terminalStore";

interface CodeBlockProps {
  code: string;
  language?: string;
  isStreaming: boolean;
}

export function CodeBlock({ code, language = "text", isStreaming }: CodeBlockProps) {
  const activeTabId = useTerminalStore((s) => s.activeTabId);

  const sendToTerminal = (execute: boolean) => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/PTYService")
      .then(({ WriteInput }) => WriteInput(activeTabId, execute ? code + "\n" : code))
      .catch(() => {});
    if (execute) {
      useCommandStore.getState().addCommand(activeTabId, {
        command: code,
        originalQuestion: "",
      });
    }
  };

  return (
    <div className="relative my-2 rounded-md overflow-hidden border border-border">
      <div className="flex items-center justify-between px-3 py-1 bg-muted text-xs text-muted-foreground">
        <span>{language}</span>
        {!isStreaming && (
          <div className="flex items-center gap-3">
            <button
              onClick={() => sendToTerminal(false)}
              className="hover:text-foreground transition-colors"
              aria-label="Copy to Terminal"
            >
              Copy to Terminal
            </button>
            <button
              onClick={() => sendToTerminal(true)}
              className="hover:text-foreground transition-colors"
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
