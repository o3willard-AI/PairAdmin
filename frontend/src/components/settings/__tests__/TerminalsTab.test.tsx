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

const environment = vi.fn();
// The Wails runtime module reads window.runtime directly, so the platform gate
// (EnvironmentInfo.platform) can only be driven from a test by mocking it.
vi.mock("../../../../wailsjs/runtime/runtime", () => ({
  Environment: (...args: unknown[]) => environment(...args),
}));

describe("TerminalsTab", () => {
  beforeEach(() => {
    getSettings.mockResolvedValue({});
    saveSettings.mockResolvedValue(undefined);
    // Default platform for this file: not Windows, i.e. the administrator
    // toggle is disabled. Tests that need Windows say so explicitly.
    environment.mockResolvedValue({ buildType: "production", platform: "linux", arch: "amd64" });
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

  it("allows free typing and clamps the width on blur", async () => {
    const user = userEvent.setup();
    render(<TerminalsTab />);
    const input = await screen.findByDisplayValue("20");

    await user.clear(input);
    await user.type(input, "500");
    // Typing is not clamped mid-keystroke (a multi-digit entry must survive).
    expect(input).toHaveValue(500);

    await user.tab(); // blur
    expect(input).toHaveValue(80);
  });

  it("reflects the saved Network Finder toggle setting", async () => {
    getSettings.mockResolvedValue({ ScannerEnabled: false });
    render(<TerminalsTab />);
    const toggle = await screen.findByRole("checkbox", { name: /Network Finder/ });
    expect(toggle).not.toBeChecked();
  });

  it("saves the Network Finder toggle through the settings save path", async () => {
    const user = userEvent.setup();
    render(<TerminalsTab />);
    const toggle = await screen.findByRole("checkbox", { name: /Network Finder/ });
    expect(toggle).toBeChecked(); // tab UI defaults the checkbox on when silent

    await user.click(toggle); // turn it off
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ ScannerEnabled: false })
    );
    // Mutation check: dropping ScannerEnabled from handleSave's merge (or from
    // the toggle's wiring) makes this fail — turning the toggle off would never
    // persist, so the Network panel could not be hidden.
  });

  it("describes the feature as a host finder, not a port scanner defaults claim", async () => {
    render(<TerminalsTab />);
    await screen.findByRole("checkbox", { name: /Network Finder/ });
    // Feature name + discoverable-SSH framing.
    expect(screen.getByText("Network Finder")).toBeInTheDocument();
    expect(screen.getByText(/search local networks for SSH hosts/i)).toBeInTheDocument();
    // The stale "On by default" claim (it's OFF by default since v2.6.1) is gone.
    expect(screen.queryByText(/On by default/i)).toBeNull();
    // Availability is gated on the checkbox being enabled.
    expect(screen.getByText(/displayed\/available when this checkbox is enabled/i)).toBeInTheDocument();
    // Mutation check: reverting the description to the old port-scanner /
    // "On by default" copy makes the queryByText(/On by default/) assertion
    // find text — this fails.
  });

  // UAT: each section states when its settings take effect. The questions the
  // notes answer are the ones support actually gets ("I cleared the clipboard
  // interval and it still clears", "I changed the width and nothing moved").
  //
  // Mutation check: deleting any of the three note paragraphs (or moving one to
  // a different section) fails — the assertions are anchored to the heading each
  // note must sit under, not just to the presence of the text somewhere.
  it("states when each section's settings take effect, under its own heading", async () => {
    render(<TerminalsTab />);
    await screen.findByRole("checkbox", { name: /Network Finder/ });

    function noteUnder(headingText: RegExp): string {
      const heading = screen.getByRole("heading", { name: headingText });
      const note = heading.nextElementSibling;
      expect(note?.tagName).toBe("P");
      return note?.textContent ?? "";
    }

    expect(noteUnder(/^Capture Settings$/)).toMatch(
      /Clipboard auto-clear applies to the next copy\.\s+The AT-SPI polling interval takes effect after restarting the app\./
    );
    // The Layout note predates this task and is kept verbatim.
    expect(noteUnder(/^Layout$/)).toMatch(/takes effect after\s+restarting the app\./);
    expect(noteUnder(/^Remote Connections$/)).toMatch(
      /These apply to the next connection \(host keys\) and immediately\s+\(Network Finder\)\./
    );
    expect(noteUnder(/^Administrator$/)).toMatch(/Takes effect on the next launch/);
  });

  // The administrator toggle is Windows-only: on macOS/Linux there is no UAC and
  // no elevation type to read, so offering the checkbox would be a lie.
  //
  // Mutation check: dropping `disabled={!isWindows}` (or defaulting the unknown
  // platform to Windows) makes the first assertion fail — the toggle would be
  // clickable on a platform where nothing implements it.
  it("disables the administrator toggle on non-Windows platforms", async () => {
    for (const platform of ["linux", "darwin"]) {
      environment.mockResolvedValue({ buildType: "production", platform, arch: "amd64" });
      const { unmount } = render(<TerminalsTab />);
      const toggle = await screen.findByRole("checkbox", {
        name: /Launch PairAdmin as Local Windows Administrator/,
      });
      expect(toggle).toBeDisabled();
      unmount();
    }
  });

  // Mutation check: inverting the platform comparison (e.g. `=== "linux"`, or
  // `!== "windows"`) makes the toggle render disabled on Windows and this fails.
  it("enables the administrator toggle on Windows", async () => {
    environment.mockResolvedValue({ buildType: "production", platform: "windows", arch: "amd64" });
    render(<TerminalsTab />);
    const toggle = await screen.findByRole("checkbox", {
      name: /Launch PairAdmin as Local Windows Administrator/,
    });
    expect(toggle).not.toBeDisabled();
  });

  // The platform arrives asynchronously. Until it does, "unknown" must not be
  // read as Windows — otherwise the toggle is briefly clickable on macOS/Linux
  // and a fast click persists a setting that can never work there.
  //
  // Mutation check: making the disabled prop `isWindows === false` (instead of
  // `!isWindows`) fails this, because null is not false.
  it("keeps the administrator toggle disabled until the platform is known", async () => {
    environment.mockReturnValue(new Promise(() => {})); // never resolves
    render(<TerminalsTab />);
    const toggle = await screen.findByRole("checkbox", {
      name: /Launch PairAdmin as Local Windows Administrator/,
    });
    expect(toggle).toBeDisabled();
  });

  it("reflects the saved administrator setting", async () => {
    environment.mockResolvedValue({ buildType: "production", platform: "windows", arch: "amd64" });
    getSettings.mockResolvedValue({ LaunchAsAdministrator: true });
    render(<TerminalsTab />);
    const toggle = await screen.findByRole("checkbox", {
      name: /Launch PairAdmin as Local Windows Administrator/,
    });
    expect(toggle).toBeChecked();
  });

  it("defaults the administrator toggle off when the setting is absent", async () => {
    environment.mockResolvedValue({ buildType: "production", platform: "windows", arch: "amd64" });
    render(<TerminalsTab />);
    const toggle = await screen.findByRole("checkbox", {
      name: /Launch PairAdmin as Local Windows Administrator/,
    });
    expect(toggle).not.toBeChecked();
    // Mutation check: flipping the Go field's default (or the frontend's
    // `!!cfg.LaunchAsAdministrator` load) to true makes this fail — the app
    // would relaunch elevated for users who never asked for it.
  });

  it("saves the administrator toggle through the settings save path", async () => {
    const user = userEvent.setup();
    environment.mockResolvedValue({ buildType: "production", platform: "windows", arch: "amd64" });
    render(<TerminalsTab />);
    const toggle = await screen.findByRole("checkbox", {
      name: /Launch PairAdmin as Local Windows Administrator/,
    });

    await user.click(toggle);
    await user.click(screen.getByRole("button", { name: /^save$/i }));

    expect(saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ LaunchAsAdministrator: true })
    );
    // Mutation check: dropping LaunchAsAdministrator from handleSave's merge (or
    // from the checkbox's onChange wiring) fails this — the toggle would appear
    // to save and the app would never relaunch elevated.
  });

  // The setting only takes effect on the NEXT launch, and the reason it can do
  // that at all is that the terminals stay embedded (a separate elevated window
  // would be invisible to the assistant). Support gets asked both.
  //
  // Mutation check: deleting the helper text (or the next-launch sentence under
  // the Administrator heading) fails — a user would save the toggle and expect
  // their current terminals to become elevated.
  it("explains when the administrator toggle takes effect and why it still works", async () => {
    environment.mockResolvedValue({ buildType: "production", platform: "windows", arch: "amd64" });
    render(<TerminalsTab />);
    await screen.findByRole("checkbox", {
      name: /Launch PairAdmin as Local Windows Administrator/,
    });

    expect(screen.getByText(/asks for administrator rights once \(a UAC prompt\)/)).toBeInTheDocument();
    expect(screen.getByText(/every terminal it\s+opens starts with an administrator token/)).toBeInTheDocument();
    expect(screen.getByText(/terminals stay inside PairAdmin, so the assistant can still read/)).toBeInTheDocument();
  });
});
