package oidc

import (
	"errors"
	"sync"
	"time"
)

// Binding links an OIDC subject (issuer + sub claim) to a specific
// Quidnug quid. Immutable once created — subsequent logins for the
// same (issuer, sub) pair resolve to the same quid.
type Binding struct {
	Issuer    string    // OIDC issuer URL, e.g. https://auth.example.com
	Subject   string    // OIDC sub claim
	QuidID    string    // Quidnug quid ID (16 hex chars)
	Email     string    // latest email seen (for audit)
	Name      string    // latest name seen (for audit)
	CreatedAt time.Time // first-login timestamp
	UpdatedAt time.Time // last-login timestamp
}

// ErrBindingExists is returned when Create attempts to overwrite an
// existing binding for a subject. The call site should call Get first
// and mutate via Update instead.
var ErrBindingExists = errors.New("binding already exists")

// BindingStore persists IdP-subject-to-quid bindings.
type BindingStore interface {
	// Get returns the binding for (issuer, subject) or (nil, nil) when
	// absent.
	Get(issuer, subject string) (*Binding, error)
	// Create inserts a new binding. Returns ErrBindingExists if a
	// binding for (issuer, subject) already exists.
	Create(b Binding) error
	// Update mutates the audit fields (email, name, updated_at) for an
	// existing binding. No-op when no binding exists.
	Update(issuer, subject, email, name string) error
	// GetByQuid returns the binding for a specific quid, if any.
	GetByQuid(quidID string) (*Binding, error)
}

// MemoryBindingStore is an in-memory implementation suitable for
// tests and single-process dev. Not durable.
type MemoryBindingStore struct {
	mu    sync.RWMutex
	bySub map[string]*Binding
	byQid map[string]*Binding
}

// NewMemoryBindingStore returns an empty in-memory store.
func NewMemoryBindingStore() *MemoryBindingStore {
	return &MemoryBindingStore{
		bySub: make(map[string]*Binding),
		byQid: make(map[string]*Binding),
	}
}

func key(issuer, subject string) string {
	return issuer + "|" + subject
}

// Get returns the binding or nil.
func (s *MemoryBindingStore) Get(issuer, subject string) (*Binding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if b, ok := s.bySub[key(issuer, subject)]; ok {
		cp := *b
		return &cp, nil
	}
	return nil, nil
}

// GetByQuid returns the binding for a quid or nil.
func (s *MemoryBindingStore) GetByQuid(quidID string) (*Binding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if b, ok := s.byQid[quidID]; ok {
		cp := *b
		return &cp, nil
	}
	return nil, nil
}

// Create inserts a new binding.
func (s *MemoryBindingStore) Create(b Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(b.Issuer, b.Subject)
	if _, exists := s.bySub[k]; exists {
		return ErrBindingExists
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	b.UpdatedAt = b.CreatedAt
	s.bySub[k] = &b
	s.byQid[b.QuidID] = &b
	return nil
}

// Update mutates audit fields on the binding.
func (s *MemoryBindingStore) Update(issuer, subject, email, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.bySub[key(issuer, subject)]
	if !ok {
		return nil
	}
	if email != "" {
		b.Email = email
	}
	if name != "" {
		b.Name = name
	}
	b.UpdatedAt = time.Now()
	return nil
}
