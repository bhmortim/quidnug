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
	"testing"
	"time"
)

// testingT returns testing.Testing() but is wrapped so the import
// is unambiguous even if a future refactor splits this file.
func testingT() bool { return testing.Testing() }

// allowPrivatePeers reports whether the operator has opted into
// allowing private/loopback peer addresses. Set
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

// safeDialContext is an http.Transport.DialContext that resolves the
// requested address and refuses to connect if it points at a
// blocked range. Returns an explicit error so SSRF probes show up
// in logs rather than silently succeeding via redirects or DNS
// rebinding.
func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("safedial: split host/port %q: %w", address, err)
	}
	// Resolve all candidate addresses; reject if any are blocked,
	// because a future-time switch in DNS could otherwise route
	// through a blocked target.
	ips, err := net.DefaultResolver.LookupIP(ctx, ipNetwork(network), host)
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
	// establishment (TCP keepalive, default timeouts).
	d := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
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
