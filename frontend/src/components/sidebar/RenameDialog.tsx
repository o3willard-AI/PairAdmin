import { useState, useEffect } from "react";
import { Dialog } from "@base-ui/react/dialog";

export interface RenameDialogProps {
  open: boolean;
  initialValue?: string;
  onSave: (name: string) => void;
  onClose: () => void;
}

export function RenameDialog({
  open,
  initialValue,
  onSave,
  onClose,
}: RenameDialogProps) {
  const [name, setName] = useState(initialValue ?? "");

  useEffect(() => {
    if (open) setName(initialValue ?? "");
  }, [open, initialValue]);

  const handleSave = () => {
    onSave(name.trim());
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
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[400px] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-1 border border-surface-border-strong shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-surface-text border-b border-surface-border">
            Rename Command
          </Dialog.Title>
          <div className="p-6 space-y-2">
            <label
              htmlFor="rename-input"
              className="text-xs font-medium text-surface-text-muted"
            >
              Name
            </label>
            <input
              id="rename-input"
              type="text"
              placeholder="e.g. Restart nginx"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  handleSave();
                } else if (e.key === "Escape") {
                  e.preventDefault();
                  onClose();
                }
              }}
              autoFocus
              className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm font-mono text-surface-text focus:border-surface-text-muted focus:outline-none"
            />
            <p className="text-xs text-surface-text-muted">
              Leave empty to clear the custom name. Press Enter to save, Esc to cancel.
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
              className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded"
            >
              Save
            </button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
