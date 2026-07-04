import { useEffect, useState } from "react";
import { Dialog } from "@base-ui/react/dialog";
import { useTerminalStore } from "@/stores/terminalStore";
import { useCommandStore } from "@/stores/commandStore";
import { wailsErrorMessage } from "@/utils/wailsError";
import type { config } from "../../../wailsjs/go/models";

export interface NewTerminalDialogProps {
  open: boolean;
  onClose: () => void;
}

type TerminalKind = "local" | "ssh" | "winrm";
type AuthType = "password" | "privatekey";

const inputClass =
  "w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none";

const typeCardClass =
  "w-full text-left px-4 py-3 rounded border border-zinc-700 hover:bg-zinc-800 text-sm text-zinc-100 transition-colors";

// Mirrors defaultTmuxSessionName in services/remote_ssh.go — shown only as a
// placeholder hint; the actual default is applied backend-side if left blank.
const defaultTmuxSessionNamePlaceholder = "pairadmin (default)";

export function NewTerminalDialog({ open, onClose }: NewTerminalDialogProps) {
  const [step, setStep] = useState<"type" | "form">("type");
  const [kind, setKind] = useState<TerminalKind>("local");

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

  const [recentHosts, setRecentHosts] = useState<config.RemoteHost[]>([]);
  const [connectStatus, setConnectStatus] = useState<"idle" | "connecting" | "error">("idle");
  const [connectError, setConnectError] = useState("");
  const [saveWarning, setSaveWarning] = useState("");

  const refreshRecentHosts = () => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/RemoteService")
      .then(({ ListRemoteHosts }) => ListRemoteHosts())
      .then((hosts: config.RemoteHost[]) => setRecentHosts(hosts || []))
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

    refreshRecentHosts();
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
  const connectToRemote = async (params: {
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
  }) => {
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
      if (params.savedHostId) {
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
      store.addTab(resolvedId, displayName, degraded, degradedMsg, params.kind, savedHostId);
      store.setActiveTab(resolvedId);

      // tmux uses the alternate screen buffer, which disables xterm.js's own
      // scrollback/scrollbar — the mouse wheel does nothing until tmux's own
      // mouse mode is on. Pin one-click toggles so the user doesn't have to
      // remember or type these; addPinnedCommand no-ops if already present,
      // so this is safe to call on every tmux connect/reconnect.
      if (params.kind === "ssh" && params.useTmux) {
        const commandStore = useCommandStore.getState();
        commandStore.addPinnedCommand(resolvedId, {
          command: "tmux set -g mouse on",
          originalQuestion: "Enables mouse-wheel scrolling inside tmux (auto-added)",
        });
        commandStore.addPinnedCommand(resolvedId, {
          command: "tmux set -g mouse off",
          originalQuestion:
            "Disables tmux mouse mode, restoring normal terminal text selection (auto-added)",
        });
      }
      // Only auto-close on a clean save; if the save/touch step warned, leave
      // the dialog open long enough for the user to actually see the warning.
      if (!warning) onClose();
    } catch (err) {
      setConnectStatus("error");
      setConnectError(wailsErrorMessage(err, "Connection failed"));
    }
  };

  const handleConnectRemote = () =>
    connectToRemote({
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
    });

  const handleReconnect = (h: config.RemoteHost) => {
    applyHostToForm(h);
    connectToRemote({
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

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[480px] max-h-[80vh] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-zinc-900 border border-zinc-700 shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-zinc-100 border-b border-zinc-800">
            New Terminal
          </Dialog.Title>
          <div className="flex-1 overflow-y-auto p-6 space-y-4">
            {step === "type" && (
              <div className="space-y-2">
                <button onClick={() => selectKind("local")} className={typeCardClass}>
                  <div className="font-medium">Local</div>
                  <div className="text-xs text-zinc-500">Open a shell on this machine</div>
                </button>
                <button onClick={() => selectKind("ssh")} className={typeCardClass}>
                  <div className="font-medium">Unix / Linux (SSH)</div>
                  <div className="text-xs text-zinc-500">Connect to a remote host over SSH</div>
                </button>
                <button onClick={() => selectKind("winrm")} className={typeCardClass}>
                  <div className="font-medium">Remote Windows (WinRM)</div>
                  <div className="text-xs text-zinc-500">
                    Command/response only — not a live shell
                  </div>
                </button>

                {recentHosts.length > 0 && (
                  <div className="pt-3">
                    <div className="text-xs text-zinc-500 uppercase tracking-wider mb-1">
                      Recent
                    </div>
                    <div className="space-y-1">
                      {recentHosts.map((h) => (
                        <div
                          key={h.ID}
                          className="flex items-center justify-between px-3 py-2 rounded border border-zinc-800 hover:bg-zinc-800"
                        >
                          <div className="text-sm text-zinc-200 truncate">
                            {h.Name || `${h.Username}@${h.Host}`}
                            <span className="text-xs text-zinc-500 ml-2">{h.Kind}</span>
                          </div>
                          <div className="flex items-center gap-1 shrink-0">
                            <button
                              className="text-xs bg-zinc-700 hover:bg-zinc-600 text-zinc-100 px-2 py-1 rounded disabled:opacity-50"
                              disabled={connectStatus === "connecting"}
                              onClick={() => handleReconnect(h)}
                            >
                              Connect
                            </button>
                            <button
                              className="text-xs text-zinc-500 hover:text-red-400 px-1.5 py-1"
                              aria-label={`Forget saved host ${h.Username}@${h.Host}`}
                              onClick={() => handleForget(h.ID)}
                            >
                              &times;
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
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

            {step === "form" && (
              <div className="space-y-3">
                <div className="space-y-1">
                  <label className="text-xs text-zinc-400">Host</label>
                  <input
                    className={inputClass}
                    value={host}
                    onChange={(e) => setHost(e.target.value)}
                    placeholder="10.0.1.5"
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1">
                    <label className="text-xs text-zinc-400">Port</label>
                    <input
                      type="number"
                      className={inputClass}
                      value={port}
                      onChange={(e) => setPort(Number(e.target.value))}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-zinc-400">Username</label>
                    <input
                      className={inputClass}
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                    />
                  </div>
                </div>

                {kind === "ssh" && (
                  <div className="space-y-1">
                    <label className="text-xs text-zinc-400">Authentication</label>
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
                  <p className="text-xs text-zinc-500">
                    WinRM supports password authentication only.
                  </p>
                )}

                {(kind === "winrm" || authType === "password") && (
                  <div className="space-y-1">
                    <label className="text-xs text-zinc-400">Password</label>
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
                      <label className="text-xs text-zinc-400">Private key file path</label>
                      <input
                        className={inputClass}
                        value={privateKeyPath}
                        onChange={(e) => setPrivateKeyPath(e.target.value)}
                        placeholder="~/.ssh/id_ed25519"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-zinc-400">Passphrase (optional)</label>
                      <input
                        type="password"
                        className={inputClass}
                        value={passphrase}
                        onChange={(e) => setPassphrase(e.target.value)}
                      />
                    </div>
                  </>
                )}

                <label className="flex items-center gap-2 text-xs text-zinc-400 pt-1">
                  <input
                    type="checkbox"
                    checked={saveTerminal}
                    onChange={(e) => setSaveTerminal(e.target.checked)}
                  />
                  Save Terminal
                </label>

                {saveTerminal && kind === "ssh" && (
                  <div className="pl-5 space-y-2 border-l border-zinc-800">
                    <label className="flex items-center gap-2 text-xs text-zinc-400">
                      <input
                        type="checkbox"
                        checked={useTmux}
                        onChange={(e) => setUseTmux(e.target.checked)}
                      />
                      Use tmux if available
                    </label>
                    {useTmux && (
                      <div className="space-y-1">
                        <label className="text-xs text-zinc-400">tmux session name</label>
                        <input
                          className={inputClass}
                          value={tmuxSessionName}
                          onChange={(e) => setTmuxSessionName(e.target.value)}
                          placeholder={defaultTmuxSessionNamePlaceholder}
                        />
                        <p className="text-xs text-zinc-600">
                          Creates this session if it doesn't exist yet, or reattaches to it if
                          it's still running — including after it was closed or lost elsewhere.
                        </p>
                      </div>
                    )}
                  </div>
                )}

                {kind === "ssh" && (
                  <p className="text-xs text-amber-500/80">
                    Host key verification is not yet implemented — connections are not
                    protected against man-in-the-middle attacks.
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
                    className="bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs px-4 py-1.5 rounded"
                  >
                    Back
                  </button>
                  <button
                    onClick={() => handleConnectRemote()}
                    disabled={connectStatus === "connecting" || !host || !username}
                    className="bg-zinc-700 hover:bg-zinc-600 text-zinc-100 text-xs px-4 py-1.5 rounded disabled:opacity-50 disabled:cursor-not-allowed"
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
