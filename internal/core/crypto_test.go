package core

import (
	"encoding/hex"
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
