// handlers_dns_attestation.go — QDP-0023 Phase 1 HTTP
// endpoints.
//
// Phase 1 surface (docs/design/0023-dns-anchored-attestation.md
// §11.1):
//
//   GET  /api/v2/dns/attestations/{domain}
//   GET  /api/v2/dns/attestations/{domain}/weighted?observer=<quid>
//   GET  /api/v2/dns/resolve/{domain}/{recordType}
//
// Plus submission endpoints for the tx types (claim, challenge,
// attestation, renewal, revocation, delegate, delegate-revoke).
//
// Authority delegation resolution (the /resolve endpoint) is
// Phase 1: it looks up the active delegation for a domain
// and returns the delegate pointer + visibility policy. The
// actual trust-gated / private record fetch from the
// delegated domain is left to the resolver client — the node
// simply exposes the pointer.

package core

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// RegisterDNSAttestationRoutes mounts the DNS attestation
// routes under the given /api/v2 subrouter.
func (node *QuidnugNode) RegisterDNSAttestationRoutes(router *mux.Router) {
	router.HandleFunc("/dns/claim",
		node.SubmitDNSClaimHandler).Methods("POST")
	router.HandleFunc("/dns/challenge",
		node.SubmitDNSChallengeHandler).Methods("POST")
	router.HandleFunc("/dns/attestation",
		node.SubmitDNSAttestationHandler).Methods("POST")
	router.HandleFunc("/dns/renewal",
		node.SubmitDNSRenewalHandler).Methods("POST")
	router.HandleFunc("/dns/revocation",
		node.SubmitDNSRevocationHandler).Methods("POST")
	router.HandleFunc("/dns/delegate",
		node.SubmitAuthorityDelegateHandler).Methods("POST")
	router.HandleFunc("/dns/delegate-revocation",
		node.SubmitAuthorityDelegateRevocationHandler).Methods("POST")

	router.HandleFunc("/dns/attestations/{domain}",
		node.GetDNSAttestationsHandler).Methods("GET")
	router.HandleFunc("/dns/attestations/{domain}/weighted",
		node.GetDNSAttestationsWeightedHandler).Methods("GET")
	router.HandleFunc("/dns/resolve/{domain}/{recordType}",
		node.ResolveDNSRecordHandler).Methods("GET")
}

// --- Submission handlers ---

func (node *QuidnugNode) SubmitDNSClaimHandler(w http.ResponseWriter, r *http.Request) {
	var tx DNSClaimTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	id, err := node.AddDNSClaimTransaction(tx)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TX", err.Error())
		return
	}
	writeJSON(w, map[string]any{"txId": id})
}

func (node *QuidnugNode) SubmitDNSChallengeHandler(w http.ResponseWriter, r *http.Request) {
	var tx DNSChallengeTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	id, err := node.AddDNSChallengeTransaction(tx)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TX", err.Error())
		return
	}
	writeJSON(w, map[string]any{"txId": id})
}

func (node *QuidnugNode) SubmitDNSAttestationHandler(w http.ResponseWriter, r *http.Request) {
	var tx DNSAttestationTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	id, err := node.AddDNSAttestationTransaction(tx)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TX", err.Error())
		return
	}
	writeJSON(w, map[string]any{"txId": id})
}

func (node *QuidnugNode) SubmitDNSRenewalHandler(w http.ResponseWriter, r *http.Request) {
	var tx DNSRenewalTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	id, err := node.AddDNSRenewalTransaction(tx)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TX", err.Error())
		return
	}
	writeJSON(w, map[string]any{"txId": id})
}

func (node *QuidnugNode) SubmitDNSRevocationHandler(w http.ResponseWriter, r *http.Request) {
	var tx DNSRevocationTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	id, err := node.AddDNSRevocationTransaction(tx)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TX", err.Error())
		return
	}
	writeJSON(w, map[string]any{"txId": id})
}

func (node *QuidnugNode) SubmitAuthorityDelegateHandler(w http.ResponseWriter, r *http.Request) {
	var tx AuthorityDelegateTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	id, err := node.AddAuthorityDelegateTransaction(tx)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TX", err.Error())
		return
	}
	writeJSON(w, map[string]any{"txId": id})
}

func (node *QuidnugNode) SubmitAuthorityDelegateRevocationHandler(w http.ResponseWriter, r *http.Request) {
	var tx AuthorityDelegateRevocationTransaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	id, err := node.AddAuthorityDelegateRevocationTransaction(tx)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_TX", err.Error())
		return
	}
	writeJSON(w, map[string]any{"txId": id})
}

// --- Query handlers ---

// GetDNSAttestationsHandler returns all attestations for a
// domain. Query param ?activeOnly=1 filters out revoked +
// expired.
func (node *QuidnugNode) GetDNSAttestationsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	if domain == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "domain required")
		return
	}
	var atts []DNSAttestationTransaction
	if r.URL.Query().Get("activeOnly") == "1" {
		atts = node.DNSAttestationRegistry.GetActiveAttestationsForDomain(
			domain, time.Now().UnixNano())
	} else {
		atts = node.DNSAttestationRegistry.GetAttestationsForDomain(domain)
	}
	writeJSON(w, map[string]any{
		"domain":       domain,
		"attestations": atts,
		"count":        len(atts),
	})
}

// GetDNSAttestationsWeightedHandler returns attestations
// sorted by trust weight from the observer's perspective.
//
// Query params:
//
//   ?observer=<quid>     required
//   ?maxDepth=<int>      default DefaultTrustMaxDepth
//   ?halfLifeDays=<int>  default 730 (2 years)
//   ?floor=<float>       default 0.2
func (node *QuidnugNode) GetDNSAttestationsWeightedHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	if domain == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "domain required")
		return
	}
	observer := r.URL.Query().Get("observer")
	if observer == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "observer required")
		return
	}
	maxDepth := DefaultTrustMaxDepth
	if s := r.URL.Query().Get("maxDepth"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			maxDepth = n
		}
	}
	cfg := DefaultDecayConfig()
	if s := r.URL.Query().Get("halfLifeDays"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			cfg.HalfLifeSeconds = int64(n) * 24 * 3600
		}
	}
	if s := r.URL.Query().Get("floor"); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			cfg.Floor = f
		}
	}
	results := node.GetWeightedAttestationsForDomain(
		domain, observer, maxDepth, cfg, time.Now().UnixNano())
	writeJSON(w, map[string]any{
		"domain":   domain,
		"observer": observer,
		"results":  results,
		"count":    len(results),
	})
}

// ResolveDNSRecordHandler returns the effective delegation
// pointer (if any) for resolving the given record type on
// the domain. Phase 1: we return the delegation metadata and
// the caller is responsible for actually fetching the record
// from the delegated domain. Future phases may proxy the
// fetch through the node.
func (node *QuidnugNode) ResolveDNSRecordHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	recordType := vars["recordType"]
	if domain == "" || recordType == "" {
		writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "domain + recordType required")
		return
	}
	now := time.Now().UnixNano()
	atts := node.DNSAttestationRegistry.GetActiveAttestationsForDomain(domain, now)
	if len(atts) == 0 {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "no active attestation for domain")
		return
	}
	// Phase 1: pick the highest-TLD-tier attestation (luxury >
	// premium > standard > free-public) as the "authoritative"
	// one. Per-observer weighting is available via the
	// weighted endpoint; /resolve favors simplicity.
	best := atts[0]
	for _, a := range atts[1:] {
		if tierRank(a.TLDTier) > tierRank(best.TLDTier) {
			best = a
		}
	}
	delegations := node.DNSAttestationRegistry.GetActiveDelegations(best.ID)
	resp := map[string]any{
		"domain":      domain,
		"recordType":  recordType,
		"attestation": best,
	}
	if len(delegations) > 0 {
		d := delegations[0]
		pol, ok := d.Visibility.RecordTypes[recordType]
		if !ok {
			pol = d.Visibility.Default
		}
		resp["delegation"] = map[string]any{
			"delegateDomain": d.DelegateDomain,
			"delegateNodes":  d.DelegateNodes,
			"visibility":     pol,
		}
	}
	writeJSON(w, resp)
}

func tierRank(tier string) int {
	switch tier {
	case "luxury":
		return 4
	case "premium":
		return 3
	case "standard":
		return 2
	case "free-public":
		return 1
	}
	return 0
}

// --- Shared JSON response helpers ---

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	env := map[string]any{"success": true, "data": data}
	_ = json.NewEncoder(w).Encode(env)
}

func writeJSONError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	env := map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	}
	_ = json.NewEncoder(w).Encode(env)
}
