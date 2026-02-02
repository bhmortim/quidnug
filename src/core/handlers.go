package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Pagination constants
const (
	DefaultPaginationLimit = 50
	MaxPaginationLimit     = 1000
)

// PaginationParams holds parsed pagination parameters
type PaginationParams struct {
	Limit  int
	Offset int
}

// PaginationMeta contains pagination metadata for responses
type PaginationMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// ParsePaginationParams extracts limit and offset from query parameters
// with validation and capping to maxLimit
func ParsePaginationParams(r *http.Request, defaultLimit, maxLimit int) PaginationParams {
	params := PaginationParams{
		Limit:  defaultLimit,
		Offset: 0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit > 0 {
				params.Limit = limit
			}
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			if offset >= 0 {
				params.Offset = offset
			}
		}
	}

	if params.Limit > maxLimit {
		params.Limit = maxLimit
	}

	return params
}

// paginateSlice is a helper to paginate a generic slice
// Returns the paginated slice and total count
func paginateSlice[T any](items []T, params PaginationParams) ([]T, int) {
	total := len(items)

	if params.Offset >= total {
		return []T{}, total
	}

	end := params.Offset + params.Limit
	if end > total {
		end = total
	}

	return items[params.Offset:end], total
}

// WriteSuccess writes a successful JSON response with envelope
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// WriteSuccessWithStatus writes a successful JSON response with custom status code
func WriteSuccessWithStatus(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// WriteError writes an error JSON response with envelope
func WriteError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// WriteFieldError writes a field validation error response
func WriteFieldError(w http.ResponseWriter, code string, message string, fields []string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"fields":  fields,
		},
	})
}

// registerAPIRoutes registers all API routes on the given router
func (node *QuidnugNode) registerAPIRoutes(router *mux.Router) {
	// Metrics endpoint
	router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// API endpoints
	router.HandleFunc("/health", node.HealthCheckHandler).Methods("GET")
	router.HandleFunc("/nodes", node.GetNodesHandler).Methods("GET")

	// Transaction endpoints
	router.HandleFunc("/transactions", node.GetTransactionsHandler).Methods("GET")
	router.HandleFunc("/transactions/trust", node.CreateTrustTransactionHandler).Methods("POST")
	router.HandleFunc("/transactions/identity", node.CreateIdentityTransactionHandler).Methods("POST")
	router.HandleFunc("/transactions/title", node.CreateTitleTransactionHandler).Methods("POST")

	// Blockchain endpoints
	router.HandleFunc("/blocks", node.GetBlocksHandler).Methods("GET")

	// Trust domain endpoints
	router.HandleFunc("/domains", node.GetDomainsHandler).Methods("GET")
	router.HandleFunc("/domains", node.RegisterDomainHandler).Methods("POST")
	router.HandleFunc("/domains/{name}/query", node.QueryDomainHandler).Methods("GET")

	// Registry query endpoints
	router.HandleFunc("/registry/trust", node.QueryTrustRegistryHandler).Methods("GET")
	router.HandleFunc("/registry/identity", node.QueryIdentityRegistryHandler).Methods("GET")
	router.HandleFunc("/registry/title", node.QueryTitleRegistryHandler).Methods("GET")

	// API spec endpoints
	router.HandleFunc("/info", node.GetInfoHandler).Methods("GET")
	router.HandleFunc("/quids", node.CreateQuidHandler).Methods("POST")
	router.HandleFunc("/trust/query", node.RelationalTrustQueryHandler).Methods("POST")
	router.HandleFunc("/trust/edges/{quidId}", node.GetTrustEdgesHandler).Methods("GET")
	router.HandleFunc("/trust/{observer}/{target}", node.GetTrustHandler).Methods("GET")
	router.HandleFunc("/identity/{quidId}", node.GetIdentityHandler).Methods("GET")
	router.HandleFunc("/title/{assetId}", node.GetTitleHandler).Methods("GET")
	router.HandleFunc("/blocks/tentative/{domain}", node.GetTentativeBlocksHandler).Methods("GET")
}

// StartServer starts the HTTP server for API endpoints
func (node *QuidnugNode) StartServer(port string) error {
	return node.StartServerWithConfig(port, DefaultRateLimitPerMinute, DefaultMaxBodySizeBytes)
}

// StartServerWithConfig starts the HTTP server with configurable middleware settings
func (node *QuidnugNode) StartServerWithConfig(port string, rateLimitPerMinute int, maxBodySizeBytes int64) error {
	router := mux.NewRouter()

	// Register versioned routes under /api/v1
	v1Router := router.PathPrefix("/api/v1").Subrouter()
	node.registerAPIRoutes(v1Router)

	// Register backward-compatible routes under /api
	apiRouter := router.PathPrefix("/api").Subrouter()
	node.registerAPIRoutes(apiRouter)

	// Apply middleware chain (outermost to innermost processing order):
	// RateLimit -> BodySizeLimit -> NodeAuth -> Metrics -> RequestID -> Router
	rateLimiter := NewIPRateLimiter(rateLimitPerMinute)
	handler := RequestIDMiddleware(router)
	handler = MetricsMiddleware(handler)
	handler = NodeAuthMiddleware(handler)
	handler = BodySizeLimitMiddleware(maxBodySizeBytes)(handler)
	handler = RateLimitMiddleware(rateLimiter)(handler)

	// Create HTTP server and store reference for graceful shutdown
	node.Server = &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	logger.Info("Starting quidnug node server", "port", port, "nodeId", node.NodeID, "rateLimit", rateLimitPerMinute, "maxBodySize", maxBodySizeBytes)
	return node.Server.ListenAndServe()
}

// HealthCheckHandler handles health check requests
func (node *QuidnugNode) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, map[string]interface{}{
		"status":  "ok",
		"node_id": node.NodeID,
		"uptime":  time.Now().Unix() - node.Blockchain[0].Timestamp,
		"version": "1.0.0",
	})
}

// GetNodesHandler returns the list of known nodes
func (node *QuidnugNode) GetNodesHandler(w http.ResponseWriter, r *http.Request) {
	params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)

	node.KnownNodesMutex.RLock()
	var nodesList []Node
	for _, n := range node.KnownNodes {
		nodesList = append(nodesList, n)
	}
	node.KnownNodesMutex.RUnlock()

	paginatedNodes, total := paginateSlice(nodesList, params)

	WriteSuccess(w, map[string]interface{}{
		"data": paginatedNodes,
		"pagination": PaginationMeta{
			Limit:  params.Limit,
			Offset: params.Offset,
			Total:  total,
		},
	})
}

// GetTransactionsHandler returns pending transactions
func (node *QuidnugNode) GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)

	node.PendingTxsMutex.RLock()
	txsCopy := make([]interface{}, len(node.PendingTxs))
	copy(txsCopy, node.PendingTxs)
	node.PendingTxsMutex.RUnlock()

	paginatedTxs, total := paginateSlice(txsCopy, params)

	WriteSuccess(w, map[string]interface{}{
		"data": paginatedTxs,
		"pagination": PaginationMeta{
			Limit:  params.Limit,
			Offset: params.Offset,
			Total:  total,
		},
	})
}

// CreateTrustTransactionHandler handles trust transaction creation
func (node *QuidnugNode) CreateTrustTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx TrustTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddTrustTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"status":         "success",
		"transaction_id": txID,
		"message":        "Trust transaction added to pending pool",
	})
}

// CreateIdentityTransactionHandler handles identity transaction creation
func (node *QuidnugNode) CreateIdentityTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx IdentityTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddIdentityTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"status":         "success",
		"transaction_id": txID,
		"message":        "Identity transaction added to pending pool",
	})
}

// CreateTitleTransactionHandler handles title transaction creation
func (node *QuidnugNode) CreateTitleTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx TitleTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddTitleTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"status":         "success",
		"transaction_id": txID,
		"message":        "Title transaction added to pending pool",
	})
}

// GetBlocksHandler returns the blockchain
func (node *QuidnugNode) GetBlocksHandler(w http.ResponseWriter, r *http.Request) {
	params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)

	node.BlockchainMutex.RLock()
	paginatedBlocks, total := paginateSlice(node.Blockchain, params)
	node.BlockchainMutex.RUnlock()

	WriteSuccess(w, map[string]interface{}{
		"data": paginatedBlocks,
		"pagination": PaginationMeta{
			Limit:  params.Limit,
			Offset: params.Offset,
			Total:  total,
		},
	})
}

// GetDomainsHandler returns the list of trust domains
func (node *QuidnugNode) GetDomainsHandler(w http.ResponseWriter, r *http.Request) {
	node.TrustDomainsMutex.RLock()
	defer node.TrustDomainsMutex.RUnlock()

	// Convert map to slice for response
	var domainsList []TrustDomain
	for _, domain := range node.TrustDomains {
		domainsList = append(domainsList, domain)
	}

	WriteSuccess(w, map[string]interface{}{
		"domains": domainsList,
	})
}

// RegisterDomainHandler handles trust domain registration
func (node *QuidnugNode) RegisterDomainHandler(w http.ResponseWriter, r *http.Request) {
	var domain TrustDomain
	if err := DecodeJSONBody(w, r, &domain); err != nil {
		return
	}

	if err := node.RegisterTrustDomain(domain); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"status":  "success",
		"message": "Trust domain registered successfully",
		"domain":  domain.Name,
	})
}

// QueryDomainHandler handles domain queries
func (node *QuidnugNode) QueryDomainHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domainName := vars["name"]
	queryType := r.URL.Query().Get("type")
	queryParam := r.URL.Query().Get("param")

	// Check if this node manages the requested domain
	node.TrustDomainsMutex.RLock()
	_, localDomain := node.TrustDomains[domainName]
	node.TrustDomainsMutex.RUnlock()

	if localDomain {
		// Handle local domain query based on query type
		var result interface{}

		switch queryType {
		case "identity":
			// Query local identity registry
			identity, exists := node.GetQuidIdentity(queryParam)
			if !exists {
				WriteError(w, http.StatusNotFound, "NOT_FOUND", "Identity not found")
				return
			}
			result = identity

		case "trust":
			// Parse observer and target from param (format: "observer:target")
			parts := strings.Split(queryParam, ":")
			if len(parts) != 2 {
				WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid trust query format. Use observer:target")
				return
			}
			observer := parts[0]
			target := parts[1]

			trustLevel, trustPath, err := node.ComputeRelationalTrust(observer, target, DefaultTrustMaxDepth)
			if err != nil {
				logger.Warn("Trust computation exceeded resource limits",
					"observer", observer,
					"target", target,
					"error", err)
			}

			pathDepth := 0
			if len(trustPath) > 1 {
				pathDepth = len(trustPath) - 1
			}

			result = RelationalTrustResult{
				Observer:   observer,
				Target:     target,
				TrustLevel: trustLevel,
				TrustPath:  trustPath,
				PathDepth:  pathDepth,
				Domain:     domainName,
			}

		case "title":
			// Query local title registry
			title, exists := node.GetAssetOwnership(queryParam)
			if !exists {
				WriteError(w, http.StatusNotFound, "NOT_FOUND", "Title not found")
				return
			}
			result = title

		default:
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Unknown query type")
			return
		}

		WriteSuccess(w, result)
	} else {
		// Forward query to other domains
		result, err := node.QueryOtherDomain(domainName, queryType, queryParam)
		if err != nil {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		WriteSuccess(w, result)
	}
}

// QueryTrustRegistryHandler handles trust registry queries
func (node *QuidnugNode) QueryTrustRegistryHandler(w http.ResponseWriter, r *http.Request) {
	// Support both old (truster/trustee) and new (observer/target) parameter names
	observer := r.URL.Query().Get("observer")
	target := r.URL.Query().Get("target")
	truster := r.URL.Query().Get("truster")
	trustee := r.URL.Query().Get("trustee")
	maxDepthStr := r.URL.Query().Get("maxDepth")

	// If observer/target provided, compute relational trust
	if observer != "" && target != "" {
		maxDepth := DefaultTrustMaxDepth
		if maxDepthStr != "" {
			if parsed, err := strconv.Atoi(maxDepthStr); err == nil && parsed > 0 {
				maxDepth = parsed
			}
		}

		trustLevel, trustPath, err := node.ComputeRelationalTrust(observer, target, maxDepth)
		if err != nil {
			logger.Warn("Trust computation exceeded resource limits",
				"observer", observer,
				"target", target,
				"error", err)
		}

		pathDepth := 0
		if len(trustPath) > 1 {
			pathDepth = len(trustPath) - 1
		}

		result := RelationalTrustResult{
			Observer:   observer,
			Target:     target,
			TrustLevel: trustLevel,
			TrustPath:  trustPath,
			PathDepth:  pathDepth,
		}

		WriteSuccess(w, result)
		return
	}

	// Fall back to existing behavior for truster/trustee (direct trust)
	if truster != "" && trustee != "" {
		trustLevel := node.GetTrustLevel(truster, trustee)
		WriteSuccess(w, map[string]interface{}{
			"truster":     truster,
			"trustee":     trustee,
			"trust_level": trustLevel,
		})
	} else if truster != "" {
		node.TrustRegistryMutex.RLock()
		relationships := node.TrustRegistry[truster]
		node.TrustRegistryMutex.RUnlock()

		WriteSuccess(w, map[string]interface{}{
			"truster":       truster,
			"relationships": relationships,
		})
	} else {
		params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)

		node.TrustRegistryMutex.RLock()
		type TrustEntry struct {
			Truster    string  `json:"truster"`
			Trustee    string  `json:"trustee"`
			TrustLevel float64 `json:"trust_level"`
		}
		var entries []TrustEntry
		for truster, relationships := range node.TrustRegistry {
			for trustee, level := range relationships {
				entries = append(entries, TrustEntry{
					Truster:    truster,
					Trustee:    trustee,
					TrustLevel: level,
				})
			}
		}
		node.TrustRegistryMutex.RUnlock()

		paginatedEntries, total := paginateSlice(entries, params)

		WriteSuccess(w, map[string]interface{}{
			"data": paginatedEntries,
			"pagination": PaginationMeta{
				Limit:  params.Limit,
				Offset: params.Offset,
				Total:  total,
			},
		})
	}
}

// QueryIdentityRegistryHandler handles identity registry queries
func (node *QuidnugNode) QueryIdentityRegistryHandler(w http.ResponseWriter, r *http.Request) {
	quidID := r.URL.Query().Get("quid_id")

	if quidID != "" {
		identity, exists := node.GetQuidIdentity(quidID)
		if !exists {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Identity not found")
			return
		}
		WriteSuccess(w, identity)
	} else {
		params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)

		node.IdentityRegistryMutex.RLock()
		type IdentityEntry struct {
			QuidID   string                 `json:"quid_id"`
			Identity map[string]interface{} `json:"identity"`
		}
		var entries []IdentityEntry
		for quidID, identity := range node.IdentityRegistry {
			identityMap := map[string]interface{}{
				"quidId":      identity.QuidID,
				"name":        identity.Name,
				"description": identity.Description,
				"attributes":  identity.Attributes,
				"creator":     identity.Creator,
				"updateNonce": identity.UpdateNonce,
			}
			entries = append(entries, IdentityEntry{
				QuidID:   quidID,
				Identity: identityMap,
			})
		}
		node.IdentityRegistryMutex.RUnlock()

		paginatedEntries, total := paginateSlice(entries, params)

		WriteSuccess(w, map[string]interface{}{
			"data": paginatedEntries,
			"pagination": PaginationMeta{
				Limit:  params.Limit,
				Offset: params.Offset,
				Total:  total,
			},
		})
	}
}

// GetInfoHandler returns node information
func (node *QuidnugNode) GetInfoHandler(w http.ResponseWriter, r *http.Request) {
	node.TrustDomainsMutex.RLock()
	managedDomains := make([]string, 0, len(node.TrustDomains))
	for domain := range node.TrustDomains {
		managedDomains = append(managedDomains, domain)
	}
	node.TrustDomainsMutex.RUnlock()

	node.BlockchainMutex.RLock()
	blockHeight := int64(0)
	if len(node.Blockchain) > 0 {
		blockHeight = node.Blockchain[len(node.Blockchain)-1].Index
	}
	node.BlockchainMutex.RUnlock()

	WriteSuccess(w, map[string]interface{}{
		"nodeQuid":       node.NodeID,
		"managedDomains": managedDomains,
		"blockHeight":    blockHeight,
		"version":        "1.0.0",
	})
}

// CreateQuidHandler creates a new quid identity
func (node *QuidnugNode) CreateQuidHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Metadata map[string]interface{} `json:"metadata"`
	}

	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body")
			return
		}
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to generate key pair")
		return
	}

	publicKeyBytes := elliptic.Marshal(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	hashBytes := sha256.Sum256(publicKeyBytes)
	quidID := hex.EncodeToString(hashBytes[:])[:16]

	created := time.Now().Unix()

	WriteSuccessWithStatus(w, http.StatusCreated, map[string]interface{}{
		"quidId":    quidID,
		"publicKey": publicKeyHex,
		"created":   created,
	})
}

// GetTrustHandler returns relational trust level between two quids
func (node *QuidnugNode) GetTrustHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	observer := vars["observer"]
	target := vars["target"]
	domain := r.URL.Query().Get("domain")
	maxDepthStr := r.URL.Query().Get("maxDepth")
	includeUnverifiedStr := r.URL.Query().Get("includeUnverified")

	if domain == "" {
		domain = "default"
	}

	maxDepth := DefaultTrustMaxDepth
	if maxDepthStr != "" {
		if parsed, err := strconv.Atoi(maxDepthStr); err == nil && parsed > 0 {
			maxDepth = parsed
		}
	}

	includeUnverified := includeUnverifiedStr == "true"

	if includeUnverified {
		result, err := node.ComputeRelationalTrustEnhanced(observer, target, maxDepth, true)
		if err != nil {
			logger.Warn("Trust computation exceeded resource limits",
				"observer", observer,
				"target", target,
				"error", err)
			// Return partial result with warning header
			w.Header().Set("X-Trust-Computation-Warning", "resource limits exceeded, partial result returned")
		}
		result.Domain = domain
		WriteSuccess(w, result)
	} else {
		trustLevel, trustPath, err := node.ComputeRelationalTrust(observer, target, maxDepth)
		if err != nil {
			logger.Warn("Trust computation exceeded resource limits",
				"observer", observer,
				"target", target,
				"error", err)
			// Return partial result with warning header
			w.Header().Set("X-Trust-Computation-Warning", "resource limits exceeded, partial result returned")
		}

		pathDepth := 0
		if len(trustPath) > 1 {
			pathDepth = len(trustPath) - 1
		}

		result := RelationalTrustResult{
			Observer:   observer,
			Target:     target,
			TrustLevel: trustLevel,
			TrustPath:  trustPath,
			PathDepth:  pathDepth,
			Domain:     domain,
		}

		WriteSuccess(w, result)
	}
}

// RelationalTrustQueryHandler handles POST requests for relational trust queries
func (node *QuidnugNode) RelationalTrustQueryHandler(w http.ResponseWriter, r *http.Request) {
	var query RelationalTrustQuery
	if err := DecodeJSONBody(w, r, &query); err != nil {
		return
	}

	if query.Observer == "" || query.Target == "" {
		WriteFieldError(w, "MISSING_PARAMETERS", "observer and target are required", []string{"observer", "target"})
		return
	}

	domain := query.Domain
	if domain == "" {
		domain = "default"
	}

	maxDepth := query.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultTrustMaxDepth
	}

	if query.IncludeUnverified {
		result, err := node.ComputeRelationalTrustEnhanced(query.Observer, query.Target, maxDepth, true)
		if err != nil {
			logger.Warn("Trust computation exceeded resource limits",
				"observer", query.Observer,
				"target", query.Target,
				"error", err)
			// Return partial result with warning header
			w.Header().Set("X-Trust-Computation-Warning", "resource limits exceeded, partial result returned")
		}
		result.Domain = domain
		WriteSuccess(w, result)
	} else {
		trustLevel, trustPath, err := node.ComputeRelationalTrust(query.Observer, query.Target, maxDepth)
		if err != nil {
			logger.Warn("Trust computation exceeded resource limits",
				"observer", query.Observer,
				"target", query.Target,
				"error", err)
			// Return partial result with warning header
			w.Header().Set("X-Trust-Computation-Warning", "resource limits exceeded, partial result returned")
		}

		pathDepth := 0
		if len(trustPath) > 1 {
			pathDepth = len(trustPath) - 1
		}

		result := RelationalTrustResult{
			Observer:   query.Observer,
			Target:     query.Target,
			TrustLevel: trustLevel,
			TrustPath:  trustPath,
			PathDepth:  pathDepth,
			Domain:     domain,
		}

		WriteSuccess(w, result)
	}
}

// GetIdentityHandler returns identity information for a quid
func (node *QuidnugNode) GetIdentityHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	quidID := vars["quidId"]

	identity, exists := node.GetQuidIdentity(quidID)
	if !exists {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Identity not found")
		return
	}

	WriteSuccess(w, identity)
}

// GetTitleHandler returns ownership information for an asset
func (node *QuidnugNode) GetTitleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["assetId"]

	title, exists := node.GetAssetOwnership(assetID)
	if !exists {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Title not found")
		return
	}

	WriteSuccess(w, title)
}

// GetTentativeBlocksHandler returns tentative blocks for a domain
func (node *QuidnugNode) GetTentativeBlocksHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]

	blocks := node.GetTentativeBlocks(domain)

	WriteSuccess(w, map[string]interface{}{
		"domain": domain,
		"blocks": blocks,
	})
}

// GetTrustEdgesHandler returns trust edges for a quid with provenance
func (node *QuidnugNode) GetTrustEdgesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	quidId := vars["quidId"]

	includeUnverifiedStr := r.URL.Query().Get("includeUnverified")
	includeUnverified := includeUnverifiedStr == "true"

	edges := node.GetTrustEdges(quidId, includeUnverified)

	WriteSuccess(w, map[string]interface{}{
		"quidId":            quidId,
		"includeUnverified": includeUnverified,
		"edges":             edges,
	})
}

// QueryTitleRegistryHandler handles title registry queries
func (node *QuidnugNode) QueryTitleRegistryHandler(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("asset_id")
	ownerID := r.URL.Query().Get("owner_id")

	if assetID != "" {
		title, exists := node.GetAssetOwnership(assetID)
		if !exists {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Title not found")
			return
		}
		WriteSuccess(w, title)
	} else if ownerID != "" {
		params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)

		node.TitleRegistryMutex.RLock()
		var ownedAssets []map[string]interface{}
		for assetID, title := range node.TitleRegistry {
			for _, stake := range title.Owners {
				if stake.OwnerID == ownerID {
					ownedAssets = append(ownedAssets, map[string]interface{}{
						"asset_id":   assetID,
						"percentage": stake.Percentage,
						"stake_type": stake.StakeType,
					})
					break
				}
			}
		}
		node.TitleRegistryMutex.RUnlock()

		paginatedAssets, total := paginateSlice(ownedAssets, params)

		WriteSuccess(w, map[string]interface{}{
			"data": paginatedAssets,
			"pagination": PaginationMeta{
				Limit:  params.Limit,
				Offset: params.Offset,
				Total:  total,
			},
		})
	} else {
		params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)

		node.TitleRegistryMutex.RLock()
		type TitleEntry struct {
			AssetID string      `json:"asset_id"`
			Title   interface{} `json:"title"`
		}
		var entries []TitleEntry
		for assetID, title := range node.TitleRegistry {
			entries = append(entries, TitleEntry{
				AssetID: assetID,
				Title:   title,
			})
		}
		node.TitleRegistryMutex.RUnlock()

		paginatedEntries, total := paginateSlice(entries, params)

		WriteSuccess(w, map[string]interface{}{
			"data": paginatedEntries,
			"pagination": PaginationMeta{
				Limit:  params.Limit,
				Offset: params.Offset,
				Total:  total,
			},
		})
	}
}
