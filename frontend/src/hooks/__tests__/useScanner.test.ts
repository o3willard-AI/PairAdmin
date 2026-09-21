import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const EVENT_NAMES = [
  "scan:progress",
  "scan:host",
  "scan:done",
  "scan:error",
  "scan:cancelled",
];

// Mock the wailsjs runtime so we can exercise EventsOn registration and the
// cleanup path without a real Wails backend.
const mockEventHandlers: Record<string, ((...args: unknown[]) => void) | undefined> = {};
const mockUnsubFns: Record<string, ReturnType<typeof vi.fn>> = {};
const mockEventsOn = vi.fn(
  (eventName: string, handler: (...args: unknown[]) => void) => {
    mockEventHandlers[eventName] = handler;
    const unsub = vi.fn();
    mockUnsubFns[eventName] = unsub;
    return unsub;
  },
);
vi.mock("../../../wailsjs/runtime/runtime", async () => ({
  EventsOn: mockEventsOn,
}));

// Mock the generated ScanService binding so the startScan/stopScan helpers can
// be exercised without window['go'] (undefined in jsdom).
const mockStart = vi.fn();
const mockStop = vi.fn();
vi.mock("../../../wailsjs/go/scanner/ScanService", () => ({
  Start: (...args: unknown[]) => mockStart(...args),
  Stop: (...args: unknown[]) => mockStop(...args),
}));

// Allow the hook's dynamic runtime import to resolve, mirroring useLLMStream.test.
const settleImport = async () => {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
};

beforeEach(async () => {
  vi.clearAllMocks();
  const { useScannerStore } = await import("@/stores/scannerStore");
  useScannerStore.setState({
    activeScanID: null,
    status: "idle",
    progress: { done: 0, total: 0, activeProbes: 0 },
    rows: [],
    stats: null,
    error: "",
  });
});

describe("useScanner", () => {
  it("subscribes to every scan event on mount", async () => {
    const { useScanner } = await import("@/hooks/useScanner");
    renderHook(() => useScanner());
    await settleImport();
    expect(mockEventsOn).toHaveBeenCalledTimes(EVENT_NAMES.length);
    for (const eventName of EVENT_NAMES) {
      expect(mockEventHandlers[eventName]).toBeDefined();
    }
    // Mutation check: a subscription that drops one of the five event names
    // makes that handler undefined and this assertion fail — that scan:*
    // event would never reach the store.
  });

  it("dispatches scan:host events into the store", async () => {
    const { useScanner } = await import("@/hooks/useScanner");
    const { useScannerStore } = await import("@/stores/scannerStore");
    useScannerStore.setState({ activeScanID: "scan-1" });
    renderHook(() => useScanner());
    await settleImport();
    const handler = mockEventHandlers["scan:host"];
    expect(handler).toBeDefined();
    act(() => {
      handler!({ scanID: "scan-1", row: { ip: "10.0.0.1", state: "ssh" } });
    });
    expect(useScannerStore.getState().rows).toEqual([
      { ip: "10.0.0.1", state: "ssh" },
    ]);
    // Mutation check: if the subscription wiring never calls the store's
    // handleHost, incoming host events are dropped and this row assertion
    // fails — the scan UI would show an empty list.
  });

  it("dispatches scan:done events into the store", async () => {
    const { useScanner } = await import("@/hooks/useScanner");
    const { useScannerStore } = await import("@/stores/scannerStore");
    renderHook(() => useScanner());
    await settleImport();
    const handler = mockEventHandlers["scan:done"];
    expect(handler).toBeDefined();
    act(() => {
      handler!({
        scanID: "scan-1",
        stats: { total: 1, ssh: 1, filtered: 0, closed: 0, durationMs: 5 },
      });
    });
    expect(useScannerStore.getState().status).toBe("done");
    // Mutation check: removing the scan:done dispatch leaves the store stuck
    // in "scanning" — the results summary never appears.
  });

  it("unsubscribes from every event on cleanup", async () => {
    const { useScanner } = await import("@/hooks/useScanner");
    const { unmount } = renderHook(() => useScanner());
    await settleImport();
    unmount();
    for (const eventName of EVENT_NAMES) {
      expect(mockUnsubFns[eventName]).toHaveBeenCalledTimes(1);
    }
    // Mutation check: dropping the cleanup's unsubscribe calls leaves the
    // handlers registered after unmount, leaking subscription closures across
    // every lifecycle — this assertion counts each unsubscribe and fails.
  });

  it("startScan helper drives the generated Start binding and enters scanning", async () => {
    const { startScan } = await import("@/hooks/useScanner");
    const { useScannerStore } = await import("@/stores/scannerStore");
    mockStart.mockResolvedValue("scan-abc");
    let result: string | null | undefined;
    await act(async () => {
      result = await startScan(["10.0.0.0/24"], 16);
    });
    expect(mockStart).toHaveBeenCalledWith({ targets: ["10.0.0.0/24"], maxProbes: 16 });
    expect(result).toBe("scan-abc");
    expect(useScannerStore.getState().status).toBe("scanning");
  });

  it("stopScan helper forwards to the Stop binding", async () => {
    const { stopScan } = await import("@/hooks/useScanner");
    const { useScannerStore } = await import("@/stores/scannerStore");
    useScannerStore.setState({ activeScanID: "scan-abc", status: "scanning" });
    await act(async () => {
      await stopScan();
    });
    expect(mockStop).toHaveBeenCalledWith("scan-abc");
  });
});