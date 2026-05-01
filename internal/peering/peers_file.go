// Package peering implements operator-managed peer admission for
// Quidnug nodes. It owns three orthogonal peer sources (a static
// `peers_file`, mDNS LAN discovery, and gossip-discovered peers)
// and a single `admitPeer` pipeline that all three feed into.
//
// This file is the static-peers loader: it reads a YAML/JSON file
// at the path configured by Config.PeersFile, parses it into a
// list of PeerEntry values, and (when Watch is called) emits
// reload events on the returned channel whenever the file
// changes on disk.
//
// Schema:
//
//	peers:
//	  - address: "node2.example.com:8080"
//	    operator_quid: "034bc467852ffa94"  # optional: pin operator quid
//	    allow_private: false                # default false
//	  - address: "192.168.1.50:8080"
//	    operator_quid: "feedfacedeadbeef"
//	    allow_private: true                 # bypass safedial for THIS peer only
//
// `allow_private: true` is the explicit escape hatch for LAN
// peers. It is honored ONLY for entries the operator wrote into
// peers_file. It is NOT honored for peers learned via gossip,
// because gossip-advertised addresses are untrusted-by-default.
//
// Path traversal / NUL injection / non-regular files are
// rejected at the safeio.ReadFile boundary.
package peering

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/quidnug/quidnug/internal/safeio"
	"gopkg.in/yaml.v3"
)

// PeerEntry is one record from peers_file. Address is the only
// required field; OperatorQuid pins the operator if known and
// AllowPrivate flips the per-peer dial-filter override on.
type PeerEntry struct {
	Address      string `json:"address" yaml:"address"`
	OperatorQuid string `json:"operatorQuid,omitempty" yaml:"operator_quid,omitempty"`
	AllowPrivate bool   `json:"allowPrivate,omitempty" yaml:"allow_private,omitempty"`
	// Note string `json:"note,omitempty" yaml:"note,omitempty"`  // free-form operator memo (future)
}

// peersFileSchema is the on-disk top-level structure. Keeping it
// distinct from PeerEntry allows future fields like rev/version
// without breaking either type.
type peersFileSchema struct {
	Peers []PeerEntry `json:"peers" yaml:"peers"`
}

// LoadPeersFile reads `path` and returns the parsed peer list.
// Detects YAML vs JSON by extension; falls through to YAML for
// unknown extensions (the Quidnug convention is YAML for
// operator-facing config).
//
// Validation:
//
//   - Address must be non-empty and parseable as host:port
//     (full TCP-endpoint validation is deferred to the admit
//     pipeline, which has access to the dial-time DNS resolver).
//   - OperatorQuid, if set, must look like a quid id (16 hex
//     chars). Anything else is rejected so a typo can't be
//     silently coerced to "no operator pin."
//   - Duplicate addresses are rejected so operators don't get
//     surprising "wins" semantics during reload.
func LoadPeersFile(path string) ([]PeerEntry, error) {
	if path == "" {
		return nil, nil
	}
	raw, err := safeio.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sch peersFileSchema
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(raw, &sch); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
	case ".yaml", ".yml", "":
		if err := yaml.Unmarshal(raw, &sch); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
	default:
		// Unknown extension; try YAML then JSON.
		if err := yaml.Unmarshal(raw, &sch); err != nil {
			if jerr := json.Unmarshal(raw, &sch); jerr != nil {
				return nil, fmt.Errorf("parse (tried YAML and JSON): %w / %v", err, jerr)
			}
		}
	}
	out := make([]PeerEntry, 0, len(sch.Peers))
	seen := make(map[string]struct{}, len(sch.Peers))
	for i, p := range sch.Peers {
		p.Address = strings.TrimSpace(p.Address)
		p.OperatorQuid = strings.ToLower(strings.TrimSpace(p.OperatorQuid))
		if p.Address == "" {
			return nil, fmt.Errorf("peers[%d]: address is empty", i)
		}
		if !looksLikeHostPort(p.Address) {
			return nil, fmt.Errorf("peers[%d]: address %q is not host:port", i, p.Address)
		}
		if p.OperatorQuid != "" && !looksLikeQuidID(p.OperatorQuid) {
			return nil, fmt.Errorf("peers[%d]: operator_quid %q is not a 16-hex-char quid id", i, p.OperatorQuid)
		}
		if _, dup := seen[p.Address]; dup {
			return nil, fmt.Errorf("peers[%d]: duplicate address %q", i, p.Address)
		}
		seen[p.Address] = struct{}{}
		out = append(out, p)
	}
	return out, nil
}

// looksLikeHostPort returns true iff s contains a port separator
// and neither side is empty. We don't validate the IP/hostname
// here; that happens later at dial time, with DNS access.
func looksLikeHostPort(s string) bool {
	// Bracketed IPv6 form [::1]:8080.
	if strings.HasPrefix(s, "[") {
		idx := strings.LastIndex(s, "]:")
		if idx < 1 {
			return false
		}
		if idx+2 >= len(s) {
			return false
		}
		return true
	}
	idx := strings.LastIndex(s, ":")
	if idx <= 0 || idx == len(s)-1 {
		return false
	}
	return true
}

// looksLikeQuidID is true iff s is exactly 16 lowercase hex
// characters (the canonical quid-id form: sha256(publicKey)[:16]).
func looksLikeQuidID(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// Watcher streams reload events for a peers_file. The first event
// arrives immediately on Start (initial load); subsequent events
// arrive whenever the file changes on disk.
//
// fsnotify is used directly rather than poll-stat so the latency
// from edit-to-effect is sub-second. Some editors (vim) write by
// rename; the watcher re-attaches to the inode after a Remove +
// Create cycle so those edits don't break the watch.
type Watcher struct {
	path  string
	out   chan []PeerEntry
	stop  chan struct{}
	once  sync.Once
	loadF func(string) ([]PeerEntry, error)
}

// NewWatcher constructs a Watcher for path. It does not open the
// file or start any goroutines until Start is called.
func NewWatcher(path string) *Watcher {
	return &Watcher{
		path:  path,
		out:   make(chan []PeerEntry, 1),
		stop:  make(chan struct{}),
		loadF: LoadPeersFile,
	}
}

// Events returns the channel reload events arrive on. Each event
// is the full peer list from the latest successful parse. A
// receiver should process the list as authoritative replacement,
// not as a delta.
func (w *Watcher) Events() <-chan []PeerEntry { return w.out }

// Start begins watching. Cancel ctx (or call Stop) to terminate.
// The first event (initial load) is delivered before Start
// returns nil so callers can deterministically observe the
// at-boot peers list.
//
// If the file is missing or unparseable at Start time, an error
// is returned and no events are emitted; the caller decides
// whether that is fatal.
func (w *Watcher) Start(ctx context.Context) error {
	if w.path == "" {
		return fmt.Errorf("peers watcher: empty path")
	}
	initial, err := w.loadF(w.path)
	if err != nil {
		return fmt.Errorf("initial load: %w", err)
	}
	// Deliver the at-boot list synchronously.
	select {
	case w.out <- initial:
	default:
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	// Watch the parent dir, not the file itself, so vim-style
	// rename-and-replace edits keep firing events. We filter to
	// the configured filename inside the goroutine.
	dir := filepath.Dir(w.path)
	if err := fsw.Add(dir); err != nil {
		_ = fsw.Close()
		return fmt.Errorf("watch %q: %w", dir, err)
	}
	go w.run(ctx, fsw)
	return nil
}

// run is the watch loop. Coalesces rapid event bursts (vim writes
// often produce three events in quick succession) by debouncing
// 200ms before re-reading.
func (w *Watcher) run(ctx context.Context, fsw *fsnotify.Watcher) {
	defer fsw.Close()
	defer close(w.out)

	target := filepath.Clean(w.path)
	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case ev, ok := <-fsw.Events:
			if !ok {
				return
			}
			if filepath.Clean(ev.Name) != target {
				continue
			}
			// Coalesce subsequent events into a single
			// debounced reload.
			debounce = time.After(200 * time.Millisecond)
		case <-debounce:
			debounce = nil
			peers, err := w.loadF(w.path)
			if err != nil {
				// Don't tear the watcher down on parse
				// failure; the operator may still be in
				// the middle of an edit. Drop the load
				// and wait for the next change.
				continue
			}
			select {
			case w.out <- peers:
			default:
				// Receiver is behind; drop. Latest
				// state is in `peers`, which the next
				// event will re-emit.
			}
		case _, ok := <-fsw.Errors:
			if !ok {
				return
			}
			// Soft error; keep watching.
		}
	}
}

// Stop terminates the watch loop. Idempotent.
func (w *Watcher) Stop() {
	w.once.Do(func() { close(w.stop) })
}
