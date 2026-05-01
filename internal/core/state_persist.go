// Cross-restart state persistence.
//
// ENG-75: data_dir was declared in config + Docker volumes were
// mounted, but the node binary only ever wrote pending_transactions.json
// to it. Every restart wiped block height, all dynamic domain
// registrations, and regenerated the per-process ECDSA keypair —
// the latter producing a brand-new NodeID on each boot, which
// silently invalidated any TRUST grants pointing at the old one.
//
// This file fixes that with three companion files in data_dir:
//
//   data_dir/
//     node_key.json         Per-process ECDSA P-256 keypair
//                           (PKCS8 DER, hex-encoded). Loaded at
//                           NewQuidnugNode; if absent, a new key
//                           is generated AND written so the
//                           identity is captured on first boot.
//     blockchain.json       Snapshot of the full block history.
//                           Loaded after NewQuidnugNode so
//                           genesis is replaced with the
//                           persisted chain. Saved on shutdown
//                           and on a 30-second ticker.
//     trust_domains.json    Snapshot of TrustDomains + the
//                           DomainRegistry index. Same lifecycle
//                           as blockchain.json.
//
// All three writes use safeio.WriteFileMode for atomic write +
// 0600 permissions. Schema versioned; future bumps are graceful
// (load fails, fall back to defaults).
package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/quidnug/quidnug/internal/safeio"
)

const (
	stateSchemaVersion       = 1
	statePersistTickInterval = 30 * time.Second
)

// nodeKeyFile is the on-disk format for the per-process
// keypair. We don't reuse OperatorQuidFile's format because the
// roles are distinct: OperatorQuidFile is the long-lived
// identity that accumulates trust; nodeKey is the per-process
// gossip-signing key that needs to be stable across restarts
// for NodeID continuity.
type nodeKeyFile struct {
	SchemaVersion int    `json:"schemaVersion"`
	NodeID        string `json:"nodeId"`
	PublicKeyHex  string `json:"publicKeyHex"`
	PrivateKeyHex string `json:"privateKeyHex"` // PKCS8 DER, hex
	CreatedAt     int64  `json:"createdAt"`     // unix seconds
}

// loadOrCreateNodeKey loads the per-process keypair from
// data_dir/node_key.json. If the file doesn't exist, generates
// a fresh keypair and writes the file before returning. Any
// other error (corrupt file, schema mismatch) is fatal — we'd
// rather refuse to boot than silently regenerate identity and
// orphan all trust grants.
func loadOrCreateNodeKey(dataDir string) (*ecdsa.PrivateKey, string, error) {
	if dataDir == "" {
		// No persistence configured (typical in tests); fall
		// back to ephemeral. This preserves the pre-ENG-75
		// behavior for callers that don't configure a data
		// directory.
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, "", fmt.Errorf("ephemeral keygen: %w", err)
		}
		pubBytes := elliptic.Marshal(k.Curve, k.X, k.Y)
		nodeID := fmt.Sprintf("%x", sha256.Sum256(pubBytes))[:16]
		return k, nodeID, nil
	}
	path := filepath.Join(dataDir, "node_key.json")
	if raw, err := safeio.ReadFile(path); err == nil {
		var f nodeKeyFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, "", fmt.Errorf("parse %q: %w", path, err)
		}
		if f.SchemaVersion != stateSchemaVersion {
			return nil, "", fmt.Errorf("node_key schema %d not supported", f.SchemaVersion)
		}
		der, err := hex.DecodeString(f.PrivateKeyHex)
		if err != nil {
			return nil, "", fmt.Errorf("decode key: %w", err)
		}
		keyAny, err := x509.ParsePKCS8PrivateKey(der)
		if err != nil {
			return nil, "", fmt.Errorf("parse key: %w", err)
		}
		priv, ok := keyAny.(*ecdsa.PrivateKey)
		if !ok || priv.Curve != elliptic.P256() {
			return nil, "", fmt.Errorf("not a P-256 ECDSA key")
		}
		// Cross-check NodeID matches the loaded public key
		// (defends against a swapped file).
		pubBytes := elliptic.Marshal(priv.Curve, priv.PublicKey.X, priv.PublicKey.Y)
		want := fmt.Sprintf("%x", sha256.Sum256(pubBytes))[:16]
		if f.NodeID != "" && f.NodeID != want {
			return nil, "", fmt.Errorf("nodeId in key file (%q) does not match public key (%q)", f.NodeID, want)
		}
		return priv, want, nil
	}
	// File doesn't exist (or unreadable) — generate fresh and
	// persist. We do NOT distinguish "missing" from "I/O
	// error" here because safeio.ReadFile already validated
	// the path; an error means first boot or wiped data dir.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("keygen: %w", err)
	}
	pubBytes := elliptic.Marshal(priv.Curve, priv.PublicKey.X, priv.PublicKey.Y)
	nodeID := fmt.Sprintf("%x", sha256.Sum256(pubBytes))[:16]
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, "", fmt.Errorf("marshal key: %w", err)
	}
	body := nodeKeyFile{
		SchemaVersion: stateSchemaVersion,
		NodeID:        nodeID,
		PublicKeyHex:  hex.EncodeToString(pubBytes),
		PrivateKeyHex: hex.EncodeToString(der),
		CreatedAt:     time.Now().Unix(),
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("marshal nodeKeyFile: %w", err)
	}
	raw = append(raw, '\n')
	// Ensure the data dir exists before writing.
	if err := safeio.MkdirAllMode(dataDir, 0o750); err != nil {
		return nil, "", fmt.Errorf("create data dir: %w", err)
	}
	if err := safeio.WriteFileMode(path, raw, 0o600); err != nil {
		return nil, "", fmt.Errorf("write %q: %w", path, err)
	}
	logger.Info("Persisted new node identity to data dir",
		"dataDir", dataDir, "nodeId", nodeID)
	return priv, nodeID, nil
}

// blockchainSnapshot is the on-disk shape for the chain.
type blockchainSnapshot struct {
	SchemaVersion int     `json:"schemaVersion"`
	SavedAt       int64   `json:"savedAt"` // unix nanoseconds
	Blocks        []Block `json:"blocks"`
}

// trustDomainsSnapshot is the on-disk shape for TrustDomains +
// the lookup indexes.
type trustDomainsSnapshot struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	SavedAt        int64                    `json:"savedAt"`
	TrustDomains   map[string]TrustDomain   `json:"trustDomains"`
	DomainRegistry map[string][]string      `json:"domainRegistry"`
}

// LoadBlockchain reads data_dir/blockchain.json into the node.
// Replaces the genesis-only chain populated by NewQuidnugNode.
// Missing file is silent (clean start).
//
// ENG-81: after the chain itself is restored, every block's
// transactions are replayed through processBlockTransactions —
// the same code path the live commit takes in ReceiveBlock. Each
// `update*Registry` call is an upsert, so the replay is
// idempotent by construction (an already-populated registry
// converges to the same end state on a second pass). Without
// this, the chain comes back on /api/v1/blocks but the in-memory
// indices (TrustRegistry, IdentityRegistry,
// NodeAdvertisementRegistry, EventStreamRegistry, TitleRegistry,
// QuidDomainIndex, etc.) start empty every restart. Visible
// symptoms: /api/v1/registry/trust returns edges:0,
// /api/v2/discovery/operator/<op> returns nodes:[], and static
// peer admission fails with "admit static: no current
// NodeAdvertisement for <peer>" until live bootstrap eventually
// rebuilds state from gossip.
//
// Genesis (Blockchain[0]) has an empty Transactions slice on
// every node and is a no-op when processed. We still iterate
// from index 0 for code symmetry with the live path: any future
// genesis-content gets handled the same way the live commit
// would handle it.
//
// processBlockTransactions runs without holding BlockchainMutex
// — it touches the per-registry locks instead. That mirrors the
// live commit (which calls processBlockTransactions AFTER
// releasing BlockchainMutex), so we can't deadlock against any
// live-traffic path that already holds a registry lock.
func (node *QuidnugNode) LoadBlockchain(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	path := filepath.Join(dataDir, "blockchain.json")
	raw, err := safeio.ReadFile(path)
	if err != nil {
		return nil // missing is fine
	}
	var snap blockchainSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return fmt.Errorf("parse %q: %w", path, err)
	}
	if snap.SchemaVersion != stateSchemaVersion {
		return fmt.Errorf("blockchain schema %d not supported", snap.SchemaVersion)
	}
	if len(snap.Blocks) == 0 {
		return nil
	}
	node.BlockchainMutex.Lock()
	node.Blockchain = snap.Blocks
	node.BlockchainMutex.Unlock()

	// ENG-81: replay each block's transactions to hydrate the
	// in-memory registries. Skip genesis (no-op anyway) only by
	// virtue of its empty Transactions slice; we don't special-
	// case index 0 so that any future genesis-payload gets the
	// same treatment as live blocks.
	totalTxReplayed := 0
	for i := range snap.Blocks {
		totalTxReplayed += len(snap.Blocks[i].Transactions)
		node.processBlockTransactions(snap.Blocks[i])
	}

	if logger != nil {
		logger.Info("Loaded persisted blockchain",
			"path", path, "blocks", len(snap.Blocks),
			"height", snap.Blocks[len(snap.Blocks)-1].Index,
			"txReplayed", totalTxReplayed)
	}
	return nil
}

// LoadTrustDomains reads data_dir/trust_domains.json into the
// node. Missing file is silent.
func (node *QuidnugNode) LoadTrustDomains(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	path := filepath.Join(dataDir, "trust_domains.json")
	raw, err := safeio.ReadFile(path)
	if err != nil {
		return nil
	}
	var snap trustDomainsSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return fmt.Errorf("parse %q: %w", path, err)
	}
	if snap.SchemaVersion != stateSchemaVersion {
		return fmt.Errorf("trust_domains schema %d not supported", snap.SchemaVersion)
	}
	if len(snap.TrustDomains) > 0 {
		node.TrustDomainsMutex.Lock()
		// Don't overwrite the genesis "default" domain that
		// NewQuidnugNode populated; merge instead.
		for k, v := range snap.TrustDomains {
			if _, ok := node.TrustDomains[k]; !ok {
				node.TrustDomains[k] = v
			}
		}
		node.TrustDomainsMutex.Unlock()
	}
	if len(snap.DomainRegistry) > 0 {
		node.DomainRegistryMutex.Lock()
		for k, v := range snap.DomainRegistry {
			node.DomainRegistry[k] = v
		}
		node.DomainRegistryMutex.Unlock()
	}
	if logger != nil {
		logger.Info("Loaded persisted trust domains",
			"path", path,
			"domains", len(snap.TrustDomains),
			"registryEntries", len(snap.DomainRegistry))
	}
	return nil
}

// SaveBlockchain snapshots the chain to data_dir/blockchain.json.
// Atomic write through safeio.
func (node *QuidnugNode) SaveBlockchain(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	node.BlockchainMutex.RLock()
	blocks := make([]Block, len(node.Blockchain))
	copy(blocks, node.Blockchain)
	node.BlockchainMutex.RUnlock()
	snap := blockchainSnapshot{
		SchemaVersion: stateSchemaVersion,
		SavedAt:       time.Now().UnixNano(),
		Blocks:        blocks,
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal blockchain: %w", err)
	}
	raw = append(raw, '\n')
	return safeio.WriteFileMode(filepath.Join(dataDir, "blockchain.json"), raw, 0o600)
}

// SaveTrustDomains snapshots TrustDomains + DomainRegistry to
// data_dir/trust_domains.json.
func (node *QuidnugNode) SaveTrustDomains(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	node.TrustDomainsMutex.RLock()
	tds := make(map[string]TrustDomain, len(node.TrustDomains))
	for k, v := range node.TrustDomains {
		tds[k] = v
	}
	node.TrustDomainsMutex.RUnlock()
	node.DomainRegistryMutex.RLock()
	dr := make(map[string][]string, len(node.DomainRegistry))
	for k, v := range node.DomainRegistry {
		cp := make([]string, len(v))
		copy(cp, v)
		dr[k] = cp
	}
	node.DomainRegistryMutex.RUnlock()
	snap := trustDomainsSnapshot{
		SchemaVersion:  stateSchemaVersion,
		SavedAt:        time.Now().UnixNano(),
		TrustDomains:   tds,
		DomainRegistry: dr,
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trust domains: %w", err)
	}
	raw = append(raw, '\n')
	return safeio.WriteFileMode(filepath.Join(dataDir, "trust_domains.json"), raw, 0o600)
}

// runStatePersistLoop periodically snapshots blockchain + trust
// domains to data_dir. Same shape as runPeerScorePersistLoop.
// Final flush on ctx cancel preserves the latest state across
// SIGTERM. Failures log but don't tear the loop down.
func (node *QuidnugNode) runStatePersistLoop(ctx context.Context, dataDir string) {
	if dataDir == "" {
		return
	}
	tk := time.NewTicker(statePersistTickInterval)
	defer tk.Stop()
	flush := func(reason string) {
		if err := node.SaveBlockchain(dataDir); err != nil && logger != nil {
			logger.Warn("Blockchain snapshot failed",
				"reason", reason, "error", err)
		}
		if err := node.SaveTrustDomains(dataDir); err != nil && logger != nil {
			logger.Warn("Trust-domain snapshot failed",
				"reason", reason, "error", err)
		}
	}
	// Initial flush on a tiny delay so the boot path can settle
	// any genesis-replacement actions before the snapshot runs.
	select {
	case <-ctx.Done():
		flush("shutdown")
		return
	case <-time.After(2 * time.Second):
	}
	flush("initial")
	for {
		select {
		case <-ctx.Done():
			flush("shutdown")
			return
		case <-tk.C:
			flush("periodic")
		}
	}
}

