import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY } from "@/hooks/useAddClipboardCommandHotkey";
import { DEFAULT_NEW_TERMINAL_HOTKEY } from "@/hooks/useNewTerminalHotkey";
import { DEFAULT_ADD_COMMAND_HOTKEY } from "@/hooks/useAddCommandHotkey";
import { HotkeysTab } from "@/components/settings/HotkeysTab";

const getSettings = vi.fn();
const saveSettings = vi.fn();
// Resolves (from frontend/src/components/settings/) to
// frontend/wailsjs/go/services/SettingsService. From this test file
// (frontend/src/components/settings/__tests__/) that is
// ../../../../wailsjs/go/services/SettingsService.
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  SaveSettings: (...args: unknown[]) => saveSettings(...args),
}));

describe("HotkeysTab", () => {
  beforeEach(() => {
    getSettings.mockResolvedValue({});
    saveSettings.mockResolvedValue(undefined);
  });

  it("shows the built-in default for Add Clipboard as Command when unset", async () => {
    render(<HotkeysTab />);

    expect(await screen.findByDisplayValue("Ctrl+Shift+A")).toBeInTheDocument();
  });

  it("loads a previously saved combo instead of the default", async () => {
    getSettings.mockResolvedValue({ HotkeyAddClipboardCommand: "Ctrl+Alt+X" });
    render(<HotkeysTab />);

    expect(await screen.findByDisplayValue("Ctrl+Alt+X")).toBeInTheDocument();
  });

  it("saves the current Add Clipboard as Command value via the Save button", async () => {
    const user = userEvent.setup();
    render(<HotkeysTab />);
    await screen.findByDisplayValue("Ctrl+Shift+A");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ HotkeyAddClipboardCommand: "Ctrl+Shift+A" })
    );
  });

  it("shows the built-in default for New Terminal when unset", async () => {
    render(<HotkeysTab />);

    expect(await screen.findByDisplayValue("Ctrl+Shift+N")).toBeInTheDocument();
  });

  it("loads a previously saved New Terminal combo instead of the default", async () => {
    getSettings.mockResolvedValue({ HotkeyNewTerminal: "Ctrl+Alt+Z" });
    render(<HotkeysTab />);

    expect(await screen.findByDisplayValue("Ctrl+Alt+Z")).toBeInTheDocument();
  });

  it("saves the current New Terminal value via the Save button", async () => {
    const user = userEvent.setup();
    render(<HotkeysTab />);
    await screen.findByDisplayValue("Ctrl+Shift+N");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ HotkeyNewTerminal: "Ctrl+Shift+N" })
    );
  });

  it("shows the built-in default for Add Command when unset", async () => {
    render(<HotkeysTab />);

    expect(await screen.findByDisplayValue(DEFAULT_ADD_COMMAND_HOTKEY)).toBeInTheDocument();
  });

  it("loads a previously saved Add Command combo instead of the default", async () => {
    getSettings.mockResolvedValue({ HotkeyAddCommand: "Ctrl+Alt+P" });
    render(<HotkeysTab />);

    expect(await screen.findByDisplayValue("Ctrl+Alt+P")).toBeInTheDocument();
  });

  it("saves the current Add Command value via the Save button", async () => {
    const user = userEvent.setup();
    render(<HotkeysTab />);
    await screen.findByDisplayValue(DEFAULT_ADD_COMMAND_HOTKEY);

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ HotkeyAddCommand: DEFAULT_ADD_COMMAND_HOTKEY })
    );
  });
});
