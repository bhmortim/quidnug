package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/quidnug/quidnug/internal/config"
	"github.com/quidnug/quidnug/internal/ratelimit"
)

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

	// Event streaming endpoints
	router.HandleFunc("/events", node.CreateEventTransactionHandler).Methods("POST")
	router.HandleFunc("/node-advertisements", node.CreateNodeAdvertisementHandler).Methods("POST")
	router.HandleFunc("/streams/{subjectId}", node.GetEventStreamHandler).Methods("GET")
	router.HandleFunc("/streams/{subjectId}/events", node.GetStreamEventsHandler).Methods("GET")

	// QDP-0015 content moderation.
	router.HandleFunc("/moderation/actions", node.CreateModerationActionHandler).Methods("POST")
	router.HandleFunc("/moderation/actions/{targetType}/{targetId}", node.GetModerationActionsHandler).Methods("GET")

	// QDP-0018 operator audit log.
	router.HandleFunc("/audit/head", node.AuditHeadHandler).Methods("GET")
	router.HandleFunc("/audit/entries", node.AuditEntriesHandler).Methods("GET")
	router.HandleFunc("/audit/entry/{sequence}", node.AuditEntryHandler).Methods("GET")

	// QDP-0017 data subject rights / privacy.
	router.HandleFunc("/privacy/dsr", node.CreateDSRHandler).Methods("POST")
	router.HandleFunc("/privacy/dsr/{requestTxId}", node.GetDSRStatusHandler).Methods("GET")
	router.HandleFunc("/privacy/consent/grants", node.CreateConsentGrantHandler).Methods("POST")
	router.HandleFunc("/privacy/consent/withdraws", node.CreateConsentWithdrawHandler).Methods("POST")
	router.HandleFunc("/privacy/consent/history", node.GetConsentHistoryHandler).Methods("GET")
	router.HandleFunc("/privacy/restrictions", node.CreateProcessingRestrictionHandler).Methods("POST")
	router.HandleFunc("/privacy/restrictions/{subjectQuid}", node.GetRestrictionsForSubjectHandler).Methods("GET")
	router.HandleFunc("/privacy/compliance", node.CreateDSRComplianceHandler).Methods("POST")

	// IPFS endpoints
	router.HandleFunc("/ipfs/pin", node.PinToIPFSHandler).Methods("POST")
	router.HandleFunc("/ipfs/{cid}", node.GetFromIPFSHandler).Methods("GET")

	// Node domain advertisement endpoints
	router.HandleFunc("/node/domains", node.GetNodeDomainsHandler).Methods("GET")
	router.HandleFunc("/node/domains", node.UpdateNodeDomainsHandler).Methods("POST")

	// Gossip endpoints
	router.HandleFunc("/gossip/domains", node.ReceiveDomainGossipHandler).Methods("POST")

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
	return node.StartServerWithConfig(port, config.DefaultRateLimitPerMinute, config.DefaultMaxBodySizeBytes)
}

// Default HTTP server timeouts. These bound how long individual client
// connections can tie up resources and are the primary defense against
// Slowloris / idle-connection DoS.
const (
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultReadTimeout       = 30 * time.Second
	DefaultWriteTimeout      = 60 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
)

// StartServerWithConfig starts the HTTP server with configurable middleware
// settings. It reads TLS_CERT_FILE and TLS_KEY_FILE from the environment;
// if both are set, the server listens with TLS. Otherwise it falls back to
// plaintext HTTP and logs a warning.
func (node *QuidnugNode) StartServerWithConfig(port string, rateLimitPerMinute int, maxBodySizeBytes int64) error {
	router := mux.NewRouter()

	// Register versioned routes under /api/v1
	v1Router := router.PathPrefix("/api/v1").Subrouter()
	node.registerAPIRoutes(v1Router)

	// Register backward-compatible routes under /api
	apiRouter := router.PathPrefix("/api").Subrouter()
	node.registerAPIRoutes(apiRouter)

	// v2 surface: new-protocol endpoints (QDP-0002 guardians +
	// QDP-0003 cross-domain gossip). Kept under /api/v2 so existing
	// v1 clients don't accidentally depend on them before the
	// protocol work is stabilized.
	v2Router := router.PathPrefix("/api/v2").Subrouter()
	node.registerGuardianRoutes(v2Router)
	node.registerCrossDomainRoutes(v2Router)
	node.RegisterDiscoveryRoutes(v2Router)
	node.RegisterDNSAttestationRoutes(v2Router)

	// Apply middleware chain (outermost to innermost processing order):
	//   RateLimit -> BodySizeLimit -> NodeAuth -> Metrics -> SecurityHeaders -> RequestID -> Router
	rateLimiter := ratelimit.New(rateLimitPerMinute)
	handler := RequestIDMiddleware(router)
	handler = SecurityHeadersMiddleware(handler)
	handler = MetricsMiddleware(handler)
	handler = NodeAuthMiddleware(handler)
	handler = BodySizeLimitMiddleware(maxBodySizeBytes)(handler)
	handler = RateLimitMiddleware(rateLimiter)(handler)

	// Create HTTP server and store reference for graceful shutdown. All
	// timeouts are set explicitly so that slow clients cannot hold a
	// goroutine indefinitely.
	node.Server = &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		ReadTimeout:       DefaultReadTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}

	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")
	if certFile != "" && keyFile != "" {
		logger.Info("Starting quidnug node server (TLS)", "port", port, "nodeId", node.NodeID, "rateLimit", rateLimitPerMinute, "maxBodySize", maxBodySizeBytes)
		return node.Server.ListenAndServeTLS(certFile, keyFile)
	}

	logger.Warn("Starting quidnug node server over plaintext HTTP; set TLS_CERT_FILE and TLS_KEY_FILE to enable TLS",
		"port", port, "nodeId", node.NodeID, "rateLimit", rateLimitPerMinute, "maxBodySize", maxBodySizeBytes)
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

// CreateEventTransactionHandler handles event transaction creation
func (node *QuidnugNode) CreateEventTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx EventTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Pre-calculate sequence if not provided (replicate AddEventTransaction logic)
	// to return accurate sequence in response
	if tx.Sequence == 0 {
		node.EventStreamMutex.RLock()
		events, exists := node.EventRegistry[tx.SubjectID]
		if exists && len(events) > 0 {
			tx.Sequence = events[len(events)-1].Sequence + 1
		} else {
			tx.Sequence = 1
		}
		node.EventStreamMutex.RUnlock()
	}

	txID, err := node.AddEventTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"id":       txID,
		"sequence": tx.Sequence,
	})
}

// GetEventStreamHandler returns event stream metadata for a
// subject. QDP-0015 moderation on the subject quid short-circuits
// before data is returned (see GetStreamEventsHandler for the
// per-event variant).
func (node *QuidnugNode) GetEventStreamHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subjectID := vars["subjectId"]
	includeHidden := r.URL.Query().Get("includeHidden") == "true"

	scope := node.EffectiveScopeFor(ModerationTargetQuid, subjectID)
	switch scope.Scope {
	case ModerationScopeSuppress:
		w.Header().Set("X-Quidnug-Moderated", scope.ReasonCode)
		WriteError(w, http.StatusUnavailableForLegalReasons,
			"MODERATED", "Stream suppressed by operator policy")
		return
	case ModerationScopeHide:
		if !includeHidden {
			w.Header().Set("X-Quidnug-Moderated", scope.ReasonCode)
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Event stream not found")
			return
		}
	case ModerationScopeAnnotate:
		w.Header().Set("X-Quidnug-Annotation", scope.AnnotationText)
	}

	stream, exists := node.GetEventStream(subjectID)
	if !exists {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Event stream not found")
		return
	}

	WriteSuccess(w, stream)
}

// GetStreamEventsHandler returns paginated events for a stream.
//
// QDP-0022: events whose payload contains an `expiresAt` field
// (Unix nanoseconds) in the past are hidden from the default
// response. Pass `?include_expired=true` to bypass the filter
// — useful for audit tooling or incident forensics, not for
// application traffic.
//
// QDP-0015: moderation actions on the stream subject
// (scope=suppress) short-circuit with HTTP 451; scope=hide
// returns 404 unless `?includeHidden=true`; scope=annotate
// passes through with the annotation included in the response
// as X-Quidnug-Annotation. Per-event suppressions are also
// applied (events suppressed individually are dropped from the
// page). Override the per-event filter with `?includeHidden=true`.
func (node *QuidnugNode) GetStreamEventsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subjectID := vars["subjectId"]

	params := ParsePaginationParams(r, DefaultPaginationLimit, MaxPaginationLimit)
	includeExpired := r.URL.Query().Get("include_expired") == "true"
	includeHidden := r.URL.Query().Get("includeHidden") == "true"

	// QDP-0015: stream-subject-level moderation. Check the
	// subject's QUID scope first, then fall back to per-event
	// filtering.
	streamScope := node.EffectiveScopeFor(ModerationTargetQuid, subjectID)
	switch streamScope.Scope {
	case ModerationScopeSuppress:
		w.Header().Set("X-Quidnug-Moderated", streamScope.ReasonCode)
		WriteError(w, http.StatusUnavailableForLegalReasons,
			"MODERATED", "Stream suppressed by operator policy")
		return
	case ModerationScopeHide:
		if !includeHidden {
			w.Header().Set("X-Quidnug-Moderated", streamScope.ReasonCode)
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Event stream not found")
			return
		}
	case ModerationScopeAnnotate:
		w.Header().Set("X-Quidnug-Annotation", streamScope.AnnotationText)
	}

	events, total := node.GetStreamEvents(subjectID, params.Limit, params.Offset)
	if !includeExpired {
		events = FilterExpiredEvents(events)
	}
	events = node.filterModeratedEvents(events, includeHidden)

	WriteSuccess(w, map[string]interface{}{
		"data": events,
		"pagination": PaginationMeta{
			Limit:  params.Limit,
			Offset: params.Offset,
			Total:  total,
		},
	})
}

// filterModeratedEvents applies per-event QDP-0015 moderation
// to a page of events:
//
//   - `suppress` scope drops the event from the slice entirely.
//   - `hide` scope drops unless includeHidden is true.
//   - `annotate` scope merges the annotation text into the
//     event's payload under the reserved key `_moderationNote`.
//
// Returns a new slice; input is not mutated.
func (node *QuidnugNode) filterModeratedEvents(events []EventTransaction, includeHidden bool) []EventTransaction {
	if node.ModerationRegistry == nil || len(events) == 0 {
		return events
	}
	out := make([]EventTransaction, 0, len(events))
	for _, ev := range events {
		scope := node.EffectiveScopeFor(ModerationTargetTx, ev.ID)
		switch scope.Scope {
		case ModerationScopeSuppress:
			continue
		case ModerationScopeHide:
			if !includeHidden {
				continue
			}
		case ModerationScopeAnnotate:
			if ev.Payload == nil {
				ev.Payload = map[string]interface{}{}
			} else {
				// Copy-on-write so we don't mutate the
				// registry's stored event.
				cp := make(map[string]interface{}, len(ev.Payload)+1)
				for k, v := range ev.Payload {
					cp[k] = v
				}
				ev.Payload = cp
			}
			ev.Payload["_moderationNote"] = scope.AnnotationText
		}
		out = append(out, ev)
	}
	return out
}

// CreateModerationActionHandler accepts a signed
// ModerationActionTransaction (QDP-0015) and queues it for
// block inclusion. The submitter must be an authorized
// moderator for the tx's trust domain (validator or
// delegated-trust ≥0.7 from a validator).
func (node *QuidnugNode) CreateModerationActionHandler(w http.ResponseWriter, r *http.Request) {
	var tx ModerationActionTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}

	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddModerationActionTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"id":            txID,
		"moderatorQuid": tx.ModeratorQuid,
		"targetType":    tx.TargetType,
		"targetId":      tx.TargetID,
		"scope":         tx.Scope,
		"reasonCode":    tx.ReasonCode,
		"nonce":         tx.Nonce,
	})
}

// GetModerationActionsHandler returns every moderation action
// in the registry for a given (targetType, targetId), together
// with the current effective scope. Useful for clients and
// transparency tooling that want the full audit trail rather
// than just the resolved decision.
func (node *QuidnugNode) GetModerationActionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetType := vars["targetType"]
	targetID := vars["targetId"]

	if err := validateModerationTarget(targetType, targetID); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	if node.ModerationRegistry == nil {
		WriteSuccess(w, map[string]interface{}{
			"actions":         []ModerationActionTransaction{},
			"effectiveScope":  EffectiveScope{},
		})
		return
	}

	actions := node.ModerationRegistry.actionsFor(targetType, targetID)
	scope := node.EffectiveScopeFor(targetType, targetID)
	WriteSuccess(w, map[string]interface{}{
		"targetType":     targetType,
		"targetId":       targetID,
		"actions":        actions,
		"effectiveScope": scope,
	})
}

// QueryTitleRegistryHandler returns title records, optionally filtered by
// asset ID or owner. Mirrors the behaviour of the other registry query
// handlers.
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

// CreateNodeAdvertisementHandler accepts a signed
// NodeAdvertisementTransaction (QDP-0014), validates + queues
// it for block inclusion, and returns the assigned tx id.
//
// The advertisement is self-published by a node; the submitter
// must own the NodeQuid's private key. Operator attestation +
// signature checks happen inside AddNodeAdvertisementTransaction.
func (node *QuidnugNode) CreateNodeAdvertisementHandler(w http.ResponseWriter, r *http.Request) {
	var tx NodeAdvertisementTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}

	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	txID, err := node.AddNodeAdvertisementTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"id":                 txID,
		"nodeQuid":           tx.NodeQuid,
		"operatorQuid":       tx.OperatorQuid,
		"expiresAt":          tx.ExpiresAt,
		"advertisementNonce": tx.AdvertisementNonce,
	})
}

// ---- QDP-0017 data subject rights / privacy handlers ----------------

// CreateDSRHandler accepts a signed DATA_SUBJECT_REQUEST
// transaction (QDP-0017 §4). The request must be signed by the
// subject's quid key — self-signature is the operator's proof
// that the requester controls the claimed quid.
func (node *QuidnugNode) CreateDSRHandler(w http.ResponseWriter, r *http.Request) {
	var tx DataSubjectRequestTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	txID, err := node.AddDataSubjectRequestTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	WriteSuccess(w, map[string]interface{}{
		"id":           txID,
		"subjectQuid":  tx.SubjectQuid,
		"requestType":  tx.RequestType,
		"jurisdiction": tx.Jurisdiction,
		"nonce":        tx.Nonce,
	})
}

// GetDSRStatusHandler returns the current request + compliance
// record for a DSR id. 404 if the request id is unknown.
func (node *QuidnugNode) GetDSRStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	requestTxID := vars["requestTxId"]
	req, comp, reqOk, compOk := node.GetDSRStatus(requestTxID)
	if !reqOk {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "DSR request not found")
		return
	}
	resp := map[string]interface{}{
		"request": req,
	}
	if compOk {
		resp["compliance"] = comp
	}
	WriteSuccess(w, resp)
}

// CreateConsentGrantHandler accepts a signed CONSENT_GRANT.
func (node *QuidnugNode) CreateConsentGrantHandler(w http.ResponseWriter, r *http.Request) {
	var tx ConsentGrantTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	txID, err := node.AddConsentGrantTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	WriteSuccess(w, map[string]interface{}{
		"id":             txID,
		"subjectQuid":    tx.SubjectQuid,
		"controllerQuid": tx.ControllerQuid,
		"scope":          tx.Scope,
		"effectiveUntil": tx.EffectiveUntil,
		"nonce":          tx.Nonce,
	})
}

// CreateConsentWithdrawHandler accepts a signed CONSENT_WITHDRAW.
func (node *QuidnugNode) CreateConsentWithdrawHandler(w http.ResponseWriter, r *http.Request) {
	var tx ConsentWithdrawTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	txID, err := node.AddConsentWithdrawTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	WriteSuccess(w, map[string]interface{}{
		"id":                   txID,
		"subjectQuid":          tx.SubjectQuid,
		"withdrawsGrantTxId":   tx.WithdrawsGrantTxID,
		"nonce":                tx.Nonce,
	})
}

// GetConsentHistoryHandler returns every consent grant +
// withdrawn flag for the subject specified by the `subject`
// query parameter.
func (node *QuidnugNode) GetConsentHistoryHandler(w http.ResponseWriter, r *http.Request) {
	subject := r.URL.Query().Get("subject")
	if !IsValidQuidID(subject) {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid or missing 'subject' query parameter")
		return
	}
	entries := node.ConsentHistoryFor(subject)
	WriteSuccess(w, map[string]interface{}{
		"subjectQuid": subject,
		"entries":     entries,
	})
}

// CreateProcessingRestrictionHandler accepts a signed
// PROCESSING_RESTRICTION transaction.
func (node *QuidnugNode) CreateProcessingRestrictionHandler(w http.ResponseWriter, r *http.Request) {
	var tx ProcessingRestrictionTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	txID, err := node.AddProcessingRestrictionTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	WriteSuccess(w, map[string]interface{}{
		"id":             txID,
		"subjectQuid":    tx.SubjectQuid,
		"restrictedUses": tx.RestrictedUses,
		"effectiveUntil": tx.EffectiveUntil,
		"nonce":          tx.Nonce,
	})
}

// GetRestrictionsForSubjectHandler returns the union of
// currently-active restricted uses for the subject quid in the
// URL path.
func (node *QuidnugNode) GetRestrictionsForSubjectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	subject := vars["subjectQuid"]
	if !IsValidQuidID(subject) {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid subject quid")
		return
	}
	uses := node.RestrictedUsesFor(subject)
	WriteSuccess(w, map[string]interface{}{
		"subjectQuid":    subject,
		"restrictedUses": uses,
	})
}

// CreateDSRComplianceHandler accepts an operator-signed
// DSR_COMPLIANCE attestation.
func (node *QuidnugNode) CreateDSRComplianceHandler(w http.ResponseWriter, r *http.Request) {
	var tx DSRComplianceTransaction
	if err := DecodeJSONBody(w, r, &tx); err != nil {
		return
	}
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	txID, err := node.AddDSRComplianceTransaction(tx)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	WriteSuccess(w, map[string]interface{}{
		"id":               txID,
		"requestTxId":      tx.RequestTxID,
		"operatorQuid":     tx.OperatorQuid,
		"completedAt":      tx.CompletedAt,
		"actionsCategory":  tx.ActionsCategory,
		"carveOutsApplied": tx.CarveOutsApplied,
		"nonce":            tx.Nonce,
	})
}
