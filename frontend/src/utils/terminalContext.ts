import type { Terminal } from "@xterm/xterm";

/**
 * Reads up to maxLines from the xterm.js active buffer.
 * Returns empty string if terminal is null/undefined.
 * Trims trailing empty lines.
 *
 * `maxLines` is explicit because the two callers want genuinely different
 * things: the LLM context builder passes a number (it must bound the prompt),
 * while Copy Mode passes nothing and means "the whole buffer". A defaulted
 * `maxLines = 200` cannot express the second case — it would silently truncate
 * a snapshot whose whole point is to show everything the terminal retained.
 */
export function readTerminalLines(term: Terminal | undefined | null, maxLines?: number): string {
  if (!term) return "";
  // A Terminal that exists but hasn't finished initializing has no buffer yet
  // (xterm sets it during open()). Read it defensively: Copy Mode calls this from
  // a mount effect, so an unguarded `term.buffer.active` would throw inside the
  // effect and take the whole overlay down instead of showing an empty snapshot.
  const buf = term.buffer?.active;
  if (!buf) return "";
  // No cap requested: start at 0 and read the entire active buffer.
  const start = maxLines == null ? 0 : Math.max(0, buf.length - maxLines);
  const lines: string[] = [];
  for (let y = start; y < buf.length; y++) {
    const line = buf.getLine(y);
    if (line) lines.push(line.translateToString(true));
  }
  return lines.join("\n").trimEnd();
}
