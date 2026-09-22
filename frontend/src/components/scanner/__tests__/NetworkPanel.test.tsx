import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import { NetworkPanel } from "@/components/scanner/NetworkPanel";
import { useScannerStore } from "@/stores/scannerStore";

const getSettings = vi.fn();
const scanStart = vi.fn();
const scanStop = vi.fn();
const openRemoteTerminal = vi.fn();

// Shim for the real NewTerminalDialog: expose the `initial` pre-fill it was
// asked to open with, so the panel test asserts the "Add to hosts" action
// routes to the dialog (never to a direct connect).
vi.mock("@/components/terminal/NewTerminalDialog", () => ({
  NewTerminalDialog: ({
    open,
    initial,
  }: {
    open: boolean;
    initial?: { kind: "ssh"; host: string; port: number };
  }) =>
    open ? (
      <div data-testid="new-terminal-dialog" data-host={initial?.host ?? ""} data-port={initial?.port ?? ""}>
        new terminal
      </div>
    ) : null,
}));

vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
}));
vi.mock("../../../../wailsjs/go/scanner/ScanService", () => ({
  Start: (...args: unknown[]) => scanStart(...args),
  Stop: (...args: unknown[]) => scanStop(...args),
}));
vi.mock("../../../../wailsjs/go/services/PTYService", () => ({
  OpenRemoteTerminal: (...args: unknown[]) => openRemoteTerminal(...args),
}));
vi.mock("../../../../wailsjs/runtime/runtime", async () => ({
  EventsOn: () => vi.fn(),
}));

const idleState = {
  activeScanID: null,
  status: "idle" as const,
  progress: { done: 0, total: 0, activeProbes: 0 },
  rows: [],
  stats: null,
  error: "",
};

beforeEach(() => {
  vi.clearAllMocks();
  getSettings.mockResolvedValue({ ScannerEnabled: true });
  useScannerStore.setState(idleState);
});

describe("NetworkPanel", () => {
  it("renders nothing (no panel, no entry point) when the scanner is disabled", async () => {
    getSettings.mockResolvedValue({ ScannerEnabled: false });
    const { container } = render(<NetworkPanel />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(container).toBeEmpty();
    expect(screen.queryByText(/Network/)).toBeNull();
    // Mutation check: if the disabled branch stopped returning null (rendering
    // a header/entry point anyway), this fails — the panel would leak into the
    // sidebar even with "Network scanner" off in Settings.
  });

  it("shows the Search control when the scanner is enabled", async () => {
    render(<NetworkPanel />);
    expect(await screen.findByRole("button", { name: /^search$/i })).toBeInTheDocument();
  });

  it("renders the three-row search layout: targets (row 1), ports (row 2), Search button (row 3)", async () => {
    render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });
    const targets = screen.getByLabelText(/Scan target CIDR/);
    const ports = screen.getByLabelText(/SSH ports to scan/);
    const search = screen.getByRole("button", { name: /^search$/i });
    expect(targets).toBeInTheDocument();
    expect(ports).toBeInTheDocument();
    expect(search).toBeInTheDocument();
    // Targets AND ports each get a full-width line (w-full from the shared
    // inputClass), so the Search button sits on its own row — the ports input
    // no longer shares a row with the button.
    expect(targets.className).toContain("w-full");
    expect(ports.className).toContain("w-full");
    // Mutation check: if the single-row / two-row layouts were reintroduced
    // (ports no longer w-full, or the Search button tucked beside it), the
    // w-full assertion on ports fails.
  });

  it("labels the ports input with the 'SSH Ports (22)' placeholder", async () => {
    render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });
    expect(screen.getByPlaceholderText(/SSH Ports \(22\)/)).toBeInTheDocument();
    // Mutation check: reverting the placeholder to "Ports (22)" makes this
    // fail — the input must read as SSH-specific, not a generic port scanner.
  });

  it("renders the panel header as 'Network Finder'", async () => {
    render(<NetworkPanel />);
    expect(await screen.findByRole("button", { name: /Network Finder/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^network$/i })).toBeNull();
    // Mutation check: if the header said just "Network", the anchored
    // /^network$/i query would find it — this fails (we're a host finder, not
    // a port scanner).
  });

  it("hides the panel when settings carry no scanner flag (default off)", async () => {
    // A fresh/absent config has no scanner_enabled → treated as disabled.
    getSettings.mockResolvedValue({});
    const { container } = render(<NetworkPanel />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(container).toBeEmpty();
    expect(screen.queryByText(/Network/)).toBeNull();
    // Mutation check: if the absent-value path defaulted to enabled, this panel
    // would render — but the scanner now defaults OFF, so absent must hide it.
  });

  it("fails CLOSED when GetSettings itself rejects", async () => {
    getSettings.mockRejectedValue(new Error("settings unavailable"));
    const { container } = render(<NetworkPanel />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(container).toBeEmpty();
    expect(screen.queryByText(/Network/)).toBeNull();
    // Mutation check: if the catch branch set enabled=true (fail-open), the
    // panel would render the scan UI even though we couldn't read config.
  });

  it("renders ssh hosts but HIDES non-actionable (filtered/closed) IPs", async () => {
    useScannerStore.setState({
      rows: [
        {
          ip: "10.0.0.3",
          port: 22,
          state: "ssh",
          ssh: {
            port: 22,
            banner: "SSH-2.0-OpenSSH_8.9p1",
            software: "OpenSSH_8.9p1",
            hostKeyType: "ssh-ed25519",
            hostKeyFingerprint: "SHA256:abc",
          },
        },
        { ip: "10.0.0.4", port: 22241, state: "filtered" },
        { ip: "10.0.0.5", port: 22, state: "closed" },
      ],
    });
    render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });
    // Actionable ssh row renders its IP + software line.
    expect(screen.getByText("10.0.0.3")).toBeInTheDocument();
    expect(screen.getByText("OpenSSH_8.9p1 · 22")).toBeInTheDocument();
    // Non-actionable rows contribute to the summary counts ONLY — their
    // individual IPs / states / ports are NOT rendered as rows.
    expect(screen.queryByText("10.0.0.4")).toBeNull();
    expect(screen.queryByText("10.0.0.5")).toBeNull();
    expect(screen.queryByText("filtered")).toBeNull();
    expect(screen.queryByText("closed")).toBeNull();
    expect(screen.queryByText("Port 22241")).toBeNull();
    // Mutation check: if every row were rendered again (filtered/closed shown
    // as their own IP lines), 10.0.0.4 / "filtered" / "Port 22241" would
    // appear — this fails. Only actionable ssh/ssh-known rows render.
  });

  it("caps the results list height with an internal scroll so a sweep can't push '+ Connect' off", async () => {
    useScannerStore.setState({
      rows: [
        { ip: "10.0.0.10", port: 22, state: "ssh", ssh: { port: 22, banner: "SSH-2.0-x", software: "OpenSSH 9.6", hostKeyType: "ssh-ed25519", hostKeyFingerprint: "SHA256:x" } },
        { ip: "10.0.0.11", port: 22, state: "filtered" },
        { ip: "10.0.0.12", port: 22, state: "closed" },
      ],
    });
    const { container } = render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });

    const results = container.querySelector(".max-h-64");
    if (!results) {
      // Mutation check: if the max-height/scroll classes were dropped from the
      // results container, a large sweep would grow the panel unbounded and push
      // "+ Connect" below the bottom of the overflow-hidden aside — this fails.
      throw new Error("results container (.max-h-64) is missing");
    }
    expect(results.className).toContain("space-y-1");
    expect(results.className).toContain("max-h-64");
    expect(results.className).toContain("overflow-y-auto");
  });

  it("Stop calls the Stop binding for the active scan", async () => {
    useScannerStore.setState({
      status: "scanning",
      activeScanID: "scan-1",
      progress: { done: 4, total: 16, activeProbes: 8 },
    });
    const user = userEvent.setup();
    render(<NetworkPanel />);
    const stop = await screen.findByRole("button", { name: /^stop$/i });
    await user.click(stop);
    expect(scanStop).toHaveBeenCalledWith("scan-1");
    // Mutation check: dropping the Stop button's onClick (or its stopScan
    // wiring) leaves the active scan running with no user way to halt it.
  });

  it("Scan starts a scan with the parsed CIDR targets and default ports", async () => {
    const user = userEvent.setup();
    render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });
    await user.type(screen.getByLabelText(/Scan target CIDR/), "10.0.1.0/24,10.0.2.0/24");
    await user.click(screen.getByRole("button", { name: /^search$/i }));
    expect(scanStart).toHaveBeenCalledWith({
      targets: ["10.0.1.0/24", "10.0.2.0/24"],
      ports: [],
      maxProbes: 16,
    });
    // A blank Ports input must forward as [] (backend default [22]) — never a
    // hardcoded port leaking from the frontend.
  });

  it("Scan parses the Ports input (list + ranges) into the port array", async () => {
    const user = userEvent.setup();
    render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });
    await user.type(screen.getByLabelText(/SSH ports to scan/), "22241-22242,22");
    await user.click(screen.getByRole("button", { name: /^search$/i }));
    expect(scanStart).toHaveBeenCalledWith({
      targets: [],
      ports: [22241, 22242, 22],
      maxProbes: 16,
    });
    // Mutation check: if the Scan handler kept calling startScan with a blank
    // port list (ignoring the Ports input), this fails — a NAT'd host on
    // 22241-22250 would never be found.
  });

  it("an empty Scan target scans the local /24 networks", async () => {
    const user = userEvent.setup();
    render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });
    await user.click(screen.getByRole("button", { name: /^search$/i }));
    expect(scanStart).toHaveBeenCalledWith({ targets: [], ports: [], maxProbes: 16 });
    // Mutation check: if parseTargets replaced the empty result with some
    // non-empty default, the local-/24 sweep the backend performs on an empty
    // targets array would never run.
  });

  it("collapses spaces around a dash so a spaced target range parses as one token", async () => {
    const user = userEvent.setup();
    render(<NetworkPanel />);
    await screen.findByRole("button", { name: /^search$/i });
    await user.type(
      screen.getByLabelText(/Scan target CIDR/),
      "192.168.5 - 192.168.15",
    );
    await user.click(screen.getByRole("button", { name: /^search$/i }));
    // "192.168.5 - 192.168.15" must reach the backend as the single range
    // token "192.168.5-192.168.15" — never a junk lone "-" token.
    expect(scanStart).toHaveBeenCalledWith({
      targets: ["192.168.5-192.168.15"],
      ports: [],
      maxProbes: 16,
    });
    // Mutation check: without dash-space normalization the input splits into
    // ["192.168.5", "-", "192.168.15"] and this toHaveBeenCalledWith fails.
  });

  it("Add to hosts pre-fills the terminal dialog and never connects directly", async () => {
    useScannerStore.setState({
      rows: [
        {
          ip: "10.0.0.9",
          port: 22,
          state: "ssh",
          ssh: {
            port: 22,
            banner: "SSH-2.0-x",
            software: "OpenSSH 9.6",
            hostKeyType: "ssh-ed25519",
            hostKeyFingerprint: "SHA256:x",
          },
        },
      ],
    });
    const user = userEvent.setup();
    render(<NetworkPanel />);
    const add = await screen.findByRole("button", { name: /Add to hosts/i });
    await user.click(add);

    const dialog = await screen.findByTestId("new-terminal-dialog");
    expect(dialog).toHaveAttribute("data-host", "10.0.0.9");
    expect(dialog).toHaveAttribute("data-port", "22");
    expect(openRemoteTerminal).not.toHaveBeenCalled();
    // Mutation check: if handleAddToHosts bypassed the dialog and dialed the
    // discovered host directly, openRemoteTerminal would fire automatically.
    // The action must route through the existing NewTerminalDialog trust flow,
    // where host-key trust is confirmed — never auto-trusted from a scan.
  });
});