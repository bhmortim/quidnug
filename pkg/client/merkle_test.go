package client

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sh(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func TestVerifyInclusionProofSingleSiblingRight(t *testing.T) {
	tx := []byte("tx-1")
	sibling := sh([]byte("tx-2"))
	leaf := sh(tx)
	root := sh(append(append([]byte{}, leaf...), sibling...))
	frames := []MerkleProofFrame{{Hash: hex.EncodeToString(sibling), Side: "right"}}
	ok, err := VerifyInclusionProof(tx, frames, hex.EncodeToString(root))
	if err != nil || !ok {
		t.Fatalf("expected ok=true err=nil, got ok=%v err=%v", ok, err)
	}
}

func TestVerifyInclusionProofSingleSiblingLeft(t *testing.T) {
	tx := []byte("tx-2")
	sibling := sh([]byte("tx-1"))
	leaf := sh(tx)
	root := sh(append(append([]byte{}, sibling...), leaf...))
	frames := []MerkleProofFrame{{Hash: hex.EncodeToString(sibling), Side: "left"}}
	ok, err := VerifyInclusionProof(tx, frames, hex.EncodeToString(root))
	if err != nil || !ok {
		t.Fatalf("expected ok=true, got ok=%v err=%v", ok, err)
	}
}

func TestVerifyInclusionProofFourLeafTree(t *testing.T) {
	leaves := [][]byte{sh([]byte("tx-0")), sh([]byte("tx-1")), sh([]byte("tx-2")), sh([]byte("tx-3"))}
	p0 := sh(append(append([]byte{}, leaves[0]...), leaves[1]...))
	p1 := sh(append(append([]byte{}, leaves[2]...), leaves[3]...))
	root := sh(append(append([]byte{}, p0...), p1...))

	frames := []MerkleProofFrame{
		{Hash: hex.EncodeToString(leaves[3]), Side: "right"},
		{Hash: hex.EncodeToString(p0), Side: "left"},
	}
	ok, err := VerifyInclusionProof([]byte("tx-2"), frames, hex.EncodeToString(root))
	if err != nil || !ok {
		t.Fatalf("expected ok=true, got ok=%v err=%v", ok, err)
	}
}

func TestVerifyInclusionProofTamperedFails(t *testing.T) {
	sibling := sh([]byte("tx-2"))
	leaf := sh([]byte("tx-1"))
	root := sh(append(append([]byte{}, leaf...), sibling...))
	frames := []MerkleProofFrame{{Hash: hex.EncodeToString(sibling), Side: "right"}}
	ok, _ := VerifyInclusionProof([]byte("tampered"), frames, hex.EncodeToString(root))
	if ok {
		t.Fatal("tampered tx should not verify")
	}
}

func TestVerifyInclusionProofMalformedFrame(t *testing.T) {
	frames := []MerkleProofFrame{{Hash: "nothex", Side: "right"}}
	_, err := VerifyInclusionProof([]byte("x"), frames, hex.EncodeToString(sh([]byte("root"))))
	if err == nil {
		t.Fatal("expected error on malformed frame")
	}
}

func TestVerifyInclusionProofRejectsBadSide(t *testing.T) {
	frames := []MerkleProofFrame{{Hash: hex.EncodeToString(sh([]byte("s"))), Side: "middle"}}
	_, err := VerifyInclusionProof([]byte("x"), frames, hex.EncodeToString(sh([]byte("r"))))
	if err == nil {
		t.Fatal("expected error on bad side")
	}
}

func TestVerifyInclusionProofEmptyTxErrors(t *testing.T) {
	_, err := VerifyInclusionProof(nil, nil, hex.EncodeToString(sh([]byte("r"))))
	if err == nil {
		t.Fatal("expected error on empty tx")
	}
}
