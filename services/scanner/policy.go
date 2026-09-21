// Package scanner holds the enable/disable seam for the network-scanner
// feature. The scanner itself is built in parallel tasks; this package is
// landed first so those tasks have a single, stable choke point to consult
// before running a scan.
package scanner

import "pairadmin/services/config"

// scanAllowed is the single choke point that decides whether the network
// scanner may run for a given configuration. The default simply honors the
// user's AppConfig.ScannerEnabled field. An Enterprise fork overrides this
// package-level var to force-disable scanning regardless of user setting —
// that override lives in the fork, NOT here, and this seam is the one place
// it can hook in. Do NOT add force-disable logic to this default.
var scanAllowed = func(c config.AppConfig) bool { return c.ScannerEnabled }