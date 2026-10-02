import { describe, it, expect } from "vitest";
import { readTerminalLines } from "@/utils/terminalContext";
import type { Terminal } from "@xterm/xterm";

/** Fake xterm Terminal whose active buffer holds `lines`. */
function fakeTerm(lines: string[]) {
  return {
    buffer: {
      active: {
        length: lines.length,
        getLine: (y: number) =>
          y < lines.length ? { translateToString: () => lines[y] } : undefined,
      },
    },
  } as unknown as Terminal;
}

describe("readTerminalLines", () => {
  // Guards the Step 1 contract change. The two callers want opposite things: the
  // LLM context builder passes a cap; Copy Mode passes nothing and means "the
  // whole buffer". A defaulted `maxLines = 200` cannot express the second case.
  it("returns the FULL buffer when maxLines is omitted", () => {
    const many = Array.from({ length: 500 }, (_, i) => `row ${i}`);
    expect(readTerminalLines(fakeTerm(many))).toBe(many.join("\n"));
  });

  // Mutation check: reverting the signature to `maxLines = 200` makes the
  // no-argument call truncate to the last 200 rows and this fails.
  it("omitting the cap returns more than the old 200-line default", () => {
    const many = Array.from({ length: 500 }, (_, i) => `row ${i}`);
    expect(readTerminalLines(fakeTerm(many)).split("\n")).toHaveLength(500);
  });

  // Mutation check: removing the `maxLines == null ? 0 : ...` branch (i.e.
  // always computing `buf.length - maxLines`, which is NaN for undefined) yields
  // an empty string here.
  it("still honors an explicit cap", () => {
    const many = Array.from({ length: 500 }, (_, i) => `row ${i}`);
    expect(readTerminalLines(fakeTerm(many), 200).split("\n")).toHaveLength(200);
    expect(readTerminalLines(fakeTerm(many), 200).startsWith("row 300")).toBe(true);
    expect(readTerminalLines(fakeTerm(many), 200).endsWith("row 499")).toBe(true);
  });

  // Mutation check: dropping the Math.max(0, ...) clamp makes a cap LARGER than
  // the buffer start at a negative index.
  it("clamps a cap larger than the buffer to the whole buffer", () => {
    expect(readTerminalLines(fakeTerm(["a", "b", "c"]), 1000)).toBe("a\nb\nc");
  });

  it("returns an empty string for a null terminal regardless of the cap", () => {
    expect(readTerminalLines(null)).toBe("");
    expect(readTerminalLines(undefined, 50)).toBe("");
  });

  // A TUI on the alternate screen has a buffer shorter than the cap; Copy Mode
  // must show the whole screen rather than nothing.
  it("returns the whole short buffer when no cap is given", () => {
    expect(readTerminalLines(fakeTerm(["only", "two", "lines"]))).toBe("only\ntwo\nlines");
  });

  // Mutation check: reverting the `term.buffer?.active` guard to `term.buffer.active`
  // makes this throw. A Terminal that exists but has not finished open() has no
  // buffer, and Copy Mode calls this from a mount effect — so the unguarded form
  // took down the whole overlay rather than showing an empty snapshot.
  it("returns an empty string for a terminal that has not initialized a buffer", () => {
    const uninitialized = { buffer: undefined } as unknown as Terminal;
    expect(() => readTerminalLines(uninitialized)).not.toThrow();
    expect(readTerminalLines(uninitialized)).toBe("");
  });

});