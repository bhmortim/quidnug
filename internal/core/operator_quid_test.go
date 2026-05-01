package core

import (
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quidnug/quidnug/pkg/client"
)

// writeQuidJSON drops a `.quid.json` file at p with the given
// fields. Used by the test cases below to exercise loadOperatorQuid.
func writeQuidJSON(t *testing.T, p string, id, pubHex, privHex string) {
	t.Helper()
	body := map[string]string{}
	if id != "" {
		body["id"] = id
	}
	if pubHex != "" {
		body["publicKeyHex"] = pubHex
	}
	if privHex != "" {
		body["privateKeyHex"] = privHex
	}
	raw, _ := json.Marshal(body)
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLoadOperatorQuid_FullKeypair: a public+private quid file
// loads, reports the private key, and the cross-check (id ==
// sha256(publicKey)[:16]) passes.
func TestLoadOperatorQuid_FullKeypair(t *testing.T) {
	q, err := client.GenerateQuid()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "op.quid.json")
	writeQuidJSON(t, p, q.ID, q.PublicKeyHex, q.PrivateKeyHex)

	got, err := loadOperatorQuid(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ID != q.ID {
		t.Fatalf("id mismatch: got %q want %q", got.ID, q.ID)
	}
	if got.PublicKeyHex != q.PublicKeyHex {
		t.Fatalf("publicKeyHex mismatch")
	}
	if got.PrivateKey == nil {
		t.Fatalf("expected private key to be loaded")
	}
	// The loaded private key should produce the same public key.
	pubBytes := elliptic.Marshal(got.PrivateKey.PublicKey.Curve,
		got.PrivateKey.PublicKey.X, got.PrivateKey.PublicKey.Y)
	if hex.EncodeToString(pubBytes) != q.PublicKeyHex {
		t.Fatalf("decoded private key does not match public key")
	}
}

// TestLoadOperatorQuid_PublicOnly: a quid file with only the
// public half loads cleanly with PrivateKey==nil. Useful for
// nodes that display the operator identity but keep the signing
// key offline.
func TestLoadOperatorQuid_PublicOnly(t *testing.T) {
	q, err := client.GenerateQuid()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "op.quid.json")
	writeQuidJSON(t, p, q.ID, q.PublicKeyHex, "")

	got, err := loadOperatorQuid(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.PrivateKey != nil {
		t.Fatalf("expected nil private key for public-only file")
	}
	if got.ID != q.ID {
		t.Fatalf("id mismatch")
	}
}

// TestLoadOperatorQuid_RejectsMismatchedID: if the file's
// declared id doesn't equal sha256(publicKey)[:16], loading must
// fail. This closes a confused-deputy hole where one operator's
// public key could be paired with another's id and pass casual
// inspection.
func TestLoadOperatorQuid_RejectsMismatchedID(t *testing.T) {
	q, err := client.GenerateQuid()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "op.quid.json")
	// Replace the id with a deliberately-wrong value.
	writeQuidJSON(t, p, "deadbeefcafef00d", q.PublicKeyHex, q.PrivateKeyHex)

	_, err = loadOperatorQuid(p)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected id-mismatch error, got %v", err)
	}
}

// TestLoadOperatorQuid_RejectsMismatchedPrivate: if the private
// scalar doesn't produce the supplied public key, loading must
// fail. Defends against accidentally pairing the wrong files.
func TestLoadOperatorQuid_RejectsMismatchedPrivate(t *testing.T) {
	a, err := client.GenerateQuid()
	if err != nil {
		t.Fatal(err)
	}
	b, err := client.GenerateQuid()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "op.quid.json")
	// a.PublicKeyHex paired with b.PrivateKeyHex.
	writeQuidJSON(t, p, a.ID, a.PublicKeyHex, b.PrivateKeyHex)

	_, err = loadOperatorQuid(p)
	if err == nil || !strings.Contains(err.Error(), "does not match publicKeyHex") {
		t.Fatalf("expected mismatched-private error, got %v", err)
	}
}

// TestLoadOperatorQuid_PathTraversal: path-traversal attempts
// must be refused at the safeio layer before any read happens.
func TestLoadOperatorQuid_PathTraversal(t *testing.T) {
	_, err := loadOperatorQuid("../../../../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "escapes working directory") {
		t.Fatalf("expected path-traversal rejection, got %v", err)
	}
}
