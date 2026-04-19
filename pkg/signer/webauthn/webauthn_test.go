package webauthn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Config{
		RPID:           "quidnug.example.com",
		RPName:         "Quidnug Test",
		Origin:         "https://quidnug.example.com",
		ChallengeTTL:   5 * time.Second,
		Store:          NewMemoryCredentialStore(),
		ChallengeStore: NewMemoryChallengeStore(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestRegistrationAndQuidIDDerivation(t *testing.T) {
	s := newTestServer(t)
	// 65-byte SEC1 uncompressed point: 0x04 || 32 bytes X || 32 bytes Y.
	pub := make([]byte, 65)
	pub[0] = 0x04
	for i := 1; i < 65; i++ {
		pub[i] = byte(i)
	}
	cred, err := s.Registration("laptop-yubikey", []byte("cred-id-1"), pub, 0)
	if err != nil {
		t.Fatalf("Registration: %v", err)
	}
	if len(cred.QuidID) != 16 {
		t.Fatalf("quid id length: %d", len(cred.QuidID))
	}
	if cred.Label != "laptop-yubikey" {
		t.Fatalf("label: %q", cred.Label)
	}
}

func TestBeginSigningProducesAllowList(t *testing.T) {
	s := newTestServer(t)
	pub := append([]byte{0x04}, bytes.Repeat([]byte{0x11}, 64)...)
	cred, err := s.Registration("t", []byte("id"), pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	req, err := s.BeginSigning(cred.QuidID, []byte(`{"tx":"fake"}`))
	if err != nil {
		t.Fatalf("BeginSigning: %v", err)
	}
	if req.RPID != "quidnug.example.com" {
		t.Fatalf("rpid: %s", req.RPID)
	}
	if len(req.AllowList) != 1 {
		t.Fatalf("allowList len %d", len(req.AllowList))
	}
	want := base64.RawURLEncoding.EncodeToString([]byte("id"))
	if req.AllowList[0] != want {
		t.Fatalf("allowList[0] = %s", req.AllowList[0])
	}
}

func TestFinishSigningValidatesOriginAndChallenge(t *testing.T) {
	s := newTestServer(t)
	pub := append([]byte{0x04}, bytes.Repeat([]byte{0x22}, 64)...)
	cred, err := s.Registration("t", []byte("cred-b"), pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	req, err := s.BeginSigning(cred.QuidID, []byte(`{"tx":"X"}`))
	if err != nil {
		t.Fatal(err)
	}

	clientDataStruct := map[string]string{
		"type":      "webauthn.get",
		"challenge": req.ChallengeB64,
		"origin":    "https://quidnug.example.com",
	}
	clientDataJSON, _ := json.Marshal(clientDataStruct)

	resp := AssertionResponse{
		CredentialIDB64:      base64.RawURLEncoding.EncodeToString([]byte("cred-b")),
		ClientDataJSONB64:    base64.RawURLEncoding.EncodeToString(clientDataJSON),
		AuthenticatorDataB64: base64.RawURLEncoding.EncodeToString([]byte("fake-authdata")),
		SignatureB64:         base64.RawURLEncoding.EncodeToString([]byte("fake-sig")),
	}

	got, signable, sig, err := s.FinishSigning(resp)
	if err != nil {
		t.Fatalf("FinishSigning: %v", err)
	}
	if got == nil || got.QuidID != cred.QuidID {
		t.Fatalf("credential mismatch")
	}
	if len(signable) != len("fake-authdata")+32 {
		t.Fatalf("signable length unexpected: %d", len(signable))
	}
	if string(sig) != "fake-sig" {
		t.Fatalf("sig: %q", sig)
	}
}

func TestFinishSigningRejectsOriginMismatch(t *testing.T) {
	s := newTestServer(t)
	pub := append([]byte{0x04}, bytes.Repeat([]byte{0x33}, 64)...)
	cred, _ := s.Registration("t", []byte("x"), pub, 0)
	req, _ := s.BeginSigning(cred.QuidID, []byte("x"))
	bad := map[string]string{
		"type":      "webauthn.get",
		"challenge": req.ChallengeB64,
		"origin":    "https://evil.example.com",
	}
	cd, _ := json.Marshal(bad)
	resp := AssertionResponse{
		CredentialIDB64:      base64.RawURLEncoding.EncodeToString([]byte("x")),
		ClientDataJSONB64:    base64.RawURLEncoding.EncodeToString(cd),
		AuthenticatorDataB64: base64.RawURLEncoding.EncodeToString([]byte("ad")),
		SignatureB64:         base64.RawURLEncoding.EncodeToString([]byte("s")),
	}
	_, _, _, err := s.FinishSigning(resp)
	if err == nil {
		t.Fatal("expected origin mismatch error")
	}
}

func TestChallengeOneShot(t *testing.T) {
	s := newTestServer(t)
	pub := append([]byte{0x04}, bytes.Repeat([]byte{0x44}, 64)...)
	cred, _ := s.Registration("t", []byte("cx"), pub, 0)
	req, _ := s.BeginSigning(cred.QuidID, []byte("x"))

	clientData := map[string]string{
		"type":      "webauthn.get",
		"challenge": req.ChallengeB64,
		"origin":    "https://quidnug.example.com",
	}
	cd, _ := json.Marshal(clientData)
	resp := AssertionResponse{
		CredentialIDB64:      base64.RawURLEncoding.EncodeToString([]byte("cx")),
		ClientDataJSONB64:    base64.RawURLEncoding.EncodeToString(cd),
		AuthenticatorDataB64: base64.RawURLEncoding.EncodeToString([]byte("ad")),
		SignatureB64:         base64.RawURLEncoding.EncodeToString([]byte("s")),
	}
	if _, _, _, err := s.FinishSigning(resp); err != nil {
		t.Fatalf("first finish: %v", err)
	}
	// Second attempt must fail — challenge should be consumed.
	_, _, _, err := s.FinishSigning(resp)
	if err == nil {
		t.Fatal("challenge should be one-shot")
	}
}
