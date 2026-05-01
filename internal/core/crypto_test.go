// Package core — crypto_test.go
//
// Methodology
// -----------
// crypto.go had no dedicated test file before the audit; behavior
// was being exercised only indirectly through upstream integration
// tests. This file fills the gap with focused tests of each exported
// helper:
//
//   - SignData + VerifySignature: sign-then-verify round-trip,
//     tampered-payload rejection, wrong-key rejection, degenerate
//     inputs (empty/garbage hex).
//   - GetPublicKeyHex: nil-receiver and nil-PublicKey safety — the
//     defensive check added during the audit that stopped a panic
//     on incompletely-initialized test nodes.
//   - GetBlockSignableData: the "signatures don't sign themselves"
//     invariant — mutating ValidatorSigs must not change the
//     canonical bytes used for signing.
//
// Tests use freshly-generated P-256 keys from crypto/ecdsa; key
// material is never shared across tests or persisted to disk.
package core

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestSignData_ProducesValidECDSASignature(t *testing.T) {
	node := newTestNode()

	payload := []byte("quidnug trust payload")
	sig, err := node.SignData(payload)
	if err != nil {
		t.Fatalf("SignData returned error: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("expected 64-byte signature (r||s), got %d", len(sig))
	}

	pubHex := node.GetPublicKeyHex()
	if !VerifySignature(pubHex, payload, hex.EncodeToString(sig)) {
		t.Fatal("VerifySignature rejected a freshly-signed payload")
	}
}

func TestVerifySignature_RejectsTamperedPayload(t *testing.T) {
	node := newTestNode()

	payload := []byte("original payload")
	sig, err := node.SignData(payload)
	if err != nil {
		t.Fatalf("SignData returned error: %v", err)
	}
	pubHex := node.GetPublicKeyHex()
	sigHex := hex.EncodeToString(sig)

	tampered := []byte("ORIGINAL payload")
	if VerifySignature(pubHex, tampered, sigHex) {
		t.Fatal("VerifySignature accepted a modified payload")
	}
}

func TestVerifySignature_RejectsWrongKey(t *testing.T) {
	nodeA := newTestNode()
	nodeB := newTestNode()

	payload := []byte("hello")
	sig, err := nodeA.SignData(payload)
	if err != nil {
		t.Fatalf("SignData returned error: %v", err)
	}

	if VerifySignature(nodeB.GetPublicKeyHex(), payload, hex.EncodeToString(sig)) {
		t.Fatal("VerifySignature accepted a signature from a different key")
	}
}

func TestVerifySignature_EmptyInputs(t *testing.T) {
	cases := []struct {
		name   string
		pubHex string
		data   []byte
		sigHex string
	}{
		{"empty pubkey", "", []byte("x"), "00"},
		{"empty sig", "04aa", []byte("x"), ""},
		{"garbage pubkey", "zzzz", []byte("x"), "aa"},
		{"odd-length sig", "04aa", []byte("x"), "aaa"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if VerifySignature(tc.pubHex, tc.data, tc.sigHex) {
				t.Fatalf("VerifySignature accepted invalid input")
			}
		})
	}
}

func TestGetPublicKeyHex_NilPublicKey(t *testing.T) {
	var node *QuidnugNode
	if got := node.GetPublicKeyHex(); got != "" {
		t.Fatalf("expected empty string for nil node, got %q", got)
	}

	empty := &QuidnugNode{}
	if got := empty.GetPublicKeyHex(); got != "" {
		t.Fatalf("expected empty string when PublicKey is nil, got %q", got)
	}
}

func TestGetBlockSignableData_ExcludesValidatorSigs(t *testing.T) {
	block := Block{
		Index:     1,
		Timestamp: 123,
		PrevHash:  "abc",
		TrustProof: TrustProof{
			TrustDomain:   "d",
			ValidatorID:   "v",
			ValidatorSigs: []string{"sig-that-should-not-be-included"},
		},
	}
	data1 := GetBlockSignableData(block)

	// Mutating ValidatorSigs must not change the canonical signable bytes.
	block.TrustProof.ValidatorSigs = append(block.TrustProof.ValidatorSigs, "another")
	data2 := GetBlockSignableData(block)

	if string(data1) != string(data2) {
		t.Fatal("GetBlockSignableData is sensitive to ValidatorSigs; signatures would sign themselves")
	}
}

// TestGetBlockSignableData_StableUnderJSONRoundTrip is the ENG-82
// regression guard: signing a locally-constructed block (transactions
// are typed structs) and verifying a JSON-decoded peer-served block
// (transactions are map[string]interface{}) must use the same canonical
// bytes. Before the fix, the typed-struct path produced
// declaration-order JSON and the map path produced alphabetical-order
// JSON; SHA-256 diverged, ECDSA verify rejected. This test fails on
// the broken implementation and passes on the canonical-form one.
func TestGetBlockSignableData_StableUnderJSONRoundTrip(t *testing.T) {
	// Build a block the way block_operations.GenerateBlockForDomain does:
	// typed transaction structs inside []interface{}.
	typedBlock := Block{
		Index:     7,
		Timestamp: 1700000000,
		Transactions: []interface{}{
			TrustTransaction{
				BaseTransaction: BaseTransaction{
					Type:        TxTypeTrust,
					TrustDomain: "default",
					Timestamp:   1700000000,
				},
				Truster:    "trusterquid000000",
				Trustee:    "trusteequid000000",
				TrustLevel: 0.95,
			},
			IdentityTransaction{
				BaseTransaction: BaseTransaction{
					Type:        TxTypeIdentity,
					TrustDomain: "default",
					Timestamp:   1700000000,
					PublicKey:   "04aabbcc",
				},
				QuidID: "alicequid00000000",
				Name:   "alice",
			},
		},
		TrustProof: TrustProof{
			TrustDomain:             "default",
			ValidatorID:             "validator00000000",
			ValidatorPublicKey:      "04ddeeff",
			ValidatorTrustInCreator: 1.0,
			ValidatorSigs:           []string{},
			ValidationTime:          1700000000,
		},
		PrevHash: "deadbeef",
	}

	signTimeBytes := GetBlockSignableData(typedBlock)
	if signTimeBytes == nil {
		t.Fatal("sign-time path produced nil signable bytes")
	}

	// Simulate the wire path: marshal and unmarshal the block.
	// json.Unmarshal of []interface{} fields produces []interface{} of
	// map[string]interface{} elements, which is what every peer-received
	// block looks like in memory.
	wireBytes, err := json.Marshal(typedBlock)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var wireBlock Block
	if err := json.Unmarshal(wireBytes, &wireBlock); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	verifyTimeBytes := GetBlockSignableData(wireBlock)
	if verifyTimeBytes == nil {
		t.Fatal("verify-time path produced nil signable bytes")
	}

	if string(signTimeBytes) != string(verifyTimeBytes) {
		t.Fatalf("ENG-82 regression: signable bytes diverge across JSON round-trip\nsign-time:   %s\nverify-time: %s",
			signTimeBytes, verifyTimeBytes)
	}
}

// TestVerifySignatureDetailed_FailureCategories ensures each
// distinct failure mode produces a unique, recognizable error so
// the ENG-83 block-validation debug log can tell operators from a
// single field whether they have a missing-input, decode, length,
// or ecdsa-rejection problem.
func TestVerifySignatureDetailed_FailureCategories(t *testing.T) {
	node := newTestNode()
	pub := node.GetPublicKeyHex()
	data := []byte("hello quidnug")
	rawSig, err := node.SignData(data)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	goodSig := hex.EncodeToString(rawSig)

	cases := []struct {
		name         string
		pub          string
		data         []byte
		sig          string
		wantOK       bool
		wantContains string
	}{
		{
			name: "happy path",
			pub:  pub, data: data, sig: goodSig,
			wantOK: true,
		},
		{
			name: "missing public key",
			pub:  "", data: data, sig: goodSig,
			wantContains: "missing public key",
		},
		{
			name: "missing signature",
			pub:  pub, data: data, sig: "",
			wantContains: "missing signature",
		},
		{
			name: "pubkey hex decode failure",
			pub:  "not-hex-zz", data: data, sig: goodSig,
			wantContains: "public key hex decode failed",
		},
		{
			name: "pubkey not on curve",
			pub:  hex.EncodeToString([]byte{0x01, 0x02, 0x03}), data: data, sig: goodSig,
			wantContains: "public key unmarshal failed",
		},
		{
			name: "signature hex decode failure",
			pub:  pub, data: data, sig: "not-hex-zz",
			wantContains: "signature hex decode failed",
		},
		{
			name: "signature wrong length",
			pub:  pub, data: data, sig: hex.EncodeToString(rawSig[:32]),
			wantContains: "signature length is 32",
		},
		{
			name: "ecdsa rejected (tampered data)",
			pub:  pub, data: []byte("HELLO QUIDNUG"), sig: goodSig,
			wantContains: "ecdsa verify rejected",
		},
		{
			name: "ecdsa rejected (wrong key)",
			pub:  newTestNode().GetPublicKeyHex(), data: data, sig: goodSig,
			wantContains: "ecdsa verify rejected",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := VerifySignatureDetailed(tc.pub, tc.data, tc.sig)
			if tc.wantOK {
				if !ok {
					t.Fatalf("expected ok=true, got false (err=%v)", err)
				}
				if err != nil {
					t.Fatalf("expected nil error on happy path, got %v", err)
				}
				return
			}
			if ok {
				t.Fatalf("expected ok=false for %s, got true", tc.name)
			}
			if err == nil {
				t.Fatalf("expected non-nil error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantContains) {
				t.Fatalf("error %q does not contain expected substring %q",
					err.Error(), tc.wantContains)
			}
		})
	}
}

// TestVerifySignature_DelegatesToDetailed confirms the boolean
// VerifySignature wrapper preserves backward-compatible behavior:
// every passing case in VerifySignatureDetailed passes here, and
// every failing case fails here.
func TestVerifySignature_DelegatesToDetailed(t *testing.T) {
	node := newTestNode()
	pub := node.GetPublicKeyHex()
	data := []byte("hello quidnug")
	rawSig, _ := node.SignData(data)
	goodSig := hex.EncodeToString(rawSig)

	if !VerifySignature(pub, data, goodSig) {
		t.Fatal("happy path returned false")
	}
	if VerifySignature("", data, goodSig) {
		t.Fatal("empty pubkey returned true")
	}
	if VerifySignature(pub, []byte("tampered"), goodSig) {
		t.Fatal("tampered data returned true")
	}
}

// TestSignedBlock_VerifiesAfterJSONRoundTrip is the higher-level
// invariant: a block signed locally must verify after being sent over
// the wire. This is the actual behavior peer nodes depend on.
func TestSignedBlock_VerifiesAfterJSONRoundTrip(t *testing.T) {
	node := newTestNode()

	block := Block{
		Index:     1,
		Timestamp: 1700000000,
		Transactions: []interface{}{
			TrustTransaction{
				BaseTransaction: BaseTransaction{
					Type:        TxTypeTrust,
					TrustDomain: "default",
					Timestamp:   1700000000,
				},
				Truster:    "trusterquid000000",
				Trustee:    "trusteequid000000",
				TrustLevel: 0.5,
			},
		},
		TrustProof: TrustProof{
			TrustDomain:             "default",
			ValidatorID:             node.NodeID,
			ValidatorPublicKey:      node.GetPublicKeyHex(),
			ValidatorTrustInCreator: 1.0,
			ValidatorSigs:           []string{},
			ValidationTime:          1700000000,
		},
		PrevHash: "deadbeef",
	}

	// Sign as block_operations.GenerateBlockForDomain does.
	sig, err := node.SignData(GetBlockSignableData(block))
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	block.TrustProof.ValidatorSigs = []string{hex.EncodeToString(sig)}

	// Local verification: must pass.
	if !VerifySignature(node.GetPublicKeyHex(), GetBlockSignableData(block), block.TrustProof.ValidatorSigs[0]) {
		t.Fatal("local-side verification failed (signing-vs-verifying mismatch even without wire)")
	}

	// Wire round-trip: must still verify on the receiving side.
	wireBytes, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var received Block
	if err := json.Unmarshal(wireBytes, &received); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !VerifySignature(received.TrustProof.ValidatorPublicKey, GetBlockSignableData(received), received.TrustProof.ValidatorSigs[0]) {
		t.Fatal("ENG-82 regression: signature fails to verify after JSON round-trip even though pubkey, signature, and validator ID all match")
	}
}
