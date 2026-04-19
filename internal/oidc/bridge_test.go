package oidc

import (
	"context"
	"testing"
)

type fakeBoundQuid struct {
	id, pub string
}

func (f *fakeBoundQuid) ID() string                    { return f.id }
func (f *fakeBoundQuid) PublicKeyHex() string          { return f.pub }
func (f *fakeBoundQuid) Sign(_ []byte) (string, error) { return "sig-" + f.id, nil }

type fakeFactory struct {
	counter int
	known   map[string]*fakeBoundQuid
}

func (f *fakeFactory) CreateNew(ctx context.Context, issuer, subject, email string) (BoundQuid, error) {
	f.counter++
	id := make([]byte, 0, 16)
	for i := 0; i < 16; i++ {
		id = append(id, "0123456789abcdef"[(f.counter+i)%16])
	}
	q := &fakeBoundQuid{id: string(id), pub: "04" + string(id)}
	if f.known == nil {
		f.known = map[string]*fakeBoundQuid{}
	}
	f.known[q.id] = q
	return q, nil
}

func (f *fakeFactory) Get(ctx context.Context, quidID string) (BoundQuid, error) {
	return f.known[quidID], nil
}

func TestResolveBindsOncePerSubject(t *testing.T) {
	store := NewMemoryBindingStore()
	fac := &fakeFactory{}
	b, err := New(Options{Store: store, Signer: fac})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	tok := IDToken{
		Issuer:  "https://issuer.example.com",
		Subject: "user-123",
		Email:   "alice@example.com",
		Name:    "Alice",
	}
	bnd1, s1, err := b.Resolve(ctx, tok)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if bnd1.QuidID == "" || s1.ID() == "" {
		t.Fatal("expected quid to be minted")
	}
	if fac.counter != 1 {
		t.Fatalf("factory counter: want 1, got %d", fac.counter)
	}

	bnd2, s2, err := b.Resolve(ctx, tok)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if bnd2.QuidID != bnd1.QuidID || s2.ID() != s1.ID() {
		t.Fatal("subsequent login must resolve to same quid")
	}
	if fac.counter != 1 {
		t.Fatalf("factory shouldn't provision a second quid; counter %d", fac.counter)
	}
}

func TestResolveRequiresIssuerAndSubject(t *testing.T) {
	b, _ := New(Options{Store: NewMemoryBindingStore(), Signer: &fakeFactory{}})
	_, _, err := b.Resolve(context.Background(), IDToken{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryBindingStoreRoundtrip(t *testing.T) {
	s := NewMemoryBindingStore()
	err := s.Create(Binding{
		Issuer: "a", Subject: "b", QuidID: "abcd1234abcd1234", Email: "e", Name: "n",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Get("a", "b")
	if b == nil || b.QuidID != "abcd1234abcd1234" {
		t.Fatal("Get missed")
	}
	b2, _ := s.GetByQuid("abcd1234abcd1234")
	if b2 == nil || b2.Subject != "b" {
		t.Fatal("GetByQuid missed")
	}
	if err := s.Create(Binding{Issuer: "a", Subject: "b", QuidID: "other"}); err == nil {
		t.Fatal("duplicate Create must fail")
	}
	_ = s.Update("a", "b", "new-email", "new-name")
	b3, _ := s.Get("a", "b")
	if b3.Email != "new-email" || b3.Name != "new-name" {
		t.Fatalf("update did not take: %+v", b3)
	}
}
