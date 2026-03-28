// Stub for Wails runtime — replaced by actual generated bindings at wails dev/build time.
// This file exists so vitest can resolve the dynamic import path during testing.
// Tests override individual functions via vi.mock().

export const EventsOn = (_eventName, _callback) => {
  return () => {};
};

export const EventsOff = (_eventName) => {};

export const EventsEmit = (_eventName, ..._data) => {};
