package scanner

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Mode B's FIRST fetcher: liveness detection — is a host UP, independent of
// whether it runs SSH. It reuses probe_ssh.go's `dialFunc` seam (default
// net.DialTimeout) for the TCP-connect probe: the same injection convention,
// the same package-level var. ICMP is deliberately NOT the primary probe —
// unprivileged ICMP is often disabled (net.ipv4.ping_group_range), so the
// reliable unprivileged signals are TCP-connect (where refused == alive) and
// the ARP table.

// aliveProbePorts are dialed in order to decide liveness. A successful
// connect marks the host up; so does a refused connection (RST) — a host that
// answers "nothing listening" is by definition alive. Only a full timeout
// across every port leaves the host "not alive by TCP".
var aliveProbePorts = []int{80, 443, 22, 139, 7}

// aliveDialTimeout bounds each per-port dial. A test seam so suite can shrink
// it instead of burning the full second.
var aliveDialTimeout = 1000 * time.Millisecond

// readARPRowsFn is the injectable seam around ARP-table reading — production
// code parses the OS ARP table (see readARPRows); tests feed a fake table.
var readARPRowsFn = readARPRows

// AliveResult is the Mode B liveness verdict for one host.
type AliveResult struct {
	// Alive is true when the host is up (proven by TCP connect, TCP RST, or
	// an ARP entry with a MAC). It is NEVER set by a probe failure.
	Alive bool
	// RTTMs is the measured round-trip of the first decisive TCP dial; 0 when
	// liveness came from ARP or wasn't established.
	RTTMs float64
	// Method is "tcp" | "arp" | "none".
	Method string
	// MAC is the host's link-layer address from the ARP table (best-effort;
	// empty when unknown or liveness came from TCP).
	MAC string
}

// ARPRow is one complete ARP table entry (a host with a resolved MAC).
type ARPRow struct {
	IP  string
	MAC string
}

// ProbeAlive reports whether host is up: TCP-connect first (reliable, with
// refused == alive), ARP second (best-effort, local-subnet only). Returns
// Method "none" only when neither signal established liveness.
func ProbeAlive(host string) AliveResult {
	// 1) TCP-connect liveness. The first decisive dial (connected OR refused,
	//    RST) proves the host is up and its RTT is measured on the spot.
	for i := 0; i < len(aliveProbePorts); i++ {
		port := aliveProbePorts[i]
		addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
		t0 := time.Now()
		conn, err := dialFunc("tcp", addr, aliveDialTimeout)
		rtt := time.Since(t0).Milliseconds()
		if err != nil {
			if isRefused(err) {
				// THE #1 RULE: refused is NEVER dead. The host answered
				// "nothing is listening here" — it is alive, it just closed
				// the port. (A firewall drop is a timeout, not a refusal.)
				return AliveResult{Alive: true, RTTMs: float64(rtt), Method: "tcp"}
			}
			// Timeout (or unroutable): unknown — try the next port.
			continue
		}
		conn.Close()
		return AliveResult{Alive: true, RTTMs: float64(rtt), Method: "tcp"}
	}

	// 2) ARP (best-effort). A host that blocks ICMP/TCP still answers ARP, so
	// an on-link ARP entry with a MAC is the most reliable LAN liveness
	// signal. An ABSENT entry is NOT proof the host is down — the local few
	// networks may be one hop away, or the host its own gateway.
	if mac := findARPMC(host); mac != "" {
		return AliveResult{Alive: true, RTTMs: 0, Method: "arp", MAC: mac}
	}
	return AliveResult{Alive: false, RTTMs: 0, Method: "none"}
}

// findARPMC returns host's MAC from the ARP table, or "" if absent.
func findARPMC(host string) string {
	rows := readARPRowsFn()
	for i := 0; i < len(rows); i++ {
		if rows[i].IP == host && rows[i].MAC != "" {
			return rows[i].MAC
		}
	}
	return ""
}

// readARPRows reads the OS ARP table and returns every complete entry
// (IP -> MAC). Linux parses /proc/net/arp; macOS/Windows shell out to arp.
// Any read/parse failure yields an empty table — ARP is best-effort, so a
// miss here must never count as "dead".
func readARPRows() []ARPRow {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/net/arp")
		if err != nil {
			return []ARPRow{}
		}
		return parseARPRows(string(data))
	}
	out, err := runARPTable()
	if err != nil {
		return []ARPRow{}
	}
	return parseARPRows(out)
}

// runARPTable executes the platform `arp` listing and returns its stdout.
// Windows prints a paged table (arp -a); macOS/Linux accept -n (numeric, no
// hostname lookups). Best-effort: any failure is an empty table upstream.
func runARPTable() (string, error) {
	if runtime.GOOS == "windows" {
		out, err := exec.CommandContext(context.Background(), "arp", "-a").Output()
		return string(out), err
	}
	out, err := exec.CommandContext(context.Background(), "arp", "-n").Output()
	return string(out), err
}

// parseARPRows parses an ARP-table dump in any supported format:
//
//	/proc/net/arp:  "IP   HWtype  Flags  HWaddress  Mask  Device"
//	arp -n (lin/mac): "Address  HWtype  HWaddress  Flags Mask  Iface"
//	arp -a (macOS):  "? (IP) at MAC on en0 ifscope [ethernet]"
//	arp -a (Windows): "IP   aa-bb-cc-dd-ee-ff   dynamic"
//
// A row is kept only when it carries a dotted-quad IP and a well-formed MAC.
func parseARPRows(text string) []ARPRow {
	rows := []ARPRow{}
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		ip, mac := parseARPLine(lines[i])
		if ip != "" && mac != "" {
			rows = append(rows, ARPRow{IP: ip, MAC: mac})
		}
	}
	return rows
}

// parseARPLine extracts (ip, mac) from a single ARP-table line, or ("", "")
// when the line carries no complete IP+MAC pair.
func parseARPLine(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", ""
	}
	// arp -a (macOS): "? (IP) at MAC on en0..." — the MAC is the token right
	// after the " at " marker and the IP is parenthesized.
	if at := strings.Index(line, " at "); at >= 0 {
		ip := parenIP(line)
		mac := ""
		rest := strings.Fields(line[at+4:])
		if len(rest) > 0 {
			mac = rest[0]
		}
		if ip != "" && isValidMAC(mac) {
			return ip, mac
		}
		return "", ""
	}
	// Columnar: pick the first dotted-quad IP and first well-formed MAC.
	ip := ""
	mac := ""
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if ip == "" && isValidIP(f) {
			ip = f
		} else if mac == "" && isValidMAC(f) {
			mac = f
		}
	}
	if ip != "" && mac != "" {
		return ip, mac
	}
	return "", ""
}

// parenIP returns the content of the first "(...)" rune-run when it is a
// valid dotted-quad IP, else "".
func parenIP(line string) string {
	o := strings.Index(line, "(")
	c := strings.Index(line, ")")
	if o < 0 || c <= o {
		return ""
	}
	inner := strings.Fields(line[o+1 : c])
	if len(inner) > 0 && isValidIP(inner[0]) {
		return inner[0]
	}
	return ""
}

// isValidIP is true for a dotted-quad IPv4 literal with octets in 0..255.
func isValidIP(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "" || len(p) > 3 {
			return false
		}
		v := 0
		b := []byte(p)
		for j := 0; j < len(b); j++ {
			c := b[j]
			if c < '0' || c > '9' {
				return false
			}
			v = v*10 + int(c-'0')
		}
		if v > 255 {
			return false
		}
	}
	return true
}

// isValidMAC is true for a 6-octet hardware address in "aa:bb:.." or
// "aa-bb-.." form (Linux/macOS use colons, Windows uses dashes). The all-zero
// MAC — a proc/net/arp "incomplete" placeholder — is explicitly rejected so a
// blank ARP entry is never taken as a resolved host.
func isValidMAC(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		parts = strings.Split(s, "-")
		if len(parts) != 6 {
			return false
		}
	}
	allZero := true
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if len(p) != 2 {
			return false
		}
		b := []byte(p)
		for j := 0; j < len(b); j++ {
			c := b[j]
			hex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !hex {
				return false
			}
			if c != '0' {
				allZero = false
			}
		}
	}
	return !allZero
}
