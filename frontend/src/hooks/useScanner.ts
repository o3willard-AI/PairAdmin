import { useEffect } from "react";
import { useScannerStore } from "@/stores/scannerStore";

// Subscribes to the scanner's scan:* events for the life of the component and
// dispatches each to the scanner store — mirrors useLLMStream's structure
// exactly. There is no per-scan tabId: a scan is app-global (only one active
// scan at a time), so a single subscription set on mount is sufficient.
export function useScanner() {
  useEffect(() => {
    const { handleProgress, handleHost, handleDone, handleError, handleCancelled } =
      useScannerStore.getState();

    let unsubProgress: (() => void) | null = null;
    let unsubHost: (() => void) | null = null;
    let unsubDone: (() => void) | null = null;
    let unsubError: (() => void) | null = null;
    let unsubCancelled: (() => void) | null = null;

    import(/* @vite-ignore */ "../../wailsjs/runtime/runtime").then((rt) => {
      unsubProgress = rt.EventsOn(
        "scan:progress",
        handleProgress as (...args: unknown[]) => void,
      );
      unsubHost = rt.EventsOn("scan:host", handleHost as (...args: unknown[]) => void);
      unsubDone = rt.EventsOn("scan:done", handleDone as (...args: unknown[]) => void);
      unsubError = rt.EventsOn("scan:error", handleError as (...args: unknown[]) => void);
      unsubCancelled = rt.EventsOn(
        "scan:cancelled",
        handleCancelled as (...args: unknown[]) => void,
      );
    });

    return () => {
      unsubProgress?.();
      unsubHost?.();
      unsubDone?.();
      unsubError?.();
      unsubCancelled?.();
    };
  }, []);
}

// Helpers that drive a scan through the generated Start/Stop bindings (via the
// store, which owns the status transitions). UI components call these without
// touching the store's binding layer directly.
export function startScan(targets: string[], maxProbes?: number) {
  return useScannerStore.getState().startScan(targets, maxProbes);
}

export function stopScan() {
  return useScannerStore.getState().stopScan();
}