import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useLLMStream } from "@/hooks/useLLMStream";
import { useSettingsStore } from "@/stores/settingsStore";

// Storage for mock EventsOn across test accesses
const mockEventHandlers: Record<string, unknown> = {};
const mockUnsubFns: Record<string, ReturnType<typeof vi.fn>> = {};

const mockEventsOn = vi.fn((eventName: string, handler: unknown) => {
  mockEventHandlers[eventName] = handler;
  const unsub = vi.fn();
  mockUnsubFns[eventName] = unsub;
  return unsub;
});

// The hook is at frontend/src/hooks/useLLMStream.ts and imports
// "../../wailsjs/runtime/runtime"; from this test file that resolves to
// ../../../wailsjs/runtime/runtime.
vi.mock("../../../wailsjs/runtime/runtime", async () => ({
  EventsOn: mockEventsOn,
}));

// [DONE] gate: "confirm nothing can flip a 'disabled' state back to
// 'connected'/'disconnected' (a stale in-flight stream, an error event, etc.)".
// useLLMStream is the only code that writes connectionStatus from stream
// events, so these tests pin that a "disabled" status always wins.
describe("useLLMStream — disabled gate", () => {
  beforeEach(() => {
    useSettingsStore.setState({
      activeModel: "",
      settingsOpen: false,
      connectionStatus: "disabled",
    });
    vi.clearAllMocks();
    Object.keys(mockEventHandlers).forEach((k) => delete mockEventHandlers[k]);
    Object.keys(mockUnsubFns).forEach((k) => delete mockUnsubFns[k]);
  });

  it("llm:done does NOT move connectionStatus off 'disabled'", async () => {
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const doneHandler = mockEventHandlers["llm:done"] as () => void;
    expect(doneHandler).toBeDefined();

    await act(async () => {
      doneHandler();
    });

    expect(useSettingsStore.getState().connectionStatus).toBe("disabled");
  });

  it("llm:error does NOT move connectionStatus off 'disabled'", async () => {
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const errorHandler = mockEventHandlers["llm:error"] as (event: {
      error: string;
    }) => void;
    expect(errorHandler).toBeDefined();

    await act(async () => {
      errorHandler({ error: "boom" });
    });

    expect(useSettingsStore.getState().connectionStatus).toBe("disabled");
  });

  it("llm:done still sets 'connected' when the LLM is not disabled", async () => {
    useSettingsStore.setState({ connectionStatus: "disconnected" });
    renderHook(() => useLLMStream("tab-1"));

    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });

    const doneHandler = mockEventHandlers["llm:done"] as () => void;
    await act(async () => {
      doneHandler();
    });

    expect(useSettingsStore.getState().connectionStatus).toBe("connected");
  });
});
