// parsePorts parses the Network panel's "Ports" input: a comma/space
// separated list of port numbers and/or "a-b" ranges, e.g.
//   "22", "22,2222", "22241-22250", "22,22241-22250"
// Blank input yields [] (callers map that to the backend default [22], which
// mirrors the parseTargets empty=>[] convention). Invalid tokens are skipped
// (non-numeric, out of 1..65535, reversed/empty ranges); a fully-invalid
// input yields [] so a bad port string degrades to the default rather than
// erroring the scan. Duplicates are intentionally NOT collapsed here — the
// backend normalizes (dedupes) the port list, so the parser stays a pure
// tokenizer.
export const parsePorts = (raw: string): number[] => {
  const out: number[] = [];
  // Collapse spaces adjacent to a dash so a spaced range ("22240 - 22249")
  // parses as one range instead of separate "22240", "-", "22249" tokens.
  const src = raw.replace(/\s*-\s*/g, "-");
  for (const token of src.split(/[\s,]+/)) {
    if (!token) continue;
    if (token.includes("-")) {
      const parts = token.split("-");
      if (parts.length !== 2) continue; // lone "-", "a-b-c", etc. -> garbage
      const lo = Number(parts[0]);
      const hi = Number(parts[1]);
      if (!Number.isInteger(lo) || !Number.isInteger(hi)) continue;
      if (lo < 1 || hi > 65535 || lo > hi) continue;
      for (let p = lo; p <= hi; p++) out.push(p);
    } else {
      const p = Number(token);
      if (Number.isInteger(p) && p >= 1 && p <= 65535) out.push(p);
    }
  }
  return out;
};