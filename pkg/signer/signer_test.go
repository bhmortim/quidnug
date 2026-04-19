package signer

import "testing"

type fakeSigner struct {
	pub, id string
	signed  []byte
	closed  bool
}

func (f *fakeSigner) PublicKeyHex() string                 { return f.pub }
func (f *fakeSigner) QuidID() string                       { return f.id }
func (f *fakeSigner) Sign(data []byte) (string, error)     { f.signed = data; return "sig", nil }
func (f *fakeSigner) Close() error                         { f.closed = true; return nil }

func TestSignableQuidDelegates(t *testing.T) {
	fs := &fakeSigner{pub: "04aa", id: "deadbeef"}
	q := NewSignableQuid(fs)

	if q.ID() != "deadbeef" {
		t.Fatalf("ID: got %q", q.ID())
	}
	if q.PublicKeyHex() != "04aa" {
		t.Fatalf("PublicKeyHex: got %q", q.PublicKeyHex())
	}
	if !q.HasPrivateKey() {
		t.Fatal("HasPrivateKey should be true")
	}
	sig, err := q.Sign([]byte("hello"))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if sig != "sig" {
		t.Fatalf("Sign: got %q", sig)
	}
	if string(fs.signed) != "hello" {
		t.Fatalf("underlying signer received: %q", fs.signed)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !fs.closed {
		t.Fatal("Close should have propagated")
	}
}
