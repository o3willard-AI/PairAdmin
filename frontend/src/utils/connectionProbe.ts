import { useSettingsStore } from "@/stores/settingsStore";

/**
 * Probe the configured LLM connection and write the result to the global
 * `connectionStatus`.
 *
 * This is the ONE probe: the app-mount effect and a successful Save in
 * Settings → LLM Config both call it, rather than each carrying its own copy.
 * They must agree, because the indicator is the only thing telling the user
 * whether their key works — a Save that leaves a stale "Disconnected" showing
 * is what made a working key look broken.
 *
 * Rules, all load-bearing:
 *
 * - **"Disable Pair LLM" always wins.** The probe writes its result only while
 *   the status is still `"checking"`. If the user opts out mid-flight, the
 *   status has moved to `"disabled"` and this stale result is discarded rather
 *   than flipping a deliberate opt-out back to connected.
 * - **No provider configured → disconnected**, without a network call.
 * - **The probe tests the STORED key.** It passes an empty key, so the backend
 *   falls back to the keychain. This must never be changed to forward a typed
 *   key: an unsaved candidate key is not the active configuration, and this
 *   probe reports on the active configuration.
 *
 * Never throws. A Wails runtime is absent in tests and dev, so the whole body
 * is guarded and a failure leaves the status as it was.
 */
export async function probeLLMConnection(): Promise<void> {
  try {
    const { GetSettings, TestConnection } = await import(
      /* @vite-ignore */ "../../wailsjs/go/services/SettingsService"
    );

    const cfg = await GetSettings();
    const provider = cfg?.Provider;
    const model = cfg?.Model;

    const { setActiveModel, setConnectionStatus } = useSettingsStore.getState();

    if (provider && model) {
      setActiveModel(`${provider}:${model}`);
    }

    if (!provider) {
      setConnectionStatus("disconnected");
      return;
    }

    // "Disable Pair LLM" is an explicit opt-out: never probe, never show
    // Connected/Disconnected — show Disabled and surface it in the chat input.
    if (provider === "disabled") {
      setConnectionStatus("disabled");
      return;
    }

    // Claim the indicator before the network call so the guard below can tell
    // "my result is still the newest authority" from "something moved on".
    setConnectionStatus("checking");

    try {
      await TestConnection(provider, model ?? "", "", "");
      if (useSettingsStore.getState().connectionStatus === "checking") {
        setConnectionStatus("connected");
      }
    } catch {
      if (useSettingsStore.getState().connectionStatus === "checking") {
        setConnectionStatus("disconnected");
      }
    }
  } catch {
    // Wails runtime unavailable (tests / dev) — leave the status untouched.
  }
}
