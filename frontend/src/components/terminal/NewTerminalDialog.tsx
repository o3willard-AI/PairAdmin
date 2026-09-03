import { useEffect, useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
import { useTerminalStore } from "@/stores/terminalStore";
import { wailsErrorMessage } from "@/utils/wailsError";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { AlertTriangle } from "lucide-react";
import type { config, services } from "../../../wailsjs/go/models";

export interface NewTerminalDialogProps {
  open: boolean;
  onClose: () => void;
}

type TerminalKind = "local" | "ssh" | "winrm";
type AuthType = "password" | "privatekey";

// How many saved hosts to show inline under "Recent" before requiring a click
// through to the searchable "Show All" view — ListRemoteHosts is already
// sorted by LastUsed descending, so these are always the most-recent ones.
const RECENT_INLINE_CAP = 6;
const RECENT_ALL_PAGE_SIZE = 20;

const inputClass =
  "w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none";

const typeCardClass =
  "w-full text-left px-4 py-3 rounded border border-surface-border-strong hover:bg-surface-2 text-sm text-surface-text transition-colors";

// Mirrors defaultTmuxSessionName in services/remote_ssh.go — shown only as a
// placeholder hint; the actual default is applied backend-side if left blank.
const defaultTmuxSessionNamePlaceholder = "pairadmin (default)";

interface ConnectRemoteParams {
  kind: TerminalKind;
  host: string;
  port: number;
  username: string;
  authType: AuthType;
  password: string;
  privateKeyPath: string;
  passphrase: string;
  saveTerminal: boolean;
  useTmux: boolean;
  tmuxSessionName: string;
  savedHostId?: string;
  /** Friendly name from the saved host record, if reconnecting to one that
   * was previously renamed. Falls back to "username@host" when absent. */
  name?: string;
  /** Set only on the retry issued right after the user accepted an
   * unrecognized host key in the "hostKeyConfirm" step — see
   * maybeConnectToRemote. */
  trustNewHostKey?: boolean;
}

interface HostKeyConfirmState {
  params: ConnectRemoteParams;
  keyType: string;
  fingerprint: string;
  /** True when this host:port previously had a DIFFERENT pinned key — the
   * real MITM-suspect case, not just an unseen-before host. The backend
   * refuses to connect in this case no matter what the user clicks here, so
   * the UI doesn't offer an Accept button for it. */
  changed: boolean;
}

export function NewTerminalDialog({ open, onClose }: NewTerminalDialogProps) {
  const [step, setStep] = useState<"type" | "form" | "recentAll" | "hostKeyConfirm">("type");
  const [kind, setKind] = useState<TerminalKind>("local");
  const [recentSearch, setRecentSearch] = useState("");
  const [recentPage, setRecentPage] = useState(0);
  const [promptNewHostKeys, setPromptNewHostKeys] = useState(false);
  const [hostKeyConfirm, setHostKeyConfirm] = useState<HostKeyConfirmState | null>(null);

  const [host, setHost] = useState("");
  const [port, setPort] = useState(22);
  const [username, setUsername] = useState("");
  const [authType, setAuthType] = useState<AuthType>("password");
  const [password, setPassword] = useState("");
  const [privateKeyPath, setPrivateKeyPath] = useState("");
  const [passphrase, setPassphrase] = useState("");
  const [saveTerminal, setSaveTerminal] = useState(false);
  const [useTmux, setUseTmux] = useState(false);
  const [tmuxSessionName, setTmuxSessionName] = useState("");

  const [recentHosts, setRecentHosts] = useState<services.RemoteHostStatus[]>([]);
  const [connectStatus, setConnectStatus] = useState<"idle" | "connecting" | "error">("idle");
  const [connectError, setConnectError] = useState("");
  const [saveWarning, setSaveWarning] = useState("");
  // When a no-credential saved host's "Connect" routes through the form (instead
  // of dialing directly), we keep its full record here so the form's connect can
  // upsert the entered credential onto that SAME record rather than creating a
  // duplicate Recent entry. Null when connecting a fresh host or a credentialed one.
  const [reconnectHost, setReconnectHost] = useState<config.RemoteHost | null>(null);

  const refreshRecentHosts = () => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/RemoteService")
      .then(({ ListRemoteHostsWithStatus }) => ListRemoteHostsWithStatus())
      .then((statuses: services.RemoteHostStatus[]) => setRecentHosts(statuses || []))
      .catch(() => setRecentHosts([]));
  };

  // Reset the wizard every time the dialog opens, and refresh the recent-hosts list.
  useEffect(() => {
    if (!open) return;
    setStep("type");
    setKind("local");
    setHost("");
    setPort(22);
    setUsername("");
    setAuthType("password");
    setPassword("");
    setPrivateKeyPath("");
    setPassphrase("");
    setSaveTerminal(false);
    setUseTmux(false);
    setTmuxSessionName("");
    setConnectStatus("idle");
    setConnectError("");
    setSaveWarning("");
    setReconnectHost(null);
    setRecentSearch("");
    setRecentPage(0);
    setHostKeyConfirm(null);

    refreshRecentHosts();

    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => setPromptNewHostKeys(!!cfg?.PromptNewHostKeys))
      .catch(() => setPromptNewHostKeys(false));
  }, [open]);

  const handleForget = async (id: string) => {
    try {
      const { ForgetRemoteHost } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/RemoteService"
      );
      await ForgetRemoteHost(id);
      refreshRecentHosts();
    } catch (err) {
      setSaveWarning(wailsErrorMessage(err, "Failed to remove saved host"));
    }
  };

  const handleConnectLocal = async () => {
    setConnectStatus("connecting");
    try {
      const { OpenNewTerminal } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/PTYService"
      );
      const id = crypto.randomUUID();
      const resolvedId = await OpenNewTerminal(id);
      if (resolvedId) {
        const store = useTerminalStore.getState();
        const num = store.takeNextTabNumber();
        store.addTab(resolvedId, `Terminal ${num}`, false, undefined, "local");
        store.setActiveTab(resolvedId);
      }
      onClose();
    } catch (err) {
      setConnectStatus("error");
      setConnectError(wailsErrorMessage(err, "Failed to open terminal"));
    }
  };

  const selectKind = (k: TerminalKind) => {
    if (k === "local") {
      handleConnectLocal();
      return;
    }
    // Picking a type freshly (not the no-credential reconnect flow) always
    // starts a brand-new connection — drop any carried-over saved-host target.
    setReconnectHost(null);
    setKind(k);
    setPort(k === "ssh" ? 22 : 5985);
    setStep("form");
  };

  const applyHostToForm = (h: config.RemoteHost) => {
    setKind(h.Kind as TerminalKind);
    setHost(h.Host);
    setPort(h.Port);
    setUsername(h.Username);
    setAuthType(h.AuthType as AuthType);
    setPrivateKeyPath(h.PrivateKeyPath || "");
    setUseTmux(h.UseTmux || false);
    setTmuxSessionName(h.TmuxSessionName || "");
  };

  // Connection details are passed explicitly rather than read from component
  // state, since the one-click "recent hosts" reconnect calls this in the same
  // event handler as applyHostToForm() — React state updates from that call
  // haven't re-rendered yet, so reading `host`/`username` etc. here would see
  // stale (pre-update) values.
  const connectToRemote = async (params: ConnectRemoteParams) => {
    setConnectStatus("connecting");
    setConnectError("");
    setSaveWarning("");
    try {
      const { OpenRemoteTerminal } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/PTYService"
      );
      const { SaveRemoteHost, TouchRemoteHost } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/RemoteService"
      );

      const tabId = `${params.kind}:${crypto.randomUUID()}`;
      const resolvedId = await OpenRemoteTerminal(tabId, {
        kind: params.kind,
        host: params.host,
        port: params.port,
        username: params.username,
        authType: params.authType,
        password: params.authType === "password" ? params.password : "",
        privateKeyPath: params.authType === "privatekey" ? params.privateKeyPath : "",
        passphrase: params.authType === "privatekey" ? params.passphrase : "",
        savePassword: params.saveTerminal,
        savedHostId: params.savedHostId || "",
        useTmux: params.kind === "ssh" ? params.useTmux : false,
        tmuxSessionName: params.kind === "ssh" ? params.tmuxSessionName : "",
        trustNewHostKey: params.kind === "ssh" ? !!params.trustNewHostKey : false,
      });

      // The primary connection above already succeeded — a failure past this
      // point (saving/touching the saved-host record) must not be swallowed
      // silently, but also must not undo the now-open terminal tab. Surface
      // it as a non-fatal warning instead. `warning` is a local (not state)
      // so the "should we auto-close" check below sees it immediately —
      // reading the `saveWarning` state variable here would still be its
      // stale pre-update value, since setState doesn't apply synchronously.
      let warning = "";
      // The tab's savedHostId — carried over on reconnect, or picked up from
      // a fresh SaveRemoteHost's generated ID so this tab's rename (below)
      // can persist even on its very first session, not just later ones.
      let savedHostId = params.savedHostId;
      if (params.savedHostId && params.saveTerminal) {
        // Re-saving a host that had no stored credential (the no-credential
        // connect flow routed through the form): upsert the entered password
        // onto the EXISTING record by ID so we don't create a duplicate Recent
        // entry. SaveRemoteHost is an upsert-by-ID on the backend.
        try {
          const saved = await SaveRemoteHost(
            {
              ID: params.savedHostId,
              Kind: params.kind,
              Name: params.name || "",
              Host: params.host,
              Port: params.port,
              Username: params.username,
              AuthType: params.authType,
              PrivateKeyPath: params.authType === "privatekey" ? params.privateKeyPath : "",
              LastUsed: "",
              UseTmux: params.kind === "ssh" ? params.useTmux : false,
              TmuxSessionName: params.kind === "ssh" ? params.tmuxSessionName : "",
            },
            params.authType === "password" ? params.password : "",
            params.authType === "privatekey" ? params.passphrase : ""
          );
          savedHostId = saved.ID;
        } catch (err) {
          warning = wailsErrorMessage(err, "Connected, but failed to save this connection for later");
        }
      } else if (params.savedHostId) {
        try {
          await TouchRemoteHost(params.savedHostId);
        } catch (err) {
          warning = wailsErrorMessage(err, "Failed to update last-used time for this saved host");
        }
      } else if (params.saveTerminal) {
        try {
          const saved = await SaveRemoteHost(
            {
              ID: "",
              Kind: params.kind,
              Name: "",
              Host: params.host,
              Port: params.port,
              Username: params.username,
              AuthType: params.authType,
              PrivateKeyPath: params.authType === "privatekey" ? params.privateKeyPath : "",
              LastUsed: "",
              UseTmux: params.kind === "ssh" ? params.useTmux : false,
              TmuxSessionName: params.kind === "ssh" ? params.tmuxSessionName : "",
            },
            params.authType === "password" ? params.password : "",
            params.authType === "privatekey" ? params.passphrase : ""
          );
          savedHostId = saved.ID;
        } catch (err) {
          warning = wailsErrorMessage(err, "Connected, but failed to save this connection for later");
        }
      }
      if (warning) setSaveWarning(warning);

      const store = useTerminalStore.getState();
      const degraded = params.kind === "winrm";
      const degradedMsg =
        params.kind === "winrm" ? "WinRM: command/response only — not a live shell" : undefined;
      const displayName = params.name || `${params.username}@${params.host}`;
      store.addTab(
        resolvedId,
        displayName,
        degraded,
        degradedMsg,
        params.kind,
        savedHostId,
        // Host/port drive the hover tooltip's second line; the remote block
        // carries the non-secret metadata needed by the tab context menu's
        // "Save Terminal" action. Credentials are never stored on the tab.
        params.host,
        params.port,
        {
          host: params.host,
          port: params.port,
          username: params.username,
          authType: params.authType,
          privateKeyPath: params.authType === "privatekey" ? params.privateKeyPath : "",
          useTmux: params.kind === "ssh" ? params.useTmux : false,
          tmuxSessionName: params.kind === "ssh" ? params.tmuxSessionName : "",
        }
      );
      store.setActiveTab(resolvedId);

      // Only auto-close on a clean save; if the save/touch step warned, leave
      // the dialog open long enough for the user to actually see the warning.
      if (!warning) onClose();
    } catch (err) {
      setConnectStatus("error");
      setConnectError(wailsErrorMessage(err, "Connection failed"));
    }
  };

  // Routes a connection attempt through the SSH host-key confirmation prompt
  // when the user has PromptNewHostKeys enabled and this host:port hasn't
  // been seen (with a matching key) before — otherwise connects directly.
  // openSSHTerminal itself always still pins/verifies the key either way;
  // this only decides whether the user is asked about it up front.
  const maybeConnectToRemote = async (params: ConnectRemoteParams) => {
    if (params.kind !== "ssh" || !promptNewHostKeys) {
      connectToRemote(params);
      return;
    }
    try {
      const { CheckHostKeyTrust } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/PTYService"
      );
      const status = await CheckHostKeyTrust(params.host, params.port);
      if (status.known) {
        connectToRemote(params);
        return;
      }
      setHostKeyConfirm({
        params,
        keyType: status.keyType,
        fingerprint: status.fingerprint,
        changed: status.changed,
      });
      setStep("hostKeyConfirm");
    } catch {
      // Couldn't even reach the host to check its key — fall through to the
      // real connection attempt, which will surface a clearer network error.
      connectToRemote(params);
    }
  };

  const handleConnectRemote = () =>
    maybeConnectToRemote({
      kind,
      host,
      port,
      username,
      authType,
      password,
      privateKeyPath,
      passphrase,
      saveTerminal,
      useTmux,
      tmuxSessionName,
      // When this form was reached via a no-credential saved host's "Connect",
      // carry that host's ID + friendly name through so the connect upserts the
      // entered credential onto the same record (no duplicate).
      savedHostId: reconnectHost?.ID || undefined,
      name: reconnectHost?.Name || undefined,
    });

  const handleReconnect = (h: config.RemoteHost) => {
    applyHostToForm(h);
    maybeConnectToRemote({
      kind: h.Kind as TerminalKind,
      host: h.Host,
      port: h.Port,
      username: h.Username,
      authType: h.AuthType as AuthType,
      password: "",
      privateKeyPath: h.PrivateKeyPath || "",
      passphrase: "",
      saveTerminal: false,
      useTmux: h.UseTmux || false,
      tmuxSessionName: h.TmuxSessionName || "",
      savedHostId: h.ID,
      name: h.Name || undefined,
    });
  };

  const handleAcceptHostKey = () => {
    if (!hostKeyConfirm) return;
    connectToRemote({ ...hostKeyConfirm.params, trustNewHostKey: true });
  };

  const handleRejectHostKey = () => {
    setHostKeyConfirm(null);
    setStep("form");
  };

  const renderHostRow = (st: services.RemoteHostStatus) => {
    const h = st.host;
    return (
      <div
        key={h.ID}
        className="flex items-center justify-between px-3 py-2 rounded border border-surface-border hover:bg-surface-2"
      >
        <div className="text-sm text-surface-text truncate">
          {h.Name || `${h.Username}@${h.Host}`}
          <span className="text-xs text-surface-text-muted ml-2">{h.Kind}</span>
          {!st.hasCredential && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger render={<span className="ml-2 inline-flex text-amber-500 align-middle" />}>
                  <AlertTriangle size={13} />
                </TooltipTrigger>
                <TooltipContent side="right" className="max-w-xs">
                  No stored credential — you&apos;ll be prompted to authenticate on connect
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button
            className="text-xs bg-surface-3 hover:bg-surface-3/80 text-surface-text px-2 py-1 rounded disabled:opacity-50"
            disabled={connectStatus === "connecting"}
            onClick={() => {
              if (st.hasCredential) {
                // A credential is stored — dial directly with the empty password
                // the keychain fills in at connect time.
                handleReconnect(h);
              } else {
                // Metadata-only host (amber "No stored credential" indicator):
                // route through the form so the user can be prompted to enter a
                // credential, pre-filling the connection and pre-checking
                // "Save Terminal" so it persists to this same record.
                applyHostToForm(h);
                setSaveTerminal(true);
                setReconnectHost(h);
                setStep("form");
              }
            }}
          >
            Connect
          </button>
          <button
            className="text-xs text-surface-text-muted hover:text-red-400 px-1.5 py-1"
            aria-label={`Forget saved host ${h.Username}@${h.Host}`}
            onClick={() => handleForget(h.ID)}
          >
            &times;
          </button>
        </div>
      </div>
    );
  };

  const filteredRecentHosts = recentHosts.filter((st) => {
    const h = st.host;
    const q = recentSearch.trim().toLowerCase();
    if (!q) return true;
    const haystack = `${h.Name || ""} ${h.Username}@${h.Host} ${h.Kind}`.toLowerCase();
    return haystack.includes(q);
  });
  const recentAllPageCount = Math.max(1, Math.ceil(filteredRecentHosts.length / RECENT_ALL_PAGE_SIZE));
  const clampedRecentPage = Math.min(recentPage, recentAllPageCount - 1);
  const pagedRecentHosts = filteredRecentHosts.slice(
    clampedRecentPage * RECENT_ALL_PAGE_SIZE,
    (clampedRecentPage + 1) * RECENT_ALL_PAGE_SIZE
  );

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[480px] max-h-[80vh] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface-1 border border-surface-border-strong shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-surface-text border-b border-surface-border">
            New Terminal
          </Dialog.Title>
          <div className="flex-1 overflow-y-auto p-6 space-y-4">
            {step === "type" && (
              <div className="space-y-2">
                <button onClick={() => selectKind("local")} className={typeCardClass}>
                  <div className="font-medium">Local</div>
                  <div className="text-xs text-surface-text-muted">Open a shell on this machine</div>
                </button>
                <button onClick={() => selectKind("ssh")} className={typeCardClass}>
                  <div className="font-medium">Unix / Linux (SSH)</div>
                  <div className="text-xs text-surface-text-muted">Connect to a remote host over SSH</div>
                </button>
                <button onClick={() => selectKind("winrm")} className={typeCardClass}>
                  <div className="font-medium">Remote Windows (WinRM)</div>
                  <div className="text-xs text-surface-text-muted">
                    Command/response only — not a live shell
                  </div>
                </button>

                {recentHosts.length > 0 && (
                  <div className="pt-3">
                    <div className="text-xs text-surface-text-muted uppercase tracking-wider mb-1">
                      Recent
                    </div>
                    <div className="space-y-1">
                      {recentHosts.slice(0, RECENT_INLINE_CAP).map(renderHostRow)}
                    </div>
                    {recentHosts.length > RECENT_INLINE_CAP && (
                      <button
                        onClick={() => {
                          setRecentSearch("");
                          setRecentPage(0);
                          setStep("recentAll");
                        }}
                        className="w-full mt-1 text-xs text-surface-text-muted hover:text-surface-text px-3 py-1.5 rounded border border-surface-border hover:bg-surface-2"
                      >
                        Show All ({recentHosts.length})
                      </button>
                    )}
                  </div>
                )}

                {saveWarning && (
                  <p className="text-xs text-amber-400">{saveWarning}</p>
                )}
                {connectStatus === "error" && (
                  <p className="text-xs text-red-400">{connectError}</p>
                )}
              </div>
            )}

            {step === "recentAll" && (
              <div className="space-y-3">
                <input
                  autoFocus
                  className={inputClass}
                  value={recentSearch}
                  onChange={(e) => {
                    setRecentSearch(e.target.value);
                    setRecentPage(0);
                  }}
                  placeholder="Filter by name, host, or username..."
                />
                <div className="space-y-1">
                  {pagedRecentHosts.length > 0 ? (
                    pagedRecentHosts.map(renderHostRow)
                  ) : (
                    <p className="text-surface-text-muted text-xs text-center py-4">
                      No saved connections match "{recentSearch}"
                    </p>
                  )}
                </div>
                {recentAllPageCount > 1 && (
                  <div className="flex items-center justify-between text-xs text-surface-text-muted">
                    <button
                      className="px-2 py-1 rounded hover:bg-surface-2 disabled:opacity-40"
                      disabled={clampedRecentPage === 0}
                      onClick={() => setRecentPage((p) => Math.max(0, p - 1))}
                    >
                      &larr; Prev
                    </button>
                    <span>
                      Page {clampedRecentPage + 1} of {recentAllPageCount}
                    </span>
                    <button
                      className="px-2 py-1 rounded hover:bg-surface-2 disabled:opacity-40"
                      disabled={clampedRecentPage >= recentAllPageCount - 1}
                      onClick={() => setRecentPage((p) => Math.min(recentAllPageCount - 1, p + 1))}
                    >
                      Next &rarr;
                    </button>
                  </div>
                )}

                {saveWarning && (
                  <p className="text-xs text-amber-400">{saveWarning}</p>
                )}
                {connectStatus === "error" && (
                  <p className="text-xs text-red-400">{connectError}</p>
                )}

                <div className="flex items-center gap-2 pt-2">
                  <button
                    onClick={() => setStep("type")}
                    className="bg-surface-2 hover:bg-surface-3 text-surface-text-muted text-xs px-4 py-1.5 rounded"
                  >
                    Back
                  </button>
                </div>
              </div>
            )}

            {step === "hostKeyConfirm" && hostKeyConfirm && (
              <div className="space-y-3">
                {hostKeyConfirm.changed ? (
                  <>
                    <p className="text-sm font-medium text-red-400">
                      Warning: this host's key has changed
                    </p>
                    <p className="text-xs text-surface-text-muted">
                      PairAdmin previously connected to{" "}
                      <span className="font-mono">
                        {hostKeyConfirm.params.host}:{hostKeyConfirm.params.port}
                      </span>{" "}
                      and trusted a different key than the one it's presenting now. This can mean
                      a man-in-the-middle attack, or that the host was legitimately
                      reinstalled/rekeyed. PairAdmin will refuse to connect until an administrator
                      removes this host's old entry from{" "}
                      <span className="font-mono">known_hosts.yaml</span>.
                    </p>
                  </>
                ) : (
                  <>
                    <p className="text-sm font-medium text-surface-text">Verify new host key</p>
                    <p className="text-xs text-surface-text-muted">
                      PairAdmin hasn't connected to{" "}
                      <span className="font-mono">
                        {hostKeyConfirm.params.host}:{hostKeyConfirm.params.port}
                      </span>{" "}
                      before. It presented the following key — accept it to continue and
                      remember it for future connections.
                    </p>
                  </>
                )}

                <div className="rounded border border-surface-border-strong bg-surface-2 px-3 py-2 space-y-1">
                  <div className="text-xs text-surface-text-muted">
                    Type: <span className="font-mono text-surface-text">{hostKeyConfirm.keyType}</span>
                  </div>
                  <div className="text-xs text-surface-text-muted break-all">
                    Fingerprint:{" "}
                    <span className="font-mono text-surface-text">{hostKeyConfirm.fingerprint}</span>
                  </div>
                </div>

                {saveWarning && (
                  <p className="text-xs text-amber-400">{saveWarning}</p>
                )}
                {connectStatus === "error" && (
                  <p className="text-xs text-red-400">{connectError}</p>
                )}

                <div className="flex items-center gap-2 pt-2">
                  <button
                    onClick={handleRejectHostKey}
                    className="bg-surface-2 hover:bg-surface-3 text-surface-text-muted text-xs px-4 py-1.5 rounded"
                  >
                    {hostKeyConfirm.changed ? "Back" : "Reject"}
                  </button>
                  {!hostKeyConfirm.changed && (
                    <button
                      onClick={handleAcceptHostKey}
                      disabled={connectStatus === "connecting"}
                      className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded disabled:opacity-50"
                    >
                      {connectStatus === "connecting" ? "Connecting..." : "Accept & Connect"}
                    </button>
                  )}
                </div>
              </div>
            )}

            {step === "form" && (
              <div className="space-y-3">
                <div className="space-y-1">
                  <label className="text-xs text-surface-text-muted">Host</label>
                  <input
                    className={inputClass}
                    value={host}
                    onChange={(e) => setHost(e.target.value)}
                    placeholder="10.0.1.5"
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1">
                    <label className="text-xs text-surface-text-muted">Port</label>
                    <input
                      type="number"
                      className={inputClass}
                      value={port}
                      onChange={(e) => setPort(Number(e.target.value))}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-surface-text-muted">Username</label>
                    <input
                      className={inputClass}
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                    />
                  </div>
                </div>

                {kind === "ssh" && (
                  <div className="space-y-1">
                    <label className="text-xs text-surface-text-muted">Authentication</label>
                    <select
                      className={inputClass}
                      value={authType}
                      onChange={(e) => setAuthType(e.target.value as AuthType)}
                    >
                      <option value="password">Password</option>
                      <option value="privatekey">Private key file</option>
                    </select>
                  </div>
                )}
                {kind === "winrm" && (
                  <p className="text-xs text-surface-text-muted">
                    WinRM supports password authentication only.
                  </p>
                )}

                {(kind === "winrm" || authType === "password") && (
                  <div className="space-y-1">
                    <label className="text-xs text-surface-text-muted">Password</label>
                    <input
                      type="password"
                      className={inputClass}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                    />
                  </div>
                )}
                {kind === "ssh" && authType === "privatekey" && (
                  <>
                    <div className="space-y-1">
                      <label className="text-xs text-surface-text-muted">Private key file path</label>
                      <input
                        className={inputClass}
                        value={privateKeyPath}
                        onChange={(e) => setPrivateKeyPath(e.target.value)}
                        placeholder="~/.ssh/id_ed25519"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-surface-text-muted">Passphrase (optional)</label>
                      <input
                        type="password"
                        className={inputClass}
                        value={passphrase}
                        onChange={(e) => setPassphrase(e.target.value)}
                      />
                    </div>
                  </>
                )}

                <label className="flex items-center gap-2 text-xs text-surface-text-muted pt-1">
                  <input
                    type="checkbox"
                    checked={saveTerminal}
                    onChange={(e) => setSaveTerminal(e.target.checked)}
                  />
                  Save Terminal
                </label>

                {saveTerminal && kind === "ssh" && (
                  <div className="pl-5 space-y-2 border-l border-surface-border">
                    <label className="flex items-center gap-2 text-xs text-surface-text-muted">
                      <input
                        type="checkbox"
                        checked={useTmux}
                        onChange={(e) => setUseTmux(e.target.checked)}
                      />
                      Use tmux if available
                    </label>
                    {useTmux && (
                      <div className="space-y-1">
                        <label className="text-xs text-surface-text-muted">tmux session name</label>
                        <input
                          className={inputClass}
                          value={tmuxSessionName}
                          onChange={(e) => setTmuxSessionName(e.target.value)}
                          placeholder={defaultTmuxSessionNamePlaceholder}
                        />
                        <p className="text-xs text-surface-text-muted">
                          Creates this session if it doesn't exist yet, or reattaches to it if
                          it's still running — including after it was closed or lost elsewhere.
                        </p>
                      </div>
                    )}
                  </div>
                )}

                {kind === "ssh" && (
                  <p className="text-xs text-surface-text-muted">
                    {promptNewHostKeys
                      ? "You'll be asked to confirm this host's key the first time you connect to it."
                      : "The first connection to a new host trusts and remembers its key automatically; a later connection presenting a different key is always refused. Enable “Prompt to accept new SSH host keys” in Settings → Terminals to review each new host's fingerprint yourself."}
                  </p>
                )}
                {kind === "winrm" && (
                  <p className="text-xs text-amber-500/80">
                    WinRM connects over plain HTTP (no TLS). Ctrl+C has no effect on a running
                    remote command.
                  </p>
                )}

                {saveWarning && (
                  <p className="text-xs text-amber-400">{saveWarning}</p>
                )}
                {connectStatus === "error" && (
                  <p className="text-xs text-red-400">{connectError}</p>
                )}

                <div className="flex items-center gap-2 pt-2">
                  <button
                    onClick={() => setStep("type")}
                    className="bg-surface-2 hover:bg-surface-3 text-surface-text-muted text-xs px-4 py-1.5 rounded"
                  >
                    Back
                  </button>
                  <button
                    onClick={() => handleConnectRemote()}
                    disabled={connectStatus === "connecting" || !host || !username}
                    className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {connectStatus === "connecting" ? "Connecting..." : "Connect"}
                  </button>
                </div>
              </div>
            )}
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
