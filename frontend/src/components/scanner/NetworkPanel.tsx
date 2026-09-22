import { useEffect, useState } from "react";
import { useScannerStore, type HostRow } from "@/stores/scannerStore";
import { useScanner, startScan, stopScan } from "@/hooks/useScanner";
import { NewTerminalDialog } from "@/components/terminal/NewTerminalDialog";
import { parsePorts } from "@/components/scanner/ports";

const inputClass =
  "w-full bg-surface-2 border border-surface-border-strong rounded px-2 py-1 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none";

const focusRingClass =
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-surface-1";

// Mode-A host states, colored by disposition: SSH is a real find (green);
// filtered is inconclusive (amber); closed is a definite negative (muted).
const stateClass = (state: HostRow["state"]) =>
  state === "ssh" || state === "ssh-known"
    ? "text-green-400"
    : state === "filtered"
      ? "text-amber-400"
      : state === "closed"
        ? "text-surface-text-muted"
        : "text-surface-text";

// Splits the target input into individual CIDRs/hosts. Empty input means
// "scan the local /24 networks" (the backend treats an empty targets array
// exactly that way), so an empty string maps to an empty array.
const parseTargets = (raw: string): string[] => {
  const t = raw.trim();
  if (!t) return [];
  return t.split(/[\s,]+/).filter(Boolean);
};

export function NetworkPanel() {
  useScanner(); // Subscribe to scan:* events for the life of this panel.

  const status = useScannerStore((s) => s.status);
  const rows = useScannerStore((s) => s.rows);
  const progress = useScannerStore((s) => s.progress);
  const stats = useScannerStore((s) => s.stats);
  const error = useScannerStore((s) => s.error);

  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [expanded, setExpanded] = useState(true);
  const [target, setTarget] = useState("");
  const [portsRaw, setPortsRaw] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogInitial, setDialogInitial] = useState<
    { kind: "ssh"; host: string; port: number } | undefined
  >(undefined);

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      // scanner_enabled defaults to false in services/config/config.go — a fresh
      // install opts OUT until the user enables it in Settings → Terminals →
      // "Network scanner". Treat an absent value as disabled (fail-closed).
      .then((cfg) => setEnabled(!!cfg?.ScannerEnabled))
      .catch(() => setEnabled(false));
  }, []);

  // Disabled in Settings → "Network scanner" off. HIDE the panel AND its entry
  // point entirely rather than rendering a disabled shell.
  if (enabled === null) return null; // still loading — avoid a flash
  if (!enabled) return null;

  const scanning = status === "scanning";

  const handleAddToHosts = (row: HostRow) => {
    // Route the discovered host through the EXISTING NewTerminalDialog trust
    // flow. Only the initial form values are steered (host/port/kind); the
    // user still completes the form and Connect goes through
    // maybeConnectToRemote -> CheckHostKeyTrust — never auto-trusted here.
    setDialogInitial({ kind: "ssh", host: row.ip, port: row.ssh?.port ?? 22 });
    setDialogOpen(true);
  };

  return (
    <div className="border-t border-surface-border">
      <button
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        className={`w-full px-3 py-1.5 flex items-center gap-1.5 text-xs font-semibold text-surface-text-muted uppercase tracking-wider hover:text-surface-text ${focusRingClass}`}
      >
        <span className="inline-block w-3 text-center">{expanded ? "▾" : "▸"}</span>
        Network
      </button>

      {expanded && (
        <div className="px-2 pt-0.5 space-y-2">
          {/* Stacked scan inputs: targets get their own full-width line so the
              field has a usable typing area in the narrow sidebar; ports + Scan
              share the second line. (Was a single horizontal row that left the
              targets input almost no width.) */}
          <div className="flex flex-col gap-1.5">
            <input
              className={inputClass}
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              placeholder="Targets (blank = local /24s)"
              aria-label="Scan target CIDR(s)"
            />
            <div className="flex items-center gap-1.5">
              <input
                className="min-w-0 flex-1 bg-surface-2 border border-surface-border-strong rounded px-2 py-1 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none"
                value={portsRaw}
                onChange={(e) => setPortsRaw(e.target.value)}
                placeholder="Ports (22)"
                aria-label="SSH ports to scan"
                title="Comma/space-separated ports and ranges, e.g. 22,2222 or 22241-22250. Blank = default (22)."
              />
              <button
                onClick={() => startScan(parseTargets(target), parsePorts(portsRaw), 16)}
                disabled={scanning}
                className={`bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-3 py-1.5 rounded disabled:opacity-50 ${focusRingClass}`}
              >
                {scanning ? "Scanning…" : "Scan"}
              </button>
            </div>
          </div>

          {scanning && (
            <div className="flex items-center gap-1.5 text-xs">
              <span className="text-surface-text-muted">
                {progress.done}/{progress.total}
                {progress.total > 0 ? ` scanned` : ` scanning…`}
              </span>
              <button
                onClick={() => stopScan()}
                className="text-red-400 hover:text-red-400/80 text-xs px-2 py-0.5 rounded border border-surface-border"
              >
                Stop
              </button>
            </div>
          )}

          {status === "done" && stats && (
            <p className="text-xs text-surface-text-muted">
              {stats.total} hosts · {stats.ssh} ssh · {stats.filtered} filtered · {stats.closed} closed · {stats.durationMs}ms
            </p>
          )}
          {status === "error" && error && (
            <p role="alert" className="text-xs text-red-400">
              {error}
            </p>
          )}

          {rows.length > 0 && (
            // Cap the results list so a large sweep can't grow the panel tall
            // and push "+ Connect" off the bottom of the overflow-hidden aside
            // (SCAN-T8). ~6 rows ≈ 256px; for a handful of results this is a
            // no-op — no scrollbar appears.
            <div className="space-y-1 max-h-64 overflow-y-auto">
              {rows.map((row) => (
                <div
                  key={row.ip}
                  className="rounded border border-surface-border px-2 py-1"
                >
                  <div className="flex items-center justify-between gap-1.5">
                    <div className="text-xs flex items-center gap-1.5 min-w-0 truncate">
                      <span className="font-mono text-surface-text">{row.ip}</span>
                      <span className={stateClass(row.state)}>{row.state}</span>
                    </div>
                    {(row.state === "ssh" || row.state === "ssh-known") && (
                      <button
                        onClick={() => handleAddToHosts(row)}
                        className={`text-xs px-2 py-0.5 rounded border border-surface-border hover:bg-surface-2 text-surface-text ${focusRingClass}`}
                      >
                        Add to hosts
                      </button>
                    )}
                  </div>
                  {row.ssh && (
                    <div className="text-xs text-surface-text-muted truncate">
                      {row.ssh.software
                        ? `${row.ssh.software} · ${row.ssh.port}`
                        : row.ssh.banner
                          ? row.ssh.banner
                          : `Port ${row.ssh.port}`}
                    </div>
                  )}
                  {!row.ssh && (
                    <div className="text-xs text-surface-text-muted truncate">Port {row.port}</div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <NewTerminalDialog
        open={dialogOpen}
        initial={dialogInitial}
        onClose={() => setDialogOpen(false)}
      />
    </div>
  );
}