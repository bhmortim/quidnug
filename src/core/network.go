package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// DiscoverNodes discovers other nodes in the network
func (node *QuidnugNode) DiscoverNodes(seedNodes []string) {
	for _, seedAddress := range seedNodes {
		// Make HTTP request to the seed node's discovery endpoint
		resp, err := http.Get(fmt.Sprintf("http://%s/api/nodes", seedAddress))
		if err != nil {
			logger.Warn("Failed to connect to seed node", "seedAddress", seedAddress, "error", err)
			continue
		}
		defer resp.Body.Close()

		var nodesResponse struct {
			Nodes []Node `json:"nodes"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&nodesResponse); err != nil {
			logger.Warn("Failed to decode node list", "seedAddress", seedAddress, "error", err)
			continue
		}

		// Add discovered nodes to our known nodes list
		node.KnownNodesMutex.Lock()
		for _, discoveredNode := range nodesResponse.Nodes {
			node.KnownNodes[discoveredNode.ID] = discoveredNode
			logger.Info("Discovered node", "nodeId", discoveredNode.ID, "address", discoveredNode.Address)
		}
		node.KnownNodesMutex.Unlock()
	}
}

// GetTrustDomainNodes gets nodes from a specific trust domain
func (node *QuidnugNode) GetTrustDomainNodes(domainName string) []Node {
	var domainNodes []Node

	node.TrustDomainsMutex.RLock()
	domain, exists := node.TrustDomains[domainName]
	node.TrustDomainsMutex.RUnlock()

	if !exists {
		return domainNodes
	}

	node.KnownNodesMutex.RLock()
	defer node.KnownNodesMutex.RUnlock()

	for _, validatorID := range domain.ValidatorNodes {
		if knownNode, exists := node.KnownNodes[validatorID]; exists {
			domainNodes = append(domainNodes, knownNode)
		}
	}

	return domainNodes
}

// BroadcastTransaction broadcasts a transaction to other nodes in the trust domain
func (node *QuidnugNode) BroadcastTransaction(tx interface{}) {
	// Extract trust domain based on transaction type
	var domainName string

	switch t := tx.(type) {
	case TrustTransaction:
		domainName = t.TrustDomain
	case IdentityTransaction:
		domainName = t.TrustDomain
	case TitleTransaction:
		domainName = t.TrustDomain
	default:
		logger.Warn("Cannot broadcast unknown transaction type")
		return
	}

	if domainName == "" {
		domainName = "default"
	}

	// Get nodes in this trust domain
	domainNodes := node.GetTrustDomainNodes(domainName)

	// Broadcast to each node
	for _, targetNode := range domainNodes {
		if targetNode.ID == node.NodeID {
			continue // Skip broadcasting to self
		}

		// Convert transaction to JSON
		txJSON, err := json.Marshal(tx)
		if err != nil {
			logger.Error("Failed to marshal transaction", "error", err)
			continue
		}

		// In a real implementation, this would make an HTTP POST request
		// to the target node's transaction endpoint
		logger.Debug("Broadcasting transaction to node",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"domain", domainName)
		_ = txJSON // Silence unused variable warning
	}
}

// QueryOtherDomain recursively queries other trust domains
func (node *QuidnugNode) QueryOtherDomain(domainName, queryType, queryParam string) (interface{}, error) {
	// Find nodes that manage this trust domain
	var domainManagers []Node

	node.KnownNodesMutex.RLock()
	for _, knownNode := range node.KnownNodes {
		for _, domain := range knownNode.TrustDomains {
			if domain == domainName {
				domainManagers = append(domainManagers, knownNode)
				break
			}
		}
	}
	node.KnownNodesMutex.RUnlock()

	if len(domainManagers) == 0 {
		return nil, fmt.Errorf("no known nodes manage trust domain: %s", domainName)
	}

	// For simplicity, query the first node that manages this domain
	targetNode := domainManagers[0]

	// In a real implementation, this would make an HTTP request to the
	// target node's query endpoint with the appropriate query parameters
	logger.Debug("Querying node for domain",
		"targetNodeId", targetNode.ID,
		"targetAddress", targetNode.Address,
		"domain", domainName,
		"queryType", queryType,
		"queryParam", queryParam)

	// Mock response for demonstration
	switch queryType {
	case "identity":
		// Mocked identity query response
		return map[string]interface{}{
			"quid_id":    queryParam,
			"name":       "Sample Quid Name",
			"domain":     domainName,
			"attributes": map[string]interface{}{"key": "value"},
		}, nil

	case "trust":
		// Mocked trust query response
		return map[string]interface{}{
			"truster":     "truster_id",
			"trustee":     queryParam,
			"trust_level": 0.85,
			"domain":      domainName,
		}, nil

	case "title":
		// Mocked title query response
		return map[string]interface{}{
			"asset_id": queryParam,
			"domain":   domainName,
			"owners": []map[string]interface{}{
				{"owner_id": "owner1", "percentage": 60.0},
				{"owner_id": "owner2", "percentage": 40.0},
			},
		}, nil

	default:
		return map[string]interface{}{
			"error": "Unknown query type",
		}, nil
	}
}
