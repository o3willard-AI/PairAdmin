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

// jsdom also doesn't implement Element.getAnimations() — base-ui's
// ScrollArea (used by CommandSidebar, among others) calls it to check
// whether a fade-out is still running before hiding the scrollbar.
if (typeof Element.prototype.getAnimations === "undefined") {
  Element.prototype.getAnimations = () => [];
}

// jsdom doesn't implement the CSS Font Loading API (document.fonts) at all —
// TerminalPreview.tsx awaits document.fonts.ready before its corrective
// terminal re-fit. Resolve immediately so that await doesn't hang forever.
if (typeof document.fonts === "undefined") {
  Object.defineProperty(document, "fonts", {
    value: { ready: Promise.resolve() },
    configurable: true,
  });
}
