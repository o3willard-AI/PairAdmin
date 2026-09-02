import { useQuickSelect } from "@/hooks/useQuickSelect";

// Floating F-key labels for the quick-select chord (Ctrl+Alt/Meta + F1..F12).
// The hook (E1) owns all chord detection and F-key routing; this component
// only renders the current { visible, items } state.
//
// Keep-mounted fade: the layer always renders and toggles `opacity` via a CSS
// transition (same "keep mounted, toggle visibility" pattern as
// ThreeColumnLayout's collapsible chat section) — unmounting/remounting per
// chord press would make the fade jittery and re-run the hook's listeners.
//
// pointer-events-none: the overlay is pure signage. It must never intercept a
// click meant for a sidebar row underneath, never become the document's
// focused element mid-chord, and never steal focus from xterm's textarea
// (focus changes mid-chord would trip the hook's isForeignTextEntry guard on
// the very next keypress).
export function QuickSelectOverlay() {
  const { visible, items } = useQuickSelect();

  const commands = items.filter((item) => item.kind === "command");
  const terminals = items.filter((item) => item.kind === "terminal");

  return (
    <div
      aria-hidden="true"
      className={`fixed inset-0 z-40 pointer-events-none transition-opacity duration-150 ease-out ${
        visible ? "opacity-100" : "opacity-0"
      }`}
    >
      {/* Terminals → left edge (over the terminal sidebar's tab rows) */}
      <div
        data-testid="quick-select-terminals"
        className="absolute left-2 top-9 flex flex-col gap-1 items-start"
      >
        {terminals.map((item) => (
          <QuickSelectBadge key={item.id} label={item.label} kind={item.kind} />
        ))}
      </div>

      {/* Pinned commands → right edge (over the command sidebar's cards) */}
      <div
        data-testid="quick-select-commands"
        className="absolute right-2 top-9 flex flex-col gap-1 items-end"
      >
        {commands.map((item) => (
          <QuickSelectBadge key={item.id} label={item.label} kind={item.kind} />
        ))}
      </div>
    </div>
  );
}

function QuickSelectBadge({ label, kind }: { label: string; kind: "command" | "terminal" }) {
  return (
    <span
      className={`px-1.5 py-0.5 rounded text-[10px] font-mono font-semibold leading-none shadow-sm border ${
        kind === "command"
          ? "bg-surface-3/90 text-surface-text border-surface-border-strong"
          : "bg-surface-1/90 text-surface-text-muted border-surface-border"
      }`}
    >
      {label}
    </span>
  );
}
