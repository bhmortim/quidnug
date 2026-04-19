package client

import (
	"testing"
)

func TestGenerateQuidHasExpectedIDFormat(t *testing.T) {
	q, err := GenerateQuid()
	if err != nil {
		t.Fatalf("GenerateQuid: %v", err)
	}
	if len(q.ID) != 16 {
		t.Fatalf("id length: want 16, got %d", len(q.ID))
	}
	if !q.HasPrivateKey() {
		t.Fatal("quid should have private key")
	}
}

func TestQuidRoundtripViaPrivateHex(t *testing.T) {
	q, _ := GenerateQuid()
	r, err := QuidFromPrivateHex(q.PrivateKeyHex)
	if err != nil {
		t.Fatalf("QuidFromPrivateHex: %v", err)
	}
	if r.ID != q.ID || r.PublicKeyHex != q.PublicKeyHex {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", q, r)
	}
}

func TestSignVerifyRoundtrip(t *testing.T) {
	q, _ := GenerateQuid()
	sig, err := q.Sign([]byte("hello"))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !q.Verify([]byte("hello"), sig) {
		t.Fatal("self-verify should pass")
	}
	if q.Verify([]byte("tampered"), sig) {
		t.Fatal("tampered data should not verify")
	}
}

func TestReadOnlyQuidCannotSign(t *testing.T) {
	q, _ := GenerateQuid()
	ro, err := QuidFromPublicHex(q.PublicKeyHex)
	if err != nil {
		t.Fatalf("QuidFromPublicHex: %v", err)
	}
	if ro.HasPrivateKey() {
		t.Fatal("read-only quid should have no private key")
	}
	if _, err := ro.Sign([]byte("x")); err == nil {
		t.Fatal("Sign should fail on read-only quid")
	}
}

func TestCanonicalBytesIsStableAcrossInsertionOrder(t *testing.T) {
	a := map[string]any{"b": 1, "a": 2}
	b := map[string]any{"a": 2, "b": 1}
	ab, err := CanonicalBytes(a)
	if err != nil {
		t.Fatalf("CanonicalBytes(a): %v", err)
	}
	bb, err := CanonicalBytes(b)
	if err != nil {
		t.Fatalf("CanonicalBytes(b): %v", err)
	}
	if string(ab) != string(bb) {
		t.Fatalf("canonical bytes differ: %s vs %s", ab, bb)
	}
}

func TestCanonicalBytesExcludeFields(t *testing.T) {
	tx := map[string]any{"type": "TRUST", "signature": "xyz", "level": 0.9}
	b, err := CanonicalBytes(tx, "signature")
	if err != nil {
		t.Fatalf("CanonicalBytes: %v", err)
	}
	if want := `{"level":0.9,"type":"TRUST"}`; string(b) != want {
		t.Fatalf("excluded canonical bytes: got %s want %s", b, want)
	}
}
