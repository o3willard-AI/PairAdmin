import { useEffect, useState } from "react";
import { ChangeMasterPasswordDialog } from "./ChangeMasterPasswordDialog";

const buttonClass =
  "text-sm bg-surface-3 hover:bg-surface-3/80 text-surface-text px-3 py-1.5 rounded disabled:opacity-50";

// Free-tier assurance suffix shown next to the resolved account. Always L0
// (declared, unverified) in the free build — never dynamic.
const RAE_L0_SUFFIX = " - declared, unverified (L0)";

// Security tab: master-password management. The "change" flow only applies
// when the encrypted file backend is actually in use (a master password has
// been configured); with a functional OS keychain there is nothing to change,
// so the section renders its disabled/absent state instead.
export function SecurityTab() {
  const [checking, setChecking] = useState(true);
  const [hasMasterPassword, setHasMasterPassword] = useState(false);
  const [changeOpen, setChangeOpen] = useState(false);
  const [raeChecking, setRaeChecking] = useState(true);
  const [raeUsername, setRaeUsername] = useState("");

  // Load the keychain status and resolve the Registered Accountable Entity
  // (the logged-in OS account) in a single binding load. RAE is informational
  // only in the free build — on failure we degrade to an "unknown" caption
  // rather than blanking or crashing the tab.
  useEffect(() => {
    let cancelled = false;
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ HasMasterPassword, GetCurrentUsername }) =>
        Promise.all([HasMasterPassword(), GetCurrentUsername()])
      )
      .then(([has, name]) => {
        if (cancelled) return;
        setHasMasterPassword(!!has);
        setRaeUsername(typeof name === "string" ? name : "");
      })
      .catch(() => {
        if (cancelled) return;
        setHasMasterPassword(false);
        setRaeUsername("");
      })
      .finally(() => {
        if (cancelled) return;
        setChecking(false);
        setRaeChecking(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="p-6 space-y-4">
      <div>
        <h3 className="text-sm font-medium text-surface-text">
          {raeChecking
            ? "Registered Accountable Entity (RAE): resolving…"
            : raeUsername === ""
              ? "Registered Accountable Entity (RAE): unknown"
              : `Registered Accountable Entity (RAE): ${raeUsername}${RAE_L0_SUFFIX}`}
        </h3>
        <p className="text-xs text-surface-text-muted mt-1">
          The OS account declared as accountable for agent-assisted actions — declared, unverified (L0).
        </p>
      </div>
      <div>
        <h3 className="text-sm font-medium text-surface-text">Master password</h3>
        <p className="text-xs text-surface-text-muted mt-1">
          {checking
            ? "Checking keychain status…"
            : hasMasterPassword
              ? "Stored credentials are encrypted on disk and unlocked with your master password."
              : "No master password configured — this is expected on Windows and macOS, where credentials are protected by the OS keychain (Windows Credential Manager / macOS Keychain). A master password is only used when no OS keychain is available (typically on Linux)."}
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
