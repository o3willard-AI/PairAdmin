import { useEffect, useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
import { wailsErrorMessage } from "@/utils/wailsError";

export interface ChangeMasterPasswordDialogProps {
  open: boolean;
  onClose: () => void;
}

const inputClass =
  "w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none";

// Shown from Settings → Security. Calls the backend ChangeMasterPassword,
// which verifies the current password before re-encrypting every stored
// item; backend errors (incorrect current password, etc.) surface verbatim.
export function ChangeMasterPasswordDialog({ open, onClose }: ChangeMasterPasswordDialogProps) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);

  useEffect(() => {
    if (open) {
      setCurrent("");
      setNext("");
      setConfirm("");
      setPending(false);
      setError("");
      setDone(false);
    }
  }, [open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (pending) return;
    setError("");

    if (!next) {
      setError("New master password must not be empty.");
      return;
    }
    if (next !== confirm) {
      setError("New passwords do not match.");
      return;
    }

    setPending(true);
    try {
      const { ChangeMasterPassword } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await ChangeMasterPassword(current, next);
      setPending(false);
      setDone(true);
    } catch (err) {
      setPending(false);
      setError(wailsErrorMessage(err, "Failed to change the master password"));
    }
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
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[420px] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-1 border border-surface-border-strong shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-surface-text border-b border-surface-border">
            Change Master Password
          </Dialog.Title>
          {done ? (
            <div className="p-6 space-y-4">
              <p className="text-sm text-surface-text">Master password changed.</p>
              <button
                className="w-full text-sm bg-surface-3 hover:bg-surface-3/80 text-surface-text px-3 py-2 rounded"
                onClick={onClose}
              >
                Close
              </button>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="p-6 space-y-4">
              <input
                autoFocus
                type="password"
                className={inputClass}
                placeholder="Current master password"
                aria-label="Current master password"
                value={current}
                onChange={(e) => setCurrent(e.target.value)}
                disabled={pending}
              />
              <input
                type="password"
                className={inputClass}
                placeholder="New master password"
                aria-label="New master password"
                value={next}
                onChange={(e) => setNext(e.target.value)}
                disabled={pending}
              />
              <input
                type="password"
                className={inputClass}
                placeholder="Confirm new master password"
                aria-label="Confirm new master password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                disabled={pending}
              />
              {error && <p className="text-xs text-red-400">{error}</p>}
              <button
                type="submit"
                className="w-full text-sm bg-surface-3 hover:bg-surface-3/80 text-surface-text px-3 py-2 rounded disabled:opacity-50"
                disabled={pending}
              >
                {pending ? "Re-encrypting…" : "Change Master Password"}
              </button>
            </form>
          )}
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
