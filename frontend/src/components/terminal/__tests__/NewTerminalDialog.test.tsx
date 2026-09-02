import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import { NewTerminalDialog } from "@/components/terminal/NewTerminalDialog";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";

const openNewTerminal = vi.fn();
const openRemoteTerminal = vi.fn();
const checkHostKeyTrust = vi.fn();
const listRemoteHostsWithStatus = vi.fn();
const saveRemoteHost = vi.fn();
const touchRemoteHost = vi.fn();
const forgetRemoteHost = vi.fn();
const getSettings = vi.fn();

vi.mock("../../../../wailsjs/go/services/PTYService", () => ({
  OpenNewTerminal: (...args: unknown[]) => openNewTerminal(...args),
  OpenRemoteTerminal: (...args: unknown[]) => openRemoteTerminal(...args),
  CheckHostKeyTrust: (...args: unknown[]) => checkHostKeyTrust(...args),
}));

vi.mock("../../../../wailsjs/go/services/RemoteService", () => ({
  ListRemoteHostsWithStatus: (...args: unknown[]) => listRemoteHostsWithStatus(...args),
  SaveRemoteHost: (...args: unknown[]) => saveRemoteHost(...args),
  TouchRemoteHost: (...args: unknown[]) => touchRemoteHost(...args),
  ForgetRemoteHost: (...args: unknown[]) => forgetRemoteHost(...args),
}));

vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
}));

// Helper to wrap a saved-host record as a `ListRemoteHostsWithStatus` result.
// Matches the Wails wire shape: the status wrapper field is lowercase `host`
// (from the Go json tag), while the nested config.RemoteHost stays PascalCase.
const status = (host: Record<string, unknown>, hasCredential = true) => ({
  host,
  hasCredential,
});

beforeEach(() => {
  vi.clearAllMocks();
  useTerminalStore.setState({ tabs: [], activeTabId: "", nextTabNumber: 1 });
  useCommandStore.setState({ commands: [] });
  listRemoteHostsWithStatus.mockResolvedValue([]);
  getSettings.mockResolvedValue({ PromptNewHostKeys: false });
});

describe("NewTerminalDialog", () => {
  it("renders the three terminal type choices when open", () => {
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);
    expect(screen.getByText("Local")).toBeInTheDocument();
    expect(screen.getByText("Unix / Linux (SSH)")).toBeInTheDocument();
    expect(screen.getByText("Remote Windows (WinRM)")).toBeInTheDocument();
  });

  it("does not render dialog content when open=false", () => {
    render(<NewTerminalDialog open={false} onClose={vi.fn()} />);
    expect(screen.queryByText("Local")).not.toBeInTheDocument();
  });

  it("clicking Local calls OpenNewTerminal and adds a local tab", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    openNewTerminal.mockResolvedValue("local-tab-1");
    render(<NewTerminalDialog open={true} onClose={onClose} />);

    await user.click(screen.getByText("Local"));

    expect(openNewTerminal).toHaveBeenCalled();
    expect(useTerminalStore.getState().tabs.some((t) => t.id === "local-tab-1")).toBe(true);
    expect(useTerminalStore.getState().tabs.find((t) => t.id === "local-tab-1")?.kind).toBe(
      "local"
    );
    expect(onClose).toHaveBeenCalled();
  });

  it("clicking Unix/Linux (SSH) advances to the connection form", async () => {
    const user = userEvent.setup();
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));

    expect(screen.getByText("Host")).toBeInTheDocument();
    expect(screen.getByText("Authentication")).toBeInTheDocument();
    expect(openNewTerminal).not.toHaveBeenCalled();
  });

  it("SSH form's Connect button is disabled until host and username are filled", async () => {
    const user = userEvent.setup();
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));
    const connectButton = screen.getByText("Connect");
    expect(connectButton).toBeDisabled();

    const inputs = screen.getAllByRole("textbox");
    // Host is the first plain text input in the form.
    await user.type(inputs[0], "10.0.1.5");
    expect(connectButton).toBeDisabled();
  });

  it("submitting the SSH form calls OpenRemoteTerminal with a ssh:-prefixed tabId and adds a tab", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
    render(<NewTerminalDialog open={true} onClose={onClose} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));

    const hostInput = screen.getByPlaceholderText("10.0.1.5");
    await user.type(hostInput, "10.0.1.5");
    const usernameLabel = screen.getByText("Username");
    const usernameInput = usernameLabel.parentElement?.querySelector("input");
    expect(usernameInput).toBeTruthy();
    await user.type(usernameInput as HTMLInputElement, "ubuntu");

    await user.click(screen.getByText("Connect"));

    expect(openRemoteTerminal).toHaveBeenCalledTimes(1);
    const [tabId, params] = openRemoteTerminal.mock.calls[0];
    expect(tabId).toMatch(/^ssh:/);
    expect(params).toMatchObject({ kind: "ssh", host: "10.0.1.5", username: "ubuntu" });
    expect(
      useTerminalStore.getState().tabs.some((t) => t.id === "ssh:resolved-id")
    ).toBe(true);
    expect(onClose).toHaveBeenCalled();
  });

  it("clicking Remote Windows (WinRM) shows the WinRM-only auth hint and marks the resulting tab degraded", async () => {
    const user = userEvent.setup();
    openRemoteTerminal.mockResolvedValue("winrm:resolved-id");
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Remote Windows (WinRM)"));
    expect(screen.getByText("WinRM supports password authentication only.")).toBeInTheDocument();

    const hostInput = screen.getByPlaceholderText("10.0.1.5");
    await user.type(hostInput, "10.0.1.6");
    const usernameLabel = screen.getByText("Username");
    const usernameInput = usernameLabel.parentElement?.querySelector("input") as HTMLInputElement;
    await user.type(usernameInput, "Administrator");

    await user.click(screen.getByText("Connect"));

    const tab = useTerminalStore.getState().tabs.find((t) => t.id === "winrm:resolved-id");
    expect(tab?.degraded).toBe(true);
    expect(tab?.degradedMsg).toMatch(/not a live shell/);
  });

  it("shows an error message when OpenRemoteTerminal rejects", async () => {
    const user = userEvent.setup();
    openRemoteTerminal.mockRejectedValue(new Error("connection refused"));
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));
    const hostInput = screen.getByPlaceholderText("10.0.1.5");
    await user.type(hostInput, "10.0.1.5");
    const usernameLabel = screen.getByText("Username");
    const usernameInput = usernameLabel.parentElement?.querySelector("input") as HTMLInputElement;
    await user.type(usernameInput, "ubuntu");

    await user.click(screen.getByText("Connect"));

    expect(await screen.findByText("connection refused")).toBeInTheDocument();
  });

  it("uses 'Save Terminal' as the save-checkbox label", async () => {
    const user = userEvent.setup();
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));

    expect(screen.getByText("Save Terminal")).toBeInTheDocument();
    expect(screen.queryByText("Remember this connection")).not.toBeInTheDocument();
  });

  it("tmux options are hidden until Save Terminal is checked, and hidden entirely for WinRM", async () => {
    const user = userEvent.setup();
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));
    expect(screen.queryByText("Use tmux if available")).not.toBeInTheDocument();

    await user.click(screen.getByText("Save Terminal"));
    expect(screen.getByText("Use tmux if available")).toBeInTheDocument();
    // Session name field only appears once "Use tmux" is also checked.
    expect(screen.queryByText("tmux session name")).not.toBeInTheDocument();

    await user.click(screen.getByText("Use tmux if available"));
    expect(screen.getByText("tmux session name")).toBeInTheDocument();

    // Back out and pick WinRM instead — tmux has no place there even with Save checked.
    await user.click(screen.getByText("Back"));
    await user.click(screen.getByText("Remote Windows (WinRM)"));
    await user.click(screen.getByText("Save Terminal"));
    expect(screen.queryByText("Use tmux if available")).not.toBeInTheDocument();
  });

  it("submits useTmux and a sanitized-by-backend tmux session name for SSH", async () => {
    const user = userEvent.setup();
    openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));
    const hostInput = screen.getByPlaceholderText("10.0.1.5");
    await user.type(hostInput, "10.0.1.5");
    const usernameInput = screen.getByText("Username").parentElement?.querySelector(
      "input"
    ) as HTMLInputElement;
    await user.type(usernameInput, "ubuntu");

    await user.click(screen.getByText("Save Terminal"));
    await user.click(screen.getByText("Use tmux if available"));
    const tmuxNameInput = screen.getByPlaceholderText("pairadmin (default)");
    await user.type(tmuxNameInput, "work");

    await user.click(screen.getByText("Connect"));

    const [, params] = openRemoteTerminal.mock.calls[0];
    expect(params).toMatchObject({ useTmux: true, tmuxSessionName: "work", savePassword: true });

    expect(saveRemoteHost).toHaveBeenCalledTimes(1);
    const [savedHost] = saveRemoteHost.mock.calls[0];
    expect(savedHost).toMatchObject({ UseTmux: true, TmuxSessionName: "work" });
  });

  it("does not send tmux fields for WinRM even if somehow set", async () => {
    const user = userEvent.setup();
    openRemoteTerminal.mockResolvedValue("winrm:resolved-id");
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Remote Windows (WinRM)"));
    const hostInput = screen.getByPlaceholderText("10.0.1.5");
    await user.type(hostInput, "10.0.1.6");
    const usernameInput = screen.getByText("Username").parentElement?.querySelector(
      "input"
    ) as HTMLInputElement;
    await user.type(usernameInput, "Administrator");

    await user.click(screen.getByText("Connect"));

    const [, params] = openRemoteTerminal.mock.calls[0];
    expect(params.useTmux).toBe(false);
    expect(params.tmuxSessionName).toBe("");
  });

  it("reconnecting to a saved host restores its tmux settings", async () => {
    const user = userEvent.setup();
    listRemoteHostsWithStatus.mockResolvedValue([
      status({
        ID: "abc-1",
        Kind: "ssh",
        Host: "10.0.1.5",
        Port: 22,
        Username: "ubuntu",
        AuthType: "password",
        PrivateKeyPath: "",
        LastUsed: "2026-01-01T00:00:00Z",
        UseTmux: true,
        TmuxSessionName: "work",
      }),
    ]);
    openRemoteTerminal.mockResolvedValue("ssh:reconnected");
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await screen.findByText(/ubuntu@10.0.1.5/);
    await user.click(screen.getByText("Connect"));

    const [, params] = openRemoteTerminal.mock.calls[0];
    expect(params).toMatchObject({ useTmux: true, tmuxSessionName: "work", savedHostId: "abc-1" });
  });

  it("lists recent hosts and one-click reconnects using the saved host ID", async () => {
    const user = userEvent.setup();
    listRemoteHostsWithStatus.mockResolvedValue([
      status({ ID: "abc-1", Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "password", PrivateKeyPath: "", LastUsed: "2026-01-01T00:00:00Z" }),
    ]);
    openRemoteTerminal.mockResolvedValue("ssh:reconnected");
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    expect(await screen.findByText(/ubuntu@10.0.1.5/)).toBeInTheDocument();
    await user.click(screen.getByText("Connect"));

    expect(openRemoteTerminal).toHaveBeenCalledTimes(1);
    const [, params] = openRemoteTerminal.mock.calls[0];
    expect(params).toMatchObject({ host: "10.0.1.5", username: "ubuntu", savedHostId: "abc-1" });
    expect(touchRemoteHost).toHaveBeenCalledWith("abc-1");
    expect(saveRemoteHost).not.toHaveBeenCalled();
  });

  it("shows the saved friendly Name instead of username@host when one is set, and uses it as the reconnected tab's name", async () => {
    const user = userEvent.setup();
    listRemoteHostsWithStatus.mockResolvedValue([
      status({ ID: "abc-1", Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "password", PrivateKeyPath: "", LastUsed: "2026-01-01T00:00:00Z", Name: "Prod Web Server" }),
    ]);
    openRemoteTerminal.mockResolvedValue("ssh:reconnected");
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    expect(await screen.findByText("Prod Web Server")).toBeInTheDocument();
    expect(screen.queryByText(/ubuntu@10.0.1.5/)).not.toBeInTheDocument();

    await user.click(screen.getByText("Connect"));

    const tab = useTerminalStore.getState().tabs.find((t) => t.id === "ssh:reconnected");
    expect(tab?.name).toBe("Prod Web Server");
    expect(tab?.savedHostId).toBe("abc-1");
  });

  it("shows the amber 'no stored credential' indicator for a metadata-only saved host", async () => {
    listRemoteHostsWithStatus.mockResolvedValue([
      status(
        { ID: "abc-1", Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "password", PrivateKeyPath: "", LastUsed: "2026-01-01T00:00:00Z" },
        // hasCredential=false — saved via the tab menu, never got a secret stored.
        false
      ),
    ]);
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);
    await screen.findByText(/ubuntu@10.0.1.5/);

    const alert = document.body.querySelector(".lucide-triangle-alert");
    expect(alert).not.toBeNull();
  });

  it("does NOT show the amber indicator for a saved host that has a stored credential", async () => {
    listRemoteHostsWithStatus.mockResolvedValue([
      status(
        { ID: "abc-1", Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "password", PrivateKeyPath: "", LastUsed: "2026-01-01T00:00:00Z" },
        // hasCredential=true — a password was stored at save time.
        true
      ),
    ]);
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);
    await screen.findByText(/ubuntu@10.0.1.5/);

    expect(document.body.querySelector(".lucide-triangle-alert")).toBeNull();
  });

  it("a fresh saved connection's tab carries the generated savedHostId for future renames", async () => {
    const user = userEvent.setup();
    openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
    saveRemoteHost.mockResolvedValue({ ID: "new-host-id", Kind: "ssh", Host: "10.0.1.5", Username: "ubuntu" });
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));
    await user.type(screen.getByPlaceholderText("10.0.1.5"), "10.0.1.5");
    const usernameInput = screen.getByText("Username").parentElement?.querySelector(
      "input"
    ) as HTMLInputElement;
    await user.type(usernameInput, "ubuntu");
    await user.click(screen.getByText("Save Terminal"));

    await user.click(screen.getByText("Connect"));

    const tab = useTerminalStore.getState().tabs.find((t) => t.id === "ssh:resolved-id");
    expect(tab?.savedHostId).toBe("new-host-id");
  });

  it("surfaces a plain-string OpenRemoteTerminal rejection verbatim (the real Wails v2 shape, not a JS Error)", async () => {
    const user = userEvent.setup();
    // Wails v2 rejects RPC failures with a raw string, never `new Error(...)`.
    openRemoteTerminal.mockRejectedValue("ssh: unable to authenticate, attempted methods [none password]");
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));
    await user.type(screen.getByPlaceholderText("10.0.1.5"), "10.0.1.5");
    const usernameInput = screen.getByText("Username").parentElement?.querySelector(
      "input"
    ) as HTMLInputElement;
    await user.type(usernameInput, "ubuntu");

    await user.click(screen.getByText("Connect"));

    expect(
      await screen.findByText("ssh: unable to authenticate, attempted methods [none password]")
    ).toBeInTheDocument();
  });

  it("does not close the dialog and shows a warning when SaveRemoteHost fails after a successful connect", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
    saveRemoteHost.mockRejectedValue("keychain write failed: access denied");
    render(<NewTerminalDialog open={true} onClose={onClose} />);

    await user.click(screen.getByText("Unix / Linux (SSH)"));
    await user.type(screen.getByPlaceholderText("10.0.1.5"), "10.0.1.5");
    const usernameInput = screen.getByText("Username").parentElement?.querySelector(
      "input"
    ) as HTMLInputElement;
    await user.type(usernameInput, "ubuntu");
    await user.click(screen.getByText("Save Terminal"));

    await user.click(screen.getByText("Connect"));

    // The terminal tab still opens — the primary connection succeeded.
    expect(
      useTerminalStore.getState().tabs.some((t) => t.id === "ssh:resolved-id")
    ).toBe(true);
    // But the save failure is visible, not swallowed, and the dialog stays open
    // so the user can actually read it.
    expect(await screen.findByText("keychain write failed: access denied")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("forgetting a saved host calls ForgetRemoteHost and removes it from the list", async () => {
    const user = userEvent.setup();
    listRemoteHostsWithStatus
      .mockResolvedValueOnce([
        status({ ID: "abc-1", Kind: "ssh", Host: "10.0.1.5", Port: 22, Username: "ubuntu", AuthType: "password", PrivateKeyPath: "", LastUsed: "2026-01-01T00:00:00Z" }),
      ])
      .mockResolvedValueOnce([]);
    forgetRemoteHost.mockResolvedValue(undefined);
    render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

    await screen.findByText(/ubuntu@10.0.1.5/);
    await user.click(screen.getByLabelText("Forget saved host ubuntu@10.0.1.5"));

    expect(forgetRemoteHost).toHaveBeenCalledWith("abc-1");
    expect(await screen.findByText("Local")).toBeInTheDocument();
    expect(screen.queryByText(/ubuntu@10.0.1.5/)).not.toBeInTheDocument();
  });

  describe("SSH host key confirmation", () => {
    const fillSshForm = async (user: ReturnType<typeof userEvent.setup>) => {
      await user.click(screen.getByText("Unix / Linux (SSH)"));
      await user.type(screen.getByPlaceholderText("10.0.1.5"), "10.0.1.5");
      const usernameInput = screen.getByText("Username").parentElement?.querySelector(
        "input"
      ) as HTMLInputElement;
      await user.type(usernameInput, "ubuntu");
    };

    it("connects directly without checking the host key when PromptNewHostKeys is off (default)", async () => {
      const user = userEvent.setup();
      getSettings.mockResolvedValue({ PromptNewHostKeys: false });
      openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
      render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

      await fillSshForm(user);
      await user.click(screen.getByText("Connect"));

      await vi.waitFor(() => expect(openRemoteTerminal).toHaveBeenCalledTimes(1));
      expect(checkHostKeyTrust).not.toHaveBeenCalled();
    });

    it("shows an accept/reject prompt with the fingerprint for an unrecognized host when PromptNewHostKeys is on", async () => {
      const user = userEvent.setup();
      getSettings.mockResolvedValue({ PromptNewHostKeys: true });
      checkHostKeyTrust.mockResolvedValue({
        known: false,
        changed: false,
        keyType: "ssh-ed25519",
        fingerprint: "SHA256:abc123fingerprint",
      });
      render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

      await fillSshForm(user);
      await user.click(screen.getByText("Connect"));

      expect(checkHostKeyTrust).toHaveBeenCalledWith("10.0.1.5", 22);
      expect(await screen.findByText("Verify new host key")).toBeInTheDocument();
      expect(screen.getByText("SHA256:abc123fingerprint")).toBeInTheDocument();
      expect(screen.getByText("ssh-ed25519")).toBeInTheDocument();
      expect(openRemoteTerminal).not.toHaveBeenCalled();
    });

    it("accepting an unrecognized host key connects with trustNewHostKey set", async () => {
      const user = userEvent.setup();
      getSettings.mockResolvedValue({ PromptNewHostKeys: true });
      checkHostKeyTrust.mockResolvedValue({
        known: false,
        changed: false,
        keyType: "ssh-ed25519",
        fingerprint: "SHA256:abc123fingerprint",
      });
      openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
      const onClose = vi.fn();
      render(<NewTerminalDialog open={true} onClose={onClose} />);

      await fillSshForm(user);
      await user.click(screen.getByText("Connect"));
      await screen.findByText("Verify new host key");
      await user.click(screen.getByText("Accept & Connect"));

      expect(openRemoteTerminal).toHaveBeenCalledTimes(1);
      const [, params] = openRemoteTerminal.mock.calls[0];
      expect(params).toMatchObject({ host: "10.0.1.5", username: "ubuntu", trustNewHostKey: true });
      expect(onClose).toHaveBeenCalled();
    });

    it("rejecting an unrecognized host key returns to the form without connecting", async () => {
      const user = userEvent.setup();
      getSettings.mockResolvedValue({ PromptNewHostKeys: true });
      checkHostKeyTrust.mockResolvedValue({
        known: false,
        changed: false,
        keyType: "ssh-ed25519",
        fingerprint: "SHA256:abc123fingerprint",
      });
      render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

      await fillSshForm(user);
      await user.click(screen.getByText("Connect"));
      await screen.findByText("Verify new host key");
      await user.click(screen.getByText("Reject"));

      expect(await screen.findByText("Host")).toBeInTheDocument(); // back on the connection form
      expect(openRemoteTerminal).not.toHaveBeenCalled();
    });

    it("skips the prompt and connects directly when the host key is already known/matching", async () => {
      const user = userEvent.setup();
      getSettings.mockResolvedValue({ PromptNewHostKeys: true });
      checkHostKeyTrust.mockResolvedValue({
        known: true,
        changed: false,
        keyType: "ssh-ed25519",
        fingerprint: "SHA256:abc123fingerprint",
      });
      openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
      render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

      await fillSshForm(user);
      await user.click(screen.getByText("Connect"));

      expect(checkHostKeyTrust).toHaveBeenCalled();
      await vi.waitFor(() => expect(openRemoteTerminal).toHaveBeenCalledTimes(1));
      expect(screen.queryByText("Verify new host key")).not.toBeInTheDocument();
    });

    it("shows a scary warning with no Accept button when the host's key has changed since it was pinned", async () => {
      const user = userEvent.setup();
      getSettings.mockResolvedValue({ PromptNewHostKeys: true });
      checkHostKeyTrust.mockResolvedValue({
        known: false,
        changed: true,
        keyType: "ssh-ed25519",
        fingerprint: "SHA256:newkeyfingerprint",
      });
      render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

      await fillSshForm(user);
      await user.click(screen.getByText("Connect"));

      expect(await screen.findByText("Warning: this host's key has changed")).toBeInTheDocument();
      expect(screen.queryByText("Accept & Connect")).not.toBeInTheDocument();
      expect(openRemoteTerminal).not.toHaveBeenCalled();
    });

    it("falls through to a direct connection attempt when the host key probe itself fails", async () => {
      const user = userEvent.setup();
      getSettings.mockResolvedValue({ PromptNewHostKeys: true });
      checkHostKeyTrust.mockRejectedValue("failed to reach 10.0.1.5:22: dial tcp: timeout");
      openRemoteTerminal.mockResolvedValue("ssh:resolved-id");
      render(<NewTerminalDialog open={true} onClose={vi.fn()} />);

      await fillSshForm(user);
      await user.click(screen.getByText("Connect"));

      await vi.waitFor(() => expect(openRemoteTerminal).toHaveBeenCalledTimes(1));
      expect(screen.queryByText("Verify new host key")).not.toBeInTheDocument();
    });
  });
});
