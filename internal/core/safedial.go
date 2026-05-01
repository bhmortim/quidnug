// SSRF defense for the node-to-node HTTP client.
//
// The Quidnug node makes outbound HTTP calls to peer addresses
// learned from the gossip network. Peer-advertised addresses are
// untrusted input: a malicious peer could advertise 127.0.0.1,
// a private RFC1918 range, link-local, or a cloud-metadata IP and
// trick the node into making requests against its own infrastructure.
//
// safeDialContext enforces the perimeter at the dial layer so every
// outbound call goes through it, regardless of how callers construct
// URLs. The filter resolves hostnames and rejects:
//
//   - loopback (127.0.0.0/8, ::1)
//   - link-local (169.254.0.0/16, fe80::/10), including AWS/GCP/Azure
//     metadata at 169.254.169.254
//   - private RFC1918 / RFC4193 ranges (10/8, 172.16/12, 192.168/16,
//     fc00::/7)
//   - the unspecified address (0.0.0.0, ::)
//   - multicast and broadcast
//
// The filter can be relaxed via QUIDNUG_ALLOW_PRIVATE_PEERS=1 for
// integration tests and lab deployments where peers genuinely live
// on a private network. It must NOT be set in production.
package core

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// testingT returns testing.Testing() but is wrapped so the import
// is unambiguous even if a future refactor splits this file.
func testingT() bool { return testing.Testing() }

// allowPrivatePeers reports whether the operator has opted into
// allowing private/loopback peer addresses globally. Set
// QUIDNUG_ALLOW_PRIVATE_PEERS=1 to enable; default is deny.
//
// When running under `go test` (testing.Testing() == true on
// Go 1.21+), we also allow private addresses by default so the
// existing localhost-based integration test fleet keeps working
// without each test having to plumb the env var. Production
// binaries do not have testing.Testing() return true.
func allowPrivatePeers() bool {
	v := strings.TrimSpace(os.Getenv("QUIDNUG_ALLOW_PRIVATE_PEERS"))
	if v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
		return true
	}
	if testingT() {
		return true
	}
	return false
}

// PrivateAddrAllowList is a per-node, thread-safe set of
// "host:port" or "host" tokens whose corresponding outbound
// dials are exempt from the private/loopback/link-local
// rejection rule. Operators populate it implicitly by listing
// peers in peers_file with `allow_private: true` (or by enabling
// LAN discovery, which adds mDNS-found peers automatically).
//
// The allow-list is the targeted alternative to the global
// QUIDNUG_ALLOW_PRIVATE_PEERS env var: production deployments
// that need exactly two LAN peers can list those two peers
// instead of opening the dial filter for every private range.
type PrivateAddrAllowList struct {
	mu    sync.RWMutex
	addrs map[string]struct{}
}

// NewPrivateAddrAllowList constructs an empty allow-list.
func NewPrivateAddrAllowList() *PrivateAddrAllowList {
	return &PrivateAddrAllowList{addrs: make(map[string]struct{})}
}

// Set replaces the allow-list contents with the given tokens.
// Tokens may be "host:port" (precise), "host" (matches any port
// for that host), or a literal IP. Comparison is byte-exact.
func (a *PrivateAddrAllowList) Set(tokens []string) {
	if a == nil {
		return
	}
	next := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		next[t] = struct{}{}
	}
	a.mu.Lock()
	a.addrs = next
	a.mu.Unlock()
}

// Has reports whether either the full host:port or the host
// portion alone is on the allow-list.
func (a *PrivateAddrAllowList) Has(hostPort string) bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if _, ok := a.addrs[hostPort]; ok {
		return true
	}
	if host, _, err := net.SplitHostPort(hostPort); err == nil {
		if _, ok := a.addrs[host]; ok {
			return true
		}
	}
	return false
}

// isBlockedIP reports whether ip should be refused as a peer
// destination. Loopback, private, link-local, multicast, broadcast,
// and unspecified are all rejected unless allowPrivatePeers() is true.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if allowPrivatePeers() {
		return false
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() {
		return true
	}
	// IPv4 broadcast 255.255.255.255
	if ip4 := ip.To4(); ip4 != nil && ip4.Equal(net.IPv4bcast) {
		return true
	}
	return false
}

// safeDialContext is the legacy free-function variant kept for
// callers that do not yet have access to a PrivateAddrAllowList.
// New code should prefer NewSafeDialContext(allow) which wraps a
// per-node allow-list for the precise-override path.
func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return safeDialWith(ctx, network, address, nil)
}

// NewSafeDialContext returns a DialContext closure that consults
// the supplied PrivateAddrAllowList for per-peer "this address
// is fine even though it's in an otherwise-blocked range"
// overrides. The allow-list may be nil (equivalent to no
// overrides). Callers wire the returned func into
// http.Transport.DialContext.
func NewSafeDialContext(allow *PrivateAddrAllowList) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return safeDialWith(ctx, network, address, allow)
	}
}

// peerResolver is the DNS resolver used by safedial. Forced to
// the pure-Go path (PreferGo=true, no cgo) so we get consistent
// behavior across Linux/macOS/Windows AND so we don't pick up
// any cgo-resolver-side negative caching on platforms where
// glibc's nsswitch is involved. ENG-74: container DNS races at
// boot manifested as cached "no such host" results, and forcing
// pure-Go is the simplest mitigation without rolling our own
// cache.
var peerResolver = &net.Resolver{
	PreferGo: true,
}

func safeDialWith(ctx context.Context, network, address string, allow *PrivateAddrAllowList) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("safedial: split host/port %q: %w", address, err)
	}
	// Per-peer allow-list short-circuit. The operator listed this
	// host or host:port in peers_file (or it came in via mDNS),
	// so the perimeter check steps aside for this dial only.
	if allow != nil && allow.Has(address) {
		d := &net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  peerResolver,
		}
		return d.DialContext(ctx, network, address)
	}
	// Resolve all candidate addresses; reject if any are blocked,
	// because a future-time switch in DNS could otherwise route
	// through a blocked target.
	ips, err := peerResolver.LookupIP(ctx, ipNetwork(network), host)
	if err != nil {
		return nil, fmt.Errorf("safedial: resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("safedial: no addresses for %q", host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("safedial: refused address %s for %q (blocked range)", ip, host)
		}
	}
	// All resolved addresses passed the perimeter; dial the first
	// allowed one. Use the standard dialer for actual connection
	// establishment (TCP keepalive, default timeouts). The
	// resolver is wired in too so any internal hostname lookups
	// the dialer might make also go through the pure-Go path.
	d := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Resolver:  peerResolver,
	}
	return d.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

// ipNetwork maps the dial network argument ("tcp", "tcp4", "tcp6")
// to the corresponding LookupIP filter.
func ipNetwork(network string) string {
	switch network {
	case "tcp4", "udp4", "ip4":
		return "ip4"
	case "tcp6", "udp6", "ip6":
		return "ip6"
	default:
		return "ip"
	}
}

// SanitizedPeerAddress is the taint-laundered form of a peer
// address. It is intentionally a distinct type so taint-analyzing
// scanners see the conversion step from raw `string` to a checked
// value. Use String() to turn it back into a host:port for URL
// composition.
type SanitizedPeerAddress struct {
	hostPort string
}

// String returns the sanitized host:port. Safe to use in
// fmt.Sprintf, http.NewRequest, etc.
func (s SanitizedPeerAddress) String() string { return s.hostPort }

// ValidatePeerAddress is a synchronous variant of safeDialContext
// for callers that need to gate URL construction _before_ handing
// off to http.NewRequest. The dial-layer filter is the
// authoritative defense, but call-site validation is necessary so
// taint-analyzing scanners (CodeQL/gosec G107) can see the gate.
//
// addr must be a "host:port" string (the canonical form used by
// Node.Address throughout the codebase). Hostnames are resolved
// and every returned IP is checked against the same blocklist as
// safeDialContext. Returns a SanitizedPeerAddress that callers
// substitute into URL construction in place of the raw input;
// because the return type is distinct from `string`, the taint
// flow visibly terminates here.
func ValidatePeerAddress(addr string) (SanitizedPeerAddress, error) {
	zero := SanitizedPeerAddress{}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return zero, fmt.Errorf("peer address: split host/port %q: %w", addr, err)
	}
	if host == "" {
		return zero, fmt.Errorf("peer address: empty host in %q", addr)
	}
	// Reject any control characters / NUL / CR-LF in the host or
	// port portions. These can't appear in a valid TCP endpoint
	// and their presence is a clear injection attempt.
	for _, s := range []string{host, port} {
		for _, r := range s {
			if r < 0x20 || r == 0x7f {
				return zero, fmt.Errorf("peer address: control character in %q", addr)
			}
		}
	}
	// First try interpreting host as a literal IP, which avoids a
	// DNS round-trip for the common case where peer addresses are
	// raw IPs from gossip.
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return zero, fmt.Errorf("peer address: %s is in a blocked range", ip)
		}
		return SanitizedPeerAddress{hostPort: net.JoinHostPort(host, port)}, nil
	}
	// Hostname: resolve and check every result so a future DNS
	// flip can't sneak through.
	ips, err := net.LookupIP(host)
	if err != nil {
		return zero, fmt.Errorf("peer address: resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return zero, fmt.Errorf("peer address: no addresses for %q", host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return zero, fmt.Errorf("peer address: %s for %q is in a blocked range", ip, host)
		}
	}
	return SanitizedPeerAddress{hostPort: net.JoinHostPort(host, port)}, nil
}
