// vectors_test.go — authoritative consumer for the v1.0
// cross-SDK test vectors at `docs/test-vectors/v1.0/`.
//
// This test uses the reference node's OWN canonical-bytes
// and signature-verification code paths. A green run proves
// the vectors are internally consistent with the server:
// canonical bytes match, transaction IDs match, reference
// signatures verify, tampered signatures reject.
//
// SDKs that claim v1.0 conformance must pass an equivalent
// suite. A companion test in pkg/client/vectors_test.go
// surfaces the current divergence between the Go SDK's
// crypto and the authoritative form; that test is
// deliberately expected to document gaps.
//
// Run with: go test -v -run TestVectors ./internal/core/

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Per-case shape (mirrors cmd/quidnug-test-vectors/main.go vectorCase).
type vectorCase struct {
	Name         string          `json:"name"`
	Comments     string          `json:"comments"`
	SignerKeyRef string          `json:"signer_key_ref"`
	Input        json.RawMessage `json:"input"`
	Expected     struct {
		CanonicalSignableBytesHex  string `json:"canonical_signable_bytes_hex"`
		CanonicalSignableBytesUTF8 string `json:"canonical_signable_bytes_utf8"`
		SHA256OfCanonicalHex       string `json:"sha256_of_canonical_hex"`
		ExpectedID                 string `json:"expected_id"`
		ReferenceSignatureHex      string `json:"reference_signature_hex"`
		SignatureLengthBytes       int    `json:"signature_length_bytes"`
	} `json:"expected"`
}

type vectorFile struct {
	SchemaVersion     string       `json:"schema_version"`
	TxType            string       `json:"tx_type"`
	GeneratedAt       string       `json:"generated_at"`
	GeneratorCommit   string       `json:"generator_commit"`
	CanonicalFormNotes string      `json:"canonical_form_notes"`
	Cases             []vectorCase `json:"cases"`
}

type keyFile struct {
	Name             string `json:"name"`
	Seed             string `json:"seed"`
	PrivateScalarHex string `json:"private_scalar_hex"`
	PublicKeySEC1Hex string `json:"public_key_sec1_hex"`
	QuidID           string `json:"quid_id"`
	Notes            string `json:"notes"`
}

const vectorsRoot = "../../docs/test-vectors/v1.0"

// loadKeys returns the deterministic test keys checked in
// alongside the vectors, keyed by name.
func loadKeys(t *testing.T) map[string]keyFile {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(vectorsRoot, "test-keys"))
	if err != nil {
		t.Fatalf("read test-keys dir: %v", err)
	}
	out := make(map[string]keyFile)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(vectorsRoot, "test-keys", e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var k keyFile
		if err := json.Unmarshal(raw, &k); err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		out[k.Name] = k
	}
	return out
}

// loadVectorFile parses a vector file.
func loadVectorFile(t *testing.T, filename string) vectorFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(vectorsRoot, filename))
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	var vf vectorFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	return vf
}

// --- Property assertions ---------------------------------------------------

// assertCanonicalBytesMatch: re-serialize the input with
// Signature cleared; hex must match expected. Uses typed
// struct so we exercise the exact code path validation uses.
func assertCanonicalBytes(t *testing.T, c vectorCase, serialized []byte) {
	t.Helper()
	got := hex.EncodeToString(serialized)
	if got != c.Expected.CanonicalSignableBytesHex {
		t.Errorf("%s: canonical bytes mismatch\n want: %s\n  got: %s",
			c.Name, c.Expected.CanonicalSignableBytesHex, got)
	}
	sum := sha256.Sum256(serialized)
	sumHex := hex.EncodeToString(sum[:])
	if sumHex != c.Expected.SHA256OfCanonicalHex {
		t.Errorf("%s: sha256 mismatch\n want: %s\n  got: %s",
			c.Name, c.Expected.SHA256OfCanonicalHex, sumHex)
	}
}

// assertReferenceSignatureVerifies: vector's signature must
// verify against the computed signable bytes + the test
// key's public key.
func assertReferenceSignatureVerifies(t *testing.T, c vectorCase, pubHex string, signable []byte) {
	t.Helper()
	if !VerifySignature(pubHex, signable, c.Expected.ReferenceSignatureHex) {
		t.Errorf("%s: reference signature did not verify", c.Name)
	}
}

// assertTamperedSignatureRejects: flipping a byte of the
// signature must cause verification to fail.
func assertTamperedSignatureRejects(t *testing.T, c vectorCase, pubHex string, signable []byte) {
	t.Helper()
	sigBytes, err := hex.DecodeString(c.Expected.ReferenceSignatureHex)
	if err != nil || len(sigBytes) == 0 {
		t.Fatalf("%s: signature hex decode: %v", c.Name, err)
	}
	tampered := make([]byte, len(sigBytes))
	copy(tampered, sigBytes)
	tampered[5] ^= 0x01 // flip a bit
	if VerifySignature(pubHex, signable, hex.EncodeToString(tampered)) {
		t.Errorf("%s: tampered signature incorrectly verified", c.Name)
	}
}

// assertSignatureLength: 64 bytes for IEEE-1363 v1.0.
func assertSignatureLength(t *testing.T, c vectorCase) {
	t.Helper()
	sig, err := hex.DecodeString(c.Expected.ReferenceSignatureHex)
	if err != nil {
		t.Fatalf("%s: decode signature hex: %v", c.Name, err)
	}
	if len(sig) != 64 {
		t.Errorf("%s: signature length %d bytes, expected 64 (IEEE-1363)", c.Name, len(sig))
	}
	if c.Expected.SignatureLengthBytes != 64 {
		t.Errorf("%s: expected.signature_length_bytes == %d, expected 64",
			c.Name, c.Expected.SignatureLengthBytes)
	}
}

// --- Per-type tests --------------------------------------------------------

// TestVectorsTrust validates trust-tx.json.
func TestVectorsTrust(t *testing.T) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, "trust-tx.json")
	if vf.TxType != "TRUST" {
		t.Fatalf("expected tx_type TRUST, got %s", vf.TxType)
	}
	if len(vf.Cases) == 0 {
		t.Fatal("no cases in trust-tx.json")
	}

	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var tx TrustTransaction
			if err := json.Unmarshal(c.Input, &tx); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}

			key, ok := keys[c.SignerKeyRef]
			if !ok {
				t.Fatalf("no key ref %q in test-keys", c.SignerKeyRef)
			}
			if tx.PublicKey != key.PublicKeySEC1Hex {
				t.Errorf("tx.PublicKey does not match key ref %s", c.SignerKeyRef)
			}

			// Reproduce canonical signable bytes (server-compat:
			// typed struct, Signature cleared).
			txCopy := tx
			txCopy.Signature = ""
			signable, err := json.Marshal(txCopy)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			assertCanonicalBytes(t, c, signable)
			assertSignatureLength(t, c)
			assertReferenceSignatureVerifies(t, c, key.PublicKeySEC1Hex, signable)
			assertTamperedSignatureRejects(t, c, key.PublicKeySEC1Hex, signable)

			// ID derivation (trust-specific).
			gotID := trustID(tx)
			if gotID != c.Expected.ExpectedID {
				t.Errorf("id mismatch\n want: %s\n  got: %s", c.Expected.ExpectedID, gotID)
			}
		})
	}
}

// TestVectorsIdentity validates identity-tx.json.
func TestVectorsIdentity(t *testing.T) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, "identity-tx.json")
	if vf.TxType != "IDENTITY" {
		t.Fatalf("expected tx_type IDENTITY, got %s", vf.TxType)
	}
	if len(vf.Cases) == 0 {
		t.Fatal("no cases in identity-tx.json")
	}

	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var tx IdentityTransaction
			if err := json.Unmarshal(c.Input, &tx); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}

			key, ok := keys[c.SignerKeyRef]
			if !ok {
				t.Fatalf("no key ref %q in test-keys", c.SignerKeyRef)
			}
			if tx.PublicKey != key.PublicKeySEC1Hex {
				t.Errorf("tx.PublicKey does not match key ref %s", c.SignerKeyRef)
			}

			txCopy := tx
			txCopy.Signature = ""
			signable, err := json.Marshal(txCopy)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			assertCanonicalBytes(t, c, signable)
			assertSignatureLength(t, c)
			assertReferenceSignatureVerifies(t, c, key.PublicKeySEC1Hex, signable)
			assertTamperedSignatureRejects(t, c, key.PublicKeySEC1Hex, signable)

			gotID := identityID(tx)
			if gotID != c.Expected.ExpectedID {
				t.Errorf("id mismatch\n want: %s\n  got: %s", c.Expected.ExpectedID, gotID)
			}
		})
	}
}

// TestVectorsEvent validates event-tx.json.
func TestVectorsEvent(t *testing.T) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, "event-tx.json")
	if vf.TxType != "EVENT" {
		t.Fatalf("expected tx_type EVENT, got %s", vf.TxType)
	}
	if len(vf.Cases) == 0 {
		t.Fatal("no cases in event-tx.json")
	}

	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var tx EventTransaction
			if err := json.Unmarshal(c.Input, &tx); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}

			key, ok := keys[c.SignerKeyRef]
			if !ok {
				t.Fatalf("no key ref %q in test-keys", c.SignerKeyRef)
			}
			if tx.PublicKey != key.PublicKeySEC1Hex {
				t.Errorf("tx.PublicKey does not match key ref %s", c.SignerKeyRef)
			}

			txCopy := tx
			txCopy.Signature = ""
			signable, err := json.Marshal(txCopy)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			assertCanonicalBytes(t, c, signable)
			assertSignatureLength(t, c)
			assertReferenceSignatureVerifies(t, c, key.PublicKeySEC1Hex, signable)
			assertTamperedSignatureRejects(t, c, key.PublicKeySEC1Hex, signable)

			gotID := eventID(tx)
			if gotID != c.Expected.ExpectedID {
				t.Errorf("id mismatch\n want: %s\n  got: %s", c.Expected.ExpectedID, gotID)
			}
		})
	}
}

// TestVectorsTitle validates title-tx.json.
func TestVectorsTitle(t *testing.T) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, "title-tx.json")
	if vf.TxType != "TITLE" {
		t.Fatalf("expected tx_type TITLE, got %s", vf.TxType)
	}
	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var tx TitleTransaction
			if err := json.Unmarshal(c.Input, &tx); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}
			key := keys[c.SignerKeyRef]
			if tx.PublicKey != key.PublicKeySEC1Hex {
				t.Errorf("pubkey mismatch")
			}
			txCopy := tx
			txCopy.Signature = ""
			signable, _ := json.Marshal(txCopy)
			assertCanonicalBytes(t, c, signable)
			assertSignatureLength(t, c)
			assertReferenceSignatureVerifies(t, c, key.PublicKeySEC1Hex, signable)
			assertTamperedSignatureRejects(t, c, key.PublicKeySEC1Hex, signable)
			if gotID := titleID(tx); gotID != c.Expected.ExpectedID {
				t.Errorf("id mismatch: want %s got %s", c.Expected.ExpectedID, gotID)
			}
		})
	}
}

// TestVectorsNodeAdvertisement validates node-advertisement-tx.json.
func TestVectorsNodeAdvertisement(t *testing.T) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, "node-advertisement-tx.json")
	if vf.TxType != "NODE_ADVERTISEMENT" {
		t.Fatalf("expected NODE_ADVERTISEMENT, got %s", vf.TxType)
	}
	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var tx NodeAdvertisementTransaction
			if err := json.Unmarshal(c.Input, &tx); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}
			key := keys[c.SignerKeyRef]
			if tx.PublicKey != key.PublicKeySEC1Hex {
				t.Errorf("pubkey mismatch")
			}
			txCopy := tx
			txCopy.Signature = ""
			signable, _ := json.Marshal(txCopy)
			assertCanonicalBytes(t, c, signable)
			assertSignatureLength(t, c)
			assertReferenceSignatureVerifies(t, c, key.PublicKeySEC1Hex, signable)
			assertTamperedSignatureRejects(t, c, key.PublicKeySEC1Hex, signable)
			if gotID := nodeAdvertisementID(tx); gotID != c.Expected.ExpectedID {
				t.Errorf("id mismatch: want %s got %s", c.Expected.ExpectedID, gotID)
			}
		})
	}
}

// TestVectorsModerationAction validates moderation-action-tx.json.
func TestVectorsModerationAction(t *testing.T) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, "moderation-action-tx.json")
	if vf.TxType != "MODERATION_ACTION" {
		t.Fatalf("expected MODERATION_ACTION, got %s", vf.TxType)
	}
	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var tx ModerationActionTransaction
			if err := json.Unmarshal(c.Input, &tx); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}
			key := keys[c.SignerKeyRef]
			if tx.PublicKey != key.PublicKeySEC1Hex {
				t.Errorf("pubkey mismatch")
			}
			txCopy := tx
			txCopy.Signature = ""
			signable, _ := json.Marshal(txCopy)
			assertCanonicalBytes(t, c, signable)
			assertSignatureLength(t, c)
			assertReferenceSignatureVerifies(t, c, key.PublicKeySEC1Hex, signable)
			assertTamperedSignatureRejects(t, c, key.PublicKeySEC1Hex, signable)
			if gotID := moderationActionID(tx); gotID != c.Expected.ExpectedID {
				t.Errorf("id mismatch: want %s got %s", c.Expected.ExpectedID, gotID)
			}
		})
	}
}

// TestVectorsDSR validates dsr-tx.json (heterogeneous: five
// privacy-family tx types in one file, each case dispatched by
// the `type` field in Input).
func TestVectorsDSR(t *testing.T) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, "dsr-tx.json")
	if vf.TxType != "DSR_FAMILY" {
		t.Fatalf("expected DSR_FAMILY, got %s", vf.TxType)
	}

	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			key := keys[c.SignerKeyRef]

			// Peek at the type field.
			var peek struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(c.Input, &peek)

			var (
				signable []byte
				gotID    string
			)
			switch peek.Type {
			case string(TxTypeDataSubjectRequest):
				var tx DataSubjectRequestTransaction
				_ = json.Unmarshal(c.Input, &tx)
				if tx.PublicKey != key.PublicKeySEC1Hex {
					t.Errorf("pubkey mismatch")
				}
				txCopy := tx
				txCopy.Signature = ""
				signable, _ = json.Marshal(txCopy)
				gotID = dsrRequestIDFromTx(tx)
			case string(TxTypeConsentGrant):
				var tx ConsentGrantTransaction
				_ = json.Unmarshal(c.Input, &tx)
				if tx.PublicKey != key.PublicKeySEC1Hex {
					t.Errorf("pubkey mismatch")
				}
				txCopy := tx
				txCopy.Signature = ""
				signable, _ = json.Marshal(txCopy)
				gotID = consentGrantIDFromTx(tx)
			case string(TxTypeConsentWithdraw):
				var tx ConsentWithdrawTransaction
				_ = json.Unmarshal(c.Input, &tx)
				if tx.PublicKey != key.PublicKeySEC1Hex {
					t.Errorf("pubkey mismatch")
				}
				txCopy := tx
				txCopy.Signature = ""
				signable, _ = json.Marshal(txCopy)
				gotID = consentWithdrawIDFromTx(tx)
			case string(TxTypeProcessingRestriction):
				var tx ProcessingRestrictionTransaction
				_ = json.Unmarshal(c.Input, &tx)
				if tx.PublicKey != key.PublicKeySEC1Hex {
					t.Errorf("pubkey mismatch")
				}
				txCopy := tx
				txCopy.Signature = ""
				signable, _ = json.Marshal(txCopy)
				gotID = processingRestrictionIDFromTx(tx)
			case string(TxTypeDSRCompliance):
				var tx DSRComplianceTransaction
				_ = json.Unmarshal(c.Input, &tx)
				if tx.PublicKey != key.PublicKeySEC1Hex {
					t.Errorf("pubkey mismatch")
				}
				txCopy := tx
				txCopy.Signature = ""
				signable, _ = json.Marshal(txCopy)
				gotID = dsrComplianceIDFromTx(tx)
			default:
				t.Fatalf("unknown dsr-family type: %q", peek.Type)
			}

			assertCanonicalBytes(t, c, signable)
			assertSignatureLength(t, c)
			assertReferenceSignatureVerifies(t, c, key.PublicKeySEC1Hex, signable)
			assertTamperedSignatureRejects(t, c, key.PublicKeySEC1Hex, signable)
			if gotID != c.Expected.ExpectedID {
				t.Errorf("id mismatch: want %s got %s", c.Expected.ExpectedID, gotID)
			}
		})
	}
}

// --- Local ID derivation (mirrors cmd/quidnug-test-vectors) ---------------
//
// The real ID derivation happens inline in
// internal/core/transactions.go:AddTrustTransaction (and
// siblings) when a tx is submitted without an ID. These
// helpers re-implement the derivation so the test can
// compute an ID for the already-submitted inputs without
// going through the full Add* path (which also mutates
// state + signs). If transactions.go changes the derivation,
// these helpers must be updated in lockstep — that is the
// intentional coupling this test locks down.

func trustID(tx TrustTransaction) string {
	payload, _ := json.Marshal(struct {
		Truster     string
		Trustee     string
		TrustLevel  float64
		TrustDomain string
		Timestamp   int64
	}{
		Truster:     tx.Truster,
		Trustee:     tx.Trustee,
		TrustLevel:  tx.TrustLevel,
		TrustDomain: tx.TrustDomain,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func identityID(tx IdentityTransaction) string {
	payload, _ := json.Marshal(struct {
		QuidID      string
		Name        string
		Creator     string
		TrustDomain string
		UpdateNonce int64
		Timestamp   int64
	}{
		QuidID:      tx.QuidID,
		Name:        tx.Name,
		Creator:     tx.Creator,
		TrustDomain: tx.TrustDomain,
		UpdateNonce: tx.UpdateNonce,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func eventID(tx EventTransaction) string {
	payload, _ := json.Marshal(struct {
		SubjectID   string
		EventType   string
		Sequence    int64
		TrustDomain string
		Timestamp   int64
	}{
		SubjectID:   tx.SubjectID,
		EventType:   tx.EventType,
		Sequence:    tx.Sequence,
		TrustDomain: tx.TrustDomain,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func titleID(tx TitleTransaction) string {
	payload, _ := json.Marshal(struct {
		AssetID     string
		Owners      []OwnershipStake
		TrustDomain string
		Timestamp   int64
	}{
		AssetID:     tx.AssetID,
		Owners:      tx.Owners,
		TrustDomain: tx.TrustDomain,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func nodeAdvertisementID(tx NodeAdvertisementTransaction) string {
	payload, _ := json.Marshal(struct {
		NodeQuid           string
		OperatorQuid       string
		TrustDomain        string
		AdvertisementNonce int64
		Timestamp          int64
	}{
		NodeQuid:           tx.NodeQuid,
		OperatorQuid:       tx.OperatorQuid,
		TrustDomain:        tx.TrustDomain,
		AdvertisementNonce: tx.AdvertisementNonce,
		Timestamp:          tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func moderationActionID(tx ModerationActionTransaction) string {
	payload, _ := json.Marshal(struct {
		ModeratorQuid string
		TargetType    string
		TargetID      string
		Scope         string
		ReasonCode    string
		Nonce         int64
		Timestamp     int64
	}{
		ModeratorQuid: tx.ModeratorQuid,
		TargetType:    tx.TargetType,
		TargetID:      tx.TargetID,
		Scope:         tx.Scope,
		ReasonCode:    tx.ReasonCode,
		Nonce:         tx.Nonce,
		Timestamp:     tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func dsrRequestIDFromTx(tx DataSubjectRequestTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject     string
		Controller  string
		RequestType string
		Nonce       int64
		Timestamp   int64
	}{tx.SubjectQuid, tx.ControllerQuid, tx.RequestType, tx.Nonce, tx.Timestamp})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func consentGrantIDFromTx(tx ConsentGrantTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject    string
		Controller string
		Scope      []string
		PolicyHash string
		Nonce      int64
		Timestamp  int64
	}{tx.SubjectQuid, tx.ControllerQuid, tx.Scope, tx.PolicyHash, tx.Nonce, tx.Timestamp})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func consentWithdrawIDFromTx(tx ConsentWithdrawTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject   string
		Withdraw  string
		Nonce     int64
		Timestamp int64
	}{tx.SubjectQuid, tx.WithdrawsGrantTxID, tx.Nonce, tx.Timestamp})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func processingRestrictionIDFromTx(tx ProcessingRestrictionTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject   string
		Uses      []string
		Nonce     int64
		Timestamp int64
	}{tx.SubjectQuid, tx.RestrictedUses, tx.Nonce, tx.Timestamp})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func dsrComplianceIDFromTx(tx DSRComplianceTransaction) string {
	payload, _ := json.Marshal(struct {
		RequestTxID string
		Operator    string
		CompletedAt int64
		Nonce       int64
		Timestamp   int64
	}{tx.RequestTxID, tx.OperatorQuid, tx.CompletedAt, tx.Nonce, tx.Timestamp})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// Compile-time guard: assures the vectorsRoot path at least
// parses. Actual existence is checked when tests run.
var _ = fmt.Sprintf("%s", vectorsRoot)
