package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	var domainName string
	var txType string

	switch t := tx.(type) {
	case TrustTransaction:
		domainName = t.TrustDomain
		txType = "trust"
	case IdentityTransaction:
		domainName = t.TrustDomain
		txType = "identity"
	case TitleTransaction:
		domainName = t.TrustDomain
		txType = "title"
	default:
		logger.Warn("Cannot broadcast unknown transaction type")
		return
	}

	if domainName == "" {
		domainName = "default"
	}

	domainNodes := node.GetTrustDomainNodes(domainName)

	txJSON, err := json.Marshal(tx)
	if err != nil {
		logger.Error("Failed to marshal transaction for broadcast", "error", err)
		return
	}

	for _, targetNode := range domainNodes {
		if targetNode.ID == node.NodeID {
			continue
		}

		go node.broadcastToNode(targetNode, txType, txJSON)
	}
}

// broadcastToNode sends a transaction to a single node (fire-and-forget)
func (node *QuidnugNode) broadcastToNode(targetNode Node, txType string, txJSON []byte) {
	endpoint := fmt.Sprintf("http://%s/api/transactions/%s", targetNode.Address, txType)

	resp, err := node.httpClient.Post(endpoint, "application/json", bytes.NewReader(txJSON))
	if err != nil {
		logger.Warn("Failed to broadcast transaction to node",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Debug("Successfully broadcast transaction to node",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"status", resp.StatusCode)
	} else {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Node rejected broadcast transaction",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"status", resp.StatusCode,
			"response", string(body))
	}
}

// QueryOtherDomain queries other trust domains with hierarchical domain walking
func (node *QuidnugNode) QueryOtherDomain(domainName, queryType, queryParam string) (interface{}, error) {
	domainManagers := node.findNodesForDomainWithHierarchy(domainName)

	if len(domainManagers) == 0 {
		return nil, fmt.Errorf("no known nodes manage trust domain: %s (or any parent domain)", domainName)
	}

	var lastErr error
	for _, targetNode := range domainManagers {
		result, err := node.queryNode(targetNode, domainName, queryType, queryParam)
		if err == nil {
			return result, nil
		}
		lastErr = err
		logger.Debug("Failed to query node, trying next",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"error", err)
	}

	return nil, fmt.Errorf("all nodes failed for domain %s: %v", domainName, lastErr)
}

// findNodesForDomainWithHierarchy finds nodes managing a domain, walking up the hierarchy if needed
func (node *QuidnugNode) findNodesForDomainWithHierarchy(domainName string) []Node {
	currentDomain := domainName

	for currentDomain != "" {
		managers := node.findNodesForExactDomain(currentDomain)
		if len(managers) > 0 {
			if currentDomain != domainName {
				logger.Debug("Found nodes via parent domain",
					"requestedDomain", domainName,
					"foundDomain", currentDomain)
			}
			return managers
		}

		parts := strings.SplitN(currentDomain, ".", 2)
		if len(parts) < 2 {
			break
		}
		currentDomain = parts[1]
	}

	return nil
}

// findNodesForExactDomain finds nodes that manage exactly the specified domain
func (node *QuidnugNode) findNodesForExactDomain(domainName string) []Node {
	var domainManagers []Node

	node.KnownNodesMutex.RLock()
	defer node.KnownNodesMutex.RUnlock()

	for _, knownNode := range node.KnownNodes {
		for _, domain := range knownNode.TrustDomains {
			if domain == domainName {
				domainManagers = append(domainManagers, knownNode)
				break
			}
		}
	}

	return domainManagers
}

// queryNode performs an HTTP GET query to a specific node
func (node *QuidnugNode) queryNode(targetNode Node, domainName, queryType, queryParam string) (interface{}, error) {
	endpoint := fmt.Sprintf("http://%s/api/domains/%s/query?type=%s&param=%s",
		targetNode.Address,
		url.PathEscape(domainName),
		url.QueryEscape(queryType),
		url.QueryEscape(queryParam))

	logger.Debug("Querying node for domain",
		"targetNodeId", targetNode.ID,
		"targetAddress", targetNode.Address,
		"domain", domainName,
		"queryType", queryType,
		"queryParam", queryParam)

	resp, err := node.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to node %s: %w", targetNode.Address, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from node %s: %w", targetNode.Address, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("node %s returned status %d: %s", targetNode.Address, resp.StatusCode, string(body))
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response from node %s: %w", targetNode.Address, err)
	}

	return result, nil
}
