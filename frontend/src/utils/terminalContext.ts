import type { Terminal } from "@xterm/xterm";

/**
 * Reads up to maxLines from the xterm.js active buffer.
 * Returns empty string if terminal is null/undefined.
 * Trims trailing empty lines.
 */
export function readTerminalLines(term: Terminal | undefined | null, maxLines = 200): string {
  if (!term) return "";
  const buf = term.buffer.active;
  const start = Math.max(0, buf.length - maxLines);
  const lines: string[] = [];
  for (let y = start; y < buf.length; y++) {
    const line = buf.getLine(y);
    if (line) lines.push(line.translateToString(true));
  }
  return lines.join("\n").trimEnd();
}
