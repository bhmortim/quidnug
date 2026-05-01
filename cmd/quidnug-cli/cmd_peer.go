// `quidnug-cli peer` — operator commands for the peering layer.
//
//   peer list                 list known peers as the queried node sees them
//   peer add ADDR [flags]     append entry to the local peers_file
//   peer remove ADDR          remove entry from the local peers_file
//   peer scan-lan             one-shot mDNS browse of the local segment
//
// `peer list` queries the standard `--node` URL via /api/v1/nodes
// (matches the existing CLI plumbing). `peer add` / `peer remove`
// edit the operator's peers.yaml directly so the live-reload
// watcher in the running node picks up the change without a
// restart. `peer scan-lan` runs an mDNS browse from the CLI host
// and prints whatever responds — useful as a "is anything out
// there?" probe.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/quidnug/quidnug/internal/peering"
)

func cmdPeer(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("peer: subcommand required (list | show | add | remove | scan-lan)")
	}
	switch args[0] {
	case "list":
		return cmdPeerList(args[1:])
	case "show":
		return cmdPeerShow(args[1:])
	case "add":
		return cmdPeerAdd(args[1:])
	case "remove", "rm":
		return cmdPeerRemove(args[1:])
	case "scan-lan":
		return cmdPeerScanLAN(args[1:])
	default:
		return fmt.Errorf("peer: unknown subcommand %q", args[0])
	}
}

// peerListEntry is the local view of one /api/v1/nodes record,
// shaped for human-readable display.
type peerListEntry struct {
	ID               string   `json:"id"`
	Address          string   `json:"address"`
	TrustDomains     []string `json:"trustDomains,omitempty"`
	IsValidator      bool     `json:"isValidator,omitempty"`
	LastSeen         int64    `json:"lastSeen,omitempty"`
	ConnectionStatus string   `json:"connectionStatus,omitempty"`
}

func cmdPeerList(args []string) error {
	fs := flag.NewFlagSet("peer list", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := c.Nodes(ctx, 250, 0)
	if err != nil {
		return fmt.Errorf("GET /nodes: %w", err)
	}
	// The envelope returned by GetNodesHandler is:
	//   { success, data: { data: [...], pagination: ... } }
	// pkg/client.Nodes returns the inner "data" map already, so
	// we re-encode + decode through peerListEntry to avoid
	// hand-coercing map[string]any.
	dataField, _ := resp["data"].([]any)
	if dataField == nil {
		// Some shapes nest under data.data.
		if inner, ok := resp["data"].(map[string]any); ok {
			if d2, ok := inner["data"].([]any); ok {
				dataField = d2
			}
		}
	}
	body, _ := json.Marshal(dataField)
	var peers []peerListEntry
	_ = json.Unmarshal(body, &peers)
	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })

	// Best-effort: pull the per-peer scores so we can show a
	// SCORE column. Failure is non-fatal — the score endpoint
	// is new, and an older node won't expose it.
	scores := map[string]float64{}
	if scoreEnv, err := c.RawGet(ctx, "/peers"); err == nil {
		var env struct {
			Success bool `json:"success"`
			Data    struct {
				Peers []struct {
					NodeQuid  string  `json:"nodeQuid"`
					Composite float64 `json:"composite"`
				} `json:"peers"`
			} `json:"data"`
		}
		if json.Unmarshal(scoreEnv, &env) == nil {
			for _, p := range env.Data.Peers {
				scores[p.NodeQuid] = p.Composite
			}
		}
	}

	if cf.JSON {
		// Annotate each peer with its score for JSON output.
		type augmented struct {
			peerListEntry
			Composite *float64 `json:"composite,omitempty"`
		}
		out := make([]augmented, 0, len(peers))
		for _, p := range peers {
			a := augmented{peerListEntry: p}
			if s, ok := scores[p.ID]; ok {
				a.Composite = &s
			}
			out = append(out, a)
		}
		body, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(body))
		return nil
	}
	if len(peers) == 0 {
		fmt.Println("(no known peers)")
		return nil
	}
	fmt.Printf("%-18s %-32s %-10s %-7s %s\n", "NODE ID", "ADDRESS", "SOURCE", "SCORE", "DOMAINS")
	for _, p := range peers {
		src := p.ConnectionStatus
		if src == "" {
			src = "?"
		}
		domains := strings.Join(p.TrustDomains, ",")
		if domains == "" {
			domains = "-"
		}
		scoreStr := "-"
		if s, ok := scores[p.ID]; ok {
			scoreStr = fmt.Sprintf("%.2f", s)
		}
		fmt.Printf("%-18s %-32s %-10s %-7s %s\n", trunc(p.ID, 16), p.Address, src, scoreStr, domains)
	}
	return nil
}

// cmdPeerShow renders the full PeerScore for a single peer:
// composite, per-class rates, severe-event totals, quarantine
// state, and the recent-events ring buffer.
func cmdPeerShow(args []string) error {
	fs := flag.NewFlagSet("peer show", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return fmt.Errorf("peer show: NODE_QUID required")
	}
	quid := rest[0]
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := c.RawGet(ctx, "/peers/"+quid)
	if err != nil {
		return fmt.Errorf("GET /peers/%s: %w", quid, err)
	}
	if cf.JSON {
		fmt.Println(string(raw))
		return nil
	}
	// Decode + render the human view.
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			NodeQuid    string  `json:"nodeQuid"`
			Composite   float64 `json:"composite"`
			AdmittedAt  string  `json:"admittedAt"`
			LastUpdated string  `json:"lastUpdated"`
			Handshake   struct {
				Successes float64 `json:"successes"`
				Failures  float64 `json:"failures"`
			} `json:"handshake"`
			Gossip struct {
				Successes float64 `json:"successes"`
				Failures  float64 `json:"failures"`
			} `json:"gossip"`
			Query struct {
				Successes float64 `json:"successes"`
				Failures  float64 `json:"failures"`
			} `json:"query"`
			Broadcast struct {
				Successes float64 `json:"successes"`
				Failures  float64 `json:"failures"`
			} `json:"broadcast"`
			Validation struct {
				Successes float64 `json:"successes"`
				Failures  float64 `json:"failures"`
			} `json:"validation"`
			ForkClaims       int    `json:"forkClaims"`
			SignatureFails   int    `json:"signatureFails"`
			AdRevocations    int    `json:"adRevocations"`
			Quarantined      bool   `json:"quarantined"`
			QuarantineReason string `json:"quarantineReason,omitempty"`
			RecentEvents     []struct {
				Timestamp string `json:"timestamp"`
				Class     string `json:"class,omitempty"`
				OK        bool   `json:"ok,omitempty"`
				Severe    string `json:"severe,omitempty"`
				Note      string `json:"note,omitempty"`
			} `json:"recentEvents,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	d := env.Data
	fmt.Printf("Peer:             %s\n", d.NodeQuid)
	fmt.Printf("Composite:        %.3f\n", d.Composite)
	if d.Quarantined {
		fmt.Printf("Quarantined:      yes — %s\n", d.QuarantineReason)
	}
	fmt.Printf("Admitted at:      %s\n", d.AdmittedAt)
	fmt.Printf("Last updated:     %s\n", d.LastUpdated)
	fmt.Printf("\nPer-class success/failure (decay-adjusted):\n")
	fmt.Printf("  handshake:      %.1f / %.1f\n", d.Handshake.Successes, d.Handshake.Failures)
	fmt.Printf("  gossip:         %.1f / %.1f\n", d.Gossip.Successes, d.Gossip.Failures)
	fmt.Printf("  query:          %.1f / %.1f\n", d.Query.Successes, d.Query.Failures)
	fmt.Printf("  broadcast:      %.1f / %.1f\n", d.Broadcast.Successes, d.Broadcast.Failures)
	fmt.Printf("  validation:     %.1f / %.1f\n", d.Validation.Successes, d.Validation.Failures)
	if d.ForkClaims+d.SignatureFails+d.AdRevocations > 0 {
		fmt.Printf("\nSevere events:\n")
		if d.ForkClaims > 0 {
			fmt.Printf("  fork claims:    %d\n", d.ForkClaims)
		}
		if d.SignatureFails > 0 {
			fmt.Printf("  signature fails: %d\n", d.SignatureFails)
		}
		if d.AdRevocations > 0 {
			fmt.Printf("  ad revocations: %d\n", d.AdRevocations)
		}
	}
	if len(d.RecentEvents) > 0 {
		fmt.Printf("\nRecent events (newest last):\n")
		for _, ev := range d.RecentEvents {
			tag := ev.Class
			if ev.Severe != "" {
				tag = "SEVERE/" + ev.Severe
			}
			result := "  ok"
			if !ev.OK && ev.Severe == "" {
				result = "FAIL"
			}
			note := ev.Note
			if note == "" {
				note = "-"
			}
			fmt.Printf("  %s  %-12s  %s  %s\n", ev.Timestamp, tag, result, note)
		}
	}
	return nil
}

func cmdPeerAdd(args []string) error {
	fs := flag.NewFlagSet("peer add", flag.ContinueOnError)
	var (
		operatorQuid string
		allowPrivate bool
		peersFile    string
	)
	fs.StringVar(&operatorQuid, "operator-quid", "", "pin the peer's operator quid (optional, recommended)")
	fs.BoolVar(&allowPrivate, "allow-private", false, "bypass the dial-filter for this address (LAN peers)")
	fs.StringVar(&peersFile, "file", "", "path to peers.yaml (defaults to $PEERS_FILE)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return fmt.Errorf("peer add: ADDRESS required (and only one)")
	}
	addr := rest[0]
	if peersFile == "" {
		peersFile = os.Getenv("PEERS_FILE")
	}
	if peersFile == "" {
		return fmt.Errorf("peer add: --file or PEERS_FILE env var is required")
	}
	entries, err := loadPeersOrEmpty(peersFile)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Address == addr {
			return fmt.Errorf("peer add: %s is already in %s", addr, peersFile)
		}
	}
	entries = append(entries, peering.PeerEntry{
		Address:      addr,
		OperatorQuid: strings.ToLower(strings.TrimSpace(operatorQuid)),
		AllowPrivate: allowPrivate,
	})
	if err := writePeersFile(peersFile, entries); err != nil {
		return err
	}
	fmt.Printf("Added %s to %s\n", addr, peersFile)
	return nil
}

func cmdPeerRemove(args []string) error {
	fs := flag.NewFlagSet("peer remove", flag.ContinueOnError)
	peersFile := fs.String("file", "", "path to peers.yaml (defaults to $PEERS_FILE)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return fmt.Errorf("peer remove: ADDRESS required")
	}
	addr := rest[0]
	if *peersFile == "" {
		*peersFile = os.Getenv("PEERS_FILE")
	}
	if *peersFile == "" {
		return fmt.Errorf("peer remove: --file or PEERS_FILE env var is required")
	}
	entries, err := loadPeersOrEmpty(*peersFile)
	if err != nil {
		return err
	}
	out := entries[:0]
	removed := false
	for _, e := range entries {
		if e.Address == addr {
			removed = true
			continue
		}
		out = append(out, e)
	}
	if !removed {
		return fmt.Errorf("peer remove: %s not found in %s", addr, *peersFile)
	}
	if err := writePeersFile(*peersFile, out); err != nil {
		return err
	}
	fmt.Printf("Removed %s from %s\n", addr, *peersFile)
	return nil
}

func cmdPeerScanLAN(args []string) error {
	fs := flag.NewFlagSet("peer scan-lan", flag.ContinueOnError)
	service := fs.String("service", "_quidnug._tcp", "mDNS service type to browse")
	timeout := fs.Duration("timeout", 5*time.Second, "how long to listen for responses")
	jsonOut := fs.Bool("json", false, "emit JSON to stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	browser := peering.NewLANBrowser()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout+1*time.Second)
	defer cancel()
	if err := browser.Start(ctx, peering.LANBrowserConfig{
		ServiceName: *service,
		Interval:    *timeout,
	}); err != nil {
		return fmt.Errorf("scan-lan: %w", err)
	}
	defer browser.Stop()

	deadline := time.NewTimer(*timeout)
	defer deadline.Stop()
	seen := map[string]peering.LANPeer{}
loop:
	for {
		select {
		case p, ok := <-browser.Events():
			if !ok {
				break loop
			}
			if p.Address == "" {
				continue
			}
			seen[p.Address] = p
		case <-deadline.C:
			break loop
		}
	}
	if *jsonOut {
		out := make([]peering.LANPeer, 0, len(seen))
		for _, p := range seen {
			out = append(out, p)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
		body, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(body))
		return nil
	}
	if len(seen) == 0 {
		fmt.Println("(no LAN peers responded within", timeout.String(), ")")
		return nil
	}
	fmt.Printf("%-30s %-22s %-18s %s\n", "ADDRESS", "HOSTNAME", "NODE ID", "OPERATOR QUID")
	addrs := make([]string, 0, len(seen))
	for a := range seen {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)
	for _, a := range addrs {
		p := seen[a]
		fmt.Printf("%-30s %-22s %-18s %s\n",
			p.Address, trunc(p.Hostname, 22), nonempty(p.NodeID, "-"), nonempty(p.OperatorQuid, "-"))
	}
	return nil
}

// --- helpers ----------------------------------------------------------------

func loadPeersOrEmpty(path string) ([]peering.PeerEntry, error) {
	clean := filepath.Clean(path)
	if _, err := os.Stat(clean); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return peering.LoadPeersFile(clean)
}

// writePeersFile rewrites the file atomically (write to tmp +
// rename) so a partial write can't corrupt the operator's
// config and trigger a parse failure on the next live-reload.
// Permissions: 0600.
func writePeersFile(path string, entries []peering.PeerEntry) error {
	type schema struct {
		Peers []peering.PeerEntry `json:"peers" yaml:"peers"`
	}
	doc := schema{Peers: entries}

	var body []byte
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		var err error
		body, err = json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		body = append(body, '\n')
	default:
		// YAML — emit by hand so the layout matches the
		// example config.
		var b strings.Builder
		b.WriteString("peers:\n")
		for _, e := range entries {
			b.WriteString("  - address: \"")
			b.WriteString(e.Address)
			b.WriteString("\"\n")
			if e.OperatorQuid != "" {
				b.WriteString("    operator_quid: \"")
				b.WriteString(e.OperatorQuid)
				b.WriteString("\"\n")
			}
			if e.AllowPrivate {
				b.WriteString("    allow_private: true\n")
			}
		}
		body = []byte(b.String())
	}
	tmp := path + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func nonempty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
