package keychain

import (
	"fmt"
	"os"
	"time"
)

// keychainLogPath is the file that keychain diagnostic lines are appended to.
// It is a package-level var (the same injected-seam pattern as
// buildProviderFn in services/settings_service.go) so tests can redirect it
// to a temp file and assert on its contents. The default is a relative cwd
// path — the same convention as the pty_debug.log precedent in
// services/pty_windows.go's logPty() — so on Windows the probe's startup
// diagnostics land next to the other debug logs.
var keychainLogPath = "keychain_debug.log"

// logProbe appends a single diagnostic line to keychainLogPath, mirroring
// services/pty_windows.go's logPty(): a timestamp + the Windows username on
// each line, and a silent no-op when the file cannot be opened (debug logging
// must never break the keychain path). It logs ONLY the failure detail, the
// probe key name, and which call failed — never credential Data or any real
// secret (matching the AGENTS.md redaction contract).
func logProbe(msg string) {
	f, err := os.OpenFile(keychainLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] [%s] %s\n", time.Now().Format(time.RFC3339), os.Getenv("USERNAME"), msg)
}
