import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import App from "@/App";

const needsMasterPassword = vi.fn();
const hasMasterPassword = vi.fn();
const loadAPIKeys = vi.fn();

// PairAdmin_MasterPasswordDialog.retryOSKeychain is wired through App: a retry
// re-calls NeedsMasterPassword() and, on false, proceeds exactly like the
// normal gate-pass path (loadAndProceed -> dialog dismissed).
vi.mock("../../wailsjs/go/services/SettingsService", () => ({
  NeedsMasterPassword: () => needsMasterPassword(),
  HasMasterPassword: () => hasMasterPassword(),
  LoadAPIKeys: () => loadAPIKeys(),
}));

// Keep the full App in jsdom light-weight: the real layout panes pull in
// heavy terminal/SSH/LLM machinery we don't need to exercise here.
vi.mock("@/components/layout/ThreeColumnLayout", () => ({
  ThreeColumnLayout: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));
vi.mock("@/components/chat/ChatPane", () => ({
  ChatPane: () => <div data-testid="chat-pane" />,
}));
vi.mock("@/components/sidebar/CommandSidebar", () => ({
  CommandSidebar: () => <div data-testid="command-sidebar" />,
}));

beforeEach(() => {
  vi.clearAllMocks();
  hasMasterPassword.mockResolvedValue(true);
  loadAPIKeys.mockResolvedValue(undefined);
});

describe("App master-password startup gate: Retry OS Keychain", () => {
  it("opens the unlock dialog on mount when a master password is needed", async () => {
    needsMasterPassword.mockResolvedValue(true);
    render(<App />);
    expect(await screen.findByText("Unlock PairAdmin")).toBeInTheDocument();
    expect(hasMasterPassword).toHaveBeenCalledTimes(1);
  });

  it("retry re-calls NeedsMasterPassword and proceeds (dismisses dialog) when it resolves false", async () => {
    // First call (mount) says a master password is needed; the retry call
    // finds a now-available OS keychain (false).
    needsMasterPassword.mockResolvedValueOnce(true).mockResolvedValueOnce(false);
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText("Unlock PairAdmin");
    expect(needsMasterPassword).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Retry OS Keychain" }));

    // Mutation check: deleting the retry button's wiring (or not re-calling
    // NeedsMasterPassword) makes NeedsMasterPassword stay at 1 call, so this
    // assertion fails.
    await waitFor(() => expect(needsMasterPassword).toHaveBeenCalledTimes(2));
    // Keychain available -> proceed path ran (keys loaded) and dialog dismissed.
    await waitFor(() => expect(loadAPIKeys).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.queryByText("Unlock PairAdmin")).not.toBeInTheDocument(),
    );
  });

  it("retry keeps the dialog open and shows the message when NeedsMasterPassword resolves true", async () => {
    needsMasterPassword.mockResolvedValue(true); // still no usable OS keychain
    const user = userEvent.setup();
    render(<App />);
    await screen.findByText("Unlock PairAdmin");

    await user.click(screen.getByRole("button", { name: "Retry OS Keychain" }));

    expect(await screen.findByText(/No OS keychain detected/)).toBeInTheDocument();
    // Dialog stays open; no proceed path ran.
    expect(screen.getByText("Unlock PairAdmin")).toBeInTheDocument();
    expect(loadAPIKeys).not.toHaveBeenCalled();
  });
});