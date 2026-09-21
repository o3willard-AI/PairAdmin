// Package scanner provides target feeders for PairAdmin's network scanner.
//
// Each feeder implements the IPFeeder interface so a scan engine can consume
// targets uniformly regardless of the source (CIDR block, IP range, file, or
// local network discovery). All feeders are pure iteration primitives — they
// perform no network I/O and no concurrency.
//
// IPv6 is out of scope for v1. Every feeder accepts or returns IPv4 addresses
// only; providing an IPv6 address or CIDR yields ErrIPv6Unsupported.
package scanner

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// ErrIPv6Unsupported is returned when an IPv6 address or CIDR is supplied.
// IPv6 is explicitly out of scope for v1; callers receive this sentinel
// rather than silently getting no results.
var ErrIPv6Unsupported = fmt.Errorf("IPv6 is not supported in v1; only IPv4 targets are accepted")

// MaxRangeSize is the largest target range a feeder will accept without
// explicit caller confirmation. It equals the number of addresses in a /16
// (65 536). Any range exceeding this requires confirmed=true at construction.
const MaxRangeSize = uint64(1) << 16

// IPFeeder is the common iteration interface shared by all target feeders.
// A scan engine consumes any feeder uniformly: call HasNext, then Next,
// until HasNext returns false; PercentageComplete reports 0–100 progress.
type IPFeeder interface {
	// HasNext reports whether the feeder can yield at least one more target.
	HasNext() bool
	// Next returns the next target IP. Returns an error if the feeder is
	// exhausted or encounters a problem during iteration.
	Next() (net.IP, error)
	// PercentageComplete returns the fraction of the range consumed,
	// as a percentage in the range [0, 100].
	PercentageComplete() float64
}

// ipToUint32 converts a 4-byte IPv4 net.IP to a uint32 (big-endian octet
// order) for arithmetic during iteration.
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// uint32ToIP converts a uint32 (big-endian octet order) to a 4-byte IPv4
// net.IP suitable for return from Next().
func uint32ToIP(n uint32) net.IP {
	return net.IP{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}

// clampPercent scales yielded/total into a 0–100 percentage and clamps to
// the valid range. Shared by every feeder's PercentageComplete.
func clampPercent(yielded, total uint64) float64 {
	if total == 0 {
		return 100.0
	}
	pct := float64(yielded) / float64(total) * 100.0
	if pct > 100.0 {
		return 100.0
	}
	return pct
}

// Collect drains a feeder into a slice, consuming it entirely. It is a
// convenience for tests and simple single-shot consumers that need all
// targets up front.
func Collect(f IPFeeder) ([]net.IP, error) {
	var result []net.IP
	for f.HasNext() {
		ip, err := f.Next()
		if err != nil {
			return result, err
		}
		result = append(result, ip)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// CIDRFeeder
// ---------------------------------------------------------------------------

// CIDRFeeder iterates every IP address in a CIDR block (e.g. 192.168.1.0/24).
// The iteration includes the network address and the broadcast address so
// that the full /24 is covered.
type CIDRFeeder struct {
	total   uint64
	current uint32
	yielded uint64
}

// NewCIDRFeeder creates a feeder that iterates every IP in the given CIDR.
// If the CIDR covers more than MaxRangeSize (a /16) addresses and confirmed
// is false, it returns an error rather than creating the feeder.
func NewCIDRFeeder(cidrStr string, confirmed bool) (*CIDRFeeder, error) {
	_, ipnet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return nil, fmt.Errorf("parse CIDR %q: %w", cidrStr, err)
	}

	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("CIDR %q: %w", cidrStr, ErrIPv6Unsupported)
	}

	ones, bits := ipnet.Mask.Size()
	total := uint64(1) << uint(bits-ones)

	if !confirmed && total > MaxRangeSize {
		return nil, fmt.Errorf(
			"CIDR %q yields %d addresses, exceeding the /16 limit (%d); "+
				"pass confirmed=true to scan ranges larger than a /16",
			cidrStr, total, MaxRangeSize,
		)
	}

	return &CIDRFeeder{
		total:   total,
		current: ipToUint32(ip4),
	}, nil
}

// HasNext implements IPFeeder.
func (f *CIDRFeeder) HasNext() bool {
	return f.yielded < f.total
}

// Next implements IPFeeder.
func (f *CIDRFeeder) Next() (net.IP, error) {
	if !f.HasNext() {
		return nil, fmt.Errorf("CIDRFeeder is exhausted")
	}

	result := uint32ToIP(f.current)
	f.yielded++

	if f.yielded < f.total {
		f.current++
	}

	return result, nil
}

// PercentageComplete implements IPFeeder.
func (f *CIDRFeeder) PercentageComplete() float64 {
	return clampPercent(f.yielded, f.total)
}

// ---------------------------------------------------------------------------
// RangeFeeder
// ---------------------------------------------------------------------------

// RangeFeeder iterates every IP address in an inclusive start..end range.
// If startIP > endIP the iteration proceeds in descending (reversed) order,
// which lets callers express a reversed sweep with the natural argument
// order.
type RangeFeeder struct {
	total      uint64
	current    uint32
	yielded    uint64
	descending bool
}

// NewRangeFeeder creates a feeder that iterates every IP from startIP to
// endIP (inclusive). If startIP sorts higher than endIP the direction is
// reversed. If the range exceeds MaxRangeSize addresses and confirmed is
// false, an error is returned.
func NewRangeFeeder(startIP, endIP string, confirmed bool) (*RangeFeeder, error) {
	s := net.ParseIP(startIP)
	if s == nil {
		return nil, fmt.Errorf("parse start IP %q: invalid address", startIP)
	}
	s4 := s.To4()
	if s4 == nil {
		return nil, fmt.Errorf("start IP %q: %w", startIP, ErrIPv6Unsupported)
	}

	e := net.ParseIP(endIP)
	if e == nil {
		return nil, fmt.Errorf("parse end IP %q: invalid address", endIP)
	}
	e4 := e.To4()
	if e4 == nil {
		return nil, fmt.Errorf("end IP %q: %w", endIP, ErrIPv6Unsupported)
	}

	sn := ipToUint32(s4)
	en := ipToUint32(e4)

	descending := sn > en

	var total uint64
	if descending {
		total = uint64(sn-en) + 1
	} else {
		total = uint64(en-sn) + 1
	}

	if !confirmed && total > MaxRangeSize {
		return nil, fmt.Errorf(
			"range %s..%s yields %d addresses, exceeding the /16 limit (%d); "+
				"pass confirmed=true to scan ranges larger than a /16",
			startIP, endIP, total, MaxRangeSize,
		)
	}

	return &RangeFeeder{
		total:      total,
		current:    sn,
		descending: descending,
	}, nil
}

// HasNext implements IPFeeder.
func (f *RangeFeeder) HasNext() bool {
	return f.yielded < f.total
}

// Next implements IPFeeder.
func (f *RangeFeeder) Next() (net.IP, error) {
	if !f.HasNext() {
		return nil, fmt.Errorf("RangeFeeder is exhausted")
	}

	result := uint32ToIP(f.current)
	f.yielded++

	if f.yielded < f.total {
		if f.descending {
			f.current--
		} else {
			f.current++
		}
	}

	return result, nil
}

// PercentageComplete implements IPFeeder.
func (f *RangeFeeder) PercentageComplete() float64 {
	return clampPercent(f.yielded, f.total)
}

// ---------------------------------------------------------------------------
// FileFeeder
// ---------------------------------------------------------------------------

// FileFeeder reads IP addresses (one per line) from a file. Blank lines and
// comment lines (beginning with '#') are skipped. Lines that do not parse as
// valid IPv4 addresses are also skipped, so a file may freely mix comments,
// blanks, and stray text with valid targets.
type FileFeeder struct {
	targets []net.IP
	total   uint64
	yielded uint64
}

// NewFileFeeder creates a feeder that reads IPv4 targets from the given file
// path. The file is parsed eagerly so that I/O or parse errors surface at
// construction time rather than mid-iteration.
func NewFileFeeder(path string) (*FileFeeder, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open target file %q: %w", path, err)
	}
	defer file.Close()

	var targets []net.IP
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ip := net.ParseIP(line)
		if ip == nil {
			// Skip lines that aren't valid IPs — a targets file may
			// legitimately contain annotations or stray text.
			continue
		}
		ip4 := ip.To4()
		if ip4 == nil {
			// IPv6 — out of scope for v1.
			continue
		}
		targets = append(targets, ip4)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan target file %q: %w", path, err)
	}

	return &FileFeeder{
		targets: targets,
		total:   uint64(len(targets)),
	}, nil
}

// HasNext implements IPFeeder.
func (f *FileFeeder) HasNext() bool {
	return f.yielded < f.total
}

// Next implements IPFeeder.
func (f *FileFeeder) Next() (net.IP, error) {
	if !f.HasNext() {
		return nil, fmt.Errorf("FileFeeder is exhausted")
	}

	result := f.targets[f.yielded]
	f.yielded++

	return result, nil
}

// PercentageComplete implements IPFeeder.
func (f *FileFeeder) PercentageComplete() float64 {
	return clampPercent(f.yielded, f.total)
}

// ---------------------------------------------------------------------------
// LocalNetsFeeder
// ---------------------------------------------------------------------------

// ipv4Mask24 is the netmask for an IPv4 /24.
var ipv4Mask24 = net.CIDRMask(24, 32)

// LocalNetsFeeder enumerates the host's local IPv4 /24 networks and iterates
// every usable IP across them. It skips the network address, the broadcast
// address, and the host's own IP(s) within each /24.
//
// Only non-loopback, up, non-point-to-point interfaces are considered.
//
// IPv6 is out of scope for v1: only IPv4 /24 networks are discovered. If the
// host has no IPv4 addresses on eligible interfaces, NewLocalNetsFeeder
// returns an error (wrapping ErrIPv6Unsupported) rather than silently
// producing an empty set.
type LocalNetsFeeder struct {
	targets  []net.IP
	total    uint64
	yielded  uint64
	networks []net.IPNet
}

// NewLocalNetsFeeder discovers the host's local IPv4 /24 networks and builds
// a feeder that iterates every usable IP across them.
func NewLocalNetsFeeder() (*LocalNetsFeeder, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("enumerate network interfaces: %w", err)
	}

	seenNet := make(map[string]bool)
	hostIPs := make(map[uint32]bool)
	var networks []net.IPNet

	for _, iface := range interfaces {
		// Only consider non-loopback, up, non-point-to-point interfaces.
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
			var ip net.IP

			// Interface.Addrs() returns *net.IPNet (for CIDR addresses) or
			// *net.IPAddr (for bare IP addresses).
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			ip4 := ip.To4()
			if ip4 == nil {
				// IPv6 address — out of scope for v1.
				continue
			}

			masked := ip4.Mask(ipv4Mask24)
			key := masked.String()

			if !seenNet[key] {
				seenNet[key] = true
				networks = append(networks, net.IPNet{
					IP:   masked,
					Mask: ipv4Mask24,
				})
			}

			// Record the host's own IP so it can be skipped during iteration.
			hostIPs[ipToUint32(ip4)] = true
		}
	}

	if len(networks) == 0 {
		return nil, fmt.Errorf(
			"no local IPv4 /24 networks found; "+
				"IPv6 addresses exist but are out of scope for v1: %w",
			ErrIPv6Unsupported,
		)
	}

	// Pre-compute the full target list across all discovered /24 networks.
	var targets []net.IP
	for _, nw := range networks {
		base := ipToUint32(nw.IP.To4())

		// A /24 has 256 slots (0–255). Slot 0 is the network address and
		// slot 255 is the broadcast address; both are skipped.
		for i := uint32(1); i <= 254; i++ {
			addr := base + i
			if hostIPs[addr] {
				continue
			}
			targets = append(targets, uint32ToIP(addr))
		}
	}

	return &LocalNetsFeeder{
		targets:  targets,
		total:    uint64(len(targets)),
		networks: networks,
	}, nil
}

// HasNext implements IPFeeder.
func (f *LocalNetsFeeder) HasNext() bool {
	return f.yielded < f.total
}

// Next implements IPFeeder.
func (f *LocalNetsFeeder) Next() (net.IP, error) {
	if !f.HasNext() {
		return nil, fmt.Errorf("LocalNetsFeeder is exhausted")
	}

	result := f.targets[f.yielded]
	f.yielded++

	return result, nil
}

// PercentageComplete implements IPFeeder.
func (f *LocalNetsFeeder) PercentageComplete() float64 {
	return clampPercent(f.yielded, f.total)
}

// Networks returns a copy of the local IPv4 /24 networks that this feeder was
// built from. Useful for diagnostics and UI display.
func (f *LocalNetsFeeder) Networks() []net.IPNet {
	result := make([]net.IPNet, len(f.networks))
	copy(result, f.networks)
	return result
}
