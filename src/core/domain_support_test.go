package main

import (
	"os"
	"testing"
)

func TestMatchDomainPattern_ExactMatch(t *testing.T) {
	tests := []struct {
		domain   string
		pattern  string
		expected bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", "sub.example.com", true},
		{"example.com", "other.com", false},
		{"", "example.com", false},
		{"example.com", "", false},
	}

	for _, tt := range tests {
		result := MatchDomainPattern(tt.domain, tt.pattern)
		if result != tt.expected {
			t.Errorf("MatchDomainPattern(%q, %q) = %v, want %v",
				tt.domain, tt.pattern, result, tt.expected)
		}
	}
}

func TestMatchDomainPattern_WildcardSubdomain(t *testing.T) {
	tests := []struct {
		domain   string
		pattern  string
		expected bool
	}{
		{"sub.example.com", "*.example.com", true},
		{"deep.sub.example.com", "*.example.com", true},
		{"a.b.c.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false},
		{"notexample.com", "*.example.com", false},
		{"subexample.com", "*.example.com", false},
		{"sub.other.com", "*.example.com", false},
	}

	for _, tt := range tests {
		result := MatchDomainPattern(tt.domain, tt.pattern)
		if result != tt.expected {
			t.Errorf("MatchDomainPattern(%q, %q) = %v, want %v",
				tt.domain, tt.pattern, result, tt.expected)
		}
	}
}

func TestIsDomainSupported_EmptyList(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{},
	}

	if !node.IsDomainSupported("any.domain.com") {
		t.Error("Empty SupportedDomains should allow all domains")
	}

	if !node.IsDomainSupported("default") {
		t.Error("Empty SupportedDomains should allow 'default' domain")
	}
}

func TestIsDomainSupported_ExactMatches(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{"example.com", "other.org", "default"},
	}

	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"other.org", true},
		{"default", true},
		{"notlisted.com", false},
		{"sub.example.com", false},
	}

	for _, tt := range tests {
		result := node.IsDomainSupported(tt.domain)
		if result != tt.expected {
			t.Errorf("IsDomainSupported(%q) = %v, want %v",
				tt.domain, result, tt.expected)
		}
	}
}

func TestIsDomainSupported_WildcardPatterns(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{"*.example.com", "specific.org"},
	}

	tests := []struct {
		domain   string
		expected bool
	}{
		{"sub.example.com", true},
		{"deep.sub.example.com", true},
		{"example.com", false},
		{"specific.org", true},
		{"sub.specific.org", false},
		{"other.com", false},
	}

	for _, tt := range tests {
		result := node.IsDomainSupported(tt.domain)
		if result != tt.expected {
			t.Errorf("IsDomainSupported(%q) = %v, want %v",
				tt.domain, result, tt.expected)
		}
	}
}

func TestIsDomainSupported_MixedPatterns(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains: []string{"example.com", "*.example.com", "*.test.org"},
	}

	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"deep.sub.example.com", true},
		{"sub.test.org", true},
		{"test.org", false},
		{"other.com", false},
	}

	for _, tt := range tests {
		result := node.IsDomainSupported(tt.domain)
		if result != tt.expected {
			t.Errorf("IsDomainSupported(%q) = %v, want %v",
				tt.domain, result, tt.expected)
		}
	}
}

func TestLoadConfigSupportedDomainsFromEnv(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	os.Setenv("SUPPORTED_DOMAINS", `["example.com", "*.test.org"]`)

	cfg := LoadConfig()

	if len(cfg.SupportedDomains) != 2 {
		t.Fatalf("Expected 2 supported domains, got %d", len(cfg.SupportedDomains))
	}

	if cfg.SupportedDomains[0] != "example.com" {
		t.Errorf("Expected first domain to be 'example.com', got %q", cfg.SupportedDomains[0])
	}

	if cfg.SupportedDomains[1] != "*.test.org" {
		t.Errorf("Expected second domain to be '*.test.org', got %q", cfg.SupportedDomains[1])
	}
}

func TestLoadConfigAllowDomainRegistrationFromEnv(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	cfg := LoadConfig()
	if !cfg.AllowDomainRegistration {
		t.Error("Default AllowDomainRegistration should be true")
	}

	os.Setenv("ALLOW_DOMAIN_REGISTRATION", "false")
	cfg = LoadConfig()
	if cfg.AllowDomainRegistration {
		t.Error("AllowDomainRegistration should be false when env var is 'false'")
	}

	os.Setenv("ALLOW_DOMAIN_REGISTRATION", "true")
	cfg = LoadConfig()
	if !cfg.AllowDomainRegistration {
		t.Error("AllowDomainRegistration should be true when env var is 'true'")
	}
}

func TestRegisterTrustDomain_DomainNotSupported(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains:        []string{"allowed.com"},
		AllowDomainRegistration: true,
		TrustDomains:            make(map[string]TrustDomain),
		NodeID:                  "testnode123456",
	}

	err := node.RegisterTrustDomain(TrustDomain{Name: "notallowed.com"})
	if err == nil {
		t.Error("Expected error when registering unsupported domain")
	}
}

func TestRegisterTrustDomain_RegistrationNotAllowed(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains:        []string{},
		AllowDomainRegistration: false,
		TrustDomains:            make(map[string]TrustDomain),
		NodeID:                  "testnode123456",
	}

	err := node.RegisterTrustDomain(TrustDomain{Name: "any.com"})
	if err == nil {
		t.Error("Expected error when domain registration is not allowed")
	}
}

func TestRegisterTrustDomain_SupportedDomain(t *testing.T) {
	node := &QuidnugNode{
		SupportedDomains:        []string{"*.example.com"},
		AllowDomainRegistration: true,
		TrustDomains:            make(map[string]TrustDomain),
		NodeID:                  "testnode123456",
	}
	node.PublicKey = nil

	err := node.RegisterTrustDomain(TrustDomain{Name: "sub.example.com"})
	if err != nil {
		t.Errorf("Unexpected error registering supported domain: %v", err)
	}
}
