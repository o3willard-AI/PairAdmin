import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { CanvasAddon } from "@xterm/addon-canvas";
import "@xterm/xterm/css/xterm.css";
import { useTerminalStore } from "@/stores/terminalStore";
import { GetWindowsContent, WriteInput, ResizeTerminal } from "../../../wailsjs/go/services/PTYService";
import { EventsOn } from "../../../wailsjs/runtime/runtime";

interface AdapterStatusInfo {
  name: string;
  status: string;
  message: string;
}

// xterm.js's own theme is a JS object, not CSS — it doesn't pick up the
// app's light/dark class automatically. This mirrors the surface-0/
// surface-text pair from index.css's dark/light blocks so the terminal
// matches the rest of the chrome instead of always being pitch black.
const DARK_XTERM_THEME = { background: "#0d0d0d", foreground: "#d4d4d4", cursor: "#d4d4d4" };
const LIGHT_XTERM_THEME = { background: "#ffffff", foreground: "#1e1e1e", cursor: "#1e1e1e" };

function getXtermTheme() {
  return document.documentElement.classList.contains("dark") ? DARK_XTERM_THEME : LIGHT_XTERM_THEME;
}

interface TerminalPreviewProps {
  tabId: string;
  adapterStatus?: AdapterStatusInfo[];
}

export function TerminalPreview({ tabId, adapterStatus }: TerminalPreviewProps) {
//...
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);

  useEffect(() => {
    if (!tabId || !containerRef.current) return;
    const container = containerRef.current;

    // Raised at the very start of cleanup so every async callback can bail
    // out before touching the terminal. Guards the race between React unmount,
    // the ResizeObserver, and in-flight pty:output events that arrive after
    // the shell exits but before disposal completes.
    const disposed = { current: false };

    const term = new Terminal({
      theme: getXtermTheme(),
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', monospace",
      scrollback: 1000,
      convertEol: true,
      cursorBlink: true,
      cursorStyle: "bar",
      allowProposedApi: true,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    term.open(container);

    // Register term ref in terminalStore so ChatPane can read terminal context
    useTerminalStore.getState().setTermRef(tabId, term);

    // CanvasAddon MUST be loaded after open()
    try {
      const canvasAddon = new CanvasAddon();
      term.loadAddon(canvasAddon);
    } catch (err) {
      console.warn("CanvasAddon failed to load, continuing without hardware acceleration:", err);
    }

    // Must be registered before the first fitAddon.fit() call below. xterm
    // only fires "resize" when the size actually changes, and the very
    // first fit (going from the constructor's default size to the real
    // container-fitted size) is exactly that one-time change — registering
    // this listener any later means it's not listening yet when that event
    // fires, so the remote PTY never learns the terminal's real size. It
    // silently stays at whatever RequestPty's placeholder was (80 cols) until
    // some later, unrelated resize (e.g. toggling Hide/Show PairAdmin, which
    // changes height) happens to fire onResize again. A plain shell just
    // wraps text a little early against that stale width and it's easy to
    // miss; tmux actively renders its status bar and pane borders to
    // whatever width it was told and doesn't redraw until that later resize
    // corrects it — which is what made this look like a rendering bug rather
    // than a missed event.
    const onResizeDisposable = term.onResize(({ cols, rows }) => {
      if (disposed.current) return;
      ResizeTerminal(tabId, cols, rows).catch(() => {});
    });

    fitAddon.fit();

    // Belt-and-suspenders: FitAddon measures character-cell pixel width using
    // whatever font is *currently* resolved. The fontFamily above is a stack
    // of locally-installed fonts, not a bundled webfont, but the browser
    // still resolves that stack asynchronously — if the fit above ran before
    // it settled, the measurement could use a wider fallback font, computing
    // fewer columns than the terminal actually has room for. Re-fitting once
    // the real font has resolved corrects that if it ever happens; the
    // onResize listener above is already in place by now to catch it.
    document.fonts.ready.then(() => {
      if (disposed.current) return;
      fitAddon.fit();
    });

    termRef.current = term;

    // theme-provider.tsx toggles the "dark"/"light" class on <html> but has
    // no event of its own to subscribe to — observe the class attribute
    // directly so an already-open terminal recolors immediately when the
    // user switches themes, instead of only picking it up on next launch.
    const themeObserver = new MutationObserver(() => {
      term.options.theme = getXtermTheme();
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    // xterm maps Ctrl+C/Ctrl+V to their literal terminal control codes
    // (\x03 SIGINT, \x16) by default — neither is connected to the OS
    // clipboard on its own. xterm's hidden textarea *also* has its own
    // internal listener for the browser's native "paste" event, so handling
    // Ctrl+V only via this keydown hook isn't enough by itself: that
    // internal listener can still fire independently and paste a second
    // time. Capture-and-block the native paste event on the textarea
    // directly so our explicit handling below is the only thing that ever
    // runs.
    const textarea = (term as unknown as { textarea?: HTMLTextAreaElement }).textarea;
    const blockNativePaste = (e: Event) => {
      e.preventDefault();
      e.stopImmediatePropagation();
    };
    textarea?.addEventListener("paste", blockNativePaste, { capture: true });

    term.attachCustomKeyEventHandler((event) => {
      if (event.type !== "keydown" || !(event.ctrlKey || event.metaKey)) return true;
      if (event.shiftKey || event.altKey) return true;

      if (event.key.toLowerCase() === "c") {
        const selection = term.getSelection();
        if (selection) {
          navigator.clipboard.writeText(selection).catch(() => {});
          return false;
        }
        return true;
      }

      if (event.key.toLowerCase() === "v") {
        navigator.clipboard
          .readText()
          .then((text) => {
            if (text && !disposed.current) {
              WriteInput(tabId, text).catch(() => {});
            }
          })
          .catch(() => {});
        return false;
      }

      return true;
    });

    // PTY output → xterm
    let unsubPtyOutput: (() => void) | null = null;
    let windowsPullInterval: number | null = null;

    if (tabId.startsWith("windows:")) {
      // Pull-based model for Windows to prevent background crashes
      windowsPullInterval = window.setInterval(() => {
        if (disposed.current) return;
        GetWindowsContent(tabId).then((content) => {
          if (content && !disposed.current) {
            const formatted = content.trimEnd().split('\n').join('\x1b[K\r\n');
            term.write("\x1b[?7l\x1b[H" + formatted + "\x1b[K\x1b[J");
          }
        }).catch(() => {});
      }, 500);
    } else {
      // Push-based model (PTY events) for other platforms/adapters
      unsubPtyOutput = EventsOn("pty:output", ((event: { tabId: string; data: string }) => {
        if (event.tabId === tabId && !disposed.current) {
          term.write(event.data);
        }
      }) as (...args: unknown[]) => void);
    }

    // xterm input → PTY
    const onDataDisposable = term.onData((data) => {
      if (disposed.current) return;
      WriteInput(tabId, data).catch(() => {});
    });

    const resizeObserver = new ResizeObserver((entries) => {
      if (disposed.current) return;
      // Hidden tabs (ancestor display:none) collapse to 0x0 — fitting to that
      // sends a near-zero resize to the PTY, which truncates the real
      // console screen buffer. Skip fitting while not actually visible.
      const { width, height } = entries[0].contentRect;
      if (width === 0 || height === 0) return;
      fitAddon.fit();
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      disposed.current = true; // must be first — blocks all in-flight callbacks
      textarea?.removeEventListener("paste", blockNativePaste, { capture: true });
      themeObserver.disconnect();
      resizeObserver.disconnect();
      unsubPtyOutput?.();
      if (windowsPullInterval) window.clearInterval(windowsPullInterval);
      onDataDisposable.dispose();
      onResizeDisposable.dispose();

      // Clear the ref in store immediately so terminal:update doesn't try to use it
      useTerminalStore.getState().setTermRef(tabId, null);

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
      <div className="h-full w-full flex items-center justify-center bg-surface-0 text-surface-text-muted">
        <div className="text-center space-y-4 max-w-md">
          <p className="text-lg">No terminal sessions detected.</p>

          {atspiOnboarding && (
            <div className="space-y-2">
              <p className="text-sm text-surface-text-muted">Enable accessibility for GUI terminals</p>
              <code className="block px-3 py-1.5 bg-surface-2 rounded text-sm text-green-400 font-mono">
                $ gsettings set org.gnome.desktop.interface toolkit-accessibility true
              </code>
              <p className="text-xs text-surface-text-muted">Then restart your terminal application.</p>
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
