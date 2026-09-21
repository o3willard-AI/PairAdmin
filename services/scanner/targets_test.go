package scanner

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// CIDRFeeder
// ---------------------------------------------------------------------------

func TestCIDRFeeder_ExactIterationCount(t *testing.T) {
	// Mutation check: removing the exhaustion logic in Next (i.e. never
	// setting f.exhausted = true) makes the feeder infinite and this test
	// would never terminate.
	f, err := NewCIDRFeeder("192.168.1.0/24", false)
	if err != nil {
		t.Fatalf("NewCIDRFeeder: %v", err)
	}

	var count uint64
	for f.HasNext() {
		ip, err := f.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		_ = ip
		count++
	}

	// A /24 has exactly 256 addresses (including network and broadcast).
	if count != 256 {
		t.Errorf("expected 256 IPs from /24, got %d", count)
	}

	if pct := f.PercentageComplete(); pct != 100.0 {
		t.Errorf("expected 100%% after exhaustion, got %f", pct)
	}
}

func TestCIDRFeeder_SmallRangeProgress(t *testing.T) {
	// A /30 has 4 addresses; progress should step in 25% increments.
	f, err := NewCIDRFeeder("192.168.1.0/30", false)
	if err != nil {
		t.Fatalf("NewCIDRFeeder: %v", err)
	}

	expected := []float64{25.0, 50.0, 75.0, 100.0}
	for i, want := range expected {
		ip, err := f.Next()
		if err != nil {
			t.Fatalf("Next() call %d: %v", i, err)
		}
		if pct := f.PercentageComplete(); pct != want {
			t.Errorf("call %d: expected %f%%, got %f (ip=%s)", i, want, pct, ip)
		}
	}
	if f.HasNext() {
		t.Error("expected HasNext to be false after draining all IPs")
	}
}

func TestCIDRFeeder_RejectsOver16(t *testing.T) {
	// Mutation check: removing the `total > MaxRangeSize` guard would let a
	// /8 feeder be created without confirmation.
	_, err := NewCIDRFeeder("10.0.0.0/8", false)
	if err == nil {
		t.Fatal("expected error for unconfirmed /8 (> /16)")
	}
}

func TestCIDRFeeder_AllowsOver16WhenConfirmed(t *testing.T) {
	f, err := NewCIDRFeeder("10.0.0.0/8", true)
	if err != nil {
		t.Fatalf("expected /8 to be allowed with confirmed=true: %v", err)
	}
	if !f.HasNext() {
		t.Fatal("expected HasNext=true after creation")
	}
}

func TestCIDRFeeder_Exact16AllowedWithoutConfirmation(t *testing.T) {
	// A /16 is exactly MaxRangeSize — it is NOT larger than /16, so it
	// should not require confirmation.
	f, err := NewCIDRFeeder("192.168.0.0/16", false)
	if err != nil {
		t.Fatalf("expected /16 to be allowed without confirmation: %v", err)
	}
	if !f.HasNext() {
		t.Fatal("expected HasNext=true")
	}
}

func TestCIDRFeeder_RejectsIPv6(t *testing.T) {
	// Mutation check: removing the To4() nil check allows IPv6 feeders.
	_, err := NewCIDRFeeder("::1/128", false)
	if err == nil {
		t.Fatal("expected error for IPv6 CIDR")
	}
	if !errors.Is(err, ErrIPv6Unsupported) {
		t.Errorf("expected ErrIPv6Unsupported, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RangeFeeder
// ---------------------------------------------------------------------------

func TestRangeFeeder_Forward(t *testing.T) {
	// Mutation check: swapping the ascending/descending comparison would
	// produce reversed output for a forward range.
	f, err := NewRangeFeeder("10.0.0.1", "10.0.0.5", false)
	if err != nil {
		t.Fatalf("NewRangeFeeder: %v", err)
	}

	got := drainStrings(t, f)
	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5"}
	assertEqualStrings(t, got, want)
}

func TestRangeFeeder_Reversed(t *testing.T) {
	// When start > end the feeder should iterate in descending order.
	f, err := NewRangeFeeder("10.0.0.5", "10.0.0.1", false)
	if err != nil {
		t.Fatalf("NewRangeFeeder: %v", err)
	}

	got := drainStrings(t, f)
	want := []string{"10.0.0.5", "10.0.0.4", "10.0.0.3", "10.0.0.2", "10.0.0.1"}
	assertEqualStrings(t, got, want)
}

func TestRangeFeeder_SingleIP(t *testing.T) {
	f, err := NewRangeFeeder("10.0.0.1", "10.0.0.1", false)
	if err != nil {
		t.Fatalf("NewRangeFeeder: %v", err)
	}

	got := drainStrings(t, f)
	assertEqualStrings(t, got, []string{"10.0.0.1"})
}

func TestRangeFeeder_RejectsOver16(t *testing.T) {
	// Range from 10.0.0.0 to 10.1.0.0 is 65537 IPs — exceeds MaxRangeSize.
	_, err := NewRangeFeeder("10.0.0.0", "10.1.0.0", false)
	if err == nil {
		t.Fatal("expected error for range exceeding /16")
	}
}

func TestRangeFeeder_AllowsOver16WhenConfirmed(t *testing.T) {
	f, err := NewRangeFeeder("10.0.0.0", "10.1.0.0", true)
	if err != nil {
		t.Fatalf("expected range to be allowed with confirmed=true: %v", err)
	}
	if !f.HasNext() {
		t.Fatal("expected HasNext=true")
	}
}

func TestRangeFeeder_RejectsInvalidStartIP(t *testing.T) {
	_, err := NewRangeFeeder("not-an-ip", "10.0.0.5", false)
	if err == nil {
		t.Fatal("expected error for invalid start IP")
	}
}

func TestRangeFeeder_RejectsIPv6(t *testing.T) {
	_, err := NewRangeFeeder("::1", "fe80::1", false)
	if err == nil {
		t.Fatal("expected error for IPv6 range")
	}
	if !errors.Is(err, ErrIPv6Unsupported) {
		t.Errorf("expected ErrIPv6Unsupported, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FileFeeder
// ---------------------------------------------------------------------------

func TestFileFeeder_ValidAndInvalidLines(t *testing.T) {
	// Mutation check: removing the blank-line skip would cause ParseIP("")
	// to return nil, but skipping handles it cleanly. Removing the comment
	// skip would treat "# comment" as an invalid IP — it is skipped, so the
	// valid IP count would still be the same; the critical assertion is the
	// exact count of 3 valid IPs.
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.txt")
	content := "192.168.1.1\n" +
		"\n" +
		"# this is a comment\n" +
		"not_an_ip\n" +
		"192.168.1.2\n" +
		"   10.0.0.5   \n" +
		"192.168.1.3\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	f, err := NewFileFeeder(path)
	if err != nil {
		t.Fatalf("NewFileFeeder: %v", err)
	}

	got := drainStrings(t, f)
	want := []string{"192.168.1.1", "192.168.1.2", "10.0.0.5", "192.168.1.3"}
	assertEqualStrings(t, got, want)

	if pct := f.PercentageComplete(); pct != 100.0 {
		t.Errorf("expected 100%% after exhaustion, got %f", pct)
	}
}

func TestFileFeeder_NotFound(t *testing.T) {
	_, err := NewFileFeeder(filepath.Join(t.TempDir(), "nonexistent.txt"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFileFeeder_OnlyInvalidLines(t *testing.T) {
	// Mutation check: removing the invalid-line skip (continue on nil ip)
	// would cause the feeder to either panic on nil or never yield targets.
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	content := "garbage\n\n# comment\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	f, err := NewFileFeeder(path)
	if err != nil {
		t.Fatalf("NewFileFeeder: %v", err)
	}

	if f.HasNext() {
		t.Fatal("expected no targets from a file with only invalid lines")
	}
	if count := len(drainStrings(t, f)); count != 0 {
		t.Errorf("expected 0 IPs, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// LocalNetsFeeder
// ---------------------------------------------------------------------------

func TestLocalNetsFeeder_ReturnsNetworksAndSkipsLoopback(t *testing.T) {
	// Mutation check: removing the loopback filter (FlagLoopback) would
	// include the 127.0.0.0/24 network. Removing the network-address or
	// broadcast-address skip would include .0 or .255 IPs in the output.
	f, err := NewLocalNetsFeeder()
	if err != nil {
		t.Skipf("no local IPv4 /24 networks on this host: %v", err)
	}

	// Must discover at least one /24.
	nets := f.Networks()
	if len(nets) == 0 {
		t.Fatal("expected at least one /24 network")
	}

	// Must not include loopback.
	for _, nw := range nets {
		if nw.IP.To4()[0] == 127 {
			t.Errorf("LocalNetsFeeder included loopback network %s", nw.String())
		}
	}

	// Collect all targets and verify invariants.
	collected, err := Collect(f)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(collected) == 0 {
		t.Fatal("expected at least one target IP from LocalNetsFeeder")
	}

	hostIPs := localNonLoopbackIPv4s(t)

	for _, ip := range collected {
		ip4 := ip.To4()

		// Must not be a network address (last octet == 0).
		if ip4[3] == 0 {
			t.Errorf("feeder returned network address %s", ip.String())
		}
		// Must not be a broadcast address (last octet == 255).
		if ip4[3] == 255 {
			t.Errorf("feeder returned broadcast address %s", ip.String())
		}
		// Must not be the host's own IP.
		if hostIPs[ipToUint32(ip4)] {
			t.Errorf("feeder returned host's own IP %s", ip.String())
		}
	}
}

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestIPFeeder_InterfaceConformance(t *testing.T) {
	// Compile-time assertion that every feeder satisfies IPFeeder.
	var _ IPFeeder = (*CIDRFeeder)(nil)
	var _ IPFeeder = (*RangeFeeder)(nil)
	var _ IPFeeder = (*FileFeeder)(nil)
	var _ IPFeeder = (*LocalNetsFeeder)(nil)
}

func TestCollect_DrainsCIDRFeeder(t *testing.T) {
	f, err := NewCIDRFeeder("192.168.1.0/30", false)
	if err != nil {
		t.Fatalf("NewCIDRFeeder: %v", err)
	}

	ips, err := Collect(f)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(ips) != 4 {
		t.Errorf("expected 4 IPs from /30, got %d", len(ips))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func drainStrings(t *testing.T, f IPFeeder) []string {
	t.Helper()
	var result []string
	for f.HasNext() {
		ip, err := f.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		result = append(result, ip.String())
	}
	return result
}

func assertEqualStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("length mismatch: got %v, want %v", got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// localNonLoopbackIPv4s mirrors LocalNetsFeeder's interface filter to obtain
// the set of host IPv4 addresses that the feeder should be skipping.
func localNonLoopbackIPv4s(t *testing.T) map[uint32]bool {
	t.Helper()
	result := make(map[uint32]bool)

	interfaces, err := net.Interfaces()
	if err != nil {
		t.Logf("could not enumerate interfaces: %v", err)
		return result
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				ip4 := ipNet.IP.To4()
				if ip4 != nil {
					result[ipToUint32(ip4)] = true
				}
			} else if ipAddr, ok := addr.(*net.IPAddr); ok {
				ip4 := ipAddr.IP.To4()
				if ip4 != nil {
					result[ipToUint32(ip4)] = true
				}
			}
		}
	}

	return result
}
