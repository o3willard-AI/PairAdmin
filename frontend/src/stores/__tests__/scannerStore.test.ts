import { describe, it, expect, vi, beforeEach } from "vitest";
import { act } from "@testing-library/react";
import {
  useScannerStore,
  type HostRow,
  type ScanStats,
} from "@/stores/scannerStore";

// Mock the generated ScanService binding so the store's startScan/stopScan
// don't reach into window['go'] (undefined in jsdom).
const mockStart = vi.fn();
const mockStop = vi.fn();
vi.mock("../../../wailsjs/go/scanner/ScanService", () => ({
  Start: (...args: unknown[]) => mockStart(...args),
  Stop: (...args: unknown[]) => mockStop(...args),
}));

const idleState = {
  activeScanID: null,
  status: "idle" as const,
  progress: { done: 0, total: 0, activeProbes: 0 },
  rows: [] as HostRow[],
  stats: null as ScanStats | null,
  error: "",
};

beforeEach(() => {
  vi.clearAllMocks();
  useScannerStore.setState(idleState);
});

const row = (ip: string, state: HostRow["state"]): HostRow => ({ ip, state });

describe("scannerStore", () => {
  it("handleHost appends rows as events arrive", () => {
    useScannerStore.setState({ activeScanID: "scan-1" });
    act(() => {
      useScannerStore.getState().handleHost({ scanID: "scan-1", row: row("10.0.0.1", "ssh") });
      useScannerStore.getState().handleHost({ scanID: "scan-1", row: row("10.0.0.2", "filtered") });
    });
    const { rows } = useScannerStore.getState();
    expect(rows).toHaveLength(2);
    expect(rows[0]).toEqual({ ip: "10.0.0.1", state: "ssh" });
    expect(rows[1]).toEqual({ ip: "10.0.0.2", state: "filtered" });
    // Mutation check: removing `state.rows.push(event.row)` in handleHost
    // makes rows stay empty, so this assertion fails — results that arrive
    // one host at a time would never be surfaced.
  });

  it("handleDone sets status=done, records stats, and clears the active scan", () => {
    useScannerStore.setState({ activeScanID: "scan-1", status: "scanning" });
    const stats: ScanStats = { total: 4, ssh: 2, filtered: 1, closed: 1, durationMs: 120 };
    act(() => {
      useScannerStore.getState().handleDone({ scanID: "scan-1", stats });
    });
    const s = useScannerStore.getState();
    expect(s.status).toBe("done");
    expect(s.stats).toEqual(stats);
    expect(s.activeScanID).toBeNull();
    // Mutation check: dropping the status/stats assignment in handleDone
    // leaves the store stuck in "scanning" with no terminal summary, so this
    // assertion fails.
  });

  it("handleError sets status=error with the message and clears the active scan", () => {
    useScannerStore.setState({ activeScanID: "scan-1", status: "scanning" });
    act(() => {
      useScannerStore.getState().handleError({ scanID: "scan-1", message: "no local networks" });
    });
    const s = useScannerStore.getState();
    expect(s.status).toBe("error");
    expect(s.error).toBe("no local networks");
    expect(s.activeScanID).toBeNull();
    // Mutation check: removing the error status/message write in handleError
    // would leave the UI showing "scanning" forever with no explanation.
  });

  it("handleCancelled sets the terminal status to cancelled", () => {
    useScannerStore.setState({ activeScanID: "scan-1", status: "scanning" });
    act(() => {
      useScannerStore.getState().handleCancelled({ scanID: "scan-1" });
    });
    const s = useScannerStore.getState();
    expect(s.status).toBe("cancelled");
    expect(s.activeScanID).toBeNull();
    // Mutation check: removing the status set in handleCancelled leaves the
    // store "scanning" even though the backend stopped — the UI would never
    // show the cancelled state.
  });

  it("reset clears all state back to idle", () => {
    useScannerStore.setState({
      activeScanID: "scan-1",
      status: "scanning",
      progress: { done: 5, total: 10, activeProbes: 3 },
      rows: [row("10.0.0.1", "ssh")],
      stats: { total: 10, ssh: 1, filtered: 4, closed: 5, durationMs: 99 },
      error: "boom",
    });
    act(() => {
      useScannerStore.getState().reset();
    });
    expect(useScannerStore.getState()).toMatchObject(idleState);
    // Mutation check: removing reset's field clearing makes this stay dirty,
    // so a "Clear results" UI could never return to idle.
  });

  it("startScan calls the Start binding and enters the scanning state", async () => {
    mockStart.mockResolvedValue("scan-abc");
    let returned: string | null | undefined;
    await act(async () => {
      returned = await useScannerStore.getState().startScan(["10.0.0.0/24"], 16);
    });
    expect(mockStart).toHaveBeenCalledWith({ targets: ["10.0.0.0/24"], maxProbes: 16 });
    expect(returned).toBe("scan-abc");
    const s = useScannerStore.getState();
    expect(s.status).toBe("scanning");
    expect(s.activeScanID).toBe("scan-abc");
    expect(s.rows).toHaveLength(0);
    // Mutation check: removing the status/activeScanID write on a successful
    // Start leaves the store idle while the backend is actually scanning.
  });

  it("startScan surfaces an ErrScanningDisabled rejection as a terminal error", async () => {
    mockStart.mockRejectedValue("network scanning is disabled in settings");
    await act(async () => {
      await useScannerStore.getState().startScan([], 16);
    });
    const s = useScannerStore.getState();
    expect(s.status).toBe("error");
    expect(s.error).toContain("scanning is disabled");
    expect(s.activeScanID).toBeNull();
    // Mutation check: returning the rejection unhandled (no catch -> terminal
    // error) would leave the store idle, hiding the "scanner disabled" reason.
  });
});