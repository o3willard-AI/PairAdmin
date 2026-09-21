import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import { MasterPasswordDialog } from "@/components/settings/MasterPasswordDialog";

const setMasterPassword = vi.fn();
const verifyMasterPassword = vi.fn();

vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  SetMasterPassword: (...args: unknown[]) => setMasterPassword(...args),
  VerifyMasterPassword: (...args: unknown[]) => verifyMasterPassword(...args),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("MasterPasswordDialog", () => {
  it("renders set mode with confirm field when mode='set'", () => {
    render(<MasterPasswordDialog open mode="set" onSuccess={vi.fn()} />);
    expect(screen.getByText("Set Master Password")).toBeInTheDocument();
    expect(screen.getByLabelText("Master password")).toBeInTheDocument();
    expect(screen.getByLabelText("Confirm master password")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create Master Password" })).toBeInTheDocument();
  });

  it("renders unlock mode without confirm field when mode='unlock'", () => {
    render(<MasterPasswordDialog open mode="unlock" onSuccess={vi.fn()} />);
    expect(screen.getByText("Unlock PairAdmin")).toBeInTheDocument();
    expect(screen.getByLabelText("Master password")).toBeInTheDocument();
    expect(screen.queryByLabelText("Confirm master password")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unlock" })).toBeInTheDocument();
  });

  it("set mode: empty password shows an error and does not call the backend", async () => {
    const user = userEvent.setup();
    render(<MasterPasswordDialog open mode="set" onSuccess={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Create Master Password" }));
    expect(screen.getByText("Master password must not be empty.")).toBeInTheDocument();
    expect(setMasterPassword).not.toHaveBeenCalled();
  });

  it("set mode: mismatched confirm shows an error and does not call the backend", async () => {
    const user = userEvent.setup();
    render(<MasterPasswordDialog open mode="set" onSuccess={vi.fn()} />);
    await user.type(screen.getByLabelText("Master password"), "hunter2");
    await user.type(screen.getByLabelText("Confirm master password"), "hunter3");
    await user.click(screen.getByRole("button", { name: "Create Master Password" }));
    expect(screen.getByText("Passwords do not match.")).toBeInTheDocument();
    expect(setMasterPassword).not.toHaveBeenCalled();
  });

  it("set mode: matching passwords call SetMasterPassword then onSuccess", async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    setMasterPassword.mockResolvedValue(undefined);
    render(<MasterPasswordDialog open mode="set" onSuccess={onSuccess} />);
    await user.type(screen.getByLabelText("Master password"), "hunter2");
    await user.type(screen.getByLabelText("Confirm master password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Create Master Password" }));
    await waitFor(() => expect(onSuccess).toHaveBeenCalledTimes(1));
    expect(setMasterPassword).toHaveBeenCalledWith("hunter2");
    expect(verifyMasterPassword).not.toHaveBeenCalled();
  });

  it("set mode: backend failure (e.g. already set) surfaces the error", async () => {
    const user = userEvent.setup();
    setMasterPassword.mockRejectedValue("master password already set: use ChangeMasterPassword to change it");
    render(<MasterPasswordDialog open mode="set" onSuccess={vi.fn()} />);
    await user.type(screen.getByLabelText("Master password"), "hunter2");
    await user.type(screen.getByLabelText("Confirm master password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Create Master Password" }));
    expect(await screen.findByText(/master password already set/)).toBeInTheDocument();
  });

  it("unlock mode: wrong password shows an error and does not call onSuccess", async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    verifyMasterPassword.mockResolvedValue(false);
    render(<MasterPasswordDialog open mode="unlock" onSuccess={onSuccess} />);
    await user.type(screen.getByLabelText("Master password"), "wrong");
    await user.click(screen.getByRole("button", { name: "Unlock" }));
    expect(await screen.findByText("Incorrect master password.")).toBeInTheDocument();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(setMasterPassword).not.toHaveBeenCalled();
  });

  it("unlock mode: correct password calls VerifyMasterPassword then onSuccess", async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    verifyMasterPassword.mockResolvedValue(true);
    render(<MasterPasswordDialog open mode="unlock" onSuccess={onSuccess} />);
    await user.type(screen.getByLabelText("Master password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Unlock" }));
    await waitFor(() => expect(onSuccess).toHaveBeenCalledTimes(1));
    expect(verifyMasterPassword).toHaveBeenCalledWith("hunter2");
  });

  it("unlock mode: backend error surfaces instead of onSuccess", async () => {
    const user = userEvent.setup();
    const onSuccess = vi.fn();
    verifyMasterPassword.mockRejectedValue("keychain unavailable");
    render(<MasterPasswordDialog open mode="unlock" onSuccess={onSuccess} />);
    await user.type(screen.getByLabelText("Master password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Unlock" }));
    expect(await screen.findByText("keychain unavailable")).toBeInTheDocument();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it("renders the Retry OS Keychain button below the form in both modes", () => {
    const { unmount } = render(
      <MasterPasswordDialog open mode="set" onSuccess={vi.fn()} onRetryOSKeychain={vi.fn()} />,
    );
    expect(
      screen.getByRole("button", { name: "Retry OS Keychain" }),
    ).toBeInTheDocument();
    unmount();
    render(
      <MasterPasswordDialog open mode="unlock" onSuccess={vi.fn()} onRetryOSKeychain={vi.fn()} />,
    );
    expect(
      screen.getByRole("button", { name: "Retry OS Keychain" }),
    ).toBeInTheDocument();
  });

  it("clicking Retry OS Keychain re-invokes onRetryOSKeychain and the parent proceeds on false (dismiss)", async () => {
    const user = userEvent.setup();
    const onRetryOSKeychain = vi.fn().mockResolvedValue(false); // keychain now available
    render(
      <MasterPasswordDialog open mode="unlock" onSuccess={vi.fn()} onRetryOSKeychain={onRetryOSKeychain} />,
    );
    await user.click(screen.getByRole("button", { name: "Retry OS Keychain" }));
    // Mutation check: deleting the button's onClick wiring means onRetryOSKeychain
    // is never invoked, so this assertion fails.
    await waitFor(() => expect(onRetryOSKeychain).toHaveBeenCalledTimes(1));
    // false => keychain is available => parent proceeds; the transient
    // "Checking" state is cleared and no "still needs" message is shown.
    await waitFor(() =>
      expect(screen.queryByText(/Checking OS keychain/)).not.toBeInTheDocument(),
    );
    expect(screen.queryByText(/No OS keychain detected/)).not.toBeInTheDocument();
  });

  it("shows a Checking state while the retry is in flight and disables the button", async () => {
    const user = userEvent.setup();
    let resolveRetry!: (v: boolean) => void;
    const onRetryOSKeychain = vi.fn(
      () =>
        new Promise<boolean>((resolve) => {
          resolveRetry = resolve;
        }),
    );
    render(
      <MasterPasswordDialog open mode="set" onSuccess={vi.fn()} onRetryOSKeychain={onRetryOSKeychain} />,
    );
    await user.click(screen.getByRole("button", { name: "Retry OS Keychain" }));
    expect(screen.getByText(/Checking OS keychain/)).toBeInTheDocument();
    // Disabled while the probe runs.
    expect(screen.getByRole("button", { name: /Checking OS keychain/ })).toBeDisabled();
    resolveRetry(false);
    await waitFor(() =>
      expect(screen.queryByText(/Checking OS keychain/)).not.toBeInTheDocument(),
    );
  });

  it("when onRetryOSKeychain resolves true the dialog stays open and the message is shown", async () => {
    const user = userEvent.setup();
    const onRetryOSKeychain = vi.fn().mockResolvedValue(true); // still unavailable
    render(
      <MasterPasswordDialog open mode="unlock" onSuccess={vi.fn()} onRetryOSKeychain={onRetryOSKeychain} />,
    );
    await user.click(screen.getByRole("button", { name: "Retry OS Keychain" }));
    expect(await screen.findByText(/No OS keychain detected/)).toBeInTheDocument();
    // Dialog still open: the title and inputs remain on screen.
    expect(screen.getByText("Unlock PairAdmin")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry OS Keychain" })).toBeInTheDocument();
  });

  it("a failed retry probe is treated as still-needing and shown, not a crash", async () => {
    const user = userEvent.setup();
    const onRetryOSKeychain = vi.fn().mockRejectedValue(new Error("transport down"));
    render(
      <MasterPasswordDialog open mode="unlock" onSuccess={vi.fn()} onRetryOSKeychain={onRetryOSKeychain} />,
    );
    await user.click(screen.getByRole("button", { name: "Retry OS Keychain" }));
    expect(await screen.findByText(/Could not re-check the OS keychain/)).toBeInTheDocument();
    expect(screen.getByText("Unlock PairAdmin")).toBeInTheDocument();
  });
});
