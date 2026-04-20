// Command quidnug-test-vectors generates the cross-SDK test
// vector suite for protocol v1.0 conformance.
//
// The generator produces canonical byte forms + reference
// signatures + transaction IDs for each v1.0 transaction
// type, using the authoritative implementation in
// `internal/core`. Output is one JSON file per transaction
// type under the specified directory.
//
// Deterministic test keys are derived from fixed seeds so
// keys (and therefore the canonical bytes containing the
// public keys) are reproducible across regenerations. The
// resulting signatures vary per regeneration because Go's
// ecdsa.Sign uses a fresh random nonce each time; SDK
// consumers verify the reference signature rather than
// reproduce it.
//
// Invocation:
//
//	go run ./cmd/quidnug-test-vectors generate \
//	    --out docs/test-vectors/v1.0
//
// See docs/test-vectors/v1.0/README.md for the consumer
// contract and canonical-form specification.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/quidnug/quidnug/internal/core"
)

func init() {
	cr = cryptorand.Reader
}

const schemaVersion = "1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "quidnug-test-vectors: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "generate":
		return cmdGenerate(args[1:])
	case "help", "-h", "--help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() error {
	fmt.Println(`quidnug-test-vectors — generate v1.0 cross-SDK test vectors

Usage:
  quidnug-test-vectors generate --out <dir>

Options:
  --out DIR      Output directory (default: docs/test-vectors/v1.0)

See docs/test-vectors/v1.0/README.md for the canonical form this
generator produces and the conformance contract for SDK consumers.`)
	return nil
}

// ---------------------------------------------------------------
// Deterministic test keys
// ---------------------------------------------------------------

// testKey is a deterministic keypair. The private scalar is
// derived from sha256(seed) reduced mod n. This is NOT a
// secure method for production keys; it's used here because
// reproducibility of the quid IDs across vector regenerations
// is more valuable than cryptographic hygiene (the keys are
// public, checked in, and never used in real networks).
type testKey struct {
	Name          string
	Seed          string
	PrivD         *big.Int
	Priv          *ecdsa.PrivateKey
	PubSEC1Bytes  []byte
	PubHex        string
	QuidID        string
	PrivScalarHex string
}

func deriveTestKey(name, seed string) (*testKey, error) {
	curve := elliptic.P256()
	digest := sha256.Sum256([]byte(seed))

	// Reduce digest mod (n-1), then +1 to guarantee in [1, n-1].
	// Guarantees a valid ECDSA private scalar.
	n := curve.Params().N
	d := new(big.Int).SetBytes(digest[:])
	nMinus1 := new(big.Int).Sub(n, big.NewInt(1))
	d.Mod(d, nMinus1)
	d.Add(d, big.NewInt(1))

	priv := new(ecdsa.PrivateKey)
	priv.Curve = curve
	priv.D = d
	priv.PublicKey.Curve = curve
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())

	pubBytes := elliptic.Marshal(curve, priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck
	pubHex := hex.EncodeToString(pubBytes)

	sum := sha256.Sum256(pubBytes)
	quidID := hex.EncodeToString(sum[:8])

	return &testKey{
		Name:          name,
		Seed:          seed,
		PrivD:         d,
		Priv:          priv,
		PubSEC1Bytes:  pubBytes,
		PubHex:        pubHex,
		QuidID:        quidID,
		PrivScalarHex: hex.EncodeToString(d.Bytes()),
	}, nil
}

// signIEEE1363 produces a 64-byte raw (r||s) hex ECDSA
// signature over SHA-256(data). This matches what the
// reference node's VerifySignature accepts.
func (k *testKey) signIEEE1363(data []byte) (string, error) {
	h := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rngFor(k), k.Priv, h[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	rb := r.Bytes()
	sb := s.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):64], sb)
	return hex.EncodeToString(sig), nil
}

// rngFor returns a reader for the signing RNG. Currently we
// use crypto/rand; when RFC 6979 deterministic signing is
// mandated this switches to an RFC-6979 reader derived from
// (k, data).
func rngFor(_ *testKey) io.Reader {
	return newRandReader()
}

// newRandReader returns a fresh crypto/rand reader. Wrapped
// so the call site stays unchanged when the signing RNG
// becomes RFC 6979 deterministic.
func newRandReader() io.Reader {
	return cryptoRand{}
}

type cryptoRand struct{}

func (cryptoRand) Read(p []byte) (int, error) {
	return readFromCryptoRand(p)
}

// ---------------------------------------------------------------
// Case + file types
// ---------------------------------------------------------------

type vectorFile struct {
	SchemaVersion       string      `json:"schema_version"`
	TxType              string      `json:"tx_type"`
	GeneratedAt         string      `json:"generated_at"`
	GeneratorCommit     string      `json:"generator_commit"`
	CanonicalFormNotes  string      `json:"canonical_form_notes"`
	Cases               []vectorCase `json:"cases"`
}

type vectorCase struct {
	Name         string          `json:"name"`
	Comments     string          `json:"comments,omitempty"`
	SignerKeyRef string          `json:"signer_key_ref"`
	Input        json.RawMessage `json:"input"`
	Expected     expectedBlock   `json:"expected"`
}

type expectedBlock struct {
	CanonicalSignableBytesHex  string `json:"canonical_signable_bytes_hex"`
	CanonicalSignableBytesUTF8 string `json:"canonical_signable_bytes_utf8"`
	SHA256OfCanonicalHex       string `json:"sha256_of_canonical_hex"`
	ExpectedID                 string `json:"expected_id"`
	ReferenceSignatureHex      string `json:"reference_signature_hex"`
	SignatureLengthBytes       int    `json:"signature_length_bytes"`
}

type keyFile struct {
	Name                string `json:"name"`
	Seed                string `json:"seed"`
	PrivateScalarHex    string `json:"private_scalar_hex"`
	PublicKeySEC1Hex    string `json:"public_key_sec1_hex"`
	QuidID              string `json:"quid_id"`
	Notes               string `json:"notes"`
}

// ---------------------------------------------------------------
// generate command
// ---------------------------------------------------------------

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	var outDir string
	fs.StringVar(&outDir, "out", "docs/test-vectors/v1.0", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(outDir, "test-keys"), 0755); err != nil {
		return err
	}

	// Derive test keys
	alice, err := deriveTestKey("alice", "quidnug-test-key-alice-v1")
	if err != nil {
		return fmt.Errorf("derive alice: %w", err)
	}
	bob, err := deriveTestKey("bob", "quidnug-test-key-bob-v1")
	if err != nil {
		return fmt.Errorf("derive bob: %w", err)
	}
	carol, err := deriveTestKey("carol", "quidnug-test-key-carol-v1")
	if err != nil {
		return fmt.Errorf("derive carol: %w", err)
	}
	keys := []*testKey{alice, bob, carol}

	// Write test-key files
	for _, k := range keys {
		kf := keyFile{
			Name:             k.Name,
			Seed:             k.Seed,
			PrivateScalarHex: k.PrivScalarHex,
			PublicKeySEC1Hex: k.PubHex,
			QuidID:           k.QuidID,
			Notes:            "deterministic test key; NEVER use in production",
		}
		if err := writeJSON(filepath.Join(outDir, "test-keys", "key-"+k.Name+".json"), kf); err != nil {
			return err
		}
	}

	commit := gitCommit()
	now := time.Now().UTC().Format(time.RFC3339)

	// Generate per-type vectors
	if err := genTrustVectors(outDir, keys, commit, now); err != nil {
		return fmt.Errorf("trust vectors: %w", err)
	}
	if err := genIdentityVectors(outDir, keys, commit, now); err != nil {
		return fmt.Errorf("identity vectors: %w", err)
	}
	if err := genEventVectors(outDir, keys, commit, now); err != nil {
		return fmt.Errorf("event vectors: %w", err)
	}

	fmt.Printf("Generated test vectors in %s\n", outDir)
	fmt.Printf("  Keys: %d (alice, bob, carol)\n", len(keys))
	fmt.Printf("  Files: trust-tx.json, identity-tx.json, event-tx.json\n")
	return nil
}

// ---------------------------------------------------------------
// Canonical bytes + ID helpers
// ---------------------------------------------------------------

// canonicalSignableBytes produces the authoritative v1.0
// signable byte form for a typed transaction struct. Matches
// the reference node's verification path:
// json.Marshal(txCopy) with Signature cleared.
func canonicalSignableBytes(v any) ([]byte, error) {
	return json.Marshal(v)
}

// trustTxID derives the transaction ID for a TRUST tx, using
// the same field subset as the reference node's
// AddTrustTransaction.
func trustTxID(tx core.TrustTransaction) string {
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

// identityTxID derives the transaction ID for an IDENTITY tx.
func identityTxID(tx core.IdentityTransaction) string {
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

// eventTxID derives the transaction ID for an EVENT tx.
func eventTxID(tx core.EventTransaction) string {
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

// ---------------------------------------------------------------
// TRUST vectors
// ---------------------------------------------------------------

func genTrustVectors(outDir string, keys []*testKey, commit, now string) error {
	alice, bob, carol := keys[0], keys[1], keys[2]

	// Fixed timestamps so the vectors are stable across runs
	// even though signatures are not.
	baseTS := int64(1729468800) // 2024-10-20T00:00:00Z Unix seconds

	cases := []struct {
		name    string
		cmt     string
		signer  *testKey
		build   func(k *testKey) core.TrustTransaction
	}{
		{
			name:   "trust_alice_to_bob_level_0.9_no_expiry",
			cmt:    "Canonical case: Alice expresses full-ish trust in Bob for the 'contractors.home' domain, no ValidUntil.",
			signer: alice,
			build: func(k *testKey) core.TrustTransaction {
				return core.TrustTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeTrust,
						TrustDomain: "contractors.home",
						Timestamp:   baseTS,
						PublicKey:   k.PubHex,
					},
					Truster:     k.QuidID,
					Trustee:     bob.QuidID,
					TrustLevel:  0.9,
					Nonce:       1,
					Description: "handyman",
				}
			},
		},
		{
			name:   "trust_alice_to_carol_with_valid_until",
			cmt:    "Exercises the QDP-0022 ValidUntil field; expiry is 1 year after Timestamp.",
			signer: alice,
			build: func(k *testKey) core.TrustTransaction {
				return core.TrustTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeTrust,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS,
						PublicKey:   k.PubHex,
					},
					Truster:    k.QuidID,
					Trustee:    carol.QuidID,
					TrustLevel: 0.7,
					Nonce:      2,
					ValidUntil: baseTS + 365*24*3600,
				}
			},
		},
		{
			name:   "trust_bob_to_alice_level_1.0_minimal_description",
			cmt:    "Maximum trust level (1.0), no Description, verifies omitempty handling.",
			signer: bob,
			build: func(k *testKey) core.TrustTransaction {
				return core.TrustTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeTrust,
						TrustDomain: "peering.consortium-a",
						Timestamp:   baseTS + 1,
						PublicKey:   k.PubHex,
					},
					Truster:    k.QuidID,
					Trustee:    alice.QuidID,
					TrustLevel: 1.0,
					Nonce:      1,
				}
			},
		},
		{
			name:   "trust_alice_to_bob_level_zero_revocation_semantic",
			cmt:    "Level=0.0 is semantically a revocation/denial. Tests float edge case.",
			signer: alice,
			build: func(k *testKey) core.TrustTransaction {
				return core.TrustTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeTrust,
						TrustDomain: "contractors.home",
						Timestamp:   baseTS + 86400,
						PublicKey:   k.PubHex,
					},
					Truster:     k.QuidID,
					Trustee:     bob.QuidID,
					TrustLevel:  0.0,
					Nonce:       3,
					Description: "revoke",
				}
			},
		},
	}

	vf := vectorFile{
		SchemaVersion:      schemaVersion,
		TxType:             "TRUST",
		GeneratedAt:        now,
		GeneratorCommit:    commit,
		CanonicalFormNotes: "json.Marshal(txCopy) on typed TrustTransaction struct with Signature=''",
	}

	for _, c := range cases {
		tx := c.build(c.signer)
		tx.ID = trustTxID(tx)
		tx.Signature = "" // signable form
		signable, err := canonicalSignableBytes(tx)
		if err != nil {
			return err
		}
		sigHex, err := c.signer.signIEEE1363(signable)
		if err != nil {
			return err
		}

		// Re-marshal the input WITH signature for the vector's
		// `input` field. Consumers strip Signature themselves
		// when testing canonical-bytes property.
		txWithSig := tx
		txWithSig.Signature = sigHex
		inputBytes, err := json.Marshal(txWithSig)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(signable)
		sigBytes, _ := hex.DecodeString(sigHex)

		vf.Cases = append(vf.Cases, vectorCase{
			Name:         c.name,
			Comments:     c.cmt,
			SignerKeyRef: c.signer.Name,
			Input:        inputBytes,
			Expected: expectedBlock{
				CanonicalSignableBytesHex:  hex.EncodeToString(signable),
				CanonicalSignableBytesUTF8: string(signable),
				SHA256OfCanonicalHex:       hex.EncodeToString(sum[:]),
				ExpectedID:                 tx.ID,
				ReferenceSignatureHex:      sigHex,
				SignatureLengthBytes:       len(sigBytes),
			},
		})
	}

	return writeJSON(filepath.Join(outDir, "trust-tx.json"), vf)
}

// ---------------------------------------------------------------
// IDENTITY vectors
// ---------------------------------------------------------------

func genIdentityVectors(outDir string, keys []*testKey, commit, now string) error {
	alice, bob, carol := keys[0], keys[1], keys[2]
	baseTS := int64(1729468800)

	cases := []struct {
		name   string
		cmt    string
		signer *testKey
		build  func(k *testKey) core.IdentityTransaction
	}{
		{
			name:   "identity_alice_self_registration",
			cmt:    "Alice self-registers her quid with minimal attributes.",
			signer: alice,
			build: func(k *testKey) core.IdentityTransaction {
				return core.IdentityTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeIdentity,
						TrustDomain: "contractors.home",
						Timestamp:   baseTS,
						PublicKey:   k.PubHex,
					},
					QuidID:      k.QuidID,
					Name:        "alice",
					Creator:     k.QuidID,
					UpdateNonce: 1,
				}
			},
		},
		{
			name:   "identity_bob_with_attributes",
			cmt:    "Bob's registration carries a role and region in attributes.",
			signer: bob,
			build: func(k *testKey) core.IdentityTransaction {
				return core.IdentityTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeIdentity,
						TrustDomain: "peering.consortium-a",
						Timestamp:   baseTS + 1,
						PublicKey:   k.PubHex,
					},
					QuidID:      k.QuidID,
					Name:        "bob",
					Description: "consortium member",
					Attributes: map[string]interface{}{
						"role":   "validator",
						"region": "us-east",
					},
					Creator:     k.QuidID,
					UpdateNonce: 1,
				}
			},
		},
		{
			name:   "identity_carol_update_nonce_2_with_home_domain",
			cmt:    "Update-nonce > 1, HomeDomain populated for QDP-0007 epoch probe.",
			signer: carol,
			build: func(k *testKey) core.IdentityTransaction {
				return core.IdentityTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeIdentity,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS + 86400,
						PublicKey:   k.PubHex,
					},
					QuidID:      k.QuidID,
					Name:        "carol",
					Creator:     k.QuidID,
					UpdateNonce: 2,
					HomeDomain:  "reviews.public.technology.laptops",
				}
			},
		},
	}

	vf := vectorFile{
		SchemaVersion:      schemaVersion,
		TxType:             "IDENTITY",
		GeneratedAt:        now,
		GeneratorCommit:    commit,
		CanonicalFormNotes: "json.Marshal(txCopy) on typed IdentityTransaction struct with Signature=''",
	}

	for _, c := range cases {
		tx := c.build(c.signer)
		tx.ID = identityTxID(tx)
		tx.Signature = ""
		signable, err := canonicalSignableBytes(tx)
		if err != nil {
			return err
		}
		sigHex, err := c.signer.signIEEE1363(signable)
		if err != nil {
			return err
		}

		txWithSig := tx
		txWithSig.Signature = sigHex
		inputBytes, err := json.Marshal(txWithSig)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(signable)
		sigBytes, _ := hex.DecodeString(sigHex)

		vf.Cases = append(vf.Cases, vectorCase{
			Name:         c.name,
			Comments:     c.cmt,
			SignerKeyRef: c.signer.Name,
			Input:        inputBytes,
			Expected: expectedBlock{
				CanonicalSignableBytesHex:  hex.EncodeToString(signable),
				CanonicalSignableBytesUTF8: string(signable),
				SHA256OfCanonicalHex:       hex.EncodeToString(sum[:]),
				ExpectedID:                 tx.ID,
				ReferenceSignatureHex:      sigHex,
				SignatureLengthBytes:       len(sigBytes),
			},
		})
	}

	return writeJSON(filepath.Join(outDir, "identity-tx.json"), vf)
}

// ---------------------------------------------------------------
// EVENT vectors
// ---------------------------------------------------------------

func genEventVectors(outDir string, keys []*testKey, commit, now string) error {
	alice, bob, _ := keys[0], keys[1], keys[2]
	baseTS := int64(1729468800)

	cases := []struct {
		name   string
		cmt    string
		signer *testKey
		build  func(k *testKey) core.EventTransaction
	}{
		{
			name:   "event_alice_review_created_inline_payload",
			cmt:    "Alice posts a reviews.review.created event with an inline payload.",
			signer: alice,
			build: func(k *testKey) core.EventTransaction {
				return core.EventTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeEvent,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS,
						PublicKey:   k.PubHex,
					},
					SubjectID:   k.QuidID,
					SubjectType: "QUID",
					Sequence:    1,
					EventType:   "reviews.review.created",
					Payload: map[string]interface{}{
						"rating":       4.5,
						"product":      "thinkpad-x1",
						"title":        "solid machine",
						"body_excerpt": "keyboard is excellent; battery could be better",
					},
				}
			},
		},
		{
			name:   "event_bob_empty_payload_payload_cid_only",
			cmt:    "Bob emits an event with only PayloadCID (off-chain IPFS blob).",
			signer: bob,
			build: func(k *testKey) core.EventTransaction {
				return core.EventTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeEvent,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS + 3600,
						PublicKey:   k.PubHex,
					},
					SubjectID:   k.QuidID,
					SubjectType: "QUID",
					Sequence:    1,
					EventType:   "reviews.review.created",
					PayloadCID:  "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
				}
			},
		},
		{
			name:   "event_alice_linked_sequence_2",
			cmt:    "Second event in Alice's stream; PreviousEventID links to first.",
			signer: alice,
			build: func(k *testKey) core.EventTransaction {
				return core.EventTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeEvent,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS + 86400,
						PublicKey:   k.PubHex,
					},
					SubjectID:       k.QuidID,
					SubjectType:     "QUID",
					Sequence:        2,
					EventType:       "reviews.review.updated",
					PreviousEventID: "0000000000000000000000000000000000000000000000000000000000000001",
					Payload: map[string]interface{}{
						"correction": "updated battery-life claim based on longer testing",
					},
				}
			},
		},
	}

	vf := vectorFile{
		SchemaVersion:      schemaVersion,
		TxType:             "EVENT",
		GeneratedAt:        now,
		GeneratorCommit:    commit,
		CanonicalFormNotes: "json.Marshal(txCopy) on typed EventTransaction struct with Signature=''",
	}

	for _, c := range cases {
		tx := c.build(c.signer)
		tx.ID = eventTxID(tx)
		tx.Signature = ""
		signable, err := canonicalSignableBytes(tx)
		if err != nil {
			return err
		}
		sigHex, err := c.signer.signIEEE1363(signable)
		if err != nil {
			return err
		}

		txWithSig := tx
		txWithSig.Signature = sigHex
		inputBytes, err := json.Marshal(txWithSig)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(signable)
		sigBytes, _ := hex.DecodeString(sigHex)

		vf.Cases = append(vf.Cases, vectorCase{
			Name:         c.name,
			Comments:     c.cmt,
			SignerKeyRef: c.signer.Name,
			Input:        inputBytes,
			Expected: expectedBlock{
				CanonicalSignableBytesHex:  hex.EncodeToString(signable),
				CanonicalSignableBytesUTF8: string(signable),
				SHA256OfCanonicalHex:       hex.EncodeToString(sum[:]),
				ExpectedID:                 tx.ID,
				ReferenceSignatureHex:      sigHex,
				SignatureLengthBytes:       len(sigBytes),
			},
		})
	}

	return writeJSON(filepath.Join(outDir, "event-tx.json"), vf)
}

// ---------------------------------------------------------------
// File I/O + helpers
// ---------------------------------------------------------------

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func gitCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// readFromCryptoRand is separated so we can stub it out in
// future RFC-6979 migration without touching call sites.
func readFromCryptoRand(p []byte) (int, error) {
	// lazy import to avoid naming clash with `rand` used elsewhere
	return cryptoRandReadImpl(p)
}

// cryptoRandReadImpl pulls from crypto/rand. Split from
// readFromCryptoRand to make the swap point explicit.
func cryptoRandReadImpl(p []byte) (int, error) {
	return cryptoRandFallback(p)
}

// cryptoRandFallback is the final fallback calling
// crypto/rand.Reader directly.
func cryptoRandFallback(p []byte) (int, error) {
	if cr == nil {
		return 0, errors.New("crypto rand reader unavailable")
	}
	return cr.Read(p)
}

// cr is the actual crypto/rand.Reader; assigned at init so
// the import stays visible to go vet.
var cr io.Reader
