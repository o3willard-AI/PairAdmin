import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { CanvasAddon } from "@xterm/addon-canvas";
import "@xterm/xterm/css/xterm.css";
import { useTerminalStore } from "@/stores/terminalStore";

interface TerminalPreviewProps {
  tabId: string;
}

export function TerminalPreview({ tabId }: TerminalPreviewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);

  // No-tmux empty state (D-03)
  if (!tabId) {
    return (
      <div className="h-full w-full flex items-center justify-center bg-[#0d0d0d] text-zinc-400">
        <div className="text-center space-y-2">
          <p>No tmux session detected.</p>
          <p>Start a tmux session to begin.</p>
          <code className="block mt-4 px-3 py-1.5 bg-zinc-800 rounded text-sm text-green-400 font-mono">
            $ tmux new-session
          </code>
        </div>
      </div>
    );
  }

  useEffect(() => {
    if (!containerRef.current) return;

    const term = new Terminal({
      theme: {
        background: "#0d0d0d",
        foreground: "#d4d4d4",
        cursor: "#d4d4d4",
      },
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
      scrollback: 1000,
      convertEol: true,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    term.open(containerRef.current);

    // Register term ref in terminalStore so ChatPane can read terminal context
    useTerminalStore.getState().setTermRef(tabId, term);

    // CanvasAddon MUST be loaded after open()
    try {
      const canvasAddon = new CanvasAddon();
      term.loadAddon(canvasAddon);
    } catch (err) {
      console.warn("CanvasAddon failed to load, continuing without hardware acceleration:", err);
    }

    fitAddon.fit();

    termRef.current = term;

    const resizeObserver = new ResizeObserver(() => {
      fitAddon.fit();
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      term.dispose();
    };
  }, [tabId]);

  return (
    <div
      ref={containerRef}
      className="h-full w-full overflow-hidden"
      style={{ minHeight: "120px" }}
    />
  );
}
