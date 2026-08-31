import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { TerminalsTab } from "@/components/settings/TerminalsTab";

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

describe("TerminalsTab", () => {
  beforeEach(() => {
    getSettings.mockResolvedValue({});
    saveSettings.mockResolvedValue(undefined);
  });

  it("shows the built-in defaults for both sidebar widths when unset", async () => {
    render(<TerminalsTab />);

    expect(await screen.findByDisplayValue("20")).toBeInTheDocument(); // Terminals list
    expect(screen.getByDisplayValue("30")).toBeInTheDocument(); // Commands list
  });

  it("loads previously saved widths instead of the defaults", async () => {
    getSettings.mockResolvedValue({
      TerminalsSidebarWidthCh: 15,
      CommandsSidebarWidthCh: 45,
    });
    render(<TerminalsTab />);

    expect(await screen.findByDisplayValue("15")).toBeInTheDocument();
    expect(screen.getByDisplayValue("45")).toBeInTheDocument();
  });

  it("saves both sidebar widths via the Save button", async () => {
    const user = userEvent.setup();
    render(<TerminalsTab />);
    await screen.findByDisplayValue("20");

    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ TerminalsSidebarWidthCh: 20, CommandsSidebarWidthCh: 30 })
    );
  });

  it("clamps the Terminals list width to the 10-80 range", async () => {
    const user = userEvent.setup();
    render(<TerminalsTab />);
    const input = await screen.findByDisplayValue("20");

    await user.clear(input);
    await user.type(input, "500");

    expect(input).toHaveValue(80);
  });
});
