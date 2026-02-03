package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DiscoverNodes discovers other nodes in the network with context support for cancellation
func (node *QuidnugNode) DiscoverNodes(ctx context.Context, seedNodes []string) {
	logger.Info("Starting node discovery", "seedNodes", len(seedNodes))

	// Initial discovery
	node.discoverFromSeeds(ctx, seedNodes)

	// Periodic re-discovery every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Node discovery stopped")
			return
		case <-ticker.C:
			node.discoverFromSeeds(ctx, seedNodes)
		}
	}
}

// discoverFromSeeds performs a single discovery round from seed nodes
func (node *QuidnugNode) discoverFromSeeds(ctx context.Context, seedNodes []string) {
	for _, seedAddress := range seedNodes {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Create request with context for cancellation
		reqURL := fmt.Sprintf("http://%s/api/nodes", seedAddress)
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			logger.Warn("Failed to create request for seed node", "seedAddress", seedAddress, "error", err)
			continue
		}

		resp, err := node.httpClient.Do(req)
		if err != nil {
			logger.Warn("Failed to connect to seed node", "seedAddress", seedAddress, "error", err)
			continue
		}

		var nodesResponse struct {
			Nodes []Node `json:"nodes"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&nodesResponse); err != nil {
			resp.Body.Close()
			logger.Warn("Failed to decode node list", "seedAddress", seedAddress, "error", err)
			continue
		}
		resp.Body.Close()

		// Add discovered nodes to our known nodes list and fetch their domain info
		for _, discoveredNode := range nodesResponse.Nodes {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Fetch domain info for this node
			domains, err := node.fetchNodeDomains(ctx, discoveredNode.Address)
			if err != nil {
				logger.Debug("Failed to fetch domains from node", "nodeId", discoveredNode.ID, "address", discoveredNode.Address, "error", err)
			} else {
				discoveredNode.TrustDomains = domains
			}

			node.KnownNodesMutex.Lock()
			node.KnownNodes[discoveredNode.ID] = discoveredNode
			node.KnownNodesMutex.Unlock()

			logger.Info("Discovered node", "nodeId", discoveredNode.ID, "address", discoveredNode.Address, "domains", discoveredNode.TrustDomains)
		}
	}

	// Refresh domain info for all known nodes
	node.refreshKnownNodeDomains(ctx)
}

// fetchNodeDomains fetches the supported domains from a remote node
func (node *QuidnugNode) fetchNodeDomains(ctx context.Context, nodeAddress string) ([]string, error) {
	reqURL := fmt.Sprintf("http://%s/api/v1/node/domains", nodeAddress)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := node.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received status %d", resp.StatusCode)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			NodeID  string   `json:"nodeId"`
			Domains []string `json:"domains"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("unsuccessful response from node")
	}

	return response.Data.Domains, nil
}

// refreshKnownNodeDomains refreshes domain info for all known nodes
func (node *QuidnugNode) refreshKnownNodeDomains(ctx context.Context) {
	node.KnownNodesMutex.RLock()
	nodesCopy := make([]Node, 0, len(node.KnownNodes))
	for _, n := range node.KnownNodes {
		nodesCopy = append(nodesCopy, n)
	}
	node.KnownNodesMutex.RUnlock()

	for _, knownNode := range nodesCopy {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Skip self
		if knownNode.ID == node.NodeID {
			continue
		}

		domains, err := node.fetchNodeDomains(ctx, knownNode.Address)
		if err != nil {
			logger.Debug("Failed to refresh domains for node", "nodeId", knownNode.ID, "error", err)
			continue
		}

		node.KnownNodesMutex.Lock()
		if existingNode, exists := node.KnownNodes[knownNode.ID]; exists {
			existingNode.TrustDomains = domains
			existingNode.LastSeen = time.Now().Unix()
			node.KnownNodes[knownNode.ID] = existingNode
		}
		node.KnownNodesMutex.Unlock()

		logger.Debug("Refreshed domains for node", "nodeId", knownNode.ID, "domains", domains)
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
	path := fmt.Sprintf("/api/transactions/%s", txType)
	endpoint := fmt.Sprintf("http://%s%s", targetNode.Address, path)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(txJSON))
	if err != nil {
		logger.Warn("Failed to create broadcast request",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Add authentication headers if secret is configured
	if secret := GetNodeAuthSecret(); secret != "" {
		timestamp := time.Now().Unix()
		signature := SignRequest("POST", path, txJSON, secret, timestamp)
		req.Header.Set(NodeSignatureHeader, signature)
		req.Header.Set(NodeTimestampHeader, strconv.FormatInt(timestamp, 10))
	}

	resp, err := node.httpClient.Do(req)
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
	path := fmt.Sprintf("/api/domains/%s/query", url.PathEscape(domainName))
	queryString := fmt.Sprintf("type=%s&param=%s", url.QueryEscape(queryType), url.QueryEscape(queryParam))
	endpoint := fmt.Sprintf("http://%s%s?%s", targetNode.Address, path, queryString)

	logger.Debug("Querying node for domain",
		"targetNodeId", targetNode.ID,
		"targetAddress", targetNode.Address,
		"domain", domainName,
		"queryType", queryType,
		"queryParam", queryParam)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for node %s: %w", targetNode.Address, err)
	}

	// Add authentication headers if secret is configured
	if secret := GetNodeAuthSecret(); secret != "" {
		timestamp := time.Now().Unix()
		// For GET requests, body is empty
		signature := SignRequest("GET", path, nil, secret, timestamp)
		req.Header.Set(NodeSignatureHeader, signature)
		req.Header.Set(NodeTimestampHeader, strconv.FormatInt(timestamp, 10))
	}

	resp, err := node.httpClient.Do(req)
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
