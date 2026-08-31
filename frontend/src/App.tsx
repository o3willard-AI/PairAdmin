import { useCallback, useEffect, useState } from "react";
import { ThreeColumnLayout } from "@/components/layout/ThreeColumnLayout";
import { ChatPane } from "@/components/chat/ChatPane";
import { CommandSidebar } from "@/components/sidebar/CommandSidebar";
import { MasterPasswordDialog } from "@/components/settings/MasterPasswordDialog";

// Startup gate states:
//   checking — asking the backend whether a master password is required
//   password — gate active: the master password dialog is on screen
//   ready    — gate passed, API keys loaded (or their load failed but was
//              logged — the app must not be blocked by that)
type StartupGate = "checking" | "password" | "ready";

function App() {
  const [gate, setGate] = useState<StartupGate>("checking");
  const [gateMode, setGateMode] = useState<"set" | "unlock">("unlock");

  // Load stored API keys and mark the gate passed. LoadAPIKeys failures are
  // logged but never block the app — the user can still fix keys in Settings.
  const loadAndProceed = useCallback(async () => {
    try {
      const { LoadAPIKeys } = await import(
        /* @vite-ignore */ "../wailsjs/go/services/SettingsService"
      );
      await LoadAPIKeys();
    } catch (err) {
      console.warn("Loading stored API keys failed; continuing without them", err);
    }
    setGate("ready");
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const { NeedsMasterPassword, HasMasterPassword } = await import(
          /* @vite-ignore */ "../wailsjs/go/services/SettingsService"
        );
        const needs = await NeedsMasterPassword();
        if (cancelled) return;
        if (!needs) {
          // Functional OS keychain — no master password involved.
          await loadAndProceed();
          return;
        }
        const has = await HasMasterPassword();
        if (cancelled) return;
        setGateMode(has ? "unlock" : "set");
        setGate("password");
      } catch (err) {
        // Binding/transport failure (e.g. dev outside wails): never wedge the
        // app behind a dialog it cannot resolve.
        console.warn("Master-password startup gate failed; continuing without it", err);
        if (!cancelled) setGate("ready");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [loadAndProceed]);

  return (
    <>
      <ThreeColumnLayout sidebar={<CommandSidebar />}>
        <ChatPane />
      </ThreeColumnLayout>
      {gate === "password" && (
        <MasterPasswordDialog open mode={gateMode} onSuccess={() => void loadAndProceed()} />
      )}
    </>
  );
}

export default App;
