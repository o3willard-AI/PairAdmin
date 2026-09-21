// ScanService orchestration: Mode-A SSH sweep.
//
// This layer ties the target feeders (targets.go), the SSH probe
// (probe_ssh.go), and the enable/disable seam (policy.go) together into one
// scan: build the target list, probe each host on :22 with bounded
// concurrency, and stream progress/results as Wails events (scan:*,
// colon-namespaced like the existing llm:* events).
package scanner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pairadmin/services/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sync/semaphore"
)

// HostState is the network scanner's result-row state. The full enum is
// declared here so the frontend contract is pinned from day one; Mode A
// (this task) only ever emits ssh / filtered / closed — "ssh-known",
// "alive", and "dead" belong to the later Mode-B task and must NOT be
// emitted by this service yet.
type HostState string

const (
	// HostStateSSH: the probe read an SSH identification line.
	HostStateSSH HostState = "ssh"
	// HostStateSSHKnown: SSH host key matches a saved/trusted host (Mode B).
	HostStateSSHKnown HostState = "ssh-known"
	// HostStateFiltered: dial timed out — unknown, host may run SSH behind
	// a firewall. Never collapse into "closed".
	HostStateFiltered HostState = "filtered"
	// HostStateClosed: dial refused (RST) — host answered, no SSH on :22.
	HostStateClosed HostState = "closed"
	// HostStateAlive: host answers something non-SSH (Mode B liveness).
	HostStateAlive HostState = "alive"
	// HostStateDead: host did not answer at all (Mode B liveness).
	HostStateDead HostState = "dead"
)

// SSHInfo is the "ssh" sub-object of a result row.
type SSHInfo struct {
	Port               int    `json:"port"`
	Banner             string `json:"banner"`
	Software           string `json:"software"`
	HostKeyType        string `json:"hostKeyType"`
	HostKeyFingerprint string `json:"hostKeyFingerprint"`
}

// HostRow is one result row, the payload of scan:host ("row" field). This
// JSON shape is the pinned frontend contract.
type HostRow struct {
	IP    string    `json:"ip"`
	State HostState `json:"state"`
	SSH   *SSHInfo  `json:"ssh,omitempty"`
}

// Event payloads. Field names are the pinned frontend contract (camelCase).

// ScanProgressEvent is emitted (throttled to ~150ms) while a scan runs.
type ScanProgressEvent struct {
	ScanID       string `json:"scanID"`
	Done         int    `json:"done"`
	Total        int    `json:"total"`
	ActiveProbes int    `json:"activeProbes"`
}

// ScanHostEvent is emitted once per discovered host, as results arrive —
// never batched at the end.
type ScanHostEvent struct {
	ScanID string  `json:"scanID"`
	Row    HostRow `json:"row"`
}

// ScanStats summarizes a completed sweep.
type ScanStats struct {
	Total      int   `json:"total"`
	SSH        int   `json:"ssh"`
	Filtered   int   `json:"filtered"`
	Closed     int   `json:"closed"`
	DurationMs int64 `json:"durationMs"`
}

// ScanDoneEvent is emitted exactly once per scan that runs to completion.
type ScanDoneEvent struct {
	ScanID string    `json:"scanID"`
	Stats  ScanStats `json:"stats"`
}

// ScanErrorEvent is emitted exactly once per scan that hits a fatal error.
type ScanErrorEvent struct {
	ScanID  string `json:"scanID"`
	Message string `json:"message"`
}

// ScanCancelledEvent is emitted exactly once per scan stopped via Stop.
type ScanCancelledEvent struct {
	ScanID string `json:"scanID"`
}

// ScanRequest describes one sweep.
type ScanRequest struct {
	// Targets is a list of CIDR strings (e.g. "192.168.1.0/24") and/or
	// "start-end" IP range strings (e.g. "192.168.1.1-192.168.1.9"). An
	// empty list means "discover the local /24 networks" via
	// LocalNetsFeeder.
	Targets []string
	// MaxProbes bounds probes in flight. Zero (or negative) selects the
	// default of 64; the value is clamped to [16, 256].
	MaxProbes int
}

// ErrScanningDisabled is returned by Start when the scanner is disabled in
// config (or force-disabled through the scanAllowed seam). Starting a scan
// in this state performs zero network dials and spawns zero goroutines.
var ErrScanningDisabled = errors.New("network scanning is disabled in settings")

// ErrScanNotFound is returned by Stop for an unknown or already-finished
// scan id.
var ErrScanNotFound = errors.New("no running scan with that id")

// Probe-concurrency defaults and clamps.
const (
	DefaultMaxProbes = 64
	MinMaxProbes     = 16
	MaxMaxProbes     = 256
)

// progressInterval is the scan:progress throttle window.
const progressInterval = 150 * time.Millisecond

// ScanService orchestrates Mode-A SSH sweeps. It is safe for concurrent use.
type ScanService struct {
	cfg    config.AppConfig
	emitFn func(ctx context.Context, event string, optionalData ...interface{})

	// liveCfgFn, when set, is the live AppConfig source the Start
	// enable/disable gate consults. NewScanService captures cfg by value, so
	// without this a frontend scanner_enabled change (Settings toggle) would
	// leave the gate stale until restart. Nil (the default) gates on the
	// construction-time snapshot — tests rely on that; main.go wires the live
	// getter via SetLiveCfgFn.
	liveCfgFn func() (*config.AppConfig, error)

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewScanService builds a ScanService. emitFn follows the LLMService pattern
// exactly: production passes (or defaults to) runtime.EventsEmit, tests
// inject a recorder. A nil emitFn selects runtime.EventsEmit.
func NewScanService(cfg config.AppConfig, emitFn func(ctx context.Context, event string, optionalData ...interface{})) *ScanService {
	if emitFn == nil {
		emitFn = runtime.EventsEmit
	}
	return &ScanService{
		cfg:     cfg,
		emitFn:  emitFn,
		cancels: make(map[string]context.CancelFunc),
	}
}

// SetLiveCfgFn wires the live AppConfig source that Start's enable/disable
// gate consults, so a frontend scanner_enabled change (Settings toggle) takes
// effect on the next Start WITHOUT restarting the app. main.go passes
// config.LoadAppConfig. When left unset the gate uses the construction-time
// snapshot passed to NewScanService — which is what the unit tests rely on.
func (s *ScanService) SetLiveCfgFn(getter func() (*config.AppConfig, error)) {
	s.liveCfgFn = getter
}

// gateCfg returns the config the Start gate consults. It prefers the live
// getter (set via SetLiveCfgFn) so current settings are honored; when no
// getter is configured, or it fails to load, it falls back to the
// construction-time snapshot — a config-read error can never bypass the gate
// into an unwanted scan.
func (s *ScanService) gateCfg() config.AppConfig {
	if s.liveCfgFn != nil {
		if cfg, err := s.liveCfgFn(); err == nil && cfg != nil {
			return *cfg
		}
	}
	return s.cfg
}

// Start launches one Mode-A SSH sweep and returns its scan id immediately.
//
// The enable/disable gate is checked FIRST: when scanning is disabled the
// call returns ErrScanningDisabled without dialing anything or spawning
// anything. Target-list build failures are reported asynchronously via the
// scan:error event (the scan id is already live by then); mid-scan the sweep
// streams scan:progress / scan:host and finishes with exactly one terminal
// event: scan:done, scan:error, or (after Stop) scan:cancelled.
func (s *ScanService) Start(req ScanRequest) (string, error) {
	// Gate — hard requirement: zero network, zero goroutines when disabled.
	if !scanAllowed(s.gateCfg()) {
		return "", ErrScanningDisabled
	}

	scanID, err := newScanID()
	if err != nil {
		return "", fmt.Errorf("generate scan id: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancels[scanID] = cancel
	s.mu.Unlock()

	go s.run(scanID, ctx, cancel, req)
	return scanID, nil
}

// Stop cancels the scan with the given id; that scan's goroutine emits
// scan:cancelled (never scan:done). In-flight probes are bounded by the
// probe deadlines (dial timeout + read deadline) and their results are
// discarded, so cancellation cannot leak goroutines or sockets.
func (s *ScanService) Stop(scanID string) error {
	s.mu.Lock()
	cancel, ok := s.cancels[scanID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("Stop(%q): %w", scanID, ErrScanNotFound)
	}
	cancel()
	return nil
}

// run is the single goroutine behind one scan. It owns exactly one terminal
// event: scan:done, scan:error, or scan:cancelled.
func (s *ScanService) run(scanID string, ctx context.Context, cancel context.CancelFunc, req ScanRequest) {
	defer func() {
		s.mu.Lock()
		delete(s.cancels, scanID)
		s.mu.Unlock()
		cancel() // release ctx resources even if the scan completed
	}()

	// Build the target list (Start gate already passed). A build failure is
	// fatal for this scan and reported via scan:error.
	targets, err := buildTargets(req.Targets)
	if err != nil {
		s.emitFn(ctx, "scan:error", ScanErrorEvent{ScanID: scanID, Message: err.Error()})
		return
	}
	total := len(targets)

	var (
		done    atomic.Int64
		ssh     atomic.Int64
		filterd atomic.Int64
		closed  atomic.Int64
		active  atomic.Int64
	)
	started := time.Now()

	sem := semaphore.NewWeighted(int64(clampMaxProbes(req.MaxProbes)))
	var wg sync.WaitGroup

	// Throttled progress: a ticker goroutine, stopped when the sweep
	// finishes. Progress events carry live counters; the last one may lag
	// behind the final state — scan:done is authoritative.
	progressStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.emitFn(ctx, "scan:progress", ScanProgressEvent{
					ScanID:       scanID,
					Done:         int(done.Load()),
					Total:        total,
					ActiveProbes: int(active.Load()),
				})
			case <-progressStop:
				return
			}
		}
	}()

	for _, ip := range targets {
		// Acquire BEFORE spawning so the dispatch loop itself stops when
		// the semaphore is full — the number of goroutines never exceeds
		// maxProbes, and ctx cancellation unblocks a waiting Acquire.
		if err := sem.Acquire(ctx, 1); err != nil {
			break // scan cancelled while waiting for a slot
		}
		if ctx.Err() != nil {
			sem.Release(1)
			break
		}
		ipStr := ip.String()
		wg.Add(1)
		go func(ipStr string) {
			defer wg.Done()
			defer sem.Release(1)
			defer active.Add(-1)
			active.Add(1)

			res := ProbeSSH(ipStr, DefaultSSHPort, true)

			// Cancelled while this probe was in flight: discard the
			// result — a cancelled scan must not keep emitting rows.
			if ctx.Err() != nil {
				return
			}
			row := rowFromProbe(ipStr, res)
			switch row.State {
			case HostStateSSH:
				ssh.Add(1)
			case HostStateFiltered:
				filterd.Add(1)
			case HostStateClosed:
				closed.Add(1)
			}
			s.emitFn(ctx, "scan:host", ScanHostEvent{ScanID: scanID, Row: row})
			done.Add(1)
		}(ipStr)
	}

	wg.Wait()
	close(progressStop)

	// Exactly one terminal event.
	if ctx.Err() != nil {
		s.emitFn(ctx, "scan:cancelled", ScanCancelledEvent{ScanID: scanID})
		return
	}
	s.emitFn(ctx, "scan:done", ScanDoneEvent{
		ScanID: scanID,
		Stats: ScanStats{
			Total:      total,
			SSH:        int(ssh.Load()),
			Filtered:   int(filterd.Load()),
			Closed:     int(closed.Load()),
			DurationMs: time.Since(started).Milliseconds(),
		},
	})
}

// localNetsFn is the injectable seam for local-network discovery; tests
// swap it to simulate a host with no discoverable IPv4 /24 networks.
var localNetsFn = NewLocalNetsFeeder

// buildTargets drains the configured feeders into one flat target list.
// An empty spec discovers the local /24 networks via LocalNetsFeeder. A
// discovery failure (e.g. a host with no active IPv4 network) is returned
// as an error — a target-build failure is fatal for the scan and reported
// via scan:error, NEVER a panic for the whole app.
func buildTargets(specs []string) ([]net.IP, error) {
	if len(specs) == 0 {
		f, err := localNetsFn()
		if err != nil {
			return nil, err
		}
		return Collect(f)
	}

	var out []net.IP
	for _, spec := range specs {
		var f IPFeeder
		var err error
		switch {
		case strings.Contains(spec, "-"):
			start, end, ok := strings.Cut(spec, "-")
			if !ok {
				return nil, fmt.Errorf("invalid target %q", spec)
			}
			f, err = NewRangeFeeder(start, end, false)
		default:
			f, err = NewCIDRFeeder(spec, false)
		}
		if err != nil {
			return nil, err
		}
		ips, err := Collect(f)
		if err != nil {
			return nil, err
		}
		out = append(out, ips...)
	}
	return out, nil
}

// rowFromProbe maps one probe outcome to the pinned result-row shape.
// filtered vs closed is carried, never collapsed.
func rowFromProbe(ip string, res SSHProbeResult) HostRow {
	switch res.State {
	case StateOpen:
		return HostRow{
			IP:    ip,
			State: HostStateSSH,
			SSH: &SSHInfo{
				Port:               DefaultSSHPort,
				Banner:             res.Banner,
				Software:           parseSoftware(res.Banner),
				HostKeyType:        res.KeyType,
				HostKeyFingerprint: res.Fingerprint,
			},
		}
	case StateClosed:
		return HostRow{IP: ip, State: HostStateClosed}
	default: // StateFiltered
		return HostRow{IP: ip, State: HostStateFiltered}
	}
}

// parseSoftware extracts the software component from an SSH identification
// line ("SSH-2.0-OpenSSH_9.6p1" → "OpenSSH 9.6p1"), dropping any trailing
// comment. Underscores become spaces per the common vendor convention.
func parseSoftware(banner string) string {
	parts := strings.SplitN(banner, "-", 3)
	if len(parts) < 3 {
		return ""
	}
	software := parts[2]
	if idx := strings.IndexByte(software, ' '); idx >= 0 {
		software = software[:idx] // drop comment ("... Debian-3+deb12u2")
	}
	return strings.ReplaceAll(software, "_", " ")
}

// clampMaxProbes bounds probe concurrency: default 64, clamped to [16, 256].
func clampMaxProbes(n int) int {
	if n <= 0 {
		n = DefaultMaxProbes
	}
	if n < MinMaxProbes {
		return MinMaxProbes
	}
	if n > MaxMaxProbes {
		return MaxMaxProbes
	}
	return n
}

// newScanID returns a short random id ("scan-<16 hex chars>").
func newScanID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "scan-" + hex.EncodeToString(b[:]), nil
}
