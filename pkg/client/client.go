package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
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
	jitter := time.Duration(rand.Int63n(100)) * time.Millisecond //nolint:gosec
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

// --- Identity ------------------------------------------------------------

// RegisterIdentity submits a signed IDENTITY transaction.
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
	tx := map[string]any{
		"type":          "IDENTITY",
		"timestamp":     time.Now().Unix(),
		"trustDomain":   domain,
		"signerQuid":    signer.ID,
		"definerQuid":   signer.ID,
		"subjectQuid":   subject,
		"updateNonce":   nonce,
		"schemaVersion": "1.0",
		"attributes":    p.Attributes,
	}
	if p.Name != "" {
		tx["name"] = p.Name
	}
	if p.Description != "" {
		tx["description"] = p.Description
	}
	if p.HomeDomain != "" {
		tx["homeDomain"] = p.HomeDomain
	}
	if err := signTx(signer, tx); err != nil {
		return nil, err
	}
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
	tx := map[string]any{
		"type":        "TRUST",
		"timestamp":   time.Now().Unix(),
		"trustDomain": domain,
		"signerQuid":  signer.ID,
		"truster":     signer.ID,
		"trustee":     p.Trustee,
		"trustLevel":  p.Level,
		"nonce":       nonce,
	}
	if p.ValidUntil != 0 {
		tx["validUntil"] = p.ValidUntil
	}
	if p.Description != "" {
		tx["description"] = p.Description
	}
	if err := signTx(signer, tx); err != nil {
		return nil, err
	}
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
	if diff := total - 100.0; diff > 0.001 || diff < -0.001 {
		return nil, newValidationError(fmt.Sprintf("owner percentages must sum to 100 (got %v)", total))
	}
	domain := p.Domain
	if domain == "" {
		domain = "default"
	}
	tx := map[string]any{
		"type":         "TITLE",
		"timestamp":    time.Now().Unix(),
		"trustDomain":  domain,
		"signerQuid":   signer.ID,
		"issuerQuid":   signer.ID,
		"assetQuid":    p.AssetID,
		"ownershipMap": p.Owners,
		"transferSigs": map[string]string{},
	}
	if p.TitleType != "" {
		tx["titleType"] = p.TitleType
	}
	if p.PrevTitleTxID != "" {
		tx["prevTitleTxID"] = p.PrevTitleTxID
	}
	if err := signTx(signer, tx); err != nil {
		return nil, err
	}
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "transactions/title", nil, tx, &out)
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
	tx := map[string]any{
		"type":        "EVENT",
		"timestamp":   time.Now().Unix(),
		"trustDomain": domain,
		"subjectId":   p.SubjectID,
		"subjectType": p.SubjectType,
		"eventType":   p.EventType,
		"sequence":    sequence,
	}
	if p.Payload != nil {
		tx["payload"] = p.Payload
	}
	if p.PayloadCID != "" {
		tx["payloadCid"] = p.PayloadCID
	}
	signable, err := CanonicalBytes(tx, "signature", "txId", "publicKey")
	if err != nil {
		return nil, err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return nil, err
	}
	tx["signature"] = sig
	tx["publicKey"] = signer.PublicKeyHex
	var out map[string]any
	return out, c.do(ctx, http.MethodPost, "events", nil, tx, &out)
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
