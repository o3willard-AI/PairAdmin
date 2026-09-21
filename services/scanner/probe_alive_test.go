package scanner

import (
	"net"
	"testing"
	"time"
)

// NOTE: go test ./services/scanner/... compiles EVERY *_test.go in the package
// into one binary, so shared helpers/fakes (errFakeTimeout, findRefusedPort,
// splitAddr, swapDialFunc) come from probe_ssh_test.go / hostkey_test.go and
// MUST NOT be redeclared here.

// startAcceptServer spins up an in-process TCP listener that accepts a
// connection and closes it — a live endpoint for the TCP-connect path.
func startAcceptServer(t *testing.T) (host string, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { c.Close() }(conn)
		}
	}()
	return splitAddr(t, listener.Addr().String())
}

// swapARPRows replaces the ARP-table seam for the duration of the test.
func swapARPRows(t *testing.T, fake func() []ARPRow) {
	t.Helper()
	orig := readARPRowsFn
	readARPRowsFn = fake
	t.Cleanup(func() { readARPRowsFn = orig })
}

// alwaysTimeout returns the fake deadline-exceeded error for every dial — a
// host that never answers TCP.
func alwaysTimeout(network, addr string, timeout time.Duration) (net.Conn, error) {
	return nil, &net.OpError{Op: "dial", Net: "tcp", Addr: nil, Err: errFakeTimeout{}}
}

// Mutation check: removing the TCP-connect step in ProbeAlive (returning ARP
// verdicts only, or skipping the dial loop) makes this test fail — a host
// with a listening port must be alive by TCP, before ARP is ever consulted.
func TestProbeAlive_TCPConnect_ReportsAlive(t *testing.T) {
	host, _ := startAcceptServer(t)

	res := ProbeAlive(host)
	if !res.Alive {
		t.Fatalf("expected a reachable listener to be alive, got alive=%v", res.Alive)
	}
	if res.Method != "tcp" {
		t.Errorf("expected Method %q, got %q", "tcp", res.Method)
	}
}

// Mutation check: removing the ECONNREFUSED→alive mapping (treating refused
// as anything other than alive, e.g. folding it into the timeout branch) makes
// this test fail — a host that answers "nothing listening here" is by
// definition up, and must never be reported as not-alive.
func TestProbeAlive_TCPRefused_ReportsAlive(t *testing.T) {
	host, _ := findRefusedPort(t)

	res := ProbeAlive(host)
	if !res.Alive {
		t.Fatal("a refused (RST) dial means the host is ALIVE — refused is never dead")
	}
	if res.Method != "tcp" {
		t.Errorf("expected Method %q for a refused dial, got %q", "tcp", res.Method)
	}
	if !(res.RTTMs >= 0) {
		t.Errorf("expected RTTMs to be measured on the refused round-trip, got %v", res.RTTMs)
	}
}

// Mutation check: removing the ARP fallback makes this test fail — a host
// that times out on every TCP port but has a resolved ARP entry (blocks
// ICMP/TCP but answers ARP) must still be alive via the link layer.
func TestProbeAlive_TCPTimesOut_ARPResolves_ReportsAlive(t *testing.T) {
	swapDialFunc(t, alwaysTimeout)
	swapARPRows(t, func() []ARPRow {
		return []ARPRow{
			{IP: "10.0.0.5", MAC: "aa:bb:cc:dd:ee:ff"},
		}
	})

	res := ProbeAlive("10.0.0.5")
	if !res.Alive {
		t.Fatal("a host with a resolved ARP entry is up even if it suppresses TCP")
	}
	if res.Method != "arp" {
		t.Errorf("expected Method %q, got %q", "arp", res.Method)
	}
	if res.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected MAC %q, got %q", "aa:bb:cc:dd:ee:ff", res.MAC)
	}
	if res.RTTMs != 0 {
		t.Errorf("expected RTTMs 0 for an ARP verdict, got %v", res.RTTMs)
	}
}

// Mutation check: making an ABSENT ARP entry (or any all-timeout outcome)
// count as alive makes this test fail — probe failures and unanswered ARP are
// unknown, never evidence of life.
func TestProbeAlive_NoSignal_ReportsNotAlive(t *testing.T) {
	swapDialFunc(t, alwaysTimeout)
	swapARPRows(t, func() []ARPRow { return []ARPRow{} })

	res := ProbeAlive("10.0.0.9")
	if res.Alive {
		t.Fatal("a host with no TCP answer and no ARP entry must NOT be reported alive")
	}
	if res.Method != "none" {
		t.Errorf("expected Method %q, got %q", "none", res.Method)
	}
	if res.RTTMs != 0 {
		t.Errorf("expected RTTMs 0 when liveness is never established, got %v", res.RTTMs)
	}
}

// Mutation check: the ARP search must match on EXACT IP — a row for a
// different host must not satisfy a lookup for another. Otherwise any single
// ARP entry would mark every scanned host alive.
func TestProbeAlive_ARPLookup_IgnoresOtherHosts(t *testing.T) {
	swapDialFunc(t, alwaysTimeout)
	swapARPRows(t, func() []ARPRow {
		return []ARPRow{
			{IP: "10.0.0.5", MAC: "aa:bb:cc:dd:ee:ff"},
		}
	})

	res := ProbeAlive("10.0.0.6")
	if res.Alive {
		t.Fatal("an ARP entry for 10.0.0.5 must not mark 10.0.0.6 as alive")
	}
}

// Mutation check: breaking proc/net/arp parsing (e.g. misreading the MAC
// column) makes this test fail — the IP/MAC pairing for the Linux table is
// the on-disk source of the ARP fallback.
func TestParseARPRows_ProcNetArp(t *testing.T) {
	sample := "IP address       HW type     Flags       HW address            Mask     Device\n" +
		"10.0.0.1         0x1         0x0         00:00:00:00:00:00     *        eth0\n" +
		"10.0.0.5         0x1         0x2         aa:bb:cc:dd:ee:ff     *        eth0\n"

	rows := parseARPRows(sample)
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 complete row (header + incomplete skipped), got %d", len(rows))
	}
	if rows[0].IP != "10.0.0.5" || rows[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected the complete row 10.0.0.5/aa:bb:cc:dd:ee:ff, got %q/%q", rows[0].IP, rows[0].MAC)
	}
}

// Mutation check: breaking the mac arp -a "(IP) at MAC" shape makes this test
// fail — that layout is how macOS reports a resolved link-layer address.
func TestParseARPRows_ArpA_MacOSStyle(t *testing.T) {
	sample := "? (10.0.0.5) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]\n" +
		"? (10.0.0.9) at (incomplete) on en0 ifscope [ethernet]\n"

	rows := parseARPRows(sample)
	if len(rows) != 1 {
		t.Fatalf("expected the complete (MAC) entry only, incomplete skipped, got %d", len(rows))
	}
	if rows[0].IP != "10.0.0.5" || rows[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected 10.0.0.5/aa:bb:cc:dd:ee:ff, got %q/%q", rows[0].IP, rows[0].MAC)
	}
}

// Mutation check: breaking the Windows dashes-MAC handling makes this test
// fail — Windows `arp -a` emits "aa-bb-cc-dd-ee-ff" (dashes), not colons.
func TestParseARPRows_WindowsDashMAC(t *testing.T) {
	sample := "  Internet Address          Physical Address      Type\n" +
		"  192.168.1.1               aa-bb-cc-dd-ee-ff     dynamic\n"

	rows := parseARPRows(sample)
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row from the Windows table, got %d", len(rows))
	}
	if rows[0].IP != "192.168.1.1" || rows[0].MAC != "aa-bb-cc-dd-ee-ff" {
		t.Errorf("expected 192.168.1.1/aa-bb-cc-dd-ee-ff, got %q/%q", rows[0].IP, rows[0].MAC)
	}
}
