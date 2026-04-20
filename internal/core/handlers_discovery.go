// Package core — QDP-0014 discovery HTTP handlers.
//
// Five endpoints, all under /api/v2/discovery/*:
//
//   GET /domain/{name}          current consortium + endpoints + block tip
//   GET /node/{quid}            raw node advertisement
//   GET /operator/{quid}        all advertisements by an operator
//   GET /quids                  per-domain quid index with filters + sort
//   GET /trusted-quids          quids the consortium has directly TRUST-ed
//
// All are read-only and CDN-cacheable; see the `sharding-model.md`
// for the client-side discovery flow.
package core

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// RegisterDiscoveryRoutes mounts the five QDP-0014 endpoints
// onto the v2 subrouter. Called from StartServerWithConfig's
// existing v2 wiring.
func (node *QuidnugNode) RegisterDiscoveryRoutes(r *mux.Router) {
	r.HandleFunc("/discovery/domain/{name}", node.DiscoveryDomainHandler).Methods("GET")
	r.HandleFunc("/discovery/node/{quid}", node.DiscoveryNodeHandler).Methods("GET")
	r.HandleFunc("/discovery/operator/{quid}", node.DiscoveryOperatorHandler).Methods("GET")
	r.HandleFunc("/discovery/quids", node.DiscoveryQuidsHandler).Methods("GET")
	r.HandleFunc("/discovery/trusted-quids", node.DiscoveryTrustedQuidsHandler).Methods("GET")
}

// DiscoveryDomainHandler returns the domain's current consortium,
// endpoint hints (derived from live advertisements), and chain
// tip. This is the main client-side entry point for "where do I
// send my query for this domain?"
func (node *QuidnugNode) DiscoveryDomainHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if name == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "domain name required")
		return
	}

	node.TrustDomainsMutex.RLock()
	d, ok := node.TrustDomains[name]
	node.TrustDomainsMutex.RUnlock()
	if !ok {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "domain not registered")
		return
	}

	// Build endpoint hints from live advertisements. We include
	// advertisements whose NodeQuid is in the consortium (so
	// reads hit validators), plus any advertisement whose
	// SupportedDomains glob matches (cache replicas).
	hints := node.buildEndpointHints(name, d)

	// Chain tip for this domain (if any blocks exist).
	tipIndex, tipHash, tipTimestamp := node.chainTipForDomain(name)

	WriteSuccess(w, map[string]interface{}{
		"domain":     name,
		"blockTip":   map[string]interface{}{"index": tipIndex, "hash": tipHash, "timestamp": tipTimestamp},
		"consortium": map[string]interface{}{"validators": d.Validators, "threshold": d.TrustThreshold},
		"endpoints":  hints,
	})
}

// DiscoveryNodeHandler returns the raw signed advertisement
// for a given node quid. Used for client-side verification —
// the domain handler's hints are a convenience; the raw
// advertisement is the ground truth.
func (node *QuidnugNode) DiscoveryNodeHandler(w http.ResponseWriter, r *http.Request) {
	quid := mux.Vars(r)["quid"]
	if quid == "" || !IsValidQuidID(quid) {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "valid 16-hex quid required")
		return
	}
	if node.NodeAdvertisementRegistry == nil {
		WriteError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "discovery not initialized")
		return
	}
	ad, ok := node.NodeAdvertisementRegistry.Get(quid)
	if !ok {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "no current advertisement for this quid")
		return
	}
	WriteSuccess(w, ad)
}

// DiscoveryOperatorHandler returns every advertisement the
// registry holds whose OperatorQuid matches the path param.
// The caller can then enumerate all nodes an operator runs
// without knowing each node quid in advance.
func (node *QuidnugNode) DiscoveryOperatorHandler(w http.ResponseWriter, r *http.Request) {
	quid := mux.Vars(r)["quid"]
	if quid == "" || !IsValidQuidID(quid) {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "valid 16-hex quid required")
		return
	}
	if node.NodeAdvertisementRegistry == nil {
		WriteError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "discovery not initialized")
		return
	}
	ads := node.NodeAdvertisementRegistry.ListByOperator(quid)
	WriteSuccess(w, map[string]interface{}{
		"operatorQuid": quid,
		"nodes":        ads,
	})
}

// DiscoveryQuidsHandler returns the per-domain quid index,
// filtered + sorted per query params:
//
//   ?domain=<name>                (required)
//   ?since=<unixNano>             (optional; filter recent)
//   ?sort=<mode>                  (optional; "activity" | "last-seen" |
//                                  "first-seen" | "trust-weight")
//   ?observer=<quid>              (optional; enables trust-weight sort +
//                                  populates trustWeight field in response)
//   ?eventType=<type>             (optional; filter to signers of that event type)
//   ?min-trust-weight=<0..1>      (optional; requires observer)
//   ?excludeQuid=a,b,c            (optional; comma-separated)
//   ?limit=<int>                  (default 50, max 500)
//   ?offset=<int>                 (pagination)
func (node *QuidnugNode) DiscoveryQuidsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	domain := q.Get("domain")
	if domain == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "domain is required")
		return
	}
	if node.QuidDomainIndex == nil {
		WriteError(w, http.StatusServiceUnavailable, "UNAVAILABLE", "quid index not initialized")
		return
	}

	since := parseInt64(q.Get("since"), 0)
	observer := q.Get("observer")
	sortMode := q.Get("sort")
	eventType := q.Get("eventType")
	minTrust := parseFloat64(q.Get("min-trust-weight"), 0)

	var excluded []string
	if raw := q.Get("excludeQuid"); raw != "" {
		excluded = strings.Split(raw, ",")
	}

	limit := parseInt(q.Get("limit"), 50)
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := parseInt(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	entries := node.QuidDomainIndex.ListByDomain(domain, since)
	entries = filterByEventType(entries, eventType)
	entries = excludeQuids(entries, excluded)

	// Trust-weight augmentation (for sort + filter + response body).
	// Only computed if the observer query param is set.
	var trustWeights map[string]float64
	if observer != "" && IsValidQuidID(observer) {
		trustWeights = make(map[string]float64, len(entries))
		for _, s := range entries {
			level, _, err := node.ComputeRelationalTrust(observer, s.QuidID, DefaultTrustMaxDepth)
			if err == nil && level > 0 {
				trustWeights[s.QuidID] = level
			}
		}
		if minTrust > 0 {
			entries = filterByMinTrust(entries, trustWeights, minTrust)
		}
	}

	SortQuidStats(entries, sortMode, trustWeights)

	total := len(entries)
	end := offset + limit
	if end > total {
		end = total
	}
	pageStart := offset
	if pageStart > total {
		pageStart = total
	}
	page := entries[pageStart:end]

	// Shape response. Include trust weights in rendered output
	// when observer was provided so clients don't need a second round trip.
	rendered := make([]map[string]interface{}, 0, len(page))
	for _, s := range page {
		row := map[string]interface{}{
			"quidId":        s.QuidID,
			"firstSeen":     s.FirstSeen,
			"lastSeen":      s.LastSeen,
			"txCount":       s.TxCount,
			"trustEdgesIn":  s.TrustEdgesIn,
			"trustEdgesOut": s.TrustEdgesOut,
		}
		if s.EventTypeCounts != nil {
			row["eventTypeCounts"] = s.EventTypeCounts
		}
		if trustWeights != nil {
			row["trustWeight"] = trustWeights[s.QuidID]
		}
		rendered = append(rendered, row)
	}

	WriteSuccess(w, map[string]interface{}{
		"domain": domain,
		"quids":  rendered,
		"pagination": PaginationMeta{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	})
}

// DiscoveryTrustedQuidsHandler returns the strict "who does
// the consortium vouch for" view: quids that at least one
// consortium member has directly TRUSTed at or above the
// given min-trust level.
//
//   ?domain=<name>      (required)
//   ?min-trust=<0..1>   (default 0.5)
//   ?limit=<int>        (default 200, max 1000)
func (node *QuidnugNode) DiscoveryTrustedQuidsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	domain := q.Get("domain")
	if domain == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "domain is required")
		return
	}
	minTrust := parseFloat64(q.Get("min-trust"), 0.5)
	limit := parseInt(q.Get("limit"), 200)
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	trusted := node.consortiumTrustedQuids(domain, minTrust)
	if trusted == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "domain not registered")
		return
	}

	// Sort by weight descending, tiebreak by quid for stability.
	type entry struct {
		QuidID string  `json:"quidId"`
		Weight float64 `json:"weight"`
	}
	rows := make([]entry, 0, len(trusted))
	for quid, w := range trusted {
		rows = append(rows, entry{QuidID: quid, Weight: w})
	}
	// Simple in-place sort without pulling in extras.
	for i := 1; i < len(rows); i++ {
		j := i
		for j > 0 && (rows[j-1].Weight < rows[j].Weight ||
			(rows[j-1].Weight == rows[j].Weight && rows[j-1].QuidID > rows[j].QuidID)) {
			rows[j-1], rows[j] = rows[j], rows[j-1]
			j--
		}
	}

	if len(rows) > limit {
		rows = rows[:limit]
	}

	WriteSuccess(w, map[string]interface{}{
		"domain":   domain,
		"minTrust": minTrust,
		"trusted":  rows,
	})
}

// ----- helpers -----

// buildEndpointHints assembles the endpoint-hint list for the
// domain-discovery response. Returns a slice of {nodeQuid,
// endpoints, capabilities} records, drawing from:
//
//  1. Advertisements whose NodeQuid is a consortium member
//     (priority).
//  2. Advertisements whose SupportedDomains glob matches.
//
// Deduplication: each NodeQuid appears at most once; consortium
// membership wins on collision.
func (node *QuidnugNode) buildEndpointHints(domain string, d TrustDomain) []map[string]interface{} {
	if node.NodeAdvertisementRegistry == nil {
		return nil
	}

	seen := make(map[string]bool)
	hints := make([]map[string]interface{}, 0)

	// Consortium members first (they're the validators).
	for quid := range d.Validators {
		if ad, ok := node.NodeAdvertisementRegistry.Get(quid); ok {
			hints = append(hints, adToHint(ad))
			seen[quid] = true
		}
	}

	// Supporting cache replicas.
	for _, ad := range node.NodeAdvertisementRegistry.ListForDomain(domain) {
		if seen[ad.NodeQuid] {
			continue
		}
		hints = append(hints, adToHint(ad))
		seen[ad.NodeQuid] = true
	}
	return hints
}

func adToHint(ad NodeAdvertisementTransaction) map[string]interface{} {
	return map[string]interface{}{
		"nodeQuid":     ad.NodeQuid,
		"operatorQuid": ad.OperatorQuid,
		"endpoints":    ad.Endpoints,
		"capabilities": ad.Capabilities,
		"expiresAt":    ad.ExpiresAt,
	}
}

// chainTipForDomain looks up the current chain tip metadata
// for a domain by walking the local blockchain from the tail
// inward. Returns (index, hash, timestamp) of the latest block
// whose TrustProof domain matches, or zero values if none.
func (node *QuidnugNode) chainTipForDomain(domain string) (int64, string, int64) {
	node.BlockchainMutex.RLock()
	defer node.BlockchainMutex.RUnlock()
	for i := len(node.Blockchain) - 1; i >= 0; i-- {
		b := node.Blockchain[i]
		if b.TrustProof.TrustDomain == domain {
			return b.Index, b.Hash, b.Timestamp
		}
	}
	return 0, "", 0
}

// parseInt64 is a tiny helper missing from the existing
// handlers.go surface.
func parseInt64(s string, def int64) int64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseFloat64(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}
