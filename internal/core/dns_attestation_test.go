// dns_attestation_test.go — QDP-0023 Phase 1 in-process
// integration test. Covers the full admission flow for each
// DNS event type + the weighted query path.

package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// dnsActor is a test actor with a P-256 keypair + derived
// quid ID. Signs DNS family transactions using the same
// IEEE-1363 convention the rest of the protocol uses.
type dnsActor struct {
	name    string
	priv    *ecdsa.PrivateKey
	pubHex  string
	quidID  string
}

func newDNSActor(t *testing.T, name string) *dnsActor {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	pubBytes := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck
	hash := sha256.Sum256(pubBytes)
	return &dnsActor{
		name:   name,
		priv:   priv,
		pubHex: hex.EncodeToString(pubBytes),
		quidID: hex.EncodeToString(hash[:8]),
	}
}

func (a *dnsActor) signIEEE1363(data []byte) string {
	// Use the reference node's RFC 6979 signer for
	// deterministic output (matches what admission does
	// internally; signatures verify on first try).
	digest := sha256.Sum256(data)
	r, s := SignRFC6979(a.priv, digest[:])
	sig := make([]byte, 64)
	rb := r.Bytes()
	sb := s.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):64], sb)
	return hex.EncodeToString(sig)
}

// signDNSClaim produces a signed DNSClaimTransaction with
// pre-computed ID (matching what production clients do).
func (a *dnsActor) signDNSClaim(tx DNSClaimTransaction) DNSClaimTransaction {
	tx.PublicKey = a.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			Domain    string
			OwnerQuid string
			RootQuid  string
			Nonce     int64
			Timestamp int64
		}{tx.Domain, tx.OwnerQuid, tx.RootQuid, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(b)
	return tx
}

func (a *dnsActor) signDNSAttestation(tx DNSAttestationTransaction) DNSAttestationTransaction {
	tx.PublicKey = a.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			Domain    string
			OwnerQuid string
			RootQuid  string
			Nonce     int64
			Timestamp int64
		}{tx.Domain, tx.OwnerQuid, tx.RootQuid, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(b)
	return tx
}

func (a *dnsActor) signDNSRenewal(tx DNSRenewalTransaction) DNSRenewalTransaction {
	tx.PublicKey = a.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			PriorRef      string
			NewValidUntil int64
			Nonce         int64
			Timestamp     int64
		}{tx.PriorAttestationRef, tx.NewValidUntil, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(b)
	return tx
}

func (a *dnsActor) signDNSRevocation(tx DNSRevocationTransaction) DNSRevocationTransaction {
	tx.PublicKey = a.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			AttRef      string
			RevokerQuid string
			Nonce       int64
			Timestamp   int64
		}{tx.AttestationRef, tx.RevokerQuid, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(b)
	return tx
}

func (a *dnsActor) signAuthorityDelegate(tx AuthorityDelegateTransaction) AuthorityDelegateTransaction {
	tx.PublicKey = a.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			AttestationRef string
			Subject        string
			DelegateDomain string
			Nonce          int64
			Timestamp      int64
		}{tx.AttestationRef, tx.Subject, tx.DelegateDomain, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = a.signIEEE1363(b)
	return tx
}

// --- Tests ---

func TestDNSAttestation_ClaimAdmission(t *testing.T) {
	node := newTestNode()
	alice := newDNSActor(t, "alice") // domain owner
	root := newDNSActor(t, "root")

	claim := alice.signDNSClaim(DNSClaimTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDNSClaim,
			TrustDomain: "attestation.claims.root",
			Timestamp:   time.Now().Unix(),
		},
		Domain:        "example.com",
		OwnerQuid:     alice.quidID,
		RootQuid:      root.quidID,
		PaymentMethod: "stripe",
		Nonce:         1,
	})
	id, err := node.AddDNSClaimTransaction(claim)
	if err != nil {
		t.Fatalf("admit claim: %v", err)
	}
	if id == "" {
		t.Fatal("empty tx id")
	}
}

func TestDNSAttestation_FullLifecycle(t *testing.T) {
	node := newTestNode()
	alice := newDNSActor(t, "alice")
	root := newDNSActor(t, "root")

	nowSec := time.Now().Unix()
	nowNs := time.Now().UnixNano()

	// 1. Claim.
	claim := alice.signDNSClaim(DNSClaimTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDNSClaim,
			TrustDomain: "attestation.claims.root",
			Timestamp:   nowSec,
		},
		Domain:        "example.com",
		OwnerQuid:     alice.quidID,
		RootQuid:      root.quidID,
		PaymentMethod: "stripe",
		Nonce:         1,
	})
	claimID, err := node.AddDNSClaimTransaction(claim)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// 2. Attestation (root signs).
	att := root.signDNSAttestation(DNSAttestationTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDNSAttestation,
			TrustDomain: "attestation.dns.root",
			Timestamp:   nowSec + 1,
		},
		ClaimRef:   claimID,
		Domain:     "example.com",
		OwnerQuid:  alice.quidID,
		RootQuid:   root.quidID,
		TLD:        "com",
		TLDTier:    "standard",
		VerifiedAt: nowNs,
		ValidUntil: nowNs + int64(365*24*time.Hour),
		RigorLevel: "standard",
		FeePaidUSD: 5.0,
		PaymentMethod: "stripe",
		Nonce:      1,
	})
	attID, err := node.AddDNSAttestationTransaction(att)
	if err != nil {
		t.Fatalf("attestation: %v", err)
	}

	// 3. Query: one active attestation.
	atts := node.DNSAttestationRegistry.GetActiveAttestationsForDomain("example.com", nowNs)
	if len(atts) != 1 {
		t.Fatalf("want 1 active attestation, got %d", len(atts))
	}
	if atts[0].ID != attID {
		t.Errorf("attestation ID mismatch")
	}

	// 4. Renewal: push validUntil forward.
	newExpiry := atts[0].ValidUntil + int64(365*24*time.Hour)
	renewal := root.signDNSRenewal(DNSRenewalTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDNSRenewal,
			TrustDomain: "attestation.dns.root",
			Timestamp:   nowSec + 100,
		},
		PriorAttestationRef: attID,
		NewValidUntil:       newExpiry,
		Nonce:               2,
	})
	if _, err := node.AddDNSRenewalTransaction(renewal); err != nil {
		t.Fatalf("renewal: %v", err)
	}
	after, _ := node.DNSAttestationRegistry.GetAttestation(attID)
	if after.ValidUntil != newExpiry {
		t.Errorf("renewal didn't update ValidUntil: got %d want %d",
			after.ValidUntil, newExpiry)
	}

	// 5. Delegation.
	del := alice.signAuthorityDelegate(AuthorityDelegateTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeAuthorityDelegate,
			TrustDomain: "governance.attestation.dns-policy.alice",
			Timestamp:   nowSec + 200,
		},
		AttestationRef:  attID,
		AttestationKind: "dns",
		Subject:         "example.com",
		DelegateDomain:  "bank.example",
		DelegateNodes:   []string{alice.quidID},
		Visibility: DelegationVisibility{
			RecordTypes: map[string]VisibilityPolicy{
				"A": {Class: "public"},
			},
			Default: VisibilityPolicy{
				Class: "trust-gated", GateDomain: "partners.example", MinTrust: 0.5,
			},
		},
		Nonce: 1,
	})
	delID, err := node.AddAuthorityDelegateTransaction(del)
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}
	active := node.DNSAttestationRegistry.GetActiveDelegations(attID)
	if len(active) != 1 || active[0].ID != delID {
		t.Errorf("delegation not stored correctly")
	}

	// 6. Revocation.
	rev := root.signDNSRevocation(DNSRevocationTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeDNSRevocation,
			TrustDomain: "attestation.dns.root",
			Timestamp:   nowSec + 300,
		},
		AttestationRef: attID,
		RevokerQuid:    root.quidID,
		RevokerRole:    "root",
		Reason:         "fraud-detected",
		RevokedAt:      nowNs + int64(300*time.Second),
		Nonce:          3,
	})
	if _, err := node.AddDNSRevocationTransaction(rev); err != nil {
		t.Fatalf("revocation: %v", err)
	}
	if !node.DNSAttestationRegistry.IsAttestationRevoked(attID) {
		t.Error("expected attestation to be marked revoked")
	}
	active2 := node.DNSAttestationRegistry.GetActiveAttestationsForDomain("example.com", nowNs+int64(400*time.Second))
	if len(active2) != 0 {
		t.Errorf("revoked attestation still active: got %d", len(active2))
	}
}

func TestDNSAttestation_ValidateRejectsMalformed(t *testing.T) {
	node := newTestNode()
	alice := newDNSActor(t, "alice")
	root := newDNSActor(t, "root")

	// Bad domain.
	bad := alice.signDNSClaim(DNSClaimTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeDNSClaim, Timestamp: time.Now().Unix()},
		Domain:          "not-a-domain", // no dot
		OwnerQuid:       alice.quidID,
		RootQuid:        root.quidID,
		PaymentMethod:   "stripe",
		Nonce:           1,
	})
	if _, err := node.AddDNSClaimTransaction(bad); err == nil {
		t.Error("expected rejection for bad domain")
	}

	// Self-attestation (owner == root) rejected.
	self := alice.signDNSClaim(DNSClaimTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeDNSClaim, Timestamp: time.Now().Unix()},
		Domain:          "example.com",
		OwnerQuid:       alice.quidID,
		RootQuid:        alice.quidID,
		PaymentMethod:   "stripe",
		Nonce:           1,
	})
	if _, err := node.AddDNSClaimTransaction(self); err == nil {
		t.Error("expected rejection for owner==root")
	}

	// Invalid payment method.
	bad2 := alice.signDNSClaim(DNSClaimTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeDNSClaim, Timestamp: time.Now().Unix()},
		Domain:          "example.com",
		OwnerQuid:       alice.quidID,
		RootQuid:        root.quidID,
		PaymentMethod:   "bitcoin-lightning", // not on the list
		Nonce:           1,
	})
	if _, err := node.AddDNSClaimTransaction(bad2); err == nil {
		t.Error("expected rejection for unknown payment method")
	}
}

func TestDNSAttestation_WeightedQuerySorting(t *testing.T) {
	node := newTestNode()
	alice := newDNSActor(t, "alice")
	rootA := newDNSActor(t, "rootA") // trusted root
	rootB := newDNSActor(t, "rootB") // untrusted root

	domain := "attestation.dns.root"
	node.TrustDomains[domain] = TrustDomain{
		Name:           domain,
		TrustThreshold: 0.5,
		Validators:     map[string]float64{node.NodeID: 1.0},
	}

	// Observer trusts rootA (weight 0.9), not rootB.
	observer := alice.quidID
	node.TrustRegistry[observer] = map[string]float64{rootA.quidID: 0.9}

	nowSec := time.Now().Unix()
	nowNs := time.Now().UnixNano()

	for i, root := range []*dnsActor{rootA, rootB} {
		att := root.signDNSAttestation(DNSAttestationTransaction{
			BaseTransaction: BaseTransaction{
				Type: TxTypeDNSAttestation, TrustDomain: domain, Timestamp: nowSec + int64(i),
			},
			ClaimRef:   fmt.Sprintf("claim-%d", i),
			Domain:     "example.com",
			OwnerQuid:  alice.quidID,
			RootQuid:   root.quidID,
			TLD:        "com",
			TLDTier:    "standard",
			VerifiedAt: nowNs,
			ValidUntil: nowNs + int64(365*24*time.Hour),
			RigorLevel: "standard",
			Nonce:      1,
		})
		if _, err := node.AddDNSAttestationTransaction(att); err != nil {
			t.Fatalf("attestation %d: %v", i, err)
		}
	}

	weighted := node.GetWeightedAttestationsForDomain(
		"example.com", observer, 5, DecayConfig{}, nowNs)
	if len(weighted) != 2 {
		t.Fatalf("want 2 weighted entries, got %d", len(weighted))
	}
	if weighted[0].Attestation.RootQuid != rootA.quidID {
		t.Errorf("expected rootA first (higher trust); got %s",
			weighted[0].Attestation.RootQuid)
	}
	if weighted[0].Weight < weighted[1].Weight {
		t.Errorf("weights not sorted descending: %v then %v",
			weighted[0].Weight, weighted[1].Weight)
	}
}
