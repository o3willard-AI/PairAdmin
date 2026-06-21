// jsdom (the test environment) doesn't implement ResizeObserver. Several
// components observe element size changes (e.g. ChatMessageList's
// auto-scroll, TerminalPreview's fit-on-resize) — stub it out so mounting
// them in tests doesn't throw.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver;
}
