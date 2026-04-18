package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// NonceSnapshot is the signed state dump defined in QDP-0001 §7.1. A
// snapshot is produced periodically by a validator; fresh-joining
// nodes compare K snapshots from trusted peers to seed their ledger
// before they trust incoming traffic.
type NonceSnapshot struct {
	SchemaVersion int                   `json:"schemaVersion"`
	BlockHeight   int64                 `json:"blockHeight"`
	BlockHash     string                `json:"blockHash"`
	Timestamp     int64                 `json:"timestamp"`
	TrustDomain   string                `json:"trustDomain"`
	Entries       []NonceSnapshotEntry  `json:"entries"`
	ProducerQuid  string                `json:"producerQuid"`
	Signature     string                `json:"signature"`
}

// NonceSnapshotEntry is a single (signer, epoch, max-accepted-nonce)
// record in a snapshot.
type NonceSnapshotEntry struct {
	Quid     string `json:"quid"`
	Epoch    uint32 `json:"epoch"`
	MaxNonce int64  `json:"maxNonce"`
}

// Schema version for snapshot wire format. Bump on breaking changes.
const SnapshotSchemaVersion = 1

// DefaultSnapshotInterval is the number of blocks between snapshot
// productions in a domain, per QDP-0001 §7.2 proposal (64).
const DefaultSnapshotInterval = 64

// Errors from snapshot production / verification.
var (
	ErrSnapshotMissingProducer = errors.New("snapshot: missing producerQuid")
	ErrSnapshotBadSchema       = errors.New("snapshot: unknown schemaVersion")
	ErrSnapshotBadSignature    = errors.New("snapshot: signature verification failed")
	ErrSnapshotNoProducerKey   = errors.New("snapshot: no public key recorded for producer quid")
)

// GetSnapshotSignableData returns canonical bytes for signing a
// snapshot. Equivalent to marshaling the struct with Signature cleared.
func GetSnapshotSignableData(s NonceSnapshot) ([]byte, error) {
	s.Signature = ""
	return json.Marshal(s)
}

// ProduceSnapshot builds an unsigned NonceSnapshot from the ledger's
// current accepted map, scoped to the supplied domain. Entries are
// sorted by (Quid, Epoch) so two honest producers at the same height
// yield byte-identical snapshots. The caller then signs it with
// SignSnapshot before broadcast.
//
// If the ledger is empty for the given domain, an empty but valid
// snapshot is still returned (not an error) — this is useful for
// establishing an audit trail even for cold domains.
func (l *NonceLedger) ProduceSnapshot(blockHeight int64, blockHash, domain, producerQuid string, now time.Time) NonceSnapshot {
	snap := l.Snapshot() // returns the accepted map

	entries := make([]NonceSnapshotEntry, 0, len(snap))
	for k, v := range snap {
		if k.Domain != domain {
			continue
		}
		entries = append(entries, NonceSnapshotEntry{
			Quid:     k.Quid,
			Epoch:    k.Epoch,
			MaxNonce: v,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Quid != entries[j].Quid {
			return entries[i].Quid < entries[j].Quid
		}
		return entries[i].Epoch < entries[j].Epoch
	})

	return NonceSnapshot{
		SchemaVersion: SnapshotSchemaVersion,
		BlockHeight:   blockHeight,
		BlockHash:     blockHash,
		Timestamp:     now.Unix(),
		TrustDomain:   domain,
		Entries:       entries,
		ProducerQuid:  producerQuid,
		// Signature filled in by SignSnapshot.
	}
}

// SignSnapshot signs the snapshot using the supplied node's private
// key. The node is responsible for establishing that its quid matches
// snap.ProducerQuid — this function just signs.
func (node *QuidnugNode) SignSnapshot(snap NonceSnapshot) (NonceSnapshot, error) {
	data, err := GetSnapshotSignableData(snap)
	if err != nil {
		return snap, fmt.Errorf("snapshot: canonicalization: %w", err)
	}
	sig, err := node.SignData(data)
	if err != nil {
		return snap, fmt.Errorf("snapshot: signing: %w", err)
	}
	snap.Signature = hex.EncodeToString(sig)
	return snap, nil
}

// VerifySnapshot returns nil if the snapshot's signature is valid for
// its declared ProducerQuid, using the ledger to resolve the producer's
// current public key. Does not validate the snapshot's *contents* —
// that is a policy decision (fresh-join agreement protocol, etc.) left
// to callers.
func VerifySnapshot(l *NonceLedger, s NonceSnapshot) error {
	if s.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("%w: %d", ErrSnapshotBadSchema, s.SchemaVersion)
	}
	if s.ProducerQuid == "" {
		return ErrSnapshotMissingProducer
	}
	if l == nil {
		return ErrSnapshotNoProducerKey
	}

	// Use the producer's *current* epoch key. If the producer has
	// rotated, their latest key is the signing key.
	epoch := l.CurrentEpoch(s.ProducerQuid)
	producerKey, ok := l.GetSignerKey(s.ProducerQuid, epoch)
	if !ok || producerKey == "" {
		return ErrSnapshotNoProducerKey
	}

	signable, err := GetSnapshotSignableData(s)
	if err != nil {
		return fmt.Errorf("snapshot: canonicalization: %w", err)
	}
	sig, err := hex.DecodeString(s.Signature)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSnapshotBadSignature, err)
	}
	if !VerifySignature(producerKey, signable, hex.EncodeToString(sig)) {
		return ErrSnapshotBadSignature
	}
	return nil
}
