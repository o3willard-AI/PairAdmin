import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import { ChangeMasterPasswordDialog } from "@/components/settings/ChangeMasterPasswordDialog";
import { SecurityTab } from "@/components/settings/SecurityTab";

const changeMasterPassword = vi.fn();
const hasMasterPassword = vi.fn();

vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  ChangeMasterPassword: (...args: unknown[]) => changeMasterPassword(...args),
  HasMasterPassword: (...args: unknown[]) => hasMasterPassword(...args),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ChangeMasterPasswordDialog", () => {
  it("empty new password shows an error and does not call the backend", async () => {
    const user = userEvent.setup();
    render(<ChangeMasterPasswordDialog open onClose={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Change Master Password" }));
    expect(screen.getByText("New master password must not be empty.")).toBeInTheDocument();
    expect(changeMasterPassword).not.toHaveBeenCalled();
  });

  it("mismatched confirmation shows an error and does not call the backend", async () => {
    const user = userEvent.setup();
    render(<ChangeMasterPasswordDialog open onClose={vi.fn()} />);
    await user.type(screen.getByLabelText("Current master password"), "old");
    await user.type(screen.getByLabelText("New master password"), "new1");
    await user.type(screen.getByLabelText("Confirm new master password"), "new2");
    await user.click(screen.getByRole("button", { name: "Change Master Password" }));
    expect(screen.getByText("New passwords do not match.")).toBeInTheDocument();
    expect(changeMasterPassword).not.toHaveBeenCalled();
  });

  it("successful change calls ChangeMasterPassword(old, new) and shows confirmation", async () => {
    const user = userEvent.setup();
    changeMasterPassword.mockResolvedValue(undefined);
    render(<ChangeMasterPasswordDialog open onClose={vi.fn()} />);
    await user.type(screen.getByLabelText("Current master password"), "old-pw");
    await user.type(screen.getByLabelText("New master password"), "new-pw");
    await user.type(screen.getByLabelText("Confirm new master password"), "new-pw");
    await user.click(screen.getByRole("button", { name: "Change Master Password" }));
    expect(await screen.findByText("Master password changed.")).toBeInTheDocument();
    expect(changeMasterPassword).toHaveBeenCalledWith("old-pw", "new-pw");
  });

  it("incorrect current password surfaces the backend error", async () => {
    const user = userEvent.setup();
    changeMasterPassword.mockRejectedValue("incorrect master password");
    render(<ChangeMasterPasswordDialog open onClose={vi.fn()} />);
    await user.type(screen.getByLabelText("Current master password"), "wrong");
    await user.type(screen.getByLabelText("New master password"), "new-pw");
    await user.type(screen.getByLabelText("Confirm new master password"), "new-pw");
    await user.click(screen.getByRole("button", { name: "Change Master Password" }));
    expect(await screen.findByText("incorrect master password")).toBeInTheDocument();
  });
});

describe("SecurityTab", () => {
  it("shows the disabled state when no master password is configured", async () => {
    hasMasterPassword.mockResolvedValue(false);
    render(<SecurityTab />);
    expect(
      await screen.findByText(/No master password configured/)
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Change master password" })).not.toBeInTheDocument();
  });

  it("shows the change button when a master password exists and opens the dialog", async () => {
    hasMasterPassword.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<SecurityTab />);
    const button = await screen.findByRole("button", { name: "Change master password" });
    await user.click(button);
    expect(screen.getByRole("heading", { name: "Change Master Password" })).toBeInTheDocument();
    expect(screen.getByLabelText("Current master password")).toBeInTheDocument();
  });
});

// Keep the tab-loading behavior covered even when the binding rejects.
describe("SecurityTab backend failure", () => {
  it("falls back to the no-master-password state when HasMasterPassword fails", async () => {
    hasMasterPassword.mockRejectedValue(new Error("binding missing"));
    render(<SecurityTab />);
    await waitFor(() =>
      expect(
        screen.getByText(/No master password configured/)
      ).toBeInTheDocument()
    );
  });
});
