package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestIsValidCID(t *testing.T) {
	tests := []struct {
		name     string
		cid      string
		expected bool
	}{
		// CIDv0 valid cases
		{
			name:     "valid CIDv0",
			cid:      "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			expected: true,
		},
		{
			name:     "valid CIDv0 another example",
			cid:      "QmT5NvUtoM5nWFfrQdVrFtvGfKFmG7AHE8P34isapyhCxX",
			expected: true,
		},

		// CIDv0 invalid cases
		{
			name:     "CIDv0 too short",
			cid:      "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbd",
			expected: false,
		},
		{
			name:     "CIDv0 too long",
			cid:      "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdGX",
			expected: false,
		},
		{
			name:     "CIDv0 invalid character (0)",
			cid:      "Qm0wAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			expected: false,
		},
		{
			name:     "CIDv0 invalid character (O)",
			cid:      "QmOwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			expected: false,
		},

		// CIDv1 valid cases
		{
			name:     "valid CIDv1 base32",
			cid:      "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			expected: true,
		},
		{
			name:     "valid CIDv1 longer",
			cid:      "bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku",
			expected: true,
		},

		// CIDv1 invalid cases
		{
			name:     "CIDv1 too short",
			cid:      "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55",
			expected: false,
		},
		{
			name:     "CIDv1 invalid character (1)",
			cid:      "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzd1",
			expected: false,
		},
		{
			name:     "CIDv1 uppercase (invalid)",
			cid:      "BAFYBEIGDYRZT5SFP7UDM7HU76UH7Y26NF3EFUYLQABF3OCLGTQY55FBZDI",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty string",
			cid:      "",
			expected: false,
		},
		{
			name:     "random string",
			cid:      "notacid",
			expected: false,
		},
		{
			name:     "starts with Qm but invalid",
			cid:      "QmInvalid",
			expected: false,
		},
		{
			name:     "starts with b but invalid",
			cid:      "binvalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidCID(tt.cid)
			if result != tt.expected {
				t.Errorf("IsValidCID(%q) = %v, expected %v", tt.cid, result, tt.expected)
			}
		})
	}
}

func TestNoOpIPFSClient_Pin(t *testing.T) {
	client := NewNoOpIPFSClient()
	ctx := context.Background()

	cid, err := client.Pin(ctx, []byte("test data"))

	if cid != "" {
		t.Errorf("NoOpIPFSClient.Pin() returned cid = %q, expected empty string", cid)
	}

	if !errors.Is(err, ErrIPFSNotConfigured) {
		t.Errorf("NoOpIPFSClient.Pin() error = %v, expected %v", err, ErrIPFSNotConfigured)
	}
}

func TestNoOpIPFSClient_Get(t *testing.T) {
	client := NewNoOpIPFSClient()
	ctx := context.Background()

	data, err := client.Get(ctx, "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")

	if data != nil {
		t.Errorf("NoOpIPFSClient.Get() returned data = %v, expected nil", data)
	}

	if !errors.Is(err, ErrIPFSNotConfigured) {
		t.Errorf("NoOpIPFSClient.Get() error = %v, expected %v", err, ErrIPFSNotConfigured)
	}
}

func TestNoOpIPFSClient_IsAvailable(t *testing.T) {
	client := NewNoOpIPFSClient()

	if client.IsAvailable() {
		t.Error("NoOpIPFSClient.IsAvailable() = true, expected false")
	}
}

func TestHTTPIPFSClient_Get_InvalidCID(t *testing.T) {
	client := NewHTTPIPFSClient("http://localhost:5001", &http.Client{})
	ctx := context.Background()

	_, err := client.Get(ctx, "invalid-cid")

	if !errors.Is(err, ErrInvalidCID) {
		t.Errorf("HTTPIPFSClient.Get() with invalid CID error = %v, expected %v", err, ErrInvalidCID)
	}
}

func TestHTTPIPFSClient_NilHttpClient(t *testing.T) {
	client := NewHTTPIPFSClient("http://localhost:5001", nil)
	ctx := context.Background()

	_, err := client.Pin(ctx, []byte("test"))
	if !errors.Is(err, ErrIPFSNotConfigured) {
		t.Errorf("HTTPIPFSClient.Pin() with nil client error = %v, expected %v", err, ErrIPFSNotConfigured)
	}

	_, err = client.Get(ctx, "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	if !errors.Is(err, ErrIPFSNotConfigured) {
		t.Errorf("HTTPIPFSClient.Get() with nil client error = %v, expected %v", err, ErrIPFSNotConfigured)
	}

	if client.IsAvailable() {
		t.Error("HTTPIPFSClient.IsAvailable() with nil client = true, expected false")
	}
}

func TestNewHTTPIPFSClient_URLNormalization(t *testing.T) {
	client := NewHTTPIPFSClient("http://localhost:5001/", nil)
	if client.gatewayURL != "http://localhost:5001" {
		t.Errorf("gatewayURL = %q, expected %q", client.gatewayURL, "http://localhost:5001")
	}

	client2 := NewHTTPIPFSClient("http://localhost:5001", nil)
	if client2.gatewayURL != "http://localhost:5001" {
		t.Errorf("gatewayURL = %q, expected %q", client2.gatewayURL, "http://localhost:5001")
	}
}

func TestHTTPIPFSClient_IsAvailable_EmptyURL(t *testing.T) {
	client := NewHTTPIPFSClient("", &http.Client{})

	if client.IsAvailable() {
		t.Error("HTTPIPFSClient.IsAvailable() with empty URL = true, expected false")
	}
}
