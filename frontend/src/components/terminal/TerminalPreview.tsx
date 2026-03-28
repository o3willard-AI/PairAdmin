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

    // Write mock terminal content
    term.writeln("\x1b[32m$ \x1b[0mls -la /home/admin");
    term.writeln("total 48");
    term.writeln("drwxr-xr-x  6 admin admin 4096 Mar 26 09:12 .");
    term.writeln("drwxr-xr-x  3 root  root  4096 Mar 20 14:30 ..");
    term.writeln("-rw-r--r--  1 admin admin  220 Mar 20 14:30 .bash_logout");
    term.writeln("-rw-r--r--  1 admin admin 3771 Mar 20 14:30 .bashrc");
    term.writeln("\x1b[32m$ \x1b[0mgit status");
    term.writeln("On branch main");
    term.writeln("nothing to commit, working tree clean");
    term.writeln("");
    term.writeln("\x1b[33m[No terminal connected — Phase 1 mock]\x1b[0m");

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
