import { useEffect, useState } from "react";
import { ChangeMasterPasswordDialog } from "./ChangeMasterPasswordDialog";

const buttonClass =
  "text-sm bg-surface-3 hover:bg-surface-3/80 text-surface-text px-3 py-1.5 rounded disabled:opacity-50";

// Security tab: master-password management. The "change" flow only applies
// when the encrypted file backend is actually in use (a master password has
// been configured); with a functional OS keychain there is nothing to change,
// so the section renders its disabled/absent state instead.
export function SecurityTab() {
  const [checking, setChecking] = useState(true);
  const [hasMasterPassword, setHasMasterPassword] = useState(false);
  const [changeOpen, setChangeOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ HasMasterPassword }) => HasMasterPassword())
      .then((has) => {
        if (!cancelled) setHasMasterPassword(!!has);
      })
      .catch(() => {
        if (!cancelled) setHasMasterPassword(false);
      })
      .finally(() => {
        if (!cancelled) setChecking(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="p-6 space-y-4">
      <div>
        <h3 className="text-sm font-medium text-surface-text">Master password</h3>
        <p className="text-xs text-surface-text-muted mt-1">
          {checking
            ? "Checking keychain status…"
            : hasMasterPassword
              ? "Stored credentials are encrypted on disk and unlocked with your master password."
              : "No master password configured — credentials are stored in the OS keychain."}
        </p>
      </div>
      {!checking && hasMasterPassword && (
        <button className={buttonClass} onClick={() => setChangeOpen(true)}>
          Change master password
        </button>
      )}
      {!checking && hasMasterPassword && (
        <ChangeMasterPasswordDialog open={changeOpen} onClose={() => setChangeOpen(false)} />
      )}
    </div>
  );
}
