import { useEffect, useState } from "react";
import { Dialog } from "@base-ui/react/dialog";

export interface EditCommandDialogProps {
  open: boolean;
  mode: "permanent" | "temporary" | "add";
  initialValue: string;
  onSave: (value: string) => void;
  onClose: () => void;
}

export function EditCommandDialog({
  open,
  mode,
  initialValue,
  onSave,
  onClose,
}: EditCommandDialogProps) {
  const [value, setValue] = useState(initialValue);

  // Re-seed from the latest command text every time the dialog opens, rather
  // than once on mount — the dialog stays mounted (just closed) between edits.
  useEffect(() => {
    if (open) setValue(initialValue);
  }, [open, initialValue]);

  const handleSave = () => {
    const trimmed = value.trim();
    if (!trimmed) return;
    onSave(trimmed);
  };

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[600px] max-h-[80vh] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-1 border border-surface-border-strong shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-surface-text border-b border-surface-border">
            {mode === "add"
              ? "Add Command"
              : mode === "permanent"
                ? "Edit Command"
                : "Edit/Append for Next Use"}
          </Dialog.Title>
          <div className="flex-1 overflow-y-auto p-6 space-y-2">
            <textarea
              autoFocus
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                  e.preventDefault();
                  handleSave();
                } else if (e.key === "Escape") {
                  e.preventDefault();
                  onClose();
                }
              }}
              rows={12}
              spellCheck={false}
              className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-2 text-sm font-mono text-surface-text focus:border-surface-text-muted focus:outline-none resize-y"
            />
            <p className="text-xs text-surface-text-muted">
              {mode === "temporary"
                ? "Applies once on next use — Ctrl+Enter to save, Esc to cancel"
                : "Ctrl+Enter to save, Esc to cancel"}
            </p>
          </div>
          <div className="flex items-center justify-end gap-2 px-6 py-3 border-t border-surface-border">
            <button
              onClick={onClose}
              className="bg-surface-2 hover:bg-surface-3 text-surface-text-muted text-xs px-4 py-1.5 rounded"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={!value.trim()}
              className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Save
            </button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
