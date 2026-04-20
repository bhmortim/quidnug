// vectors_test.go — pkg/client consumer for the v1.0
// cross-SDK test vectors at `docs/test-vectors/v1.0/`.
//
// As of commit [pkg/client convergence], pkg/client.(*Quid).Sign
// and (*Quid).Verify use IEEE-1363 (64-byte raw r||s), matching
// the authoritative v1.0 form. All five conformance properties
// are exercised against pkg/client's public API; none are
// skipped.
//
// Historical context: prior to convergence, pkg/client produced
// DER-encoded signatures incompatible with server-side
// verification, and pkg/client.CanonicalBytes produced
// alphabetical-order JSON incompatible with the struct-
// declaration-order form used by the server. The two
// divergence-probe tests at the end of this file auto-detect
// when the divergence is resolved; if they log "converged!"
// and no divergence-specific assertions run, the SDK has
// caught up.
//
// Run with: go test -v -run TestVectors ./pkg/client/

package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// --- Vector file schema (mirrors internal/core/vectors_test.go) ------------

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

// quidFromKeyFile reconstructs a signing Quid from a test-key
// file's private_scalar_hex. Used so the SDK round-trip
// actually signs with the vector's original key.
func quidFromKeyFile(t *testing.T, kf keyFile) *Quid {
	t.Helper()
	d, ok := new(big.Int).SetString(kf.PrivateScalarHex, 16)
	if !ok {
		t.Fatalf("parse private scalar hex for %s", kf.Name)
	}
	curve := elliptic.P256()
	priv := new(ecdsa.PrivateKey)
	priv.Curve = curve
	priv.D = d
	priv.PublicKey.Curve = curve
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	q, err := quidFromKey(priv)
	if err != nil {
		t.Fatalf("quidFromKey: %v", err)
	}
	if q.ID != kf.QuidID {
		t.Fatalf("quid ID mismatch: %s vs %s", q.ID, kf.QuidID)
	}
	return q
}

// --- Conformance tests against vectors -------------------------------------

// TestVectorsTrust exercises the TRUST vectors against the
// pkg/client public API.
func TestVectorsTrust(t *testing.T) {
	runVectorConformance(t, "trust-tx.json")
}

// TestVectorsIdentity exercises IDENTITY vectors.
func TestVectorsIdentity(t *testing.T) {
	runVectorConformance(t, "identity-tx.json")
}

// TestVectorsEvent exercises EVENT vectors.
func TestVectorsEvent(t *testing.T) {
	runVectorConformance(t, "event-tx.json")
}

// TestVectorsTitle exercises TITLE vectors, if present.
func TestVectorsTitle(t *testing.T) {
	if _, err := os.Stat(filepath.Join(vectorsRoot, "title-tx.json")); os.IsNotExist(err) {
		t.Skip("title-tx.json not yet generated")
	}
	runVectorConformance(t, "title-tx.json")
}

// TestVectorsNodeAdvertisement exercises NODE_ADVERTISEMENT
// vectors, if present.
func TestVectorsNodeAdvertisement(t *testing.T) {
	if _, err := os.Stat(filepath.Join(vectorsRoot, "node-advertisement-tx.json")); os.IsNotExist(err) {
		t.Skip("node-advertisement-tx.json not yet generated")
	}
	runVectorConformance(t, "node-advertisement-tx.json")
}

// TestVectorsModerationAction exercises MODERATION_ACTION
// vectors, if present.
func TestVectorsModerationAction(t *testing.T) {
	if _, err := os.Stat(filepath.Join(vectorsRoot, "moderation-action-tx.json")); os.IsNotExist(err) {
		t.Skip("moderation-action-tx.json not yet generated")
	}
	runVectorConformance(t, "moderation-action-tx.json")
}

// TestVectorsDSR exercises DSR-family vectors, if present.
func TestVectorsDSR(t *testing.T) {
	if _, err := os.Stat(filepath.Join(vectorsRoot, "dsr-tx.json")); os.IsNotExist(err) {
		t.Skip("dsr-tx.json not yet generated")
	}
	runVectorConformance(t, "dsr-tx.json")
}

// runVectorConformance runs all five conformance properties
// against pkg/client's public API.
func runVectorConformance(t *testing.T, filename string) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, filename)
	if len(vf.Cases) == 0 {
		t.Fatal("no cases")
	}

	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			kf, ok := keys[c.SignerKeyRef]
			if !ok {
				t.Fatalf("no key ref %q", c.SignerKeyRef)
			}

			// Property 1: canonical_signable_bytes_utf8 hashes to
			// the declared SHA-256.
			signableBytes := []byte(c.Expected.CanonicalSignableBytesUTF8)
			sum := sha256.Sum256(signableBytes)
			if hex.EncodeToString(sum[:]) != c.Expected.SHA256OfCanonicalHex {
				t.Errorf("sha256 mismatch for utf8 canonical")
			}

			// Property 2: hex and utf8 canonical forms equivalent.
			hexDecoded, err := hex.DecodeString(c.Expected.CanonicalSignableBytesHex)
			if err != nil {
				t.Fatalf("decode hex canonical: %v", err)
			}
			if string(hexDecoded) != c.Expected.CanonicalSignableBytesUTF8 {
				t.Errorf("hex and utf8 canonical forms diverge")
			}

			// Property 3: reference signature verifies via
			// pkg/client.(*Quid).Verify (the public SDK entry
			// point).
			qRO, err := QuidFromPublicHex(kf.PublicKeySEC1Hex)
			if err != nil {
				t.Fatalf("QuidFromPublicHex: %v", err)
			}
			if !qRO.Verify(signableBytes, c.Expected.ReferenceSignatureHex) {
				t.Errorf("pkg/client.(*Quid).Verify rejected the reference signature")
			}

			// Property 4: tampered signature rejects.
			sigBytes, _ := hex.DecodeString(c.Expected.ReferenceSignatureHex)
			tampered := make([]byte, len(sigBytes))
			copy(tampered, sigBytes)
			tampered[5] ^= 0x01
			if qRO.Verify(signableBytes, hex.EncodeToString(tampered)) {
				t.Errorf("tampered signature accepted")
			}

			// Property 5: SDK sign-then-verify round-trip via
			// pkg/client. Signs the canonical bytes with the
			// test key and verifies through the same SDK. Doesn't
			// need to match the reference signature (non-
			// deterministic K), but MUST produce a valid one.
			qSign := quidFromKeyFile(t, kf)
			sdkSig, err := qSign.Sign(signableBytes)
			if err != nil {
				t.Fatalf("pkg/client.(*Quid).Sign: %v", err)
			}
			if !qSign.Verify(signableBytes, sdkSig) {
				t.Errorf("pkg/client sign-then-verify round-trip failed")
			}
			// Length check: v1.0 mandates 64-byte IEEE-1363.
			sdkSigBytes, _ := hex.DecodeString(sdkSig)
			if len(sdkSigBytes) != 64 {
				t.Errorf("SDK signature length %d; v1.0 mandates 64-byte IEEE-1363",
					len(sdkSigBytes))
			}
		})
	}
}

// --- Divergence probes (self-healing) --------------------------------------

// TestPkgClientCanonicalBytesDivergesFromAuthoritative
// documents the state of pkg/client.CanonicalBytes against
// the authoritative form. As of pkg/client's direct-typed-
// mirror submit paths, CanonicalBytes is no longer on the
// critical signing path; its alphabetical output is retained
// for backward compat but NOT used for any v1.0 submission.
func TestPkgClientCanonicalBytesDivergesFromAuthoritative(t *testing.T) {
	vf := loadVectorFile(t, "trust-tx.json")
	if len(vf.Cases) == 0 {
		t.Fatal("no cases")
	}
	c := vf.Cases[0]

	var asMap map[string]any
	if err := json.Unmarshal(c.Input, &asMap); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}

	sdkBytes, err := CanonicalBytes(asMap, "signature")
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}

	authoritativeBytes := []byte(c.Expected.CanonicalSignableBytesUTF8)

	if string(sdkBytes) == string(authoritativeBytes) {
		t.Logf("pkg/client.CanonicalBytes has converged with " +
			"authoritative form. This divergence probe can be " +
			"removed.")
		return
	}

	t.Logf("pkg/client.CanonicalBytes retains alphabetical key " +
		"ordering for backward compat; authoritative v1.0 form " +
		"(struct-declaration order) is achieved via the typed " +
		"mirror structs in pkg/client/types_wire.go. No v1.0 " +
		"submission path uses CanonicalBytes anymore; this test " +
		"documents the retained legacy behavior for observability.")
	t.Logf("authoritative (struct-decl order): %s", string(authoritativeBytes))
	t.Logf("pkg/client legacy (alphabetical):   %s", string(sdkBytes))
}

// TestPkgClientSignNowMatchesAuthoritative verifies
// convergence: pkg/client.(*Quid).Sign produces 64-byte
// IEEE-1363 signatures that verify via the authoritative
// verifier defined in internal/core.
func TestPkgClientSignNowMatchesAuthoritative(t *testing.T) {
	q, err := GenerateQuid()
	if err != nil {
		t.Fatalf("GenerateQuid: %v", err)
	}

	data := []byte("quidnug-test-data-for-signing-comparison")
	sdkSigHex, err := q.Sign(data)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	sdkSig, err := hex.DecodeString(sdkSigHex)
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}

	// Property: length is exactly 64 bytes (IEEE-1363).
	if len(sdkSig) != 64 {
		t.Errorf("pkg/client.(*Quid).Sign produced %d bytes; " +
			"v1.0 mandates 64-byte IEEE-1363. "+
			"Regression from the convergence commit.", len(sdkSig))
		return
	}

	// Property: signature verifies via pkg/client.(*Quid).Verify.
	if !q.Verify(data, sdkSigHex) {
		t.Errorf("pkg/client Sign/Verify round-trip failed")
	}

	// Property: signature verifies via an independent
	// authoritative verifier (replicated here to avoid
	// importing internal/core).
	if !authoritativeVerify(q.PublicKeyHex, data, sdkSigHex) {
		t.Errorf("authoritative verifier rejected pkg/client's " +
			"IEEE-1363 signature; cross-SDK compatibility broken")
	}

	t.Logf("pkg/client signatures are 64-byte IEEE-1363 and " +
		"verify both via the SDK and via the authoritative " +
		"verifier. Convergence confirmed.")
}

// authoritativeVerify replicates internal/core.VerifySignature
// so this test file doesn't need to import internal/core.
// Kept local to vectors_test.go; once pkg/client.(*Quid).Verify
// is confirmed to match this logic byte-for-byte across all
// cases, this duplicate can be removed.
func authoritativeVerify(pubHex string, data []byte, sigHex string) bool {
	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil {
		return false
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), pubBytes) //nolint:staticcheck
	if x == nil {
		return false
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	h := sha256.Sum256(data)
	return ecdsa.Verify(pub, h[:], r, s)
}
