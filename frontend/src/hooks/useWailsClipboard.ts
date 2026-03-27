// Clipboard abstraction over Wails Go binding with browser API fallback.
// The Wails binding is generated at runtime by `wails dev` / `wails build`.
// When running outside Wails (e.g. vitest, browser dev mode), the binding
// import will fail and we fall back to navigator.clipboard.

export function useWailsClipboard() {
  const copyToClipboard = async (text: string) => {
    try {
      // Dynamic import so a missing module doesn't crash the entire app.
      const { CopyToClipboard } = await import(
        /* @vite-ignore */
        "../../wailsjs/go/services/CommandService"
      );
      await CopyToClipboard(text);
    } catch (err) {
      // Fallback to browser clipboard API if Wails binding fails.
      // This happens in dev mode before bindings are generated.
      console.warn("Wails clipboard failed, trying browser API:", err);
      await navigator.clipboard.writeText(text);
    }
  };

  return { copyToClipboard };
}
