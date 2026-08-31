import { describe, it, expect } from "vitest";
import { parseHotkey, matchesHotkey, isForeignTextEntry } from "@/utils/hotkey";

describe("parseHotkey", () => {
  it("parses a combo with all four modifiers", () => {
    expect(parseHotkey("Ctrl+Shift+Alt+Meta+A")).toEqual({
      ctrl: true,
      shift: true,
      alt: true,
      meta: true,
      key: "a",
    });
  });

  it("parses a combo with a single modifier", () => {
    expect(parseHotkey("Ctrl+Shift+N")).toEqual({
      ctrl: true,
      shift: true,
      alt: false,
      meta: false,
      key: "n",
    });
  });

  it("parses a bare key with no modifiers", () => {
    expect(parseHotkey("F5")).toEqual({
      ctrl: false,
      shift: false,
      alt: false,
      meta: false,
      key: "f5",
    });
  });

  it("returns null for an empty string", () => {
    expect(parseHotkey("")).toBeNull();
  });
});

function makeEvent(init: Partial<KeyboardEventInit> & { key: string }): KeyboardEvent {
  return new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
}

describe("matchesHotkey", () => {
  it("matches when every modifier and the key line up exactly", () => {
    const parsed = parseHotkey("Ctrl+Shift+N")!;
    const event = makeEvent({ key: "N", ctrlKey: true, shiftKey: true, altKey: false, metaKey: false });
    expect(matchesHotkey(event, parsed)).toBe(true);
  });

  it("does not match when an extra modifier is held", () => {
    const parsed = parseHotkey("Ctrl+Shift+N")!;
    const event = makeEvent({ key: "n", ctrlKey: true, shiftKey: true, altKey: true, metaKey: false });
    expect(matchesHotkey(event, parsed)).toBe(false);
  });

  it("does not match when a required modifier is missing", () => {
    const parsed = parseHotkey("Ctrl+Shift+N")!;
    const event = makeEvent({ key: "n", ctrlKey: true, shiftKey: false, altKey: false, metaKey: false });
    expect(matchesHotkey(event, parsed)).toBe(false);
  });

  it("is case-insensitive on the key itself", () => {
    const parsed = parseHotkey("Ctrl+Shift+N")!;
    const lower = makeEvent({ key: "n", ctrlKey: true, shiftKey: true, altKey: false, metaKey: false });
    const upper = makeEvent({ key: "N", ctrlKey: true, shiftKey: true, altKey: false, metaKey: false });
    expect(matchesHotkey(lower, parsed)).toBe(true);
    expect(matchesHotkey(upper, parsed)).toBe(true);
  });
});

describe("isForeignTextEntry", () => {
  it("returns false for null", () => {
    expect(isForeignTextEntry(null, undefined)).toBe(false);
  });

  it("returns true for a textarea that is not the terminal's own", () => {
    const el = document.createElement("textarea");
    expect(isForeignTextEntry(el, undefined)).toBe(true);
  });

  it("returns false when the element IS the terminal's own textarea", () => {
    const el = document.createElement("textarea");
    expect(isForeignTextEntry(el, el)).toBe(false);
  });

  it("returns true for an input element", () => {
    const el = document.createElement("input");
    expect(isForeignTextEntry(el, undefined)).toBe(true);
  });

  it("returns true for a contentEditable element", () => {
    // jsdom doesn't compute isContentEditable from the attribute at all (a
    // known jsdom gap, unrelated to this function) — stub the getter
    // directly so this test exercises the logic branch, not jsdom's fidelity.
    const el = document.createElement("div");
    Object.defineProperty(el, "isContentEditable", { value: true });
    expect(isForeignTextEntry(el, undefined)).toBe(true);
  });

  it("returns false for a plain button or div", () => {
    expect(isForeignTextEntry(document.createElement("button"), undefined)).toBe(false);
    expect(isForeignTextEntry(document.createElement("div"), undefined)).toBe(false);
  });
});
