import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { devtools } from "zustand/middleware";
import {
  Start as scanStart,
  Stop as scanStop,
} from "../../wailsjs/go/scanner/ScanService";

// --- Scanner domain types ---------------------------------------------------
// These mirror the pinned JSON contract in services/scanner/service.go (the
// scan:* event payloads). They are deliberately declared here (NOT in the
// wails models): the models file only carries types that appear as bound
// function args/returns (ScanRequest), and event payloads are a runtime
// contract, not a wails signature, so the store owns its own row/event types.

export type ScannerStatus = "idle" | "scanning" | "done" | "error" | "cancelled";

/** Host state enum is declared in the backend; Mode A emits ssh/filtered/closed. */
export type HostState = "ssh" | "ssh-known" | "filtered" | "closed" | "alive" | "dead";

export interface SSHInfo {
  port: number;
  banner: string;
  software: string;
  hostKeyType: string;
  hostKeyFingerprint: string;
}

export interface HostRow {
  ip: string;
  state: HostState;
  ssh?: SSHInfo;
}

export interface ScanProgress {
  done: number;
  total: number;
  activeProbes: number;
}

export interface ScanStats {
  total: number;
  ssh: number;
  filtered: number;
  closed: number;
  durationMs: number;
}

// Pinned event payloads (scan:*).
export interface ScanProgressEvent {
  scanID: string;
  done: number;
  total: number;
  activeProbes: number;
}
export interface ScanHostEvent {
  scanID: string;
  row: HostRow;
}
export interface ScanDoneEvent {
  scanID: string;
  stats: ScanStats;
}
export interface ScanErrorEvent {
  scanID: string;
  message: string;
}
export interface ScanCancelledEvent {
  scanID: string;
}

const emptyProgress: ScanProgress = { done: 0, total: 0, activeProbes: 0 };

interface ScannerState {
  activeScanID: string | null;
  status: ScannerStatus;
  progress: ScanProgress;
  rows: HostRow[];
  stats: ScanStats | null;
  error: string;

  startScan: (targets: string[], maxProbes?: number) => Promise<string | null>;
  stopScan: () => Promise<void>;
  handleProgress: (event: ScanProgressEvent) => void;
  handleHost: (event: ScanHostEvent) => void;
  handleDone: (event: ScanDoneEvent) => void;
  handleError: (event: ScanErrorEvent) => void;
  handleCancelled: (event: ScanCancelledEvent) => void;
  reset: () => void;
}

export const useScannerStore = create<ScannerState>()(
  devtools(
    immer((set, get) => ({
      activeScanID: null,
      status: "idle",
      progress: emptyProgress,
      rows: [],
      stats: null,
      error: "",

      startScan: async (targets, maxProbes) => {
        try {
          const scanID = await scanStart({
            targets,
            maxProbes: maxProbes ?? 0,
          });
          set((state) => {
            state.activeScanID = scanID;
            state.status = "scanning";
            state.progress = emptyProgress;
            state.rows = [];
            state.stats = null;
            state.error = "";
          });
          return scanID;
        } catch (err) {
          // Start rejects synchronously e.g. when scanning is disabled
          // (ErrScanningDisabled) — before any scan:error event could fire, so
          // surface it here as a terminal error state.
          const message =
            typeof err === "string"
              ? err
              : err instanceof Error
                ? err.message
                : String(err);
          set((state) => {
            state.activeScanID = null;
            state.status = "error";
            state.error = message;
          });
          return null;
        }
      },

      stopScan: async () => {
        const { activeScanID } = get();
        if (!activeScanID) return;
        await scanStop(activeScanID);
      },

      handleProgress: (event) => {
        set((state) => {
          if (state.activeScanID && event.scanID !== state.activeScanID) return;
          state.progress = {
            done: event.done,
            total: event.total,
            activeProbes: event.activeProbes,
          };
        });
      },

      handleHost: (event) => {
        set((state) => {
          if (state.activeScanID && event.scanID !== state.activeScanID) return;
          state.rows.push(event.row);
        });
      },

      handleDone: (event) => {
        set((state) => {
          state.activeScanID = null;
          state.status = "done";
          state.stats = event.stats;
        });
      },

      handleError: (event) => {
        set((state) => {
          state.activeScanID = null;
          state.status = "error";
          state.error = event.message;
        });
      },

      handleCancelled: (event) => {
        set((state) => {
          state.activeScanID = null;
          state.status = "cancelled";
        });
      },

      reset: () => {
        set((state) => {
          state.activeScanID = null;
          state.status = "idle";
          state.progress = emptyProgress;
          state.rows = [];
          state.stats = null;
          state.error = "";
        });
      },
    })),
    { name: "scanner-store" }
  )
);