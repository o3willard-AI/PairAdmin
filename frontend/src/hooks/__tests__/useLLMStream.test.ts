import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useChatStore } from "@/stores/chatStore";

// Storage for mock EventsOn across test accesses
const mockEventHandlers: Record<string, unknown> = {};
const mockUnsubFns: Record<string, ReturnType<typeof vi.fn>> = {};

const mockEventsOn = vi.fn((eventName: string, handler: unknown) => {
  mockEventHandlers[eventName] = handler;
  const unsub = vi.fn();
  mockUnsubFns[eventName] = unsub;
  return unsub;
});

// Mock the Wails runtime module.
// The hook is at frontend/src/hooks/useLLMStream.ts and imports:
//   import(/* @vite-ignore */ "../../wailsjs/runtime/runtime")
// which resolves (from frontend/src/hooks/) to frontend/wailsjs/runtime/runtime.
// From this test file (frontend/src/hooks/__tests__/) that is ../../../wailsjs/runtime/runtime
vi.mock("../../../wailsjs/runtime/runtime", async () => ({
  EventsOn: mockEventsOn,
}));

describe("useLLMStream", () => {
  beforeEach(() => {
    useChatStore.setState({ messagesByTab: {} });
    vi.clearAllMocks();
    // Reset handler capture (clearAllMocks resets call history but NOT implementations)
    Object.keys(mockEventHandlers).forEach((k) => delete mockEventHandlers[k]);
    Object.keys(mockUnsubFns).forEach((k) => delete mockUnsubFns[k]);
  });

  it("hook subscribes to llm:chunk, llm:done, and llm:error on mount", async () => {
    const { useLLMStream } = await import("@/hooks/useLLMStream");

    renderHook(() => useLLMStream("tab-1"));

    // Allow the dynamic import promise to resolve
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const eventNames = mockEventsOn.mock.calls.map((call) => call[0]);
    expect(eventNames).toContain("llm:chunk");
    expect(eventNames).toContain("llm:done");
    expect(eventNames).toContain("llm:error");
  });

  it("hook unsubscribes all three on unmount", async () => {
    const { useLLMStream } = await import("@/hooks/useLLMStream");
    const { unmount } = renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    // Capture unsub fns before unmount
    const chunkUnsub = mockUnsubFns["llm:chunk"];
    const doneUnsub = mockUnsubFns["llm:done"];
    const errorUnsub = mockUnsubFns["llm:error"];

    unmount();

    expect(chunkUnsub).toHaveBeenCalled();
    expect(doneUnsub).toHaveBeenCalled();
    expect(errorUnsub).toHaveBeenCalled();
  });

  it("receiving llm:chunk event calls startStreamingMessage then appendChunk", async () => {
    const startSpy = vi.spyOn(useChatStore.getState(), "startStreamingMessage");
    const appendSpy = vi.spyOn(useChatStore.getState(), "appendChunk");

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const chunkHandler = mockEventHandlers["llm:chunk"] as (event: { seq: number; text: string }) => void;
    expect(chunkHandler).toBeDefined();

    act(() => {
      chunkHandler({ seq: 0, text: "hello" });
    });

    expect(startSpy).toHaveBeenCalledWith("tab-1");
    expect(appendSpy).toHaveBeenCalledWith("tab-1", expect.any(String), "hello");
  });

  it("receiving llm:done event calls finalizeMessage", async () => {
    const finalizeSpy = vi.spyOn(useChatStore.getState(), "finalizeMessage");

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const chunkHandler = mockEventHandlers["llm:chunk"] as (event: { seq: number; text: string }) => void;
    const doneHandler = mockEventHandlers["llm:done"] as () => void;

    // First send a chunk to create a message
    act(() => {
      chunkHandler({ seq: 0, text: "hello" });
    });

    const msgId = useChatStore.getState().messagesByTab["tab-1"]?.[0]?.id;

    act(() => {
      doneHandler();
    });

    expect(finalizeSpy).toHaveBeenCalledWith("tab-1", msgId);
  });

  it("receiving llm:error event calls setStreamError", async () => {
    const setStreamErrorSpy = vi.spyOn(useChatStore.getState(), "setStreamError");

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const errorHandler = mockEventHandlers["llm:error"] as (event: { error: string }) => void;
    expect(errorHandler).toBeDefined();

    act(() => {
      errorHandler({ error: "Connection failed" });
    });

    expect(setStreamErrorSpy).toHaveBeenCalledWith("tab-1", null, "Connection failed");
  });

  it("out-of-order chunks (seq 1 arrives before seq 0) are reordered before applying", async () => {
    const appendSpy = vi.spyOn(useChatStore.getState(), "appendChunk");

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const chunkHandler = mockEventHandlers["llm:chunk"] as (event: { seq: number; text: string }) => void;
    expect(chunkHandler).toBeDefined();

    // Send seq 1 before seq 0
    act(() => {
      chunkHandler({ seq: 1, text: "world" });
    });

    // seq 1 should be buffered — no appendChunk calls yet
    const callsBefore = appendSpy.mock.calls.length;
    expect(callsBefore).toBe(0);

    act(() => {
      chunkHandler({ seq: 0, text: "hello " });
    });

    // After seq 0, both seq 0 and seq 1 should have been applied
    const callsAfter = appendSpy.mock.calls.length;
    expect(callsAfter).toBe(2);

    // Verify order: seq 0 first, then seq 1
    const texts = appendSpy.mock.calls.map((c) => c[2]);
    expect(texts[0]).toBe("hello ");
    expect(texts[1]).toBe("world");
  });
});
