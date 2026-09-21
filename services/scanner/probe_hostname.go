package scanner

import (
	"context"
	"net"
	"strings"
	"time"
)

// lookupAddrFn is the injectable seam for reverse-DNS lookups.
// Production code uses net.LookupAddr directly; tests swap it to avoid
// real DNS and to simulate empty results, errors, or slow responses.
var lookupAddrFn = net.LookupAddr

// HostnameResult is the outcome of ProbeHostname.
// Hostname is empty when no PTR record exists or the lookup failed —
// neither case is an error from the caller's perspective.
type HostnameResult struct {
	Hostname string // reverse-DNS name; empty if none resolved
}

// ProbeHostname resolves the reverse-DNS name for host (an IP address or
// hostname string accepted by net.LookupAddr), bounded by timeout.
//
// A missing PTR record returns an empty Hostname — not an error — because
// a host without rDNS is normal. A DNS error or timeout also yields an
// empty Hostname (the error is not propagated to the caller).
func ProbeHostname(host string, timeout time.Duration) HostnameResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type lookupOutcome struct {
		names []string
		err   error
	}
	resultCh := make(chan lookupOutcome, 1)

	go func() {
		names, err := lookupAddrFn(host)
		resultCh <- lookupOutcome{names, err}
	}()

	select {
	case res := <-resultCh:
		if res.err != nil {
			return HostnameResult{} // error swallowed: empty hostname, no panic
		}
		if len(res.names) == 0 {
			return HostnameResult{} // no PTR record: normal, not an error
		}
		// PTR records often carry a trailing dot; strip it for a clean name.
		return HostnameResult{Hostname: strings.TrimSuffix(res.names[0], ".")}
	case <-ctx.Done():
		return HostnameResult{} // timeout: empty hostname, no error propagated
	}
}
