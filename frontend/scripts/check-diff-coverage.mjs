#!/usr/bin/env node
/**
 * Diff-coverage gate (R-18).
 *
 * HARD-FAILS (exit 1) when a pull request's CHANGED feature lines fall below
 * the 80% coverage floor. This is deliberately NOT a vanity total-coverage
 * metric: it intersects the changed files + line ranges from `git diff` with
 * the per-line statement coverage Vitest v8 emitted to coverage-final.json.
 *
 * Usage (from frontend/):
 *   node scripts/check-diff-coverage.mjs <base-sha>
 *
 * Env:
 *   COVERAGE_THRESHOLD  minimum % of changed feature lines that must be
 *                       covered for the gate to pass (default: 80).
 *   COVERAGE_JSON       path to the v8 report (default: coverage/coverage-final.json).
 *
 * Details that matter:
 *   - Only lines that exist in the coverage report as *statements* count in
 *     the denominator. Blank/comment/JSX-only lines aren't statements, so a
 *     change that merely touches formatting isn't penalized.
 *   - Only `.ts` / `.tsx` source files are counted (test files, config, and
 *     coverage-excluded paths cannot be "uncovered" and would otherwise skew
 *     the gate toward false failures).
 *   - Timers/act-warnings don't reach here; this is purely a line-coverage
 *     intersection, which is deterministic given a green `vitest --coverage`.
 */
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const FRONTEND_DIR = process.cwd();

function fail(msg) {
  console.error(`\u2717 diff-coverage gate: ${msg}`);
  process.exit(1);
}

function die(err) {
  console.error(`error: ${err?.message ?? err}`);
  process.exit(2);
}

// ---- 1. Resolve config -----------------------------------------------------
let baseSha;
try {
  baseSha = process.argv[2];
} catch {
  /* noop */
}
if (!baseSha) fail("missing <base-sha> argument");

const thresholdPct = Number(process.env.COVERAGE_THRESHOLD ?? 80);
const coverageJson = resolve(
  process.env.COVERAGE_JSON ?? resolve(FRONTEND_DIR, "coverage/coverage-final.json")
);

// ---- 2. Read the changed file + line ranges -------------------------------
// diff --unified=0 gives per-hunk start/added-count, which we expand into
// concrete changed line numbers.
let diffOut;
try {
  diffOut = execFileSync("git", ["diff", "--relative", "--unified=0", baseSha, "HEAD"], {
    cwd: FRONTEND_DIR,
    encoding: "utf8",
  });
} catch (e) {
  die(`failed to run git diff against ${baseSha}: ${e?.stderr ?? e}`);
}

/** file -> Set<line> */
const changedLines = new Map();
const DIFF_HUNK = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;
let currentFile = null;
for (const line of diffOut.split("\n")) {
  if (line.startsWith("+++ b/")) {
    currentFile = line.slice(6);
    if (!changedLines.has(currentFile)) changedLines.set(currentFile, new Set());
    continue;
  }
  if (line.startsWith("--- ")) {
    continue;
  }
  const m = DIFF_HUNK.exec(line);
  if (m && currentFile) {
    // New-file side: start at +newStart for +count added lines.
    const newStart = Number(m[3]);
    const newCount = m[4] ? Number(m[4]) : 1;
    const set = changedLines.get(currentFile);
    for (let i = 0; i < newCount; i++) set.add(newStart + i);
  }
}

const changedFiles = [...changedLines.keys()];
if (changedFiles.length === 0) {
  console.log("\u2713 diff-coverage gate: no source files changed vs " + baseSha);
  process.exit(0);
}

// ---- 3. Read coverage report ----------------------------------------------
let coverage;
try {
  coverage = JSON.parse(readFileSync(coverageJson, "utf8"));
} catch {
  fail(`could not read coverage report at ${coverageJson} — did the coverage step run?`);
}

// ---- 4. Intersect ----------------------------------------------------------
const ALLOWED_EXT = /\.tsx?$/;
const EXCLUDED_PATH = /(?:^|\/)__tests__\/|\.test\.tsx?$|\.spec\.tsx?$/;

let changedFeatureLines = 0;
let coveredChangedLines = 0;
const misses = [];

for (const file of changedFiles) {
  if (!ALLOWED_EXT.test(file) || EXCLUDED_PATH.test(file)) continue;
  if (!file.startsWith("src/")) continue; // gate only the app's feature code

  // Coverage report keys are absolute paths (e.g.
  // /.../frontend/src/settings/AppearanceTab.tsx); git diff paths are
  // repo-relative (src/settings/AppearanceTab.tsx). Resolve the diff path
  // against the frontend dir to hit the same key.
  const abs = resolve(FRONTEND_DIR, file);
  const entry = coverage[abs] ?? coverage[abs.replace(/^\//, "")];
  if (!entry) {
    // A changed source file with no coverage entry at all is a hard miss:
    // every changed line is uncovered.
    const lines = changedLines.get(file);
    changedFeatureLines += lines.size;
    coveredChangedLines += 0;
    misses.push([file, [...lines].sort((a, b) => a - b)]);
    continue;
  }

  const statementHits = entry.s;
  const statementMap = entry.statementMap;
  const statementLineHits = new Map(); // line -> boolean (coverage reached)
  for (const [idx, stmt] of Object.entries(statementMap)) {
    const hits = statementHits[idx] ?? 0;
    for (let ln = stmt.start.line; ln <= stmt.end.line; ln++) {
      statementLineHits.set(ln, statementLineHits.get(ln) || hits > 0);
    }
  }

  const fileChanged = [...changedLines.get(file)].filter((ln) =>
    statementLineHits.has(ln)
  );
  const fileCovered = fileChanged.filter((ln) => statementLineHits.get(ln));
  changedFeatureLines += fileChanged.length;
  coveredChangedLines += fileCovered.length;
  const fileMisses = fileChanged.filter((ln) => !statementLineHits.get(ln));
  if (fileMisses.length) misses.push([file, fileMisses]);
}

const pct = changedFeatureLines
  ? ((coveredChangedLines / changedFeatureLines) * 100).toFixed(1)
  : "100.0";

console.log(
  `diff-coverage: ${coveredChangedLines}/${changedFeatureLines} changed feature lines covered (${pct}%, floor ${thresholdPct}%)`
);
if (misses.length) {
  console.error("uncovered changed lines:");
  for (const [file, lines] of misses) {
    console.error(`  ${file}: ${lines.join(", ")}`);
  }
}

const ok = changedFeatureLines === 0 || Number(pct) >= thresholdPct;
if (!ok) {
  fail(
    `changed-line coverage ${pct}% is below the ${thresholdPct}% floor — add tests for the lines above`
  );
}
console.log(`\u2713 diff-coverage gate passed (>=${thresholdPct}%)`);