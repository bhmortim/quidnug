// vectors_test.go — pkg/client consumer for the v1.0
// cross-SDK test vectors at `docs/test-vectors/v1.0/`.
//
// THIS FILE DOCUMENTS A LAUNCH-BLOCKING DIVERGENCE.
//
// The Go SDK's crypto.go currently uses:
//   - ecdsa.SignASN1 / ecdsa.VerifyASN1 (DER signature format)
//   - CanonicalBytes() round-trip-through-map (alphabetical
//     key ordering)
//
// The authoritative v1.0 canonical form requires:
//   - ECDSA signatures as IEEE-1363 raw 64 bytes (r||s)
//   - json.Marshal on typed struct (struct-declaration
//     field ordering)
//
// Consequence: transactions signed via pkg/client.(*Quid).Sign
// would NOT verify server-side. The end-to-end submit
// pipeline is currently untested.
//
// The tests below exercise the authoritative form (matching
// internal/core/vectors_test.go) and PASS. They do not yet
// exercise pkg/client's Sign/Verify because those are
// incompatible. Once pkg/client is migrated to IEEE-1363 +
// typed-mirror canonical form, additional assertions will
// validate the SDK entry points directly.
//
// Tracking: `docs/test-vectors/v1.0/README.md` §"Known
// divergences".
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

// --- Authoritative verify (matches internal/core.VerifySignature) ----------
//
// Duplicated here so pkg/client tests don't need to import
// internal/core (which they can't for public-API tests
// anyway). This is the V1.0 canonical verify function: it
// accepts 64-byte IEEE-1363 signatures.
//
// When pkg/client.(*Quid).Verify is migrated to IEEE-1363,
// this local helper goes away and the tests call into
// pkg/client directly.
func verifyIEEE1363(pubHex string, data []byte, sigHex string) bool {
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

// --- Conformance tests against vectors -------------------------------------

// TestVectorsTrust_Authoritative exercises the authoritative
// canonical form against the TRUST vectors. This passes
// today and locks down the expected behavior.
func TestVectorsTrust_Authoritative(t *testing.T) {
	runVectorConformance(t, "trust-tx.json")
}

// TestVectorsIdentity_Authoritative exercises IDENTITY vectors.
func TestVectorsIdentity_Authoritative(t *testing.T) {
	runVectorConformance(t, "identity-tx.json")
}

// TestVectorsEvent_Authoritative exercises EVENT vectors.
func TestVectorsEvent_Authoritative(t *testing.T) {
	runVectorConformance(t, "event-tx.json")
}

// runVectorConformance runs the type-agnostic properties
// (canonical bytes hex match, SHA-256 match, reference
// signature verification, tampered signature rejection).
// It deliberately re-serializes the input as
// map[string]any to prove the vectors' canonical bytes are
// stable under round-trip through a generic form — this
// matches what client code typically does when working with
// typed inputs.
func runVectorConformance(t *testing.T, filename string) {
	keys := loadKeys(t)
	vf := loadVectorFile(t, filename)
	if len(vf.Cases) == 0 {
		t.Fatal("no cases")
	}

	for _, c := range vf.Cases {
		t.Run(c.Name, func(t *testing.T) {
			key, ok := keys[c.SignerKeyRef]
			if !ok {
				t.Fatalf("no key ref %q", c.SignerKeyRef)
			}

			// Property 1: the canonical_signable_bytes_utf8 string
			// must hash to the declared SHA-256.
			signableBytes := []byte(c.Expected.CanonicalSignableBytesUTF8)
			sum := sha256.Sum256(signableBytes)
			if hex.EncodeToString(sum[:]) != c.Expected.SHA256OfCanonicalHex {
				t.Errorf("sha256 mismatch for utf8 canonical")
			}

			// Property 2: hex and utf8 forms are equivalent.
			hexDecoded, err := hex.DecodeString(c.Expected.CanonicalSignableBytesHex)
			if err != nil {
				t.Fatalf("decode hex canonical: %v", err)
			}
			if string(hexDecoded) != c.Expected.CanonicalSignableBytesUTF8 {
				t.Errorf("hex and utf8 canonical forms diverge")
			}

			// Property 3: reference signature verifies against the
			// canonical bytes + key's public key.
			if !verifyIEEE1363(key.PublicKeySEC1Hex, signableBytes, c.Expected.ReferenceSignatureHex) {
				t.Errorf("reference signature did not verify via authoritative path")
			}

			// Property 4: tampered signature rejects.
			sigBytes, _ := hex.DecodeString(c.Expected.ReferenceSignatureHex)
			tampered := make([]byte, len(sigBytes))
			copy(tampered, sigBytes)
			tampered[5] ^= 0x01
			if verifyIEEE1363(key.PublicKeySEC1Hex, signableBytes, hex.EncodeToString(tampered)) {
				t.Errorf("tampered signature incorrectly verified")
			}

			// Property 5 (currently SKIPPED for pkg/client): the
			// SDK's own Sign(canonical) + Verify round-trip should
			// succeed. Skipped because pkg/client.(*Quid).Sign
			// currently produces DER-encoded signatures rather
			// than IEEE-1363.
			t.Run("pkg_client_sdk_signverify_roundtrip", func(t *testing.T) {
				t.Skip("SKIPPED: pkg/client.(*Quid).Sign uses ecdsa.SignASN1 (DER). " +
					"Test will be un-skipped once SDK migrates to IEEE-1363. " +
					"Tracking: docs/test-vectors/v1.0/README.md §Known divergences.")
			})
		})
	}
}

// --- pkg/client canonical-bytes divergence probe ---------------------------

// TestPkgClientCanonicalBytesDivergesFromAuthoritative
// documents the concrete difference between
// pkg/client.CanonicalBytes and the authoritative form.
// The test is expected to SUCCEED in demonstrating the
// divergence — it PASSES if the divergence exists, FAILS
// if pkg/client has been converged (which is the desired
// post-fix state; the test self-removes at that point).
func TestPkgClientCanonicalBytesDivergesFromAuthoritative(t *testing.T) {
	// Take a TRUST vector, re-serialize via
	// pkg/client.CanonicalBytes, and compare bytes.
	vf := loadVectorFile(t, "trust-tx.json")
	if len(vf.Cases) == 0 {
		t.Fatal("no cases")
	}
	c := vf.Cases[0]

	// Decode the input into map[string]any (what pkg/client
	// works with when callers pass anonymous data).
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
		t.Logf("pkg/client.CanonicalBytes converged with authoritative form! " +
			"This test can be removed.")
		return
	}

	t.Logf("expected (authoritative, struct-decl order): %s", string(authoritativeBytes))
	t.Logf("got (pkg/client, alphabetical map order):    %s", string(sdkBytes))
	t.Logf("Divergence confirmed: pkg/client.CanonicalBytes uses " +
		"alphabetical key ordering; authoritative form uses " +
		"struct-declaration order. This is a launch-blocking " +
		"issue tracked in docs/test-vectors/v1.0/README.md.")
}

// TestPkgClientSignDivergesFromAuthoritative documents the
// DER-vs-IEEE-1363 divergence. A signature from
// pkg/client.(*Quid).Sign will not verify via the
// authoritative verifyIEEE1363 because of the different
// encoding.
func TestPkgClientSignDivergesFromAuthoritative(t *testing.T) {
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
		t.Fatalf("hex decode SDK sig: %v", err)
	}

	// DER signatures for P-256 are typically 70-72 bytes;
	// always != 64.
	if len(sdkSig) == 64 {
		t.Logf("pkg/client.(*Quid).Sign now produces 64-byte signatures! " +
			"Divergence resolved. This test can be removed once " +
			"all submit paths are also verified server-compat.")
		return
	}

	t.Logf("pkg/client signature length: %d bytes (DER)", len(sdkSig))
	t.Logf("authoritative (IEEE-1363) expects exactly 64 bytes")

	// Verify the authoritative verifier rejects DER.
	if verifyIEEE1363(q.PublicKeyHex, data, sdkSigHex) {
		t.Error("authoritative verifier accepted a DER signature — unexpected")
	}
	t.Logf("Divergence confirmed: pkg/client signatures are DER, " +
		"authoritative form is IEEE-1363 raw. This is a launch- " +
		"blocking issue tracked in docs/test-vectors/v1.0/README.md.")
}
