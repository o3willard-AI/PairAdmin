package services

// RemoteKind identifies which remote transport a terminal tab uses.
type RemoteKind string

const (
	RemoteKindSSH   RemoteKind = "ssh"
	RemoteKindWinRM RemoteKind = "winrm"
)

// RemoteAuthType identifies how the user authenticates to a remote host.
type RemoteAuthType string

const (
	RemoteAuthPassword   RemoteAuthType = "password"
	RemoteAuthPrivateKey RemoteAuthType = "privatekey"
)

// RemoteConnectParams is the transient, per-connection-attempt payload sent from the
// frontend's "+ New" dialog. It is never persisted as-is (it may carry a plaintext
// password or key passphrase) — persisted state is config.RemoteHost, which holds no
// secrets. Secrets are stored in the OS keychain only after a successful connection,
// keyed by RemoteHost.ID (see RemoteService).
type RemoteConnectParams struct {
	Kind           RemoteKind     `json:"kind"`
	Host           string         `json:"host"`
	Port           int            `json:"port"`
	Username       string         `json:"username"`
	AuthType       RemoteAuthType `json:"authType"`
	Password       string         `json:"password,omitempty"`
	PrivateKeyPath string         `json:"privateKeyPath,omitempty"`
	Passphrase     string         `json:"passphrase,omitempty"`
	SavePassword   bool           `json:"savePassword"`
	SavedHostId    string         `json:"savedHostId,omitempty"`
	// UseTmux and TmuxSessionName apply only when Kind == RemoteKindSSH — tmux
	// is a Unix/Linux tool with no WinRM equivalent. When UseTmux is true, the
	// SSH session runs `tmux new-session -A -s <name>` immediately after the
	// shell starts, creating the named session if it doesn't exist or
	// attaching to it if it does (this also naturally covers the case where a
	// previously-used session was destroyed by someone else in the meantime).
	UseTmux         bool   `json:"useTmux"`
	TmuxSessionName string `json:"tmuxSessionName,omitempty"`
}
