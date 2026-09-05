import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { AppearanceTab } from "@/components/settings/AppearanceTab";

const getSettings = vi.fn();
const saveSettings = vi.fn();
// Resolves (from frontend/src/components/settings/) to
// frontend/wailsjs/go/services/SettingsService. mergeAndSaveSettings and the
// mount effect both dynamically import this module.
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  SaveSettings: (...args: unknown[]) => saveSettings(...args),
}));

const setTheme = vi.fn();
vi.mock("@/theme/theme-provider", () => ({
  useTheme: () => ({ theme: "dark", setTheme }),
}));

beforeEach(() => {
  getSettings.mockReset().mockResolvedValue({});
  saveSettings.mockReset().mockResolvedValue(undefined);
  setTheme.mockReset();
});

describe("AppearanceTab — fonts load and persist", () => {
  it("pre-fills the font size from stored settings when a non-zero FontSize is present", async () => {
    getSettings.mockResolvedValue({ FontSize: 18 });
    render(<AppearanceTab />);

    expect(await screen.findByDisplayValue("18")).toBeInTheDocument();
  });

  it("does not overwrite the default font size when settings carry no FontSize", async () => {
    render(<AppearanceTab />);

    expect(await screen.findByDisplayValue("14")).toBeInTheDocument();
  });

  it("saves the current theme and font size on Save and surfaces a transient 'Saved!' state", async () => {
    const user = userEvent.setup();
    render(<AppearanceTab />);
    await screen.findByDisplayValue("14");

    const fontSizeInput = screen.getByRole("spinbutton");
    fireEvent.change(fontSizeInput, { target: { value: "16" } });
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ Theme: "dark", FontSize: 16 })
    );
    expect(await screen.findByText("Saved!")).toBeInTheDocument();
  });

  it("shows 'Save failed' when persisting throws", async () => {
    const user = userEvent.setup();
    saveSettings.mockRejectedValue(new Error("disk error"));
    render(<AppearanceTab />);
    await screen.findByDisplayValue("14");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(await screen.findByText("Save failed")).toBeInTheDocument();
  });
});

describe("AppearanceTab — theme switching", () => {
  it("switches to the dark theme when Dark is clicked", async () => {
    const user = userEvent.setup();
    render(<AppearanceTab />);
    await screen.findByDisplayValue("14");

    await user.click(screen.getByRole("button", { name: "Dark" }));

    expect(setTheme).toHaveBeenCalledWith("dark");
  });

  it("switches to the light theme when Light is clicked", async () => {
    const user = userEvent.setup();
    render(<AppearanceTab />);
    await screen.findByDisplayValue("14");

    await user.click(screen.getByRole("button", { name: "Light" }));

    expect(setTheme).toHaveBeenCalledWith("light");
  });
});