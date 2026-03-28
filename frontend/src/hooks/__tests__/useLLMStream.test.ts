import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useChatStore } from "@/stores/chatStore";

// Mock Wails runtime dynamic import
vi.mock(
  "../../wailsjs/runtime/runtime",
  async () => ({
    EventsOn: vi.fn(() => vi.fn()), // returns unsub fn
  }),
  { virtual: true }
);

// We also need to mock the path that useLLMStream will actually use
// The hook is in frontend/src/hooks/ so the relative path is ../wailsjs/runtime/runtime
vi.mock(
  "../wailsjs/runtime/runtime",
  async () => ({
    EventsOn: vi.fn(() => vi.fn()),
  }),
  { virtual: true }
);

describe("useLLMStream", () => {
  beforeEach(() => {
    useChatStore.setState({ messagesByTab: {} });
    vi.clearAllMocks();
  });

  it("hook subscribes to llm:chunk, llm:done, and llm:error on mount", async () => {
    const { useLLMStream } = await import("@/hooks/useLLMStream");
    const { default: runtimeMod } = await import(
      /* @vite-ignore */
      "../wailsjs/runtime/runtime" as string
    );
    const EventsOn = runtimeMod.EventsOn as ReturnType<typeof vi.fn>;

    renderHook(() => useLLMStream("tab-1"));

    // Allow the dynamic import promise to resolve
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const eventNames = EventsOn.mock.calls.map((call: unknown[]) => call[0]);
    expect(eventNames).toContain("llm:chunk");
    expect(eventNames).toContain("llm:done");
    expect(eventNames).toContain("llm:error");
  });

  it("hook unsubscribes all three on unmount", async () => {
    const unsubFns = [vi.fn(), vi.fn(), vi.fn()];
    let callIdx = 0;
    const { default: runtimeMod } = await import(
      /* @vite-ignore */
      "../wailsjs/runtime/runtime" as string
    );
    const EventsOn = runtimeMod.EventsOn as ReturnType<typeof vi.fn>;
    EventsOn.mockImplementation(() => unsubFns[callIdx++] ?? vi.fn());

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    const { unmount } = renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    unmount();

    // All unsub functions should have been called
    unsubFns.forEach((fn) => expect(fn).toHaveBeenCalled());
  });

  it("receiving llm:chunk event calls startStreamingMessage then appendChunk", async () => {
    const startSpy = vi.spyOn(useChatStore.getState(), "startStreamingMessage");
    const appendSpy = vi.spyOn(useChatStore.getState(), "appendChunk");

    let capturedChunkHandler: ((event: { seq: number; text: string }) => void) | null = null;
    const { default: runtimeMod } = await import(
      /* @vite-ignore */
      "../wailsjs/runtime/runtime" as string
    );
    const EventsOn = runtimeMod.EventsOn as ReturnType<typeof vi.fn>;
    EventsOn.mockImplementation((eventName: string, handler: unknown) => {
      if (eventName === "llm:chunk") {
        capturedChunkHandler = handler as (event: { seq: number; text: string }) => void;
      }
      return vi.fn();
    });

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    expect(capturedChunkHandler).not.toBeNull();

    act(() => {
      capturedChunkHandler!({ seq: 0, text: "hello" });
    });

    expect(startSpy).toHaveBeenCalledWith("tab-1");
    expect(appendSpy).toHaveBeenCalledWith("tab-1", expect.any(String), "hello");
  });

  it("receiving llm:done event calls finalizeMessage", async () => {
    const finalizeSpy = vi.spyOn(useChatStore.getState(), "finalizeMessage");

    let capturedChunkHandler: ((event: { seq: number; text: string }) => void) | null = null;
    let capturedDoneHandler: (() => void) | null = null;
    const { default: runtimeMod } = await import(
      /* @vite-ignore */
      "../wailsjs/runtime/runtime" as string
    );
    const EventsOn = runtimeMod.EventsOn as ReturnType<typeof vi.fn>;
    EventsOn.mockImplementation((eventName: string, handler: unknown) => {
      if (eventName === "llm:chunk") capturedChunkHandler = handler as (event: { seq: number; text: string }) => void;
      if (eventName === "llm:done") capturedDoneHandler = handler as () => void;
      return vi.fn();
    });

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    // First send a chunk to create a message
    act(() => {
      capturedChunkHandler!({ seq: 0, text: "hello" });
    });

    const msgId = useChatStore.getState().messagesByTab["tab-1"]?.[0]?.id;

    act(() => {
      capturedDoneHandler!();
    });

    expect(finalizeSpy).toHaveBeenCalledWith("tab-1", msgId);
  });

  it("receiving llm:error event calls setStreamError", async () => {
    const setStreamErrorSpy = vi.spyOn(useChatStore.getState(), "setStreamError");

    let capturedErrorHandler: ((event: { error: string }) => void) | null = null;
    const { default: runtimeMod } = await import(
      /* @vite-ignore */
      "../wailsjs/runtime/runtime" as string
    );
    const EventsOn = runtimeMod.EventsOn as ReturnType<typeof vi.fn>;
    EventsOn.mockImplementation((eventName: string, handler: unknown) => {
      if (eventName === "llm:error") capturedErrorHandler = handler as (event: { error: string }) => void;
      return vi.fn();
    });

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    act(() => {
      capturedErrorHandler!({ error: "Connection failed" });
    });

    expect(setStreamErrorSpy).toHaveBeenCalledWith("tab-1", null, "Connection failed");
  });

  it("out-of-order chunks (seq 1 arrives before seq 0) are reordered before applying", async () => {
    const appendSpy = vi.spyOn(useChatStore.getState(), "appendChunk");

    let capturedChunkHandler: ((event: { seq: number; text: string }) => void) | null = null;
    const { default: runtimeMod } = await import(
      /* @vite-ignore */
      "../wailsjs/runtime/runtime" as string
    );
    const EventsOn = runtimeMod.EventsOn as ReturnType<typeof vi.fn>;
    EventsOn.mockImplementation((eventName: string, handler: unknown) => {
      if (eventName === "llm:chunk") capturedChunkHandler = handler as (event: { seq: number; text: string }) => void;
      return vi.fn();
    });

    const { useLLMStream } = await import("@/hooks/useLLMStream");
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    // Send seq 1 before seq 0
    act(() => {
      capturedChunkHandler!({ seq: 1, text: "world" });
    });

    // seq 1 should be buffered, appendChunk not called yet
    const callsBefore = appendSpy.mock.calls.length;

    act(() => {
      capturedChunkHandler!({ seq: 0, text: "hello " });
    });

    // After seq 0, both seq 0 and seq 1 should have been applied
    const callsAfter = appendSpy.mock.calls.length;
    expect(callsAfter - callsBefore).toBe(2);

    // Verify order: seq 0 first, then seq 1
    const texts = appendSpy.mock.calls.slice(callsBefore).map((c: unknown[]) => c[2]);
    expect(texts[0]).toBe("hello ");
    expect(texts[1]).toBe("world");
  });
});
