package client

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const sdkVersion = "2.0.0"

// Client is a strongly-typed HTTP client for a Quidnug node.
//
// Construct with New() or NewWithOptions(). All methods accept a
// context.Context for cancellation and deadlines. Safe for concurrent
// use by multiple goroutines.
type Client struct {
	baseURL    string
	apiBase    string
	http       *http.Client
	timeout    time.Duration
	maxRetries int
	retryBase  time.Duration
	authToken  string
	userAgent  string
}

// New constructs a client pointing at baseURL with sensible defaults.
func New(baseURL string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, newValidationError("baseURL is required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, newValidationError("baseURL must be an http(s) URL")
	}
	trimmed := strings.TrimRight(baseURL, "/")
	c := &Client{
		baseURL:    trimmed,
		apiBase:    trimmed + "/api",
		timeout:    30 * time.Second,
		maxRetries: 3,
		retryBase:  1 * time.Second,
		userAgent:  "quidnug-go-sdk/" + sdkVersion,
	}
	for _, o := range opts {
		o(c)
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: c.timeout}
	}
	return c, nil
}

// --- Request plumbing ----------------------------------------------------

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error,omitempty"`
}

func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
) error {
	attempts := 1
	retry := method == http.MethodGet
	if retry {
		attempts = c.maxRetries + 1
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		var bodyReader io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return newValidationError("marshal body: " + err.Error())
			}
			bodyReader = bytes.NewReader(b)
		}

		full := c.apiBase + "/" + strings.TrimLeft(path, "/")
		if len(query) > 0 {
			full += "?" + query.Encode()
		}

		req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
		if err != nil {
			return newValidationError("build request: " + err.Error())
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = newNodeError(fmt.Sprintf("%s %s: %v", method, path, err), 0, "")
			if attempt < attempts-1 {
				c.sleepBackoff(attempt, "")
				continue
			}
			return lastErr
		}

		if (resp.StatusCode >= 500 || resp.StatusCode == 429) && attempt < attempts-1 {
			retryAfter := resp.Header.Get("Retry-After")
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			c.sleepBackoff(attempt, retryAfter)
			continue
		}

		return c.parseResponse(method, path, resp, out)
	}
	if lastErr == nil {
		return newNodeError(method+" "+path+": retries exhausted", 0, "")
	}
	return lastErr
}

func (c *Client) sleepBackoff(attempt int, retryAfter string) {
	if retryAfter != "" {
		if secs, err := strconv.Atoi(retryAfter); err == nil && secs >= 0 {
			delay := time.Duration(secs) * time.Second
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}
			time.Sleep(delay)
			return
		}
	}
	// Retry jitter: 0-99ms drawn from crypto/rand. The math/rand
	// version triggered gosec G404 even with //nolint, and the
	// performance cost of crypto/rand for a 64-bit draw on a
	// retry path is negligible. If cryptorand fails (e.g.
	// /dev/urandom unreadable), fall through with zero jitter
	// rather than panicking; the worst case is thundering herd,
	// which the exponential backoff already mitigates.
	var jitter time.Duration
	if jitterN, err := cryptorand.Int(cryptorand.Reader, big.NewInt(100)); err == nil {
		jitter = time.Duration(jitterN.Int64()) * time.Millisecond
	}
	delay := c.retryBase*(1<<attempt) + jitter
	if delay > 60*time.Second {
		delay = 60 * time.Second
	}
	time.Sleep(delay)
}

func (c *Client) parseResponse(method, path string, resp *http.Response, out any) error {
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return newNodeError(
			fmt.Sprintf("%s %s: read body: %v", method, path, err),
			resp.StatusCode,
			"",
		)
	}

	var env envelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			return newNodeError(
				fmt.Sprintf("%s %s: non-JSON response (HTTP %d)", method, path, resp.StatusCode),
				resp.StatusCode,
				string(raw),
			)
		}
	}

	if env.Success {
		if out == nil {
			return nil
		}
		if len(env.Data) == 0 {
			return nil
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return newNodeError(
				fmt.Sprintf("%s %s: decode data: %v", method, path, err),
				resp.StatusCode,
				string(raw),
			)
		}
		return nil
	}

	code, message := "UNKNOWN_ERROR", fmt.Sprintf("HTTP %d", resp.StatusCode)
	var details map[string]any
	if env.Error != nil {
		code = env.Error.Code
		if env.Error.Message != "" {
			message = env.Error.Message
		}
		details = env.Error.Details
	}
	return serverErrorToSDKError(resp.StatusCode, code, message, details)
}

// --- Health / info -------------------------------------------------------

// Health checks node reachability and liveness.
func (c *Client) Health(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "health", nil, nil, &out)
}

// Info returns node identity, version, features, and managed domains.
func (c *Client) Info(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "info", nil, nil, &out)
}

// Nodes lists known peers. Pagination applies.
func (c *Client) Nodes(ctx context.Context, limit, offset int) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "nodes", paginationQuery(limit, offset), nil, &out)
}

// RawGet performs a GET to the supplied path (without /api
// prefix; client adds it) and returns the raw response body.
// Useful for ad-hoc CLI commands that don't yet have a typed
// wrapper. The returned bytes are the full JSON envelope —
// callers parse it themselves.
func (c *Client) RawGet(ctx context.Context, path string) ([]byte, error) {
	url := c.apiBase + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, fmt.Errorf("status %d", resp.StatusCode)
	}
	return body, nil
}

// --- Identity ------------------------------------------------------------

// RegisterIdentity submits a signed IDENTITY transaction.
//
// v1.0 conformant: builds a typed identityTxWire struct whose
// field order matches core.IdentityTransaction exactly, derives
// the ID via the same seed fields the server uses, signs with
// IEEE-1363, submits.
func (c *Client) RegisterIdentity(ctx context.Context, signer *Quid, p IdentityParams) (map[string]any, error) {
	if signer == nil || !signer.HasPrivateKey() {
		return nil, newValidationError("signer must have a private key")
	}
	subject := p.SubjectQuid
	if subject == "" {
		subject = signer.ID
	}
	domain := p.Domain
	if domain == "" {
		domain = "default"
	}
	nonce := p.UpdateNonce
	if nonce == 0 {
		nonce = 1
	}
	tx := identityTxWire{
		Type:        "IDENTITY",
		TrustDomain: domain,
		Timestamp:   time.Now().Unix(),
		Signature:   "",
		PublicKey:   signer.PublicKeyHex,
		QuidID:      subject,
		Name:        p.Name,
		Description: p.Description,
		Attributes:  p.Attributes,
		Creator:     signer.ID,
		UpdateNonce: nonce,
		HomeDomain:  p.HomeDomain,
	}
	tx.ID = deriveIdentityID(&tx)
	signable, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return nil, err
	}
	tx.Signature = sig
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "transactions/identity", nil, tx, &out)
}

// GetIdentity returns the current identity record or (nil, nil) on 404.
func (c *Client) GetIdentity(ctx context.Context, quidID, domain string) (*IdentityRecord, error) {
	if quidID == "" {
		return nil, newValidationError("quidID is required")
	}
	q := url.Values{}
	if domain != "" {
		q.Set("domain", domain)
	}
	var out IdentityRecord
	err := c.do(ctx, http.MethodGet, "identity/"+url.PathEscape(quidID), q, nil, &out)
	if isNotFound(err) {
		return nil, nil
	}
	return &out, err
}

// --- Trust ---------------------------------------------------------------

// GrantTrust submits a signed TRUST transaction from signer → trustee.
//
// v1.0 conformant. Uses typed trustTxWire + IEEE-1363 signature.
func (c *Client) GrantTrust(ctx context.Context, signer *Quid, p TrustParams) (map[string]any, error) {
	if signer == nil || !signer.HasPrivateKey() {
		return nil, newValidationError("signer must have a private key")
	}
	if p.Trustee == "" {
		return nil, newValidationError("trustee is required")
	}
	if p.Level < 0 || p.Level > 1 {
		return nil, newValidationError("level must be in [0, 1]")
	}
	domain := p.Domain
	if domain == "" {
		domain = "default"
	}
	nonce := p.Nonce
	if nonce == 0 {
		nonce = 1
	}
	tx := trustTxWire{
		Type:        "TRUST",
		TrustDomain: domain,
		Timestamp:   time.Now().Unix(),
		Signature:   "",
		PublicKey:   signer.PublicKeyHex,
		Truster:     signer.ID,
		Trustee:     p.Trustee,
		TrustLevel:  p.Level,
		Nonce:       nonce,
		Description: p.Description,
		ValidUntil:  p.ValidUntil,
	}
	tx.ID = deriveTrustID(&tx)
	signable, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return nil, err
	}
	tx.Signature = sig
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "transactions/trust", nil, tx, &out)
}

// GetTrust runs a relational-trust query from observer to target.
func (c *Client) GetTrust(ctx context.Context, observer, target, domain string, maxDepth int) (*TrustResult, error) {
	if observer == "" || target == "" {
		return nil, newValidationError("observer and target are required")
	}
	q := url.Values{}
	if domain != "" {
		q.Set("domain", domain)
	}
	if maxDepth > 0 {
		q.Set("maxDepth", strconv.Itoa(maxDepth))
	}
	var out TrustResult
	err := c.do(
		ctx, http.MethodGet,
		"trust/"+url.PathEscape(observer)+"/"+url.PathEscape(target),
		q, nil, &out,
	)
	return &out, err
}

// GetTrustEdges fetches a quid's direct outbound edges.
func (c *Client) GetTrustEdges(ctx context.Context, quidID string) ([]TrustEdge, error) {
	var wrapper struct {
		Edges []TrustEdge `json:"edges"`
		Data  []TrustEdge `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, "trust/edges/"+url.PathEscape(quidID), nil, nil, &wrapper)
	if err != nil {
		return nil, err
	}
	if len(wrapper.Edges) > 0 {
		return wrapper.Edges, nil
	}
	return wrapper.Data, nil
}

// --- Title ---------------------------------------------------------------

// RegisterTitle submits a signed TITLE transaction.
//
// v1.0 conformant. Uses typed titleTxWire + IEEE-1363 signature.
//
// NOTE: validates that owner percentages sum to 1.0 (the server
// checks for exact sum). Callers passing percentages on the
// 0..100 scale get a validation error pointing at the new
// 0..1 convention.
func (c *Client) RegisterTitle(ctx context.Context, signer *Quid, p TitleParams) (map[string]any, error) {
	if signer == nil || !signer.HasPrivateKey() {
		return nil, newValidationError("signer must have a private key")
	}
	if p.AssetID == "" {
		return nil, newValidationError("assetID is required")
	}
	if len(p.Owners) == 0 {
		return nil, newValidationError("owners is required")
	}
	total := 0.0
	for _, s := range p.Owners {
		total += s.Percentage
	}
	// Accept either 1.0 (fraction) or 100.0 (percent) for
	// caller ergonomics; normalize to fraction before wire.
	var ownersWire []ownershipStakeWire
	switch {
	case total > 0.999 && total < 1.001:
		// already fractional
		ownersWire = stakeToWire(p.Owners, 1.0)
	case total > 99.99 && total < 100.01:
		// normalize from percent to fraction
		ownersWire = stakeToWire(p.Owners, 0.01)
	default:
		return nil, newValidationError(
			fmt.Sprintf("owner percentages must sum to 1.0 (or 100.0 for percent); got %v", total))
	}
	domain := p.Domain
	if domain == "" {
		domain = "default"
	}
	tx := titleTxWire{
		Type:        "TITLE",
		TrustDomain: domain,
		Timestamp:   time.Now().Unix(),
		Signature:   "",
		PublicKey:   signer.PublicKeyHex,
		AssetID:     p.AssetID,
		Owners:      ownersWire,
		Signatures:  map[string]string{},
		TitleType:   p.TitleType,
	}
	// p.PrevTitleTxID has no on-wire counterpart in the v1.0
	// TitleTransaction struct; transfers use PreviousOwners.
	// We accept the field for caller backward-compat but omit
	// it from the signed bytes.
	_ = p.PrevTitleTxID
	tx.ID = deriveTitleID(&tx)
	signable, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return nil, err
	}
	tx.Signature = sig
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "transactions/title", nil, tx, &out)
}

// stakeToWire converts []OwnershipStake (public API) to the
// wire form, multiplying by normFactor to rescale to the
// server's 0..1 convention when callers pass percent.
func stakeToWire(in []OwnershipStake, normFactor float64) []ownershipStakeWire {
	out := make([]ownershipStakeWire, len(in))
	for i, s := range in {
		out[i] = ownershipStakeWire{
			OwnerID:    s.OwnerID,
			Percentage: s.Percentage * normFactor,
			StakeType:  s.StakeType,
		}
	}
	return out
}

// GetTitle returns current title state or (nil, nil) on 404.
func (c *Client) GetTitle(ctx context.Context, assetID, domain string) (*Title, error) {
	if assetID == "" {
		return nil, newValidationError("assetID is required")
	}
	q := url.Values{}
	if domain != "" {
		q.Set("domain", domain)
	}
	var out Title
	err := c.do(ctx, http.MethodGet, "title/"+url.PathEscape(assetID), q, nil, &out)
	if isNotFound(err) {
		return nil, nil
	}
	return &out, err
}

// --- Events --------------------------------------------------------------

// EmitEvent submits a signed EVENT transaction.
//
// v1.0 conformant. Uses typed eventTxWire + IEEE-1363 signature.
func (c *Client) EmitEvent(ctx context.Context, signer *Quid, p EventParams) (map[string]any, error) {
	if signer == nil || !signer.HasPrivateKey() {
		return nil, newValidationError("signer must have a private key")
	}
	if p.SubjectType != "QUID" && p.SubjectType != "TITLE" {
		return nil, newValidationError(`subjectType must be "QUID" or "TITLE"`)
	}
	if p.EventType == "" {
		return nil, newValidationError("eventType is required")
	}
	if (p.Payload == nil) == (p.PayloadCID == "") {
		return nil, newValidationError("exactly one of Payload or PayloadCID is required")
	}
	domain := p.Domain
	if domain == "" {
		domain = "default"
	}
	sequence := p.Sequence
	if sequence == 0 {
		stream, err := c.GetEventStream(ctx, p.SubjectID, domain)
		sequence = 1
		if err == nil && stream != nil {
			if latest, ok := stream["latestSequence"].(float64); ok {
				sequence = int64(latest) + 1
			}
		}
	}
	tx := eventTxWire{
		Type:        "EVENT",
		TrustDomain: domain,
		Timestamp:   time.Now().Unix(),
		Signature:   "",
		PublicKey:   signer.PublicKeyHex,
		SubjectID:   p.SubjectID,
		SubjectType: p.SubjectType,
		Sequence:    sequence,
		EventType:   p.EventType,
		Payload:     p.Payload,
		PayloadCID:  p.PayloadCID,
	}
	tx.ID = deriveEventID(&tx)
	signable, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return nil, err
	}
	tx.Signature = sig
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "events", nil, tx, &out)
}

// --- QDP-0014: Node advertisement -----------------------------------------

// nodeAdvertisementWire is the client-side mirror of
// core.NodeAdvertisementTransaction. Field order MUST match
// the server's struct declaration exactly — the server
// verifies signatures via json.Marshal on the typed struct,
// so the bytes need to be byte-identical.
//
// Do not reorder fields without also updating internal/core.
type nodeAdvertisementWire struct {
	// BaseTransaction fields (inlined for control over field order).
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`
	// NodeAdvertisementTransaction-specific fields.
	NodeQuid           string                 `json:"nodeQuid"`
	OperatorQuid       string                 `json:"operatorQuid"`
	Endpoints          []NodeAdvertEndpoint   `json:"endpoints"`
	SupportedDomains   []string               `json:"supportedDomains,omitempty"`
	Capabilities       NodeAdvertCapabilities `json:"capabilities"`
	ProtocolVersion    string                 `json:"protocolVersion"`
	ExpiresAt          int64                  `json:"expiresAt"`
	AdvertisementNonce int64                  `json:"advertisementNonce"`
}

// PublishNodeAdvertisement builds, signs, and submits a
// QDP-0014 NodeAdvertisementTransaction. The signer's
// keypair is the node's own; NodeQuid is derived from
// signer.ID. The OperatorQuid must have a current direct
// TRUST edge (weight ≥ 0.5) to the node, otherwise the
// node rejects the submission.
func (c *Client) PublishNodeAdvertisement(
	ctx context.Context, signer *Quid, p NodeAdvertisementParams,
) (map[string]any, error) {
	if signer == nil || !signer.HasPrivateKey() {
		return nil, newValidationError("signer must have a private key")
	}
	if p.OperatorQuid == "" {
		return nil, newValidationError("operatorQuid is required")
	}
	if len(p.Endpoints) == 0 {
		return nil, newValidationError("at least one endpoint is required")
	}
	if p.AdvertisementNonce <= 0 {
		return nil, newValidationError("advertisementNonce must be positive")
	}
	domain := p.Domain
	if domain == "" {
		return nil, newValidationError("domain is required (typically operators.network.<your-domain>)")
	}
	ttl := p.TTL
	if ttl == 0 {
		ttl = 6 * time.Hour
	}
	if ttl > 7*24*time.Hour {
		return nil, newValidationError("ttl must be <= 7 days")
	}
	protoVer := p.ProtocolVersion
	if protoVer == "" {
		protoVer = "1.0"
	}

	nowUnix := time.Now().Unix()
	expires := time.Now().Add(ttl).UnixNano()

	// Assign a random tx ID before signing so the submit path
	// doesn't re-hash (which would invalidate the signature).
	var idRaw [16]byte
	if _, err := cryptorand.Read(idRaw[:]); err != nil {
		return nil, err
	}
	idBytes := hex.EncodeToString(idRaw[:])

	tx := nodeAdvertisementWire{
		ID:                 idBytes,
		Type:               "NODE_ADVERTISEMENT",
		TrustDomain:        domain,
		Timestamp:          nowUnix,
		Signature:          "",
		PublicKey:          signer.PublicKeyHex,
		NodeQuid:           signer.ID,
		OperatorQuid:       p.OperatorQuid,
		Endpoints:          p.Endpoints,
		SupportedDomains:   p.SupportedDomains,
		Capabilities:       p.Capabilities,
		ProtocolVersion:    protoVer,
		ExpiresAt:          expires,
		AdvertisementNonce: p.AdvertisementNonce,
	}

	// Sign the canonical bytes (= json.Marshal of the struct
	// with Signature cleared). Since we're marshaling a struct,
	// field order is deterministic by declaration.
	signable, err := json.Marshal(tx)
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return nil, err
	}
	tx.Signature = sig

	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "node-advertisements", nil, tx, &out)
}

// --- QDP-0014: Discovery queries -------------------------------------------

// DiscoverDomain returns the current consortium, endpoint
// hints, and block tip for a domain.
func (c *Client) DiscoverDomain(ctx context.Context, domain string) (map[string]any, error) {
	if domain == "" {
		return nil, newValidationError("domain is required")
	}
	var out map[string]any
	err := c.do(ctx, http.MethodGet,
		"v2/discovery/domain/"+url.PathEscape(domain), nil, nil, &out)
	return out, err
}

// DiscoverNode returns the raw signed advertisement for a quid.
func (c *Client) DiscoverNode(ctx context.Context, quid string) (map[string]any, error) {
	if quid == "" {
		return nil, newValidationError("quid is required")
	}
	var out map[string]any
	err := c.do(ctx, http.MethodGet,
		"v2/discovery/node/"+url.PathEscape(quid), nil, nil, &out)
	return out, err
}

// DiscoverOperator lists all advertisements for a given
// operator quid.
func (c *Client) DiscoverOperator(ctx context.Context, operatorQuid string) (map[string]any, error) {
	if operatorQuid == "" {
		return nil, newValidationError("operatorQuid is required")
	}
	var out map[string]any
	err := c.do(ctx, http.MethodGet,
		"v2/discovery/operator/"+url.PathEscape(operatorQuid), nil, nil, &out)
	return out, err
}

// DiscoverQuidsParams is the argument struct for
// DiscoverQuids. All fields optional except Domain.
type DiscoverQuidsParams struct {
	Domain         string
	Since          int64   // UnixNano
	Sort           string  // "activity" | "last-seen" | "first-seen" | "trust-weight"
	Observer       string  // enables trust-weight sort and populates trustWeight
	EventType      string
	MinTrustWeight float64
	ExcludeQuids   []string
	Limit          int // default 50, max 500
	Offset         int
}

// DiscoverQuids queries the per-domain quid index.
func (c *Client) DiscoverQuids(ctx context.Context, p DiscoverQuidsParams) (map[string]any, error) {
	if p.Domain == "" {
		return nil, newValidationError("domain is required")
	}
	q := url.Values{}
	q.Set("domain", p.Domain)
	if p.Since > 0 {
		q.Set("since", strconv.FormatInt(p.Since, 10))
	}
	if p.Sort != "" {
		q.Set("sort", p.Sort)
	}
	if p.Observer != "" {
		q.Set("observer", p.Observer)
	}
	if p.EventType != "" {
		q.Set("eventType", p.EventType)
	}
	if p.MinTrustWeight > 0 {
		q.Set("min-trust-weight", strconv.FormatFloat(p.MinTrustWeight, 'f', -1, 64))
	}
	if len(p.ExcludeQuids) > 0 {
		q.Set("excludeQuid", strings.Join(p.ExcludeQuids, ","))
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		q.Set("offset", strconv.Itoa(p.Offset))
	}
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "v2/discovery/quids", q, nil, &out)
	return out, err
}

// DiscoverTrustedQuids returns quids the consortium members
// have directly TRUSTed above the given threshold.
func (c *Client) DiscoverTrustedQuids(
	ctx context.Context, domain string, minTrust float64, limit int,
) (map[string]any, error) {
	if domain == "" {
		return nil, newValidationError("domain is required")
	}
	q := url.Values{}
	q.Set("domain", domain)
	if minTrust > 0 {
		q.Set("min-trust", strconv.FormatFloat(minTrust, 'f', -1, 64))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "v2/discovery/trusted-quids", q, nil, &out)
	return out, err
}

// GetEventStream returns stream metadata or (nil, nil) on 404.
func (c *Client) GetEventStream(ctx context.Context, subjectID, domain string) (map[string]any, error) {
	q := url.Values{}
	if domain != "" {
		q.Set("domain", domain)
	}
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "streams/"+url.PathEscape(subjectID), q, nil, &out)
	if isNotFound(err) {
		return nil, nil
	}
	return out, err
}

// GetStreamEvents returns events and pagination metadata.
func (c *Client) GetStreamEvents(
	ctx context.Context,
	subjectID, domain string,
	limit, offset int,
) ([]Event, Pagination, error) {
	q := url.Values{}
	if domain != "" {
		q.Set("domain", domain)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var wrapper struct {
		Data       []Event    `json:"data"`
		Events     []Event    `json:"events"`
		Pagination Pagination `json:"pagination"`
	}
	err := c.do(ctx, http.MethodGet,
		"streams/"+url.PathEscape(subjectID)+"/events", q, nil, &wrapper)
	if err != nil {
		return nil, wrapper.Pagination, err
	}
	if len(wrapper.Data) > 0 {
		return wrapper.Data, wrapper.Pagination, nil
	}
	return wrapper.Events, wrapper.Pagination, nil
}

// --- Guardians (QDP-0002, QDP-0006) --------------------------------------

// SubmitGuardianSetUpdate installs or rotates a guardian set.
func (c *Client) SubmitGuardianSetUpdate(ctx context.Context, u GuardianSetUpdate) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "guardian/set-update", nil, u, &out)
}

// SubmitRecoveryInit starts the delayed recovery flow.
func (c *Client) SubmitRecoveryInit(ctx context.Context, i GuardianRecoveryInit) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "guardian/recovery/init", nil, i, &out)
}

// SubmitRecoveryVeto aborts an in-flight recovery.
func (c *Client) SubmitRecoveryVeto(ctx context.Context, v GuardianRecoveryVeto) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "guardian/recovery/veto", nil, v, &out)
}

// SubmitRecoveryCommit finalizes a recovery after the delay.
func (c *Client) SubmitRecoveryCommit(ctx context.Context, cm GuardianRecoveryCommit) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "guardian/recovery/commit", nil, cm, &out)
}

// SubmitGuardianResignation removes a guardian from the set.
func (c *Client) SubmitGuardianResignation(ctx context.Context, r GuardianResignation) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "guardian/resign", nil, r, &out)
}

// GetGuardianSet returns current guardians or (nil, nil) on 404.
func (c *Client) GetGuardianSet(ctx context.Context, quid string) (*GuardianSet, error) {
	var out GuardianSet
	err := c.do(ctx, http.MethodGet, "guardian/set/"+url.PathEscape(quid), nil, nil, &out)
	if isNotFound(err) {
		return nil, nil
	}
	return &out, err
}

// GetPendingRecovery returns the pending recovery record or nil.
func (c *Client) GetPendingRecovery(ctx context.Context, quid string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodGet, "guardian/pending-recovery/"+url.PathEscape(quid), nil, nil, &out)
	if isNotFound(err) {
		return nil, nil
	}
	return out, err
}

// --- Gossip / fingerprints (QDP-0003, QDP-0005) --------------------------

// SubmitDomainFingerprint publishes a signed fingerprint.
func (c *Client) SubmitDomainFingerprint(ctx context.Context, fp DomainFingerprint) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "domain-fingerprints", nil, fp, &out)
}

// GetLatestDomainFingerprint returns the newest known fingerprint for a domain.
func (c *Client) GetLatestDomainFingerprint(ctx context.Context, domain string) (*DomainFingerprint, error) {
	var out DomainFingerprint
	err := c.do(ctx, http.MethodGet,
		"domain-fingerprints/"+url.PathEscape(domain)+"/latest", nil, nil, &out)
	if isNotFound(err) {
		return nil, nil
	}
	return &out, err
}

// SubmitAnchorGossip delivers a cross-domain anchor message.
func (c *Client) SubmitAnchorGossip(ctx context.Context, m AnchorGossipMessage) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "anchor-gossip", nil, m, &out)
}

// PushAnchor is the push-mode variant of anchor gossip (QDP-0005).
func (c *Client) PushAnchor(ctx context.Context, m AnchorGossipMessage) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "gossip/push-anchor", nil, m, &out)
}

// PushFingerprint is the push-mode variant of fingerprint gossip.
func (c *Client) PushFingerprint(ctx context.Context, fp DomainFingerprint) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "gossip/push-fingerprint", nil, fp, &out)
}

// --- Bootstrap (QDP-0008) ------------------------------------------------

// SubmitNonceSnapshot publishes a K-of-K bootstrap snapshot.
func (c *Client) SubmitNonceSnapshot(ctx context.Context, s NonceSnapshot) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "nonce-snapshots", nil, s, &out)
}

// GetLatestNonceSnapshot returns the most recent snapshot for a domain.
func (c *Client) GetLatestNonceSnapshot(ctx context.Context, domain string) (*NonceSnapshot, error) {
	var out NonceSnapshot
	err := c.do(ctx, http.MethodGet,
		"nonce-snapshots/"+url.PathEscape(domain)+"/latest", nil, nil, &out)
	if isNotFound(err) {
		return nil, nil
	}
	return &out, err
}

// BootstrapStatus reports whether the node is still catching up.
func (c *Client) BootstrapStatus(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "bootstrap/status", nil, nil, &out)
}

// --- Fork-block (QDP-0009) -----------------------------------------------

// SubmitForkBlock submits a signed fork-activation block.
func (c *Client) SubmitForkBlock(ctx context.Context, f ForkBlock) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "fork-block", nil, f, &out)
}

// ForkBlockStatus reports activation status across features.
func (c *Client) ForkBlockStatus(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "fork-block/status", nil, nil, &out)
}

// --- Blocks, domains, transactions ---------------------------------------

// GetBlocks returns paginated blocks.
func (c *Client) GetBlocks(ctx context.Context, limit, offset int) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "blocks", paginationQuery(limit, offset), nil, &out)
}

// GetPendingTransactions returns paginated pending txs.
func (c *Client) GetPendingTransactions(ctx context.Context, limit, offset int) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "transactions", paginationQuery(limit, offset), nil, &out)
}

// ListDomains returns all known domains.
func (c *Client) ListDomains(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.do(ctx, http.MethodGet, "domains", nil, nil, &out)
}

// RegisterDomain submits a new trust domain to the node. Fails
// with an "already exists" error if the domain is already known;
// see EnsureDomain for an idempotent variant.
//
// Extra attributes beyond the domain name can be passed via the
// attrs map; they are merged into the POST body alongside
// {"name": domain}.
func (c *Client) RegisterDomain(ctx context.Context, domain string, attrs map[string]any) (map[string]any, error) {
	body := map[string]any{"name": domain}
	for k, v := range attrs {
		body[k] = v
	}
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "domains", nil, body, &out)
}

// EnsureDomain registers a trust domain if it does not already
// exist. It is idempotent — calling it twice is cheap and does
// not error.
//
// This is the recommended way for demo code and bootstrap scripts
// to guarantee a domain is registered before issuing identity,
// trust, title, or event transactions against it. Every
// non-default domain must be registered first; the node rejects
// any tx whose trust-domain is unknown.
func (c *Client) EnsureDomain(ctx context.Context, domain string, attrs map[string]any) (map[string]any, error) {
	out, err := c.RegisterDomain(ctx, domain, attrs)
	if err == nil {
		return out, nil
	}
	// Already-exists is the expected idempotent path.
	if strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return map[string]any{
			"status":  "success",
			"domain":  domain,
			"message": "trust domain already exists",
		}, nil
	}
	return nil, err
}

// WaitForIdentity blocks until the identity with the given quid
// ID is visible in the committed registry, or returns
// context.DeadlineExceeded / the passed ctx error on timeout.
//
// A just-submitted identity transaction lives in the node's
// pending pool until the next block is sealed. Code that
// immediately emits events or title transactions referencing the
// new quid must wait for commit first; this helper polls
// GetIdentity at the given interval.
//
// Pass a context with a deadline to bound the wait (recommended
// ~30 seconds for a dev node with short BLOCK_INTERVAL).
func (c *Client) WaitForIdentity(ctx context.Context, quidID, domain string, pollInterval time.Duration) (*IdentityRecord, error) {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	for {
		rec, err := c.GetIdentity(ctx, quidID, domain)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			return rec, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// WaitForIdentities blocks until every listed quid ID is
// committed, sharing one context deadline across all ids.
func (c *Client) WaitForIdentities(ctx context.Context, quidIDs []string, domain string, pollInterval time.Duration) error {
	for _, id := range quidIDs {
		if _, err := c.WaitForIdentity(ctx, id, domain, pollInterval); err != nil {
			return fmt.Errorf("wait for identity %s: %w", id, err)
		}
	}
	return nil
}

// WaitForTitle blocks until the title with the given asset ID is
// visible in the committed registry. Same rationale as
// WaitForIdentity: just-submitted title transactions live in the
// pending pool until the next block is sealed, and events on the
// title fail with "Subject TITLE not found" until commit.
func (c *Client) WaitForTitle(ctx context.Context, assetID, domain string, pollInterval time.Duration) (*Title, error) {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	for {
		t, err := c.GetTitle(ctx, assetID, domain)
		if err != nil {
			return nil, err
		}
		if t != nil {
			return t, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// --- Helpers -------------------------------------------------------------

func signTx(signer *Quid, tx map[string]any) error {
	signable, err := CanonicalBytes(tx, "signature", "txId")
	if err != nil {
		return err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return err
	}
	tx["signature"] = sig
	return nil
}

func paginationQuery(limit, offset int) url.Values {
	if limit <= 0 && offset <= 0 {
		return nil
	}
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	return q
}

func isNotFound(err error) bool {
	var ve *ValidationError
	if err == nil {
		return false
	}
	if asValidationError(err, &ve) {
		return ve.Code() == "NOT_FOUND"
	}
	return false
}

func asValidationError(err error, target **ValidationError) bool {
	var v *ValidationError
	if errorsAs(err, &v) {
		*target = v
		return true
	}
	return false
}

// Avoid the direct errors dependency loop by shimming.
func errorsAs(err error, target any) bool {
	if err == nil {
		return false
	}
	type asErr interface {
		As(any) bool
	}
	if ae, ok := err.(asErr); ok { //nolint:errorlint
		return ae.As(target)
	}
	// Fallback to the standard As; import here keeps the file single-dep.
	return errorsAsStd(err, target)
}
