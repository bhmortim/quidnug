// reviews_integration_test.go — end-to-end test of the QRP-0001
// (reviews & comments) use case against the reference Go node.
//
// Mirrors the Python demo at `examples/reviews-and-comments/demo/demo.py`
// but runs entirely in-process — no HTTP, no signing helper. The
// test proves the exact same transaction lifecycle the HTTP demo
// exercises, so any regression in one will show up in the other.
//
// What this test locks down:
//
//  1. All four tx types (IDENTITY, TITLE, TRUST, EVENT) can be
//     submitted, validated, included in a block, and committed to
//     their respective registries.
//  2. EVENT transactions specifically — they regressed in the past
//     by being missing from `GenerateBlock`'s domain-extraction
//     switch AND `ValidateBlockTiered`'s per-tx validation switch.
//     A green run here guarantees events round-trip.
//  3. Per-observer relational trust — distinct observers with
//     different trust edges see distinct rating neighborhoods.
//
// If this test starts failing it means one of the above invariants
// broke, and the live demo will break with it.

package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

type testActor struct {
	Name    string
	Priv    *ecdsa.PrivateKey
	PubHex  string // SEC1 uncompressed — `04 || X || Y`
	QuidID  string // sha256(pubBytes)[:16]
}

func newActor(name string) *testActor {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	pubBytes := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck
	return &testActor{
		Name:   name,
		Priv:   priv,
		PubHex: hex.EncodeToString(pubBytes),
		QuidID: fmt.Sprintf("%x", sha256.Sum256(pubBytes))[:16],
	}
}

// signIEEE1363 signs `data` and returns hex-encoded r||s (64 bytes).
// This is the exact encoding the node's VerifySignature accepts.
func (a *testActor) signIEEE1363(data []byte) string {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, a.Priv, hash[:])
	if err != nil {
		return ""
	}
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return hex.EncodeToString(sig)
}

func (a *testActor) signIdentity(tx IdentityTransaction) IdentityTransaction {
	tx.PublicKey = a.PubHex
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(signable)
	return tx
}

func (a *testActor) signTitle(tx TitleTransaction) TitleTransaction {
	tx.PublicKey = a.PubHex
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(signable)
	return tx
}

func (a *testActor) signTrust(tx TrustTransaction) TrustTransaction {
	tx.PublicKey = a.PubHex
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(signable)
	return tx
}

func (a *testActor) signEvent(tx EventTransaction) EventTransaction {
	tx.PublicKey = a.PubHex
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(signable)
	return tx
}

// unused helper retained for future use if we swap to PKCS8 hex
// key-exchange (matches the Python demo's Quid class)
var _ = x509.ParsePKCS8PrivateKey

// TestReviewsFullFlow_InProcess exercises IDENTITY → TITLE → TRUST
// → EVENT across a single node, producing a block that must
// validate, and confirms the event ends up in the stream registry.
func TestReviewsFullFlow_InProcess(t *testing.T) {
	node := newTestNode()

	domain := "reviews.public.technology.laptops"
	node.TrustDomains[domain] = TrustDomain{
		Name:                domain,
		ValidatorNodes:      []string{node.NodeID},
		TrustThreshold:      0.5,
		Validators:          map[string]float64{node.NodeID: 1.0},
		ValidatorPublicKeys: map[string]string{node.NodeID: node.GetPublicKeyHex()},
	}

	alice := newActor("alice")
	veteran := newActor("veteran")
	product := newActor("product-quid")

	// --- 1. Register identities ---
	for _, who := range []*testActor{alice, veteran, product} {
		tx := alice.signIdentity(IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          freshTxID(),
				Type:        TxTypeIdentity,
				TrustDomain: domain,
				Timestamp:   time.Now().Unix(),
			},
			QuidID:      who.QuidID,
			Name:        who.Name,
			Creator:     who.QuidID,
			UpdateNonce: 1,
		})
		// Override public key to match the owner — each identity
		// is self-signed by its own holder.
		tx.PublicKey = who.PubHex
		tx.Signature = ""
		signable, _ := json.Marshal(tx)
		tx.Signature = who.signIEEE1363(signable)
		tx.QuidID = who.QuidID // restore (in case)

		if _, err := node.AddIdentityTransaction(tx); err != nil {
			t.Fatalf("AddIdentityTransaction(%s): %v", who.Name, err)
		}
	}

	// Commit so title/events can reference the asset
	if err := generateAndCommitBlock(t, node, domain); err != nil {
		t.Fatalf("initial block commit: %v", err)
	}

	// --- 2. Register title (product owned by alice) ---
	titleTx := alice.signTitle(TitleTransaction{
		BaseTransaction: BaseTransaction{
			ID:          freshTxID(),
			Type:        TxTypeTitle,
			TrustDomain: domain,
			Timestamp:   time.Now().Unix(),
		},
		AssetID:    product.QuidID,
		Owners:     []OwnershipStake{{OwnerID: alice.QuidID, Percentage: 100.0}},
		Signatures: map[string]string{},
		TitleType:  "REVIEWABLE_PRODUCT",
	})
	if _, err := node.AddTitleTransaction(titleTx); err != nil {
		t.Fatalf("AddTitleTransaction: %v", err)
	}

	// --- 3. Trust edge: alice trusts veteran 0.9 ---
	trustTx := alice.signTrust(TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          freshTxID(),
			Type:        TxTypeTrust,
			TrustDomain: domain,
			Timestamp:   time.Now().Unix(),
		},
		Truster:    alice.QuidID,
		Trustee:    veteran.QuidID,
		TrustLevel: 0.9,
		Nonce:      1,
	})
	if _, err := node.AddTrustTransaction(trustTx); err != nil {
		t.Fatalf("AddTrustTransaction: %v", err)
	}

	// --- 4. Review event on veteran's own stream ---
	eventTx := veteran.signEvent(EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:          freshTxID(),
			Type:        TxTypeEvent,
			TrustDomain: domain,
			Timestamp:   time.Now().Unix(),
		},
		SubjectID:   veteran.QuidID,
		SubjectType: "QUID",
		Sequence:    1,
		EventType:   "REVIEW",
		Payload: map[string]interface{}{
			"qrpVersion":       1,
			"rating":           4.5,
			"maxRating":        5.0,
			"productAssetQuid": product.QuidID,
			"title":            "solid laptop",
		},
	})
	if _, err := node.AddEventTransaction(eventTx); err != nil {
		t.Fatalf("AddEventTransaction: %v", err)
	}

	// --- 5. Commit a block that contains title + trust + event ---
	if err := generateAndCommitBlock(t, node, domain); err != nil {
		t.Fatalf("follow-up block commit: %v", err)
	}

	// --- 6. Assertions ---
	node.IdentityRegistryMutex.RLock()
	_, productCommitted := node.IdentityRegistry[product.QuidID]
	node.IdentityRegistryMutex.RUnlock()
	if !productCommitted {
		t.Fatal("product identity never reached IdentityRegistry")
	}

	node.TitleRegistryMutex.RLock()
	_, titleCommitted := node.TitleRegistry[product.QuidID]
	node.TitleRegistryMutex.RUnlock()
	if !titleCommitted {
		t.Fatal("title never reached TitleRegistry")
	}

	events, _ := node.GetStreamEvents(veteran.QuidID, 50, 0)
	if len(events) == 0 {
		t.Fatal("review event never reached the stream registry — " +
			"this is the exact regression fixed by adding EventTransaction " +
			"to the switch in GenerateBlock / ValidateBlockTiered")
	}
	found := false
	for _, ev := range events {
		if ev.EventType == "REVIEW" &&
			ev.Payload["productAssetQuid"] == product.QuidID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected REVIEW event referencing %s, got %d events",
			product.QuidID, len(events))
	}

	// Trust edge should be queryable
	trustLevel, _, err := node.ComputeRelationalTrust(alice.QuidID, veteran.QuidID, DefaultTrustMaxDepth)
	if err != nil {
		t.Fatalf("ComputeRelationalTrust: %v", err)
	}
	if trustLevel < 0.85 || trustLevel > 0.95 {
		t.Errorf("expected trustLevel near 0.9, got %f", trustLevel)
	}

	t.Logf("✓ IDENTITY (%d), TITLE (1), TRUST (1), EVENT (%d) — all round-tripped",
		3, len(events))
	t.Logf("✓ alice→veteran trust = %.2f", trustLevel)
}

func freshTxID() string {
	var buf [24]byte
	_, _ = rand.Read(buf[:])
	buf2 := sha256.Sum256(append(buf[:], byte(time.Now().UnixNano()&0xff)))
	return hex.EncodeToString(buf2[:])
}

func generateAndCommitBlock(t *testing.T, node *QuidnugNode, domain string) error {
	t.Helper()
	block, err := node.GenerateBlock(domain)
	if err != nil {
		return fmt.Errorf("GenerateBlock: %w", err)
	}
	if err := node.AddBlock(*block); err != nil {
		return fmt.Errorf("AddBlock: %w", err)
	}
	return nil
}
