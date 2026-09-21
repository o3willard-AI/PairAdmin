import { useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
import { wailsErrorMessage } from "@/utils/wailsError";

export interface MasterPasswordDialogProps {
  /** true while the startup gate is showing this dialog. */
  open: boolean;
  /** "set" = first run (create + confirm); "unlock" = subsequent launch. */
  mode: "set" | "unlock";
  /** Called after the password was set or verified successfully. */
  onSuccess: () => void;
  /**
   * Re-run the OS keychain probe (NeedsMasterPassword already re-probes fresh
   * on every call, so this is just a re-invocation). Return `true` when the OS
   * keychain is STILL unavailable (dialog stays open); return `false`/`undefined`
   * when the keychain is now available (the parent proceeds and dismisses this
   * dialog).
   */
  onRetryOSKeychain?: () => Promise<boolean> | boolean | void;
}

const inputClass =
  "w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none";

// Non-dismissable by design: there is no usable app behind this dialog until
// the master password gate passes, so no onOpenChange handler is wired up
// (backdrop click / Escape are no-ops) and there is no close button. The
// "Retry OS Keychain" button is the only additional escape: it re-probes the
// OS keychain and, if one is now available, the parent closes the dialog.
export function MasterPasswordDialog({
  open,
  mode,
  onSuccess,
  onRetryOSKeychain = () => {},
}: MasterPasswordDialogProps) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [retrying, setRetrying] = useState(false);
  const [retryMessage, setRetryMessage] = useState("");

  const isSet = mode === "set";

  const reset = () => {
    setPassword("");
    setConfirm("");
    setPending(false);
    setError("");
    setRetrying(false);
    setRetryMessage("");
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (pending) return;
    setError("");

    if (!password) {
      setError("Master password must not be empty.");
      return;
    }
    if (isSet && password !== confirm) {
      setError("Passwords do not match.");
      return;
    }

    setPending(true);
    try {
      const { SetMasterPassword, VerifyMasterPassword } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      if (isSet) {
        await SetMasterPassword(password);
      } else {
        const ok = await VerifyMasterPassword(password);
        if (!ok) {
          setError("Incorrect master password.");
          setPending(false);
          return;
        }
      }
      reset();
      onSuccess();
    } catch (err) {
      setPending(false);
      setError(wailsErrorMessage(err, "Failed to set the master password"));
    }
  };

  const handleRetryOSKeychain = async () => {
    if (retrying || pending) return;
    setRetrying(true);
    // The button label flips to "Checking OS keychain…" while running; only
    // the outcome is reported here.
    setRetryMessage("");
    try {
      const stillNeedsPassword = await onRetryOSKeychain();
      // false/undefined means the parent found an OS keychain and is dismissing
      // this dialog (open -> false); true means still unavailable, stay open.
      if (stillNeedsPassword) {
        setRetryMessage(
          "No OS keychain detected — set or unlock a master password, or retry.",
        );
      }
    } catch {
      // A failed re-check is treated as "still needs the password", not a crash.
      setRetryMessage("Could not re-check the OS keychain — set or unlock a master password, or retry.");
    } finally {
      setRetrying(false);
    }
  };

  return (
    <Dialog.Root open={open}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[420px] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-1 border border-surface-border-strong shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-surface-text border-b border-surface-border">
            {isSet ? "Set Master Password" : "Unlock PairAdmin"}
          </Dialog.Title>
          <form onSubmit={handleSubmit} className="p-6 space-y-4">
            <p className="text-xs text-surface-text-muted">
              {isSet
                ? "No OS keychain is available, so PairAdmin encrypts stored credentials on disk. Choose a master password — you will need it at every launch."
                : "Enter your master password to unlock stored credentials."}
            </p>
            <input
              autoFocus
              type="password"
              className={inputClass}
              placeholder="Master password"
              aria-label="Master password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={pending}
            />
            {isSet && (
              <input
                type="password"
                className={inputClass}
                placeholder="Confirm master password"
                aria-label="Confirm master password"
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
                disabled={pending}
              />
            )}
            {error && <p className="text-xs text-red-400">{error}</p>}
            <button
              type="submit"
              className="w-full text-sm bg-surface-3 hover:bg-surface-3/80 text-surface-text px-3 py-2 rounded disabled:opacity-50"
              disabled={pending}
            >
              {pending ? "Working…" : isSet ? "Create Master Password" : "Unlock"}
            </button>
          </form>
          <div className="px-6 pb-6 -mt-2 space-y-2">
            {retryMessage && <p className="text-xs text-surface-text-muted">{retryMessage}</p>}
            <button
              type="button"
              className="w-full text-sm text-surface-text-muted hover:text-surface-text bg-transparent border border-surface-border-strong rounded px-3 py-2 disabled:opacity-50"
              disabled={retrying || pending}
              onClick={() => void handleRetryOSKeychain()}
            >
              {retrying ? "Checking OS keychain…" : "Retry OS Keychain"}
            </button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}