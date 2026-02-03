package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Package-level errors for IPFS operations
var (
	ErrIPFSNotConfigured = errors.New("IPFS not configured")
	ErrInvalidCID        = errors.New("invalid CID format")
	ErrIPFSUnavailable   = errors.New("IPFS service unavailable")
)

// IPFSClient defines the interface for IPFS content addressing operations
type IPFSClient interface {
	// Pin stores content and returns its CID
	Pin(ctx context.Context, data []byte) (cid string, err error)
	// Get retrieves content by its CID
	Get(ctx context.Context, cid string) (data []byte, err error)
	// IsAvailable checks if IPFS is configured and reachable
	IsAvailable() bool
}

// HTTPIPFSClient implements IPFSClient using the IPFS HTTP API (Kubo/go-ipfs compatible)
type HTTPIPFSClient struct {
	gatewayURL string
	httpClient *http.Client
}

// NewHTTPIPFSClient creates a new HTTP-based IPFS client
func NewHTTPIPFSClient(gatewayURL string, httpClient *http.Client) *HTTPIPFSClient {
	gatewayURL = strings.TrimSuffix(gatewayURL, "/")

	return &HTTPIPFSClient{
		gatewayURL: gatewayURL,
		httpClient: httpClient,
	}
}

// ipfsAddResponse represents the JSON response from /api/v0/add
type ipfsAddResponse struct {
	Name string `json:"Name"`
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

// Pin stores content in IPFS and returns its CID
func (c *HTTPIPFSClient) Pin(ctx context.Context, data []byte) (string, error) {
	if c.httpClient == nil {
		return "", ErrIPFSNotConfigured
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "data")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("failed to write data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	reqURL := c.gatewayURL + "/api/v0/add"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrIPFSUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: status %d: %s", ErrIPFSUnavailable, resp.StatusCode, string(body))
	}

	var addResp ipfsAddResponse
	if err := json.NewDecoder(resp.Body).Decode(&addResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if addResp.Hash == "" {
		return "", fmt.Errorf("IPFS returned empty CID")
	}

	return addResp.Hash, nil
}

// Get retrieves content from IPFS by CID
func (c *HTTPIPFSClient) Get(ctx context.Context, cid string) ([]byte, error) {
	if c.httpClient == nil {
		return nil, ErrIPFSNotConfigured
	}

	if !IsValidCID(cid) {
		return nil, ErrInvalidCID
	}

	reqURL := c.gatewayURL + "/api/v0/cat?arg=" + url.QueryEscape(cid)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIPFSUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrIPFSUnavailable, resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// IsAvailable checks if the IPFS service is configured and reachable
func (c *HTTPIPFSClient) IsAvailable() bool {
	if c.httpClient == nil || c.gatewayURL == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.httpClient.Timeout)
	defer cancel()

	reqURL := c.gatewayURL + "/api/v0/id"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// NoOpIPFSClient implements IPFSClient for when IPFS is disabled
type NoOpIPFSClient struct{}

// NewNoOpIPFSClient creates a no-op IPFS client
func NewNoOpIPFSClient() *NoOpIPFSClient {
	return &NoOpIPFSClient{}
}

// Pin returns an error indicating IPFS is not configured
func (c *NoOpIPFSClient) Pin(ctx context.Context, data []byte) (string, error) {
	return "", ErrIPFSNotConfigured
}

// Get returns an error indicating IPFS is not configured
func (c *NoOpIPFSClient) Get(ctx context.Context, cid string) ([]byte, error) {
	return nil, ErrIPFSNotConfigured
}

// IsAvailable always returns false for the no-op client
func (c *NoOpIPFSClient) IsAvailable() bool {
	return false
}

// CID validation patterns
var (
	// CIDv0: starts with "Qm", 46 characters total, base58btc encoded
	// Base58btc alphabet excludes 0, O, I, l to avoid visual ambiguity
	cidV0Regex = regexp.MustCompile(`^Qm[1-9A-HJ-NP-Za-km-z]{44}$`)
	// CIDv1: starts with "b" (base32 lowercase multibase prefix)
	// Uses RFC 4648 base32 alphabet: a-z2-7
	// Standard CIDv1 with SHA-256 is 59 characters total (b + 58 chars)
	cidV1Regex = regexp.MustCompile(`^b[a-z2-7]{58,}$`)
)

// IsValidCID validates a CID string for both CIDv0 and CIDv1 formats
func IsValidCID(cid string) bool {
	if cid == "" {
		return false
	}

	// CIDv0 format: starts with "Qm", 46 chars, base58btc
	if strings.HasPrefix(cid, "Qm") {
		return cidV0Regex.MatchString(cid)
	}

	// CIDv1 format: starts with "b", base32 lowercase
	if strings.HasPrefix(cid, "b") {
		return cidV1Regex.MatchString(cid)
	}

	return false
}
