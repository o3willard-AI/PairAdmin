// Test stub for Wails runtime — replaced by actual Wails-generated bindings at dev/build time.
// This file exists so vitest can resolve the dynamic import path during testing.
// Tests override individual functions via vi.mock().

export const EventsOn = (_eventName: string, _callback: unknown): (() => void) => {
  return () => {};
};

export const EventsOff = (_eventName: string): void => {};

export const EventsEmit = (_eventName: string, ..._data: unknown[]): void => {};
