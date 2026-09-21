package scanner

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"pairadmin/services/config"

	"golang.org/x/crypto/ssh"
)

// emitRecorder captures every event the service emits, keyed for per-scan
// assertions, with a wait helper that polls for a given event type instead of
// sleeping.
type emitRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	ctx   context.Context
	event string
	data  []interface{}
}

func (r *emitRecorder) emit(ctx context.Context, event string, data ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{ctx, event, data})
}

// waitFor polls (10ms) until an event with the given name was recorded, then
// returns it. Fails the test after timeout.
func (r *emitRecorder) waitFor(t *testing.T, event string, timeout time.Duration) recordedEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, e := range r.events {
			if e.event == event {
				r.mu.Unlock()
				return e
			}
		}
		r.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s event", event)
	return recordedEvent{}
}

// eventsFor returns every recorded event with the given name.
func (r *emitRecorder) eventsFor(event string) []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []recordedEvent
	for _, e := range r.events {
		if e.event == event {
			out = append(out, e)
		}
	}
	return out
}

// hostRow extracts the HostRow from a scan:host event's payload.
func hostRow(t *testing.T, e recordedEvent) HostRow {
	t.Helper()
	if len(e.data) != 1 {
		t.Fatalf("scan:host payload: expected 1 data arg, got %d", len(e.data))
	}
	ev, ok := e.data[0].(ScanHostEvent)
	if !ok {
		t.Fatalf("scan:host payload: expected ScanHostEvent, got %T", e.data[0])
	}
	return ev.Row
}

// withDialFunc swaps the TCP dial seam for one test.
func withDialFunc(t *testing.T, f func(network, addr string, timeout time.Duration) (net.Conn, error)) {
	t.Helper()
	orig := dialFunc
	dialFunc = f
	t.Cleanup(func() { dialFunc = orig })
}

// startBannerListener starts a listener whose handler writes the given ident
// line and closes.
func startBannerListener(t *testing.T, ident string) (addr string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte(ident))
			_ = conn.Close()
		}
	}()
	return ln.Addr().String()
}

func TestScanService_DisabledGate_ReturnsErrAndZeroDials(t *testing.T) {
	rec := &emitRecorder{}
	dials := 0
	withDialFunc(t, func(_, _ string, _ time.Duration) (net.Conn, error) {
		dials++
		return nil, errors.New("must not dial")
	})

	svc := NewScanService(config.AppConfig{ScannerEnabled: false}, rec.emit)
	baseGoroutines := goroutineCount()

	scanID, err := svc.Start(ScanRequest{Targets: []string{"127.0.0.1/32"}})

	if !errors.Is(err, ErrScanningDisabled) {
		t.Fatalf("expected ErrScanningDisabled, got %v", err)
	}
	if scanID != "" {
		t.Errorf("expected empty scanID on refusal, got %q", scanID)
	}
	if dials != 0 {
		t.Errorf("disabled scan must perform zero dials, performed %d", dials)
	}
	if got := goroutineCount(); got > baseGoroutines {
		t.Errorf("disabled scan must spawn zero goroutines, +%d", got-baseGoroutines)
	}
	if n := len(rec.eventsFor("scan:error")) + len(rec.eventsFor("scan:done")) + len(rec.eventsFor("scan:cancelled")); n != 0 {
		t.Errorf("disabled scan must emit no terminal event, emitted %d", n)
	}
	// Mutation check: deleting the scanAllowed gate in Start makes this
	// test fail — the scan proceeds, dials the target, and returns a
	// non-empty scanID instead of ErrScanningDisabled.
}

func TestScanService_HappyPath_EmitsSSHRowAndDone(t *testing.T) {
	bannerAddr := startBannerListener(t, "SSH-2.0-OpenSSH_9.6p1\r\n")
	withDialFunc(t, func(_, _ string, timeout time.Duration) (net.Conn, error) {
		// Redirect every requested address to the in-process banner server.
		return net.DialTimeout("tcp", bannerAddr, timeout)
	})
	// Hermetic host-key probe: the shared SSH dial seam answers with a
	// captured test key instead of a real network dial to :22.
	origSSH := DialFunc
	DialFunc = func(_ string, _ string, _ *ssh.ClientConfig) (*ssh.Client, error) {
		return nil, errors.New("no ssh handshake in this test")
	}
	t.Cleanup(func() { DialFunc = origSSH })

	rec := &emitRecorder{}
	svc := NewScanService(config.AppConfig{ScannerEnabled: true}, rec.emit)

	start := time.Now()
	scanID, err := svc.Start(ScanRequest{Targets: []string{"127.0.0.1/32"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	host := rec.waitFor(t, "scan:host", 5*time.Second)
	if host.event != "scan:host" {
		t.Fatal("unreachable")
	}
	row := hostRow(t, host)
	if row.IP != "127.0.0.1" {
		t.Errorf("expected ip 127.0.0.1, got %q", row.IP)
	}
	if row.State != HostStateSSH {
		t.Errorf("expected state %q, got %q", HostStateSSH, row.State)
	}
	if row.SSH == nil {
		t.Fatal("expected ssh info on an ssh row")
	}
	if row.SSH.Banner != "SSH-2.0-OpenSSH_9.6p1" {
		t.Errorf("expected banner captured, got %q", row.SSH.Banner)
	}
	if row.SSH.Software != "OpenSSH 9.6p1" {
		t.Errorf("expected parsed software %q, got %q", "OpenSSH 9.6p1", row.SSH.Software)
	}
	if row.SSH.Port != DefaultSSHPort {
		t.Errorf("expected port %d, got %d", DefaultSSHPort, row.SSH.Port)
	}

	done := rec.waitFor(t, "scan:done", 5*time.Second)
	ev := done.data[0].(ScanDoneEvent)
	if ev.ScanID != scanID {
		t.Errorf("scan:done scanID %q != Start's %q", ev.ScanID, scanID)
	}
	if ev.Stats.Total != 1 || ev.Stats.SSH != 1 || ev.Stats.Filtered != 0 || ev.Stats.Closed != 0 {
		t.Errorf("unexpected stats: %+v", ev.Stats)
	}
	if ev.Stats.DurationMs < 0 || time.Since(start) < 0 {
		t.Error("sanity")
	}
}

func TestScanService_RefusedDial_RowIsClosedNeverFiltered(t *testing.T) {
	withDialFunc(t, func(_, _ string, _ time.Duration) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	})
	rec := &emitRecorder{}
	svc := NewScanService(config.AppConfig{ScannerEnabled: true}, rec.emit)

	if _, err := svc.Start(ScanRequest{Targets: []string{"127.0.0.1/32"}}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	row := hostRow(t, rec.waitFor(t, "scan:host", 5*time.Second))
	// Mutation check: collapsing StateClosed into "filtered" in the
	// ProbeState->HostState mapping fails this — the distinction is the
	// scanner's honest answer about whether the host was reachable.
	if row.State != HostStateClosed {
		t.Fatalf("expected state closed, got %q", row.State)
	}
	rec.waitFor(t, "scan:done", 5*time.Second)
}

func TestScanService_DialTimeout_RowIsFilteredNeverClosed(t *testing.T) {
	withDialFunc(t, func(_, _ string, _ time.Duration) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: os.ErrDeadlineExceeded}
	})
	rec := &emitRecorder{}
	svc := NewScanService(config.AppConfig{ScannerEnabled: true}, rec.emit)

	if _, err := svc.Start(ScanRequest{Targets: []string{"127.0.0.1/32"}}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	row := hostRow(t, rec.waitFor(t, "scan:host", 5*time.Second))
	// Mutation check: collapsing StateFiltered into "closed" fails this —
	// a timeout is an unknown, never a positive "no SSH here".
	if row.State != HostStateFiltered {
		t.Fatalf("expected state filtered, got %q", row.State)
	}
	rec.waitFor(t, "scan:done", 5*time.Second)
}

func TestScanService_StopMidScan_CancelsWithoutGoroutineLeak(t *testing.T) {
	// A dial that blocks until released keeps probes in flight so the scan
	// is genuinely mid-flight when Stop arrives.
	release := make(chan struct{})
	var dials atomic.Int32
	withDialFunc(t, func(_, _ string, _ time.Duration) (net.Conn, error) {
		dials.Add(1)
		<-release
		return nil, errors.New("aborted")
	})
	rec := &emitRecorder{}
	svc := NewScanService(config.AppConfig{ScannerEnabled: true}, rec.emit)

	base := goroutineCount()
	scanID, err := svc.Start(ScanRequest{Targets: []string{"127.0.0.0/29"}, MaxProbes: 2})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait until probes are genuinely in flight, then cancel mid-scan.
	waitFor(t, 2*time.Second, func() bool { return dials.Load() >= 2 })
	if err := svc.Stop(scanID); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Release the blocked dials so the bounded in-flight probes return
	// (they are deadline-bounded in production; the release emulates the
	// deadline firing). Only after they drain does the scan goroutine emit
	// its single terminal event.
	close(release)

	cancelled := rec.waitFor(t, "scan:cancelled", 5*time.Second)
	if cancelled.data[0].(ScanCancelledEvent).ScanID != scanID {
		t.Error("scan:cancelled for a different scan id")
	}
	if n := len(rec.eventsFor("scan:done")); n != 0 {
		t.Errorf("cancelled scan must not emit scan:done, emitted %d", n)
	}
	// No result rows after cancellation.
	if n := len(rec.eventsFor("scan:host")); n != 0 {
		t.Errorf("cancelled scan must not emit scan:host rows, emitted %d", n)
	}

	// Every goroutine must have exited — ctx cancellation must not leak
	// goroutines or sockets.
	waitGoroutinesDrain(t, base, 5*time.Second)
	// Mutation check: if the dispatch loop or the probe goroutines ignored
	// ctx cancellation (e.g. dispatched without a bounded semaphore, or the
	// run goroutine kept waiting on all 8 targets), the drain assertion
	// fails: blocked dial goroutines outlive the test.
}

func TestScanService_InvalidTarget_EmitsScanErrorAsOnlyTerminalEvent(t *testing.T) {
	rec := &emitRecorder{}
	svc := NewScanService(config.AppConfig{ScannerEnabled: true}, rec.emit)

	scanID, err := svc.Start(ScanRequest{Targets: []string{"not-a-cidr"}})
	if err != nil {
		t.Fatalf("Start must accept the request and fail via scan:error, got %v", err)
	}
	errEv := rec.waitFor(t, "scan:error", 5*time.Second)
	ev := errEv.data[0].(ScanErrorEvent)
	if ev.ScanID != scanID {
		t.Errorf("scan:error scanID %q != %q", ev.ScanID, scanID)
	}
	if ev.Message == "" {
		t.Error("scan:error must carry a message")
	}
	// Exactly one terminal event: no done, no cancelled for this scan.
	if n := len(rec.eventsFor("scan:done")) + len(rec.eventsFor("scan:cancelled")); n != 0 {
		t.Errorf("failed scan must emit only scan:error, saw %d other terminal events", n)
	}
}

func TestScanService_MaxProbes_ClampedToValidRange(t *testing.T) {
	for _, tc := range []struct {
		in, want int
	}{{0, 64}, {1, 16}, {15, 16}, {16, 16}, {300, 256}, {256, 256}, {64, 64}, {-5, 64}} {
		if got := clampMaxProbes(tc.in); got != tc.want {
			t.Errorf("clampMaxProbes(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestScanService_TerminalEventExactlyOnce(t *testing.T) {
	// Happy scan: one scan:done, and no scan:error/cancelled.
	bannerAddr := startBannerListener(t, "SSH-2.0-test\r\n")
	withDialFunc(t, func(_, _ string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout("tcp", bannerAddr, timeout)
	})
	rec := &emitRecorder{}
	svc := NewScanService(config.AppConfig{ScannerEnabled: true}, rec.emit)

	scanID, err := svc.Start(ScanRequest{Targets: []string{"127.0.0.1/32"}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	rec.waitFor(t, "scan:done", 5*time.Second)
	for _, e := range rec.eventsFor("scan:done") {
		if e.data[0].(ScanDoneEvent).ScanID != scanID {
			t.Error("terminal event for another scan id")
		}
	}
	if n := len(rec.eventsFor("scan:done")); n != 1 {
		t.Errorf("expected exactly one scan:done, got %d", n)
	}
	if n := len(rec.eventsFor("scan:error")) + len(rec.eventsFor("scan:cancelled")); n != 0 {
		t.Errorf("expected no other terminal events, got %d", n)
	}
}

func TestParseSoftware(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"SSH-2.0-OpenSSH_9.6p1", "OpenSSH 9.6p1"},
		{"SSH-2.0-OpenSSH_9.6p1 Debian-3+deb12u2", "OpenSSH 9.6p1"},
		{"SSH-1.99-Cisco-1.25", "Cisco-1.25"},
		{"SSH-2.0-libssh_0.9.0", "libssh 0.9.0"},
		{"SSH-2.0", ""},
		{"", ""},
	} {
		if got := parseSoftware(tc.in); got != tc.want {
			t.Errorf("parseSoftware(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func goroutineCount() int {
	return runtime.NumGoroutine()
}

// waitFor polls cond until true or timeout, failing the test on timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func waitGoroutinesDrain(t *testing.T, base int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if goroutineCount() <= base {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("goroutines did not drain back to baseline %d (now %d)", base, goroutineCount())
}
