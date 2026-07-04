import { describe, it, expect, vi, beforeEach } from "vitest";
import { mergeAndSaveSettings } from "@/utils/settingsSync";

const getSettings = vi.fn();
const saveSettings = vi.fn();

vi.mock("../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  SaveSettings: (...args: unknown[]) => saveSettings(...args),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("mergeAndSaveSettings", () => {
  it("merges the partial update onto the currently-persisted config before saving", async () => {
    getSettings.mockResolvedValue({
      Provider: "openrouter",
      Model: "deepseek/deepseek-v4-flash",
      Theme: "dark",
      FontSize: 16,
      RemoteHosts: [{ ID: "abc-1", Kind: "ssh", Host: "10.0.1.5" }],
    });
    saveSettings.mockResolvedValue(undefined);

    await mergeAndSaveSettings({ CustomPrompt: "Be terse." });

    expect(getSettings).toHaveBeenCalledTimes(1);
    expect(saveSettings).toHaveBeenCalledTimes(1);
    const [saved] = saveSettings.mock.calls[0];

    // The tab's own edit is applied...
    expect(saved.CustomPrompt).toBe("Be terse.");
    // ...but every field that tab doesn't manage survives untouched, including
    // RemoteHosts, which no Settings tab is ever supposed to modify.
    expect(saved.Provider).toBe("openrouter");
    expect(saved.Model).toBe("deepseek/deepseek-v4-flash");
    expect(saved.Theme).toBe("dark");
    expect(saved.FontSize).toBe(16);
    expect(saved.RemoteHosts).toEqual([{ ID: "abc-1", Kind: "ssh", Host: "10.0.1.5" }]);
  });

  it("lets the partial update override a field also present in the current config", async () => {
    getSettings.mockResolvedValue({ Provider: "openai", Model: "gpt-4" });
    saveSettings.mockResolvedValue(undefined);

    await mergeAndSaveSettings({ Provider: "anthropic", Model: "claude-3-5-sonnet" });

    const [saved] = saveSettings.mock.calls[0];
    expect(saved.Provider).toBe("anthropic");
    expect(saved.Model).toBe("claude-3-5-sonnet");
  });
});
