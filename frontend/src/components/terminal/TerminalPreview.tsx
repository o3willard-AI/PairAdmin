import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { CanvasAddon } from "@xterm/addon-canvas";
import "@xterm/xterm/css/xterm.css";
import { useTerminalStore } from "@/stores/terminalStore";

interface AdapterStatusInfo {
  name: string;
  status: string;
  message: string;
}

interface TerminalPreviewProps {
  tabId: string;
  adapterStatus?: AdapterStatusInfo[];
}

export function TerminalPreview({ tabId, adapterStatus }: TerminalPreviewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);

  useEffect(() => {
    if (!tabId || !containerRef.current) return;

    // Raised at the very start of cleanup so every async callback can bail
    // out before touching the terminal. Guards the race between React unmount,
    // the ResizeObserver, and in-flight pty:output events that arrive after
    // the shell exits but before disposal completes.
    const disposed = { current: false };

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

    // PTY output → xterm
    let unsubPtyOutput: (() => void) | null = null;
    import(/* @vite-ignore */ "../../../wailsjs/runtime/runtime").then((rt) => {
      unsubPtyOutput = rt.EventsOn("pty:output", ((event: { tabId: string; data: string }) => {
        if (event.tabId === tabId && !disposed.current) {
          term.write(event.data);
        }
      }) as (...args: unknown[]) => void);
    }).catch(() => {});

    // xterm input → PTY
    const onDataDisposable = term.onData((data) => {
      if (disposed.current) return;
      import(/* @vite-ignore */ "../../../wailsjs/go/services/PTYService")
        .then(({ WriteInput }) => WriteInput(tabId, data))
        .catch(() => {});
    });

    // xterm resize → PTY
    const onResizeDisposable = term.onResize(({ cols, rows }) => {
      if (disposed.current) return;
      import(/* @vite-ignore */ "../../../wailsjs/go/services/PTYService")
        .then(({ ResizeTerminal }) => ResizeTerminal(tabId, cols, rows))
        .catch(() => {});
    });

    const resizeObserver = new ResizeObserver(() => {
      if (!disposed.current) fitAddon.fit();
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      disposed.current = true; // must be first — blocks all in-flight callbacks
      resizeObserver.disconnect();
      unsubPtyOutput?.();
      onDataDisposable.dispose();
      onResizeDisposable.dispose();
      // xterm buffers writes asynchronously via setTimeout. Calling dispose()
      // immediately destroys _linkifier2 while a buffered write may still be
      // pending, causing the "undefined is not an object (_linkifier2)" crash.
      // Writing an empty string with a callback flushes the queue in FIFO
      // order; dispose fires only after all queued writes have rendered.
      term.write("", () => term.dispose());
    };
  }, [tabId]);

  // Extended empty state (D-06/D-07): shows AT-SPI2 onboarding when applicable
  if (!tabId) {
    const atspiOnboarding = adapterStatus?.find(
      (a) => a.name === "atspi" && a.status === "onboarding"
    );

    return (
      <div className="h-full w-full flex items-center justify-center bg-[#0d0d0d] text-zinc-400">
        <div className="text-center space-y-4 max-w-md">
          <p className="text-lg">No terminal sessions detected.</p>

          <div className="space-y-2">
            <p className="text-sm text-zinc-500">Option 1: Start a tmux session</p>
            <code className="block px-3 py-1.5 bg-zinc-800 rounded text-sm text-green-400 font-mono">
              $ tmux new-session
            </code>
          </div>

          {atspiOnboarding && (
            <div className="space-y-2">
              <p className="text-sm text-zinc-500">Option 2: Enable accessibility for GUI terminals</p>
              <code className="block px-3 py-1.5 bg-zinc-800 rounded text-sm text-green-400 font-mono">
                $ gsettings set org.gnome.desktop.interface toolkit-accessibility true
              </code>
              <p className="text-xs text-zinc-600">Then restart your terminal application.</p>
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="h-full w-full overflow-hidden"
      style={{ minHeight: "120px" }}
    />
  );
}
