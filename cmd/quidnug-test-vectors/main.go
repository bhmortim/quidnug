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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/quidnug/quidnug/internal/core"
)

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

// signIEEE1363 produces a deterministic 64-byte raw (r||s)
// hex ECDSA signature over SHA-256(data) per RFC 6979 with
// low-s normalization. Matches the reference node's
// SignData exactly; test-vector reference signatures are now
// bit-stable across regenerations.
func (k *testKey) signIEEE1363(data []byte) (string, error) {
	h := sha256.Sum256(data)
	r, s := core.SignRFC6979(k.Priv, h[:])
	sig := make([]byte, 64)
	rb := r.Bytes()
	sb := s.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):64], sb)
	return hex.EncodeToString(sig), nil
}

// (rngFor and related plumbing removed: RFC 6979 is fully
// deterministic and needs no external entropy source.)

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
	if err := genTitleVectors(outDir, keys, commit, now); err != nil {
		return fmt.Errorf("title vectors: %w", err)
	}
	if err := genNodeAdvertisementVectors(outDir, keys, commit, now); err != nil {
		return fmt.Errorf("node-advertisement vectors: %w", err)
	}
	if err := genModerationActionVectors(outDir, keys, commit, now); err != nil {
		return fmt.Errorf("moderation-action vectors: %w", err)
	}
	if err := genDSRVectors(outDir, keys, commit, now); err != nil {
		return fmt.Errorf("DSR vectors: %w", err)
	}

	fmt.Printf("Generated test vectors in %s\n", outDir)
	fmt.Printf("  Keys: %d (alice, bob, carol)\n", len(keys))
	fmt.Printf("  Files: trust-tx.json, identity-tx.json, event-tx.json,\n")
	fmt.Printf("         title-tx.json, node-advertisement-tx.json,\n")
	fmt.Printf("         moderation-action-tx.json, dsr-tx.json\n")
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
		{
			// REGRESSION GUARD: payload has keys inserted in
			// deliberately non-alphabetical order, with a nested
			// object also in non-alphabetical order. Any SDK that
			// preserves insertion order instead of sorting
			// nested-map keys alphabetically (to match Go's
			// encoding/json default) will produce different
			// canonical bytes than this vector expects and will
			// fail signature verification server-side.
			//
			// This vector exists because several SDKs shipped with
			// the alphabetical-keys-by-accident bug masked by
			// test payloads that happened to already be sorted.
			name: "event_payload_key_sort_regression_guard",
			cmt: "Payload keys in non-alphabetical insertion order; nested object also non-alphabetical. SDKs MUST sort nested map keys alphabetically to match Go's encoding/json.",
			signer: alice,
			build: func(k *testKey) core.EventTransaction {
				return core.EventTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeEvent,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS + 172800,
						PublicKey:   k.PubHex,
					},
					SubjectID:   k.QuidID,
					SubjectType: "QUID",
					Sequence:    3,
					EventType:   "generic.test-payload",
					Payload: map[string]interface{}{
						// Reverse-alphabetical insertion order at top of payload.
						"zebra":  1,
						"apple":  2,
						"mango":  3,
						// Nested object with reverse-alphabetical insertion
						// order. Serializers that only sort the top level of
						// the payload but not nested objects will diverge here.
						"nested": map[string]interface{}{
							"yankee": "A",
							"alpha":  "B",
							"mike":   "C",
						},
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

// (Legacy crypto/rand plumbing removed — RFC 6979 is
// deterministic and needs no external entropy source.)

// ---------------------------------------------------------------
// Additional ID derivations
// ---------------------------------------------------------------

// titleTxID mirrors AddTitleTransaction. Payload: AssetID,
// Owners (the full stake list), TrustDomain, Timestamp.
func titleTxID(tx core.TitleTransaction) string {
	payload, _ := json.Marshal(struct {
		AssetID     string
		Owners      []core.OwnershipStake
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

// nodeAdvertisementTxID mirrors AddNodeAdvertisementTransaction.
func nodeAdvertisementTxID(tx core.NodeAdvertisementTransaction) string {
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

// moderationActionTxID mirrors AddModerationActionTransaction.
func moderationActionTxID(tx core.ModerationActionTransaction) string {
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

func dsrRequestTxID(tx core.DataSubjectRequestTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject     string
		Controller  string
		RequestType string
		Nonce       int64
		Timestamp   int64
	}{
		Subject:     tx.SubjectQuid,
		Controller:  tx.ControllerQuid,
		RequestType: tx.RequestType,
		Nonce:       tx.Nonce,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func consentGrantTxID(tx core.ConsentGrantTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject    string
		Controller string
		Scope      []string
		PolicyHash string
		Nonce      int64
		Timestamp  int64
	}{
		Subject:    tx.SubjectQuid,
		Controller: tx.ControllerQuid,
		Scope:      tx.Scope,
		PolicyHash: tx.PolicyHash,
		Nonce:      tx.Nonce,
		Timestamp:  tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func consentWithdrawTxID(tx core.ConsentWithdrawTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject   string
		Withdraw  string
		Nonce     int64
		Timestamp int64
	}{
		Subject:   tx.SubjectQuid,
		Withdraw:  tx.WithdrawsGrantTxID,
		Nonce:     tx.Nonce,
		Timestamp: tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func processingRestrictionTxID(tx core.ProcessingRestrictionTransaction) string {
	payload, _ := json.Marshal(struct {
		Subject   string
		Uses      []string
		Nonce     int64
		Timestamp int64
	}{
		Subject:   tx.SubjectQuid,
		Uses:      tx.RestrictedUses,
		Nonce:     tx.Nonce,
		Timestamp: tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func dsrComplianceTxID(tx core.DSRComplianceTransaction) string {
	payload, _ := json.Marshal(struct {
		RequestTxID string
		Operator    string
		CompletedAt int64
		Nonce       int64
		Timestamp   int64
	}{
		RequestTxID: tx.RequestTxID,
		Operator:    tx.OperatorQuid,
		CompletedAt: tx.CompletedAt,
		Nonce:       tx.Nonce,
		Timestamp:   tx.Timestamp,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------
// TITLE vectors
// ---------------------------------------------------------------

func genTitleVectors(outDir string, keys []*testKey, commit, now string) error {
	alice, bob, carol := keys[0], keys[1], keys[2]
	baseTS := int64(1729468800)

	cases := []struct {
		name   string
		cmt    string
		signer *testKey
		build  func(k *testKey) core.TitleTransaction
	}{
		{
			name:   "title_alice_sole_owner",
			cmt:    "Sole-owner title: Alice owns 100% of asset-1.",
			signer: alice,
			build: func(k *testKey) core.TitleTransaction {
				return core.TitleTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeTitle,
						TrustDomain: "titles.home",
						Timestamp:   baseTS,
						PublicKey:   k.PubHex,
					},
					AssetID: "asset-sku-000001",
					Owners: []core.OwnershipStake{
						{OwnerID: k.QuidID, Percentage: 1.0, StakeType: "full-ownership"},
					},
					Signatures: map[string]string{},
					TitleType:  "inventory",
				}
			},
		},
		{
			name:   "title_alice_bob_joint_ownership",
			cmt:    "Two-owner title: Alice 60%, Bob 40%.",
			signer: alice,
			build: func(k *testKey) core.TitleTransaction {
				return core.TitleTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeTitle,
						TrustDomain: "titles.home",
						Timestamp:   baseTS + 3600,
						PublicKey:   k.PubHex,
					},
					AssetID: "asset-joint-002",
					Owners: []core.OwnershipStake{
						{OwnerID: k.QuidID, Percentage: 0.6},
						{OwnerID: bob.QuidID, Percentage: 0.4},
					},
					Signatures: map[string]string{},
				}
			},
		},
		{
			name:   "title_carol_with_expiry_three_owners",
			cmt:    "Three-owner title with explicit ExpiryDate (90 days).",
			signer: carol,
			build: func(k *testKey) core.TitleTransaction {
				return core.TitleTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeTitle,
						TrustDomain: "titles.lease",
						Timestamp:   baseTS + 86400,
						PublicKey:   k.PubHex,
					},
					AssetID: "lease-apt-3f",
					Owners: []core.OwnershipStake{
						{OwnerID: alice.QuidID, Percentage: 0.4, StakeType: "tenant"},
						{OwnerID: bob.QuidID, Percentage: 0.3, StakeType: "tenant"},
						{OwnerID: k.QuidID, Percentage: 0.3, StakeType: "tenant"},
					},
					Signatures: map[string]string{},
					ExpiryDate: baseTS + 86400 + 90*86400,
					TitleType:  "residential-lease",
				}
			},
		},
	}

	vf := vectorFile{
		SchemaVersion:      schemaVersion,
		TxType:             "TITLE",
		GeneratedAt:        now,
		GeneratorCommit:    commit,
		CanonicalFormNotes: "json.Marshal(txCopy) on typed TitleTransaction struct with Signature=''",
	}
	for _, c := range cases {
		tx := c.build(c.signer)
		tx.ID = titleTxID(tx)
		tx.Signature = ""
		signable, err := json.Marshal(tx)
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
			Name: c.name, Comments: c.cmt, SignerKeyRef: c.signer.Name,
			Input: inputBytes,
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
	return writeJSON(filepath.Join(outDir, "title-tx.json"), vf)
}

// ---------------------------------------------------------------
// NODE_ADVERTISEMENT vectors
// ---------------------------------------------------------------

func genNodeAdvertisementVectors(outDir string, keys []*testKey, commit, now string) error {
	alice, bob, _ := keys[0], keys[1], keys[2]
	baseTS := int64(1729468800)

	cases := []struct {
		name   string
		cmt    string
		signer *testKey
		build  func(k *testKey) core.NodeAdvertisementTransaction
	}{
		{
			name:   "node_ad_alice_validator_single_endpoint",
			cmt:    "Minimal validator node advertisement; 7-day TTL; single endpoint.",
			signer: alice,
			build: func(k *testKey) core.NodeAdvertisementTransaction {
				return core.NodeAdvertisementTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeNodeAdvertisement,
						TrustDomain: "operators.network.reviews.public",
						Timestamp:   baseTS,
						PublicKey:   k.PubHex,
					},
					NodeQuid:     k.QuidID,
					OperatorQuid: bob.QuidID,
					Endpoints: []core.NodeEndpoint{
						{
							URL:      "https://node-alice.example.com:443",
							Protocol: "http/2",
							Region:   "iad",
							Priority: 10,
							Weight:   100,
						},
					},
					SupportedDomains: []string{"reviews.public.*"},
					Capabilities: core.NodeCapabilities{
						Validator: true,
						Cache:     true,
						Archive:   false,
					},
					ProtocolVersion:    "1.0",
					ExpiresAt:          (baseTS + 7*86400) * 1_000_000_000,
					AdvertisementNonce: 1,
				}
			},
		},
		{
			name:   "node_ad_bob_multi_region_cache_only",
			cmt:    "Multi-region cache-only node; two endpoints with different priorities.",
			signer: bob,
			build: func(k *testKey) core.NodeAdvertisementTransaction {
				return core.NodeAdvertisementTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeNodeAdvertisement,
						TrustDomain: "operators.network.reviews.public",
						Timestamp:   baseTS + 1,
						PublicKey:   k.PubHex,
					},
					NodeQuid:     k.QuidID,
					OperatorQuid: alice.QuidID,
					Endpoints: []core.NodeEndpoint{
						{
							URL: "https://cache-lhr.example.com:443",
							Protocol: "http/2", Region: "lhr", Priority: 10, Weight: 100,
						},
						{
							URL: "https://cache-sin.example.com:443",
							Protocol: "http/2", Region: "sin", Priority: 20, Weight: 50,
						},
					},
					Capabilities: core.NodeCapabilities{
						Cache: true, MaxBodyBytes: 1048576,
					},
					ProtocolVersion:    "1.0",
					ExpiresAt:          (baseTS + 1 + 6*3600) * 1_000_000_000,
					AdvertisementNonce: 1,
				}
			},
		},
	}

	vf := vectorFile{
		SchemaVersion:      schemaVersion,
		TxType:             "NODE_ADVERTISEMENT",
		GeneratedAt:        now,
		GeneratorCommit:    commit,
		CanonicalFormNotes: "json.Marshal(txCopy) on typed NodeAdvertisementTransaction struct with Signature=''",
	}
	for _, c := range cases {
		tx := c.build(c.signer)
		tx.ID = nodeAdvertisementTxID(tx)
		tx.Signature = ""
		signable, err := json.Marshal(tx)
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
			Name: c.name, Comments: c.cmt, SignerKeyRef: c.signer.Name,
			Input: inputBytes,
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
	return writeJSON(filepath.Join(outDir, "node-advertisement-tx.json"), vf)
}

// ---------------------------------------------------------------
// MODERATION_ACTION vectors
// ---------------------------------------------------------------

func genModerationActionVectors(outDir string, keys []*testKey, commit, now string) error {
	alice, bob, _ := keys[0], keys[1], keys[2]
	baseTS := int64(1729468800)

	cases := []struct {
		name   string
		cmt    string
		signer *testKey
		build  func(k *testKey) core.ModerationActionTransaction
	}{
		{
			name:   "mod_alice_suppress_event_dmca",
			cmt:    "Classic DMCA takedown: suppress scope, EVENT target.",
			signer: alice,
			build: func(k *testKey) core.ModerationActionTransaction {
				return core.ModerationActionTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeModerationAction,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS,
						PublicKey:   k.PubHex,
					},
					ModeratorQuid: k.QuidID,
					TargetType:    "EVENT",
					TargetID:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
					Scope:         "suppress",
					ReasonCode:    "DMCA",
					EvidenceURL:   "https://legal.example/dmca/2026-04-01.pdf",
					Nonce:         1,
				}
			},
		},
		{
			name:   "mod_bob_hide_quid_policy_violation",
			cmt:    "Hide a QUID for policy violation; bounded 30-day duration.",
			signer: bob,
			build: func(k *testKey) core.ModerationActionTransaction {
				return core.ModerationActionTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeModerationAction,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS + 3600,
						PublicKey:   k.PubHex,
					},
					ModeratorQuid:  k.QuidID,
					TargetType:     "QUID",
					TargetID:       alice.QuidID,
					Scope:          "hide",
					ReasonCode:     "policy-violation",
					EffectiveFrom:  baseTS + 3600,
					EffectiveUntil: baseTS + 3600 + 30*86400,
					Nonce:          1,
				}
			},
		},
		{
			name:   "mod_alice_annotate_event_with_supersede",
			cmt:    "Annotate (soft moderation) + supersedes an earlier action.",
			signer: alice,
			build: func(k *testKey) core.ModerationActionTransaction {
				return core.ModerationActionTransaction{
					BaseTransaction: core.BaseTransaction{
						Type:        core.TxTypeModerationAction,
						TrustDomain: "reviews.public.technology.laptops",
						Timestamp:   baseTS + 86400,
						PublicKey:   k.PubHex,
					},
					ModeratorQuid:  k.QuidID,
					TargetType:     "EVENT",
					TargetID:       "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
					Scope:          "annotate",
					ReasonCode:     "disputed-claim",
					AnnotationText: "Flagged pending fact-check by external editor.",
					SupersedesTxID: "0000000000000000000000000000000000000000000000000000000000000001",
					Nonce:          2,
				}
			},
		},
	}

	vf := vectorFile{
		SchemaVersion:      schemaVersion,
		TxType:             "MODERATION_ACTION",
		GeneratedAt:        now,
		GeneratorCommit:    commit,
		CanonicalFormNotes: "json.Marshal(txCopy) on typed ModerationActionTransaction struct with Signature=''",
	}
	for _, c := range cases {
		tx := c.build(c.signer)
		tx.ID = moderationActionTxID(tx)
		tx.Signature = ""
		signable, err := json.Marshal(tx)
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
			Name: c.name, Comments: c.cmt, SignerKeyRef: c.signer.Name,
			Input: inputBytes,
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
	return writeJSON(filepath.Join(outDir, "moderation-action-tx.json"), vf)
}

// ---------------------------------------------------------------
// DSR family vectors (5 types in one file)
// ---------------------------------------------------------------

type dsrVectorFile struct {
	SchemaVersion      string       `json:"schema_version"`
	TxType             string       `json:"tx_type"`
	GeneratedAt        string       `json:"generated_at"`
	GeneratorCommit    string       `json:"generator_commit"`
	CanonicalFormNotes string       `json:"canonical_form_notes"`
	Cases              []vectorCase `json:"cases"`
}

func genDSRVectors(outDir string, keys []*testKey, commit, now string) error {
	alice, bob, carol := keys[0], keys[1], keys[2]
	baseTS := int64(1729468800)

	vf := dsrVectorFile{
		SchemaVersion: schemaVersion,
		TxType:        "DSR_FAMILY",
		GeneratedAt:   now,
		GeneratorCommit: commit,
		CanonicalFormNotes: "DSR family: json.Marshal(txCopy) on typed struct with Signature=''. " +
			"Each case's Input carries a `type` field naming which of the five " +
			"tx types (DATA_SUBJECT_REQUEST | CONSENT_GRANT | CONSENT_WITHDRAW | " +
			"PROCESSING_RESTRICTION | DSR_COMPLIANCE).",
	}

	// DATA_SUBJECT_REQUEST
	{
		tx := core.DataSubjectRequestTransaction{
			BaseTransaction: core.BaseTransaction{
				Type:        core.TxTypeDataSubjectRequest,
				TrustDomain: "reviews.public.technology.laptops",
				Timestamp:   baseTS,
				PublicKey:   alice.PubHex,
			},
			SubjectQuid:    alice.QuidID,
			ControllerQuid: bob.QuidID,
			RequestType:    "access",
			ContactEmail:   "alice@example.com",
			Jurisdiction:   "EU",
			Nonce:          1,
		}
		tx.ID = dsrRequestTxID(tx)
		if err := appendDSRCase(&vf, "dsr_alice_access_gdpr",
			"GDPR Art. 15 right of access; minimal request.",
			alice, tx); err != nil {
			return err
		}
	}

	// DATA_SUBJECT_REQUEST (erasure)
	{
		tx := core.DataSubjectRequestTransaction{
			BaseTransaction: core.BaseTransaction{
				Type:        core.TxTypeDataSubjectRequest,
				TrustDomain: "reviews.public.technology.laptops",
				Timestamp:   baseTS + 60,
				PublicKey:   bob.PubHex,
			},
			SubjectQuid:    bob.QuidID,
			ControllerQuid: alice.QuidID,
			RequestType:    "erasure",
			RequestDetails: "Please delete all my reviews older than 2 years per GDPR Art. 17.",
			Jurisdiction:   "EU",
			Nonce:          1,
		}
		tx.ID = dsrRequestTxID(tx)
		if err := appendDSRCase(&vf, "dsr_bob_erasure_gdpr_art17",
			"GDPR Art. 17 right to erasure with details.",
			bob, tx); err != nil {
			return err
		}
	}

	// CONSENT_GRANT
	{
		tx := core.ConsentGrantTransaction{
			BaseTransaction: core.BaseTransaction{
				Type:        core.TxTypeConsentGrant,
				TrustDomain: "reviews.public.technology.laptops",
				Timestamp:   baseTS + 120,
				PublicKey:   alice.PubHex,
			},
			SubjectQuid:    alice.QuidID,
			ControllerQuid: bob.QuidID,
			Scope: []string{
				"reputation-computation",
				"recommendations",
			},
			PolicyURL:      "https://example.com/privacy-policy/v2",
			PolicyHash:     "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
			EffectiveUntil: baseTS + 120 + 365*86400,
			Nonce:          1,
		}
		tx.ID = consentGrantTxID(tx)
		if err := appendDSRCase(&vf, "consent_grant_alice_reputation_and_recs",
			"Consent to reputation computation + recommendations; 1-year TTL.",
			alice, tx); err != nil {
			return err
		}
	}

	// CONSENT_WITHDRAW
	{
		tx := core.ConsentWithdrawTransaction{
			BaseTransaction: core.BaseTransaction{
				Type:        core.TxTypeConsentWithdraw,
				TrustDomain: "reviews.public.technology.laptops",
				Timestamp:   baseTS + 180,
				PublicKey:   alice.PubHex,
			},
			SubjectQuid:        alice.QuidID,
			WithdrawsGrantTxID: "deadbeef0000000000000000000000000000000000000000000000000000cafe",
			Reason:             "no-longer-using-service",
			Nonce:              2,
		}
		tx.ID = consentWithdrawTxID(tx)
		if err := appendDSRCase(&vf, "consent_withdraw_alice_service_exit",
			"Consent withdrawal referencing a prior grant tx.",
			alice, tx); err != nil {
			return err
		}
	}

	// PROCESSING_RESTRICTION
	{
		tx := core.ProcessingRestrictionTransaction{
			BaseTransaction: core.BaseTransaction{
				Type:        core.TxTypeProcessingRestriction,
				TrustDomain: "reviews.public.technology.laptops",
				Timestamp:   baseTS + 240,
				PublicKey:   bob.PubHex,
			},
			SubjectQuid:    bob.QuidID,
			ControllerQuid: alice.QuidID,
			RestrictedUses: []string{"marketing-analytics", "ml-training"},
			Reason:         "gdpr-art-18-objection",
			Nonce:          1,
		}
		tx.ID = processingRestrictionTxID(tx)
		if err := appendDSRCase(&vf, "processing_restriction_bob_two_uses",
			"Restrict two processing uses under GDPR Art. 18; no explicit end.",
			bob, tx); err != nil {
			return err
		}
	}

	// DSR_COMPLIANCE (operator-signed)
	{
		tx := core.DSRComplianceTransaction{
			BaseTransaction: core.BaseTransaction{
				Type:        core.TxTypeDSRCompliance,
				TrustDomain: "reviews.public.technology.laptops",
				Timestamp:   baseTS + 300,
				PublicKey:   carol.PubHex, // operator
			},
			RequestTxID:      "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
			RequestType:      "erasure",
			OperatorQuid:     carol.QuidID,
			CompletedAt:      baseTS + 300,
			ActionsCategory:  "manifest-generated",
			CarveOutsApplied: []string{"legal-hold-tax-records"},
			Nonce:            1,
		}
		tx.ID = dsrComplianceTxID(tx)
		if err := appendDSRCase(&vf, "dsr_compliance_carol_erasure_fulfilled",
			"Operator-signed DSR_COMPLIANCE attesting fulfilled erasure.",
			carol, tx); err != nil {
			return err
		}
	}

	return writeJSON(filepath.Join(outDir, "dsr-tx.json"), vf)
}

// appendDSRCase is a typed-generic helper: marshals the tx,
// signs, populates the vector case. Uses reflection-free
// json.Marshal since each DSR type has its own struct.
func appendDSRCase[T any](vf *dsrVectorFile, name, cmt string, signer *testKey, tx T) error {
	// Clear Signature before signing. We reach into the
	// struct via json round-trip to a map so we don't need
	// reflection; faster than writing per-type sign helpers.
	signable, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	// The tx has Signature="" already since we never set it.
	// But we double-check by parsing + re-serializing if
	// needed. For structs where Signature is part of embedded
	// BaseTransaction, the zero value means "" at marshal time.
	sigHex, err := signer.signIEEE1363(signable)
	if err != nil {
		return err
	}

	// Compute expected ID (already populated on tx, but we
	// hash from signable bytes to compute SHA-256 of canonical).
	sum := sha256.Sum256(signable)
	sigBytes, _ := hex.DecodeString(sigHex)

	// The tx already has its ID filled (caller did this via
	// xxxTxID()). Extract it from the marshaled bytes.
	var temp map[string]any
	_ = json.Unmarshal(signable, &temp)
	expectedID, _ := temp["id"].(string)

	// Re-marshal with signature for the input field.
	temp["signature"] = sigHex
	inputBytes, err := json.Marshal(temp)
	if err != nil {
		return err
	}

	vf.Cases = append(vf.Cases, vectorCase{
		Name:         name,
		Comments:     cmt,
		SignerKeyRef: signer.Name,
		Input:        inputBytes,
		Expected: expectedBlock{
			CanonicalSignableBytesHex:  hex.EncodeToString(signable),
			CanonicalSignableBytesUTF8: string(signable),
			SHA256OfCanonicalHex:       hex.EncodeToString(sum[:]),
			ExpectedID:                 expectedID,
			ReferenceSignatureHex:      sigHex,
			SignatureLengthBytes:       len(sigBytes),
		},
	})
	return nil
}

