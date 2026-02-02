package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// StartServer starts the HTTP server for API endpoints
func (node *QuidnugNode) StartServer(port string) error {
	return node.StartServerWithConfig(port, DefaultRateLimitPerMinute, DefaultMaxBodySizeBytes)
}

// StartServerWithConfig starts the HTTP server with configurable middleware settings
func (node *QuidnugNode) StartServerWithConfig(port string, rateLimitPerMinute int, maxBodySizeBytes int64) error {
	router := mux.NewRouter()

	// API endpoints
	router.HandleFunc("/api/health", node.HealthCheckHandler).Methods("GET")
	router.HandleFunc("/api/nodes", node.GetNodesHandler).Methods("GET")

	// Transaction endpoints
	router.HandleFunc("/api/transactions", node.GetTransactionsHandler).Methods("GET")
	router.HandleFunc("/api/transactions/trust", node.CreateTrustTransactionHandler).Methods("POST")
	router.HandleFunc("/api/transactions/identity", node.CreateIdentityTransactionHandler).Methods("POST")
	router.HandleFunc("/api/transactions/title", node.CreateTitleTransactionHandler).Methods("POST")

	// Blockchain endpoints
	router.HandleFunc("/api/blocks", node.GetBlocksHandler).Methods("GET")

	// Trust domain endpoints
	router.HandleFunc("/api/domains", node.GetDomainsHandler).Methods("GET")
	router.HandleFunc("/api/domains", node.RegisterDomainHandler).Methods("POST")
	router.HandleFunc("/api/domains/{name}/query", node.QueryDomainHandler).Methods("GET")

	// Registry query endpoints
	router.HandleFunc("/api/registry/trust", node.QueryTrustRegistryHandler).Methods("GET")
	router.HandleFunc("/api/registry/identity", node.QueryIdentityRegistryHandler).Methods("GET")
	router.HandleFunc("/api/registry/title", node.QueryTitleRegistryHandler).Methods("GET")

	// API spec endpoints
	router.HandleFunc("/api/info", node.GetInfoHandler).Methods("GET")
	router.HandleFunc("/api/quids", node.CreateQuidHandler).Methods("POST")
	router.HandleFunc("/api/trust/{truster}/{trustee}", node.GetTrustHandler).Methods("GET")
	router.HandleFunc("/api/identity/{quidId}", node.GetIdentityHandler).Methods("GET")
	router.HandleFunc("/api/title/{assetId}", node.GetTitleHandler).Methods("GET")

	// Apply middleware: body size limit first, then rate limiting
	rateLimiter := NewIPRateLimiter(rateLimitPerMinute)
	handler := BodySizeLimitMiddleware(maxBodySizeBytes)(router)
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"node_id": node.NodeID,
		"uptime":  time.Now().Unix() - node.Blockchain[0].Timestamp,
		"version": "1.0.0",
	})
}

// GetNodesHandler returns the list of known nodes
func (node *QuidnugNode) GetNodesHandler(w http.ResponseWriter, r *http.Request) {
	node.KnownNodesMutex.RLock()
	defer node.KnownNodesMutex.RUnlock()

	// Convert map to slice for response
	var nodesList []Node
	for _, n := range node.KnownNodes {
		nodesList = append(nodesList, n)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodesList,
	})
}

// GetTransactionsHandler returns pending transactions
func (node *QuidnugNode) GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"pending_transactions": node.PendingTxs,
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"transaction_id": txID,
		"message":        "Title transaction added to pending pool",
	})
}

// GetBlocksHandler returns the blockchain
func (node *QuidnugNode) GetBlocksHandler(w http.ResponseWriter, r *http.Request) {
	node.BlockchainMutex.RLock()
	defer node.BlockchainMutex.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"blocks": node.Blockchain,
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

	json.NewEncoder(w).Encode(map[string]interface{}{
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
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
				http.Error(w, "Identity not found", http.StatusNotFound)
				return
			}
			result = identity

		case "trust":
			// Parse truster and trustee from param (format: "truster:trustee")
			parts := strings.Split(queryParam, ":")
			if len(parts) != 2 {
				http.Error(w, "Invalid trust query format", http.StatusBadRequest)
				return
			}
			trustLevel := node.GetTrustLevel(parts[0], parts[1])
			result = map[string]interface{}{
				"truster":     parts[0],
				"trustee":     parts[1],
				"trust_level": trustLevel,
				"domain":      domainName,
			}

		case "title":
			// Query local title registry
			title, exists := node.GetAssetOwnership(queryParam)
			if !exists {
				http.Error(w, "Title not found", http.StatusNotFound)
				return
			}
			result = title

		default:
			http.Error(w, "Unknown query type", http.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(result)
	} else {
		// Forward query to other domains
		result, err := node.QueryOtherDomain(domainName, queryType, queryParam)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(result)
	}
}

// QueryTrustRegistryHandler handles trust registry queries
func (node *QuidnugNode) QueryTrustRegistryHandler(w http.ResponseWriter, r *http.Request) {
	truster := r.URL.Query().Get("truster")
	trustee := r.URL.Query().Get("trustee")

	if truster != "" && trustee != "" {
		// Query specific trust relationship
		trustLevel := node.GetTrustLevel(truster, trustee)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"truster":     truster,
			"trustee":     trustee,
			"trust_level": trustLevel,
		})
	} else if truster != "" {
		// Query all relationships for a truster
		node.TrustRegistryMutex.RLock()
		relationships := node.TrustRegistry[truster]
		node.TrustRegistryMutex.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"truster":       truster,
			"relationships": relationships,
		})
	} else {
		// Return overview of trust registry
		node.TrustRegistryMutex.RLock()
		defer node.TrustRegistryMutex.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"trust_registry": node.TrustRegistry,
		})
	}
}

// QueryIdentityRegistryHandler handles identity registry queries
func (node *QuidnugNode) QueryIdentityRegistryHandler(w http.ResponseWriter, r *http.Request) {
	quidID := r.URL.Query().Get("quid_id")

	if quidID != "" {
		// Query specific identity
		identity, exists := node.GetQuidIdentity(quidID)
		if !exists {
			http.Error(w, "Identity not found", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(identity)
	} else {
		// Return all identities (could be paginated in a real implementation)
		node.IdentityRegistryMutex.RLock()
		defer node.IdentityRegistryMutex.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"identities": node.IdentityRegistry,
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		http.Error(w, "Failed to generate key pair", http.StatusInternalServerError)
		return
	}

	publicKeyBytes := elliptic.Marshal(privateKey.PublicKey.Curve, privateKey.PublicKey.X, privateKey.PublicKey.Y)
	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	hashBytes := sha256.Sum256(publicKeyBytes)
	quidID := hex.EncodeToString(hashBytes[:])[:16]

	created := time.Now().Unix()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"quidId":    quidID,
		"publicKey": publicKeyHex,
		"created":   created,
	})
}

// GetTrustHandler returns trust level between two quids
func (node *QuidnugNode) GetTrustHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	truster := vars["truster"]
	trustee := vars["trustee"]
	domain := r.URL.Query().Get("domain")

	if domain == "" {
		domain = "default"
	}

	node.TrustRegistryMutex.RLock()
	trustMap, trusterExists := node.TrustRegistry[truster]
	var trustLevel float64
	var relationshipExists bool
	if trusterExists {
		trustLevel, relationshipExists = trustMap[trustee]
	}
	node.TrustRegistryMutex.RUnlock()

	if !relationshipExists {
		http.Error(w, "No trust relationship found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"truster":    truster,
		"trustee":    trustee,
		"domain":     domain,
		"trustLevel": trustLevel,
	})
}

// GetIdentityHandler returns identity information for a quid
func (node *QuidnugNode) GetIdentityHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	quidID := vars["quidId"]

	identity, exists := node.GetQuidIdentity(quidID)
	if !exists {
		http.Error(w, "Identity not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(identity)
}

// GetTitleHandler returns ownership information for an asset
func (node *QuidnugNode) GetTitleHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	assetID := vars["assetId"]

	title, exists := node.GetAssetOwnership(assetID)
	if !exists {
		http.Error(w, "Title not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(title)
}

// QueryTitleRegistryHandler handles title registry queries
func (node *QuidnugNode) QueryTitleRegistryHandler(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("asset_id")
	ownerID := r.URL.Query().Get("owner_id")

	if assetID != "" {
		// Query specific asset title
		title, exists := node.GetAssetOwnership(assetID)
		if !exists {
			http.Error(w, "Title not found", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(title)
	} else if ownerID != "" {
		// Query all assets owned by a specific owner
		node.TitleRegistryMutex.RLock()
		defer node.TitleRegistryMutex.RUnlock()

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

		json.NewEncoder(w).Encode(map[string]interface{}{
			"owner_id":     ownerID,
			"owned_assets": ownedAssets,
		})
	} else {
		// Return overview of title registry
		node.TitleRegistryMutex.RLock()
		defer node.TitleRegistryMutex.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"title_registry": node.TitleRegistry,
		})
	}
}
