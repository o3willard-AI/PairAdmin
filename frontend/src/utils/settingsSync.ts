import type { config } from "../../wailsjs/go/models";

// SaveSettings (Go) writes the entire AppConfig object it receives to disk — it does
// NOT merge onto the existing file. Every Settings-dialog tab manages only a handful
// of fields (e.g. LLMConfigTab manages Provider/Model/OllamaHost/LMStudioHost), so
// calling SaveSettings with just those fields would silently zero out every other
// persisted setting (theme, hotkeys, custom prompt, terminal polling interval, etc).
// Always fetch the current full config and merge onto it before saving.
export async function mergeAndSaveSettings(
  partial: Partial<config.AppConfig>
): Promise<config.AppConfig> {
  const { GetSettings, SaveSettings } = await import(
    /* @vite-ignore */ "../../wailsjs/go/services/SettingsService"
  );
  const current = await GetSettings();
  const merged = { ...current, ...partial } as config.AppConfig;
  await SaveSettings(merged);
  return merged;
}
