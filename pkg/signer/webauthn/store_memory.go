package webauthn

import (
	"bytes"
	"sync"
	"time"
)

// MemoryCredentialStore is an in-memory CredentialStore suitable for
// tests and small single-process deployments. Not safe across
// restarts — persist to a real DB for production.
type MemoryCredentialStore struct {
	mu    sync.RWMutex
	byID  map[string]Credential // key = hex(credentialID)
	byQID map[string]string     // quidID -> hex(credentialID)
}

// NewMemoryCredentialStore returns an empty in-memory store.
func NewMemoryCredentialStore() *MemoryCredentialStore {
	return &MemoryCredentialStore{
		byID:  make(map[string]Credential),
		byQID: make(map[string]string),
	}
}

func (m *MemoryCredentialStore) Save(c Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := hexEncode(c.CredentialID)
	m.byID[key] = c
	m.byQID[c.QuidID] = key
	return nil
}

func (m *MemoryCredentialStore) Get(credID []byte) (*Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.byID[hexEncode(credID)]; ok {
		return &c, nil
	}
	return nil, nil
}

func (m *MemoryCredentialStore) GetByQuidID(quidID string) (*Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key, ok := m.byQID[quidID]
	if !ok {
		return nil, nil
	}
	c := m.byID[key]
	return &c, nil
}

func (m *MemoryCredentialStore) UpdateSignCount(credID []byte, newCount uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := hexEncode(credID)
	c, ok := m.byID[key]
	if !ok {
		return nil
	}
	c.SignCount = newCount
	m.byID[key] = c
	return nil
}

// MemoryChallengeStore is an in-memory ChallengeStore with per-entry
// expiry. Challenges are consumed on Read to prevent replay.
type MemoryChallengeStore struct {
	mu sync.Mutex
	m  map[string]Challenge // key = hex(challenge)
}

// NewMemoryChallengeStore returns an empty challenge store.
func NewMemoryChallengeStore() *MemoryChallengeStore {
	return &MemoryChallengeStore{m: make(map[string]Challenge)}
}

func (s *MemoryChallengeStore) Put(challenge []byte, ch Challenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[hexEncode(challenge)] = ch
	return nil
}

func (s *MemoryChallengeStore) Consume(challenge []byte) (*Challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := hexEncode(challenge)
	ch, ok := s.m[key]
	if !ok {
		return nil, nil
	}
	delete(s.m, key)
	if time.Now().After(ch.ExpiresAt) {
		return nil, nil
	}
	return &ch, nil
}

func hexEncode(b []byte) string {
	const hexDigits = "0123456789abcdef"
	buf := make([]byte, len(b)*2)
	for i, x := range b {
		buf[i*2] = hexDigits[x>>4]
		buf[i*2+1] = hexDigits[x&0x0f]
	}
	return string(buf)
}

// bytesEqual is a shim in case we need it from other files.
var _ = bytes.Equal
