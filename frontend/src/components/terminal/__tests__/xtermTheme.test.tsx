import { describe, it, expect } from "vitest";
import { LIGHT_XTERM_THEME, DARK_XTERM_THEME } from "@/components/terminal/TerminalPreview";

// The 16 ANSI color slots xterm.js accepts on an ITheme. If any slot is
// removed from either theme object, the "defines all 16 ANSI slots" test
// below fails — the fix is reverted.
const ANSI_SLOTS = [
  "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
  "brightBlack", "brightRed", "brightGreen", "brightYellow",
  "brightBlue", "brightMagenta", "brightCyan", "brightWhite",
] as const;

// --- WCAG 2.1 luminance / contrast helpers ---

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  const n = parseInt(h, 16);
  return [(n >> 16) & 0xff, (n >> 8) & 0xff, n & 0xff];
}

function relativeLuminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex);
  const lin = (c: number) => {
    const v = c / 255;
    return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrastRatio(l1: number, l2: number): number {
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

// Mutation check: if the light theme reverts to xterm.js's default palette
// (by removing brightGreen/brightYellow/brightCyan slots), this test fails —
// those colors would either be missing or near-#ffffff.
describe("light xterm theme", () => {
  it("defines all 16 ANSI slots (no reliance on xterm defaults)", () => {
    for (const slot of ANSI_SLOTS) {
      expect(LIGHT_XTERM_THEME[slot], `missing slot: ${slot}`).toBeDefined();
      expect(LIGHT_XTERM_THEME[slot]).toMatch(/^#[0-9a-f]{6}$/);
    }
  });

  // The 4 colors the UAT report called out: they must be strictly darker
  // than #ffffff so ANSI "bright" text is not invisible on the white bg.
  // "Darker than white" here means L < 0.95 — anything above that is
  // near-white and would be the exact bug being fixed.
  it("white, brightWhite, yellow, and brightYellow are strictly darker than #ffffff", () => {
    for (const slot of ["white", "brightWhite", "yellow", "brightYellow"]) {
      const L = relativeLuminance(LIGHT_XTERM_THEME[slot]);
      expect(L).toBeLessThan(0.95);
    }
  });

  it("every ANSI color meets WCAG AA (>= 4.5:1) against the white background", () => {
    const bgL = relativeLuminance("#ffffff");
    for (const slot of ANSI_SLOTS) {
      const L = relativeLuminance(LIGHT_XTERM_THEME[slot]);
      const ratio = contrastRatio(bgL, L);
      expect(ratio).toBeGreaterThanOrEqual(4.5);
    }
  });
});

// xterm.js Tango defaults — the dark theme is pinned verbatim to these.
// ANSI black is *supposed* to be dark; brightening it to hit a WCAG bar on a
// dark background breaks \e[30m dimmed text and \e[40m backgrounds.
const TANGO_DARK: Record<string, string> = {
  black: "#2e3436",
  red: "#cc0000",
  green: "#4e9a06",
  yellow: "#c4a000",
  blue: "#3465a4",
  magenta: "#75507b",
  cyan: "#06989a",
  white: "#d3d7cf",
  brightBlack: "#555753",
  brightRed: "#ef2929",
  brightGreen: "#8ae234",
  brightYellow: "#fce94f",
  brightBlue: "#729fcf",
  brightMagenta: "#ad7fa8",
  brightCyan: "#34e2e2",
  brightWhite: "#eeeeec",
} as const;

// Mutation check: if any dark-theme ANSI slot drifts from the pinned Tango
// default, this test fails. This guards against accidentally re-brightening
// black (the original PR #76 mistake) or swapping to a Dracula palette.
describe("dark xterm theme", () => {
  it("defines all 16 ANSI slots as xterm.js Tango defaults", () => {
    expect(DARK_XTERM_THEME.background).toBe("#0d0d0d");
    expect(DARK_XTERM_THEME.foreground).toBe("#d4d4d4");
    expect(DARK_XTERM_THEME.cursor).toBe("#d4d4d4");
    for (const slot of ANSI_SLOTS) {
      expect(DARK_XTERM_THEME[slot], `dark slot ${slot} drifted from Tango default`).toBe(TANGO_DARK[slot]);
    }
  });
});
