// mDNS / DNS-SD LAN discovery for Quidnug nodes.
//
// When LAN discovery is enabled, each node:
//
//  1. Advertises itself on the local segment via the
//     `_quidnug._tcp.local.` service type. The TXT record
//     carries the node's NodeID, OperatorQuid (if configured),
//     and supported domain count so peers can short-circuit the
//     /api/v1/info handshake when those values match.
//
//  2. Browses the same service type and emits a stream of
//     LANPeer events for every responder.
//
// The library used (github.com/grandcat/zeroconf) is pure Go,
// no cgo, no Avahi/Bonjour dependency — it speaks the Multicast
// DNS wire protocol directly. That keeps the deploy surface
// the same on Linux, macOS, and Windows.
//
// LAN discovery is opt-in (Config.LANDiscovery=true). It does
// NOT grant any trust privilege: discovered peers go through
// the same admit pipeline as gossip peers, with the only
// difference being that mDNS-found addresses get added to the
// PrivateAddrAllowList automatically (since "on the same LAN"
// means the address is necessarily private).
package peering

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// LANPeer is one record from the mDNS browse stream.
type LANPeer struct {
	// Address is the host:port the peer claims to serve at,
	// derived from the mDNS A/AAAA record + SRV port. Already
	// shaped for direct dial.
	Address string

	// NodeID is the peer's claimed quid identifier (16 hex
	// chars), pulled from the TXT record. May be empty if the
	// peer is an older node that doesn't advertise it.
	NodeID string

	// OperatorQuid is the peer's claimed operator quid, also
	// from TXT. Empty when the responder runs without a
	// configured operator quid (ephemeral mode).
	OperatorQuid string

	// Hostname is the .local hostname of the responder, useful
	// for diagnostics ("peer X is on alice-laptop.local").
	Hostname string
}

// LANServer publishes the local node on mDNS. Construction
// returns immediately; the first announcement happens on a
// background goroutine.
type LANServer struct {
	srv *zeroconf.Server
}

// LANServerConfig parameters the announcement.
type LANServerConfig struct {
	// ServiceName is the mDNS service type, e.g.
	// "_quidnug._tcp". Defaults to "_quidnug._tcp" when empty.
	ServiceName string

	// InstanceName is the per-host instance name visible in
	// browse listings. Should be human-readable and unique on
	// the local segment (the library appends a numeric suffix
	// on collision). Defaults to "quidnug-<NodeID-prefix>".
	InstanceName string

	// Port is the TCP port the node listens on for HTTP API.
	// Required.
	Port int

	// NodeID is the local node's quid (16 hex). Optional; goes
	// into the TXT record so browsers can pre-bind the address
	// to the identity without a handshake round-trip.
	NodeID string

	// OperatorQuid is the local node's operator quid (16 hex).
	// Optional; goes into the TXT record for the same reason
	// as NodeID.
	OperatorQuid string

	// Domain is the mDNS domain. Always "local." for LAN
	// discovery; left configurable so tests can override.
	Domain string
}

// AnnounceLANServer starts publishing. Stop() releases the
// listener and goodbye-packets the announcement.
func AnnounceLANServer(cfg LANServerConfig) (*LANServer, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "_quidnug._tcp"
	}
	if cfg.Domain == "" {
		cfg.Domain = "local."
	}
	if cfg.InstanceName == "" {
		// Use NodeID prefix for uniqueness; mDNS treats
		// collisions automatically anyway.
		idHint := cfg.NodeID
		if len(idHint) > 8 {
			idHint = idHint[:8]
		}
		if idHint == "" {
			idHint = "anon"
		}
		cfg.InstanceName = "quidnug-" + idHint
	}
	if cfg.Port <= 0 {
		return nil, fmt.Errorf("mdns announce: port required")
	}
	txt := []string{
		"protocol=quidnug-v1",
	}
	if cfg.NodeID != "" {
		txt = append(txt, "nodeId="+cfg.NodeID)
	}
	if cfg.OperatorQuid != "" {
		txt = append(txt, "operatorQuid="+cfg.OperatorQuid)
	}
	srv, err := zeroconf.Register(
		cfg.InstanceName,
		cfg.ServiceName,
		cfg.Domain,
		cfg.Port,
		txt,
		nil, // ifaces=nil: announce on all
	)
	if err != nil {
		return nil, fmt.Errorf("mdns register: %w", err)
	}
	return &LANServer{srv: srv}, nil
}

// Stop shuts the announcement down. Idempotent.
func (s *LANServer) Stop() {
	if s == nil || s.srv == nil {
		return
	}
	s.srv.Shutdown()
	s.srv = nil
}

// LANBrowser subscribes to mDNS announcements for the configured
// service type and emits LANPeer events on the returned channel.
type LANBrowser struct {
	out      chan LANPeer
	stop     chan struct{}
	once     sync.Once
}

// LANBrowserConfig parameters the browse loop.
type LANBrowserConfig struct {
	// ServiceName is the mDNS service type, e.g.
	// "_quidnug._tcp". Must match the publishers.
	ServiceName string

	// Domain defaults to "local.".
	Domain string

	// Interval is how often the browser re-queries. Defaults
	// to 30s. Below 5s is wasteful (mDNS is stateful, the
	// local cache covers most queries).
	Interval time.Duration
}

// NewLANBrowser constructs a browser. Start it with Start().
func NewLANBrowser() *LANBrowser {
	return &LANBrowser{
		out:  make(chan LANPeer, 16),
		stop: make(chan struct{}),
	}
}

// Events returns the channel discovered peers arrive on.
func (b *LANBrowser) Events() <-chan LANPeer { return b.out }

// Start runs the browse loop until ctx is cancelled. Returns an
// error if the resolver could not be initialized; otherwise nil
// (events arrive asynchronously on the channel).
func (b *LANBrowser) Start(ctx context.Context, cfg LANBrowserConfig) error {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "_quidnug._tcp"
	}
	if cfg.Domain == "" {
		cfg.Domain = "local."
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("mdns resolver: %w", err)
	}
	go b.run(ctx, resolver, cfg)
	return nil
}

func (b *LANBrowser) run(ctx context.Context, resolver *zeroconf.Resolver, cfg LANBrowserConfig) {
	defer close(b.out)
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stop:
			return
		default:
		}
		entries := make(chan *zeroconf.ServiceEntry, 8)
		// Each browse round runs for a quarter of the
		// interval so we have time to process before the
		// next round queues up. The library doesn't dedup
		// entries across rounds, so the consumer should
		// idempotently handle repeat events (the admit
		// pipeline does — repeat AdmitPeer calls for the
		// same NodeQuid are no-ops).
		roundCtx, cancel := context.WithTimeout(ctx, cfg.Interval/4)
		err := resolver.Browse(roundCtx, cfg.ServiceName, cfg.Domain, entries)
		if err != nil {
			cancel()
			// Soft error; sleep then retry.
			select {
			case <-ctx.Done():
				return
			case <-time.After(cfg.Interval):
				continue
			}
		}
		// Drain entries until the round closes the channel.
		for entry := range entries {
			peer := lanPeerFromEntry(entry)
			if peer.Address == "" {
				continue
			}
			select {
			case b.out <- peer:
			default:
				// Buffer full; drop. Next round will
				// re-emit if the peer is still up.
			}
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-b.stop:
			return
		case <-time.After(cfg.Interval):
		}
	}
}

// Stop terminates the browse loop. Idempotent.
func (b *LANBrowser) Stop() {
	b.once.Do(func() { close(b.stop) })
}

// lanPeerFromEntry converts a zeroconf ServiceEntry to a LANPeer,
// pulling NodeID + OperatorQuid out of the TXT record and the
// dial address from the SRV/A/AAAA records.
func lanPeerFromEntry(e *zeroconf.ServiceEntry) LANPeer {
	if e == nil {
		return LANPeer{}
	}
	out := LANPeer{Hostname: e.HostName}
	// Prefer first IPv4; fall back to IPv6.
	var host string
	if len(e.AddrIPv4) > 0 {
		host = e.AddrIPv4[0].String()
	} else if len(e.AddrIPv6) > 0 {
		host = "[" + e.AddrIPv6[0].String() + "]"
	}
	if host == "" || e.Port == 0 {
		return LANPeer{}
	}
	out.Address = host + ":" + strconv.Itoa(e.Port)
	for _, kv := range e.Text {
		idx := strings.IndexByte(kv, '=')
		if idx <= 0 {
			continue
		}
		k := kv[:idx]
		v := kv[idx+1:]
		switch k {
		case "nodeId":
			out.NodeID = strings.ToLower(strings.TrimSpace(v))
		case "operatorQuid":
			out.OperatorQuid = strings.ToLower(strings.TrimSpace(v))
		}
	}
	return out
}
