import { useQuickSelect } from "@/hooks/useQuickSelect";

// Historical note: this component used to render the F-key labels itself in a
// fixed absolute column. Badges now render inline on their own rows instead —
// CommandCard / TerminalTab read their id's label from quickSelectStore — so
// this component is a deliberate render-nothing hook mount: it exists only so
// useQuickSelect's capture-phase keydown/keyup listeners (chord detection +
// F-key routing) stay registered while the layout is mounted. Delete this and
// the chord silently dies.
export function QuickSelectOverlay() {
  useQuickSelect();
  return null;
}
