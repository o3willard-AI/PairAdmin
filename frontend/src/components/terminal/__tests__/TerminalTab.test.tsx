import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "@testing-library/jest-dom";
import { TerminalTab } from "@/components/terminal/TerminalTab";
import { useTerminalStore } from "@/stores/terminalStore";

const renameRemoteHost = vi.fn();
const saveRemoteHost = vi.fn();

vi.mock("../../../../wailsjs/go/services/RemoteService", () => ({
  RenameRemoteHost: (...args: unknown[]) => renameRemoteHost(...args),
  SaveRemoteHost: (...args: unknown[]) => saveRemoteHost(...args),
}));

describe("TerminalTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    renameRemoteHost.mockResolvedValue(undefined);
    saveRemoteHost.mockResolvedValue({ ID: "saved-host-99" });
    useTerminalStore.setState({ tabs: [], activeTabId: "" });
  });

  // Test A1: hover on a remote tab shows name + host:port; local shows name only.
  it("shows host:port in the tooltip when the tab has a host", async () => {
    const remote = {
      host: "10.0.1.5",
      port: 22,
      username: "ubuntu",
      authType: "password" as const,
      privateKeyPath: "",
      useTmux: false,
      tmuxSessionName: "",
    };
    const tab = {
      id: "ssh:abc",
      name: "ubuntu@10.0.1.5",
      kind: "ssh" as const,
      host: "10.0.1.5",
      port: 22,
      remote,
    };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.hover(screen.getByText("ubuntu@10.0.1.5"));

    expect(await screen.findByText("10.0.1.5:22")).toBeInTheDocument();
  });

  it("does NOT show host:port in the tooltip for a local tab", async () => {
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.hover(screen.getByText("main:0.0"));

    // A little wait to give an erroneously-rendered tooltip a chance to appear.
    await new Promise((r) => setTimeout(r, 30));
    // The tab name itself contains a colon ("main:0.0"), so match the distinct
    // host:port shape (an IP followed by :port) rather than any colon.
    expect(screen.queryByText(/\d+\.\d+\.\d+\.\d+:\d+/)).not.toBeInTheDocument();
  });

  // Test A2: context-menu states.
  it("local tab context menu offers Rename only (no Save/Saved)", async () => {
    const tab = { id: "tmux:%0", name: "main:0.0", kind: "local" as const };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("main:0.0") });

    expect(screen.getByText("Rename")).toBeInTheDocument();
    expect(screen.queryByText("Save Terminal")).not.toBeInTheDocument();
    expect(screen.queryByText("Saved")).not.toBeInTheDocument();
  });

  it("unsaved remote tab offers Rename and Save Terminal", async () => {
    const tab = {
      id: "ssh:abc",
      name: "ubuntu@10.0.1.5",
      kind: "ssh" as const,
      host: "10.0.1.5",
      port: 22,
      remote: {
        host: "10.0.1.5",
        port: 22,
        username: "ubuntu",
        authType: "password" as const,
        privateKeyPath: "",
        useTmux: false,
        tmuxSessionName: "",
      },
    };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("ubuntu@10.0.1.5") });

    expect(screen.getByText("Rename")).toBeInTheDocument();
    expect(screen.getByText("Save Terminal")).toBeInTheDocument();
    expect(screen.queryByText("Saved")).not.toBeInTheDocument();
  });

  it("saved remote tab shows a disabled 'Saved' item instead of 'Save Terminal'", async () => {
    const tab = {
      id: "ssh:abc",
      name: "Prod Web Server",
      kind: "ssh" as const,
      savedHostId: "host-id-1",
      host: "10.0.1.5",
      port: 22,
    };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("Prod Web Server") });

    const saved = screen.getByText("Saved");
    expect(saved).toBeInTheDocument();
    expect(screen.queryByText("Save Terminal")).not.toBeInTheDocument();
    // It must be non-selectable (disabled / pointer-events-none per the styling).
    expect(saved.closest("[data-disabled]")).not.toBeNull();
  });

  it("'Save Terminal' persists metadata only (no secrets) and flips the tab to saved", async () => {
    useTerminalStore.getState().addTab(
      "ssh:abc",
      "ubuntu@10.0.1.5",
      false,
      undefined,
      "ssh",
      undefined,
      "10.0.1.5",
      22,
      {
        host: "10.0.1.5",
        port: 22,
        username: "ubuntu",
        authType: "password" as const,
        privateKeyPath: "",
        useTmux: true,
        tmuxSessionName: "work",
      }
    );
    const tab = {
      id: "ssh:abc",
      name: "ubuntu@10.0.1.5",
      kind: "ssh" as const,
      host: "10.0.1.5",
      port: 22,
      remote: {
        host: "10.0.1.5",
        port: 22,
        username: "ubuntu",
        authType: "password" as const,
        privateKeyPath: "",
        useTmux: true,
        tmuxSessionName: "work",
      },
    };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("ubuntu@10.0.1.5") });
    await user.click(screen.getByText("Save Terminal"));

    await vi.waitFor(() => expect(saveRemoteHost).toHaveBeenCalledTimes(1));
    const [record, password, passphrase] = saveRemoteHost.mock.calls[0];
    expect(record).toMatchObject({
      Host: "10.0.1.5",
      Port: 22,
      Username: "ubuntu",
      AuthType: "password",
      Name: "ubuntu@10.0.1.5",
      UseTmux: true,
      TmuxSessionName: "work",
      Kind: "ssh",
    });
    // No credentials may be passed — metadata only.
    expect(password).toBe("");
    expect(passphrase).toBe("");
    // The returned ID is written back onto the tab so the menu flips to "Saved".
    expect(useTerminalStore.getState().tabs[0].savedHostId).toBe("saved-host-99");
  });

  it("'Save Terminal' is not offered for a tab with no remote metadata", async () => {
    const tab = { id: "ssh:orphan", name: "orphan@10.0.1.5", kind: "ssh" as const };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("orphan@10.0.1.5") });

    expect(screen.queryByText("Save Terminal")).not.toBeInTheDocument();
    expect(saveRemoteHost).not.toHaveBeenCalled();
  });

  // Test 4: renders warning icon when tab.degraded is true
  it("renders warning badge when tab is degraded", () => {
    const tab = {
      id: "atspi::1.200/org/a11y/atspi/accessible/0",
      name: "Konsole",
      degraded: true,
      degradedMsg: "Konsole text extraction not available on this system.",
    };
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    // The warning icon button should be present (Tooltip.Trigger renders as a button)
    const warningButton = screen.getByText("⚠");
    expect(warningButton).toBeInTheDocument();
    // Tab name should still be visible
    expect(screen.getByText("Konsole")).toBeInTheDocument();
  });

  it("does NOT render warning badge for non-degraded tabs", () => {
    const tab = {
      id: "tmux:%0",
      name: "main:0.0",
      degraded: false,
    };
    render(<TerminalTab tab={tab} isActive={true} onClick={vi.fn()} />);

    const button = screen.getByRole("button");
    expect(button).toBeInTheDocument();
    // No warning icon
    expect(button.textContent).not.toMatch(/⚠/);
  });

  it("renders tab name in button", () => {
    const tab = { id: "tmux:%0", name: "main:0.0" };
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);
    expect(screen.getByText("main:0.0")).toBeInTheDocument();
  });

  it("right-click shows a Rename option", async () => {
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("main:0.0") });

    expect(screen.getByText("Rename")).toBeInTheDocument();
  });

  it("renaming via the context menu updates the tab name in terminalStore", async () => {
    useTerminalStore.getState().addTab("tmux:%0", "main:0.0");
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("main:0.0") });
    await user.click(screen.getByText("Rename"));
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "My Custom Name{Enter}");

    expect(useTerminalStore.getState().tabs[0].name).toBe("My Custom Name");
  });

  it("renaming a tab backed by a saved host also persists the name via RenameRemoteHost", async () => {
    useTerminalStore.getState().addTab("ssh:abc", "user@192.0.2.10", false, undefined, "ssh", "host-id-1");
    const tab = { id: "ssh:abc", name: "user@192.0.2.10", savedHostId: "host-id-1" };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("user@192.0.2.10") });
    await user.click(screen.getByText("Rename"));
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "Prod Web Server{Enter}");

    expect(useTerminalStore.getState().tabs[0].name).toBe("Prod Web Server");
    expect(renameRemoteHost).toHaveBeenCalledWith("host-id-1", "Prod Web Server");
  });

  it("renaming a tab with no savedHostId does not call RenameRemoteHost", async () => {
    useTerminalStore.getState().addTab("tmux:%0", "main:0.0");
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    await user.pointer({ keys: "[MouseRight]", target: screen.getByText("main:0.0") });
    await user.click(screen.getByText("Rename"));
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "Local Session{Enter}");

    expect(useTerminalStore.getState().tabs[0].name).toBe("Local Session");
    expect(renameRemoteHost).not.toHaveBeenCalled();
  });

  it("wraps the tab name in a tooltip without turning it into its own interactive button", () => {
    // Regression guard: base-ui's Tooltip.Trigger defaults to rendering a
    // <button> unless overridden — that would both introduce a second
    // "button" role (breaking the "does NOT render warning badge" test's
    // getByRole("button") uniqueness assumption above) and could interfere
    // with the row's own click/right-click handling.
    const tab = { id: "tmux:%0", name: "a-very-long-session-name-that-gets-truncated" };
    render(<TerminalTab tab={tab} isActive={false} onClick={vi.fn()} />);

    expect(screen.getAllByRole("button")).toHaveLength(1); // just the close (×) button
    expect(screen.getByText(tab.name).tagName).toBe("SPAN");
  });

  it("clicking the tab still triggers onClick (not blocked by the context menu wrapper)", async () => {
    const tab = { id: "tmux:%0", name: "main:0.0" };
    const onClick = vi.fn();
    const user = userEvent.setup();
    render(<TerminalTab tab={tab} isActive={false} onClick={onClick} />);

    await user.click(screen.getByText("main:0.0"));

    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
