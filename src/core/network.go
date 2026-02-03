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

			// Update domain registry for efficient subdomain lookups
			node.updateDomainRegistry(discoveredNode.ID, discoveredNode.TrustDomains)

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

		// Update domain registry for efficient subdomain lookups
		node.updateDomainRegistry(knownNode.ID, domains)

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

// QueryOtherDomain queries other trust domains with hierarchical domain walking.
// First tries exact match and parent domains, then falls back to subdomain nodes.
func (node *QuidnugNode) QueryOtherDomain(domainName, queryType, queryParam string) (interface{}, error) {
	// First try exact match and parent domains (walking up the hierarchy)
	domainManagers := node.findNodesForDomainWithHierarchy(domainName)

	// If no nodes found via parent hierarchy, try subdomain nodes
	if len(domainManagers) == 0 {
		domainManagers = node.findNodesForSubdomains(domainName)
		if len(domainManagers) > 0 {
			logger.Debug("Found nodes via subdomain discovery",
				"requestedDomain", domainName,
				"nodeCount", len(domainManagers))
		}
	}

	if len(domainManagers) == 0 {
		return nil, fmt.Errorf("no known nodes manage trust domain: %s (or any related domain)", domainName)
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

// findNodesForSubdomains finds nodes that manage any subdomain of the given domain.
// For example, if domainName is "example.com", this finds nodes managing
// "api.example.com", "auth.example.com", "deep.sub.example.com", etc.
func (node *QuidnugNode) findNodesForSubdomains(domainName string) []Node {
	suffix := "." + domainName
	nodeIDs := make(map[string]bool)

	// Use domain registry for efficient lookup
	node.DomainRegistryMutex.RLock()
	for domain, ids := range node.DomainRegistry {
		if strings.HasSuffix(domain, suffix) {
			for _, id := range ids {
				nodeIDs[id] = true
			}
		}
	}
	node.DomainRegistryMutex.RUnlock()

	// Convert node IDs to Node objects
	var subdomainNodes []Node
	node.KnownNodesMutex.RLock()
	for nodeID := range nodeIDs {
		if knownNode, exists := node.KnownNodes[nodeID]; exists {
			subdomainNodes = append(subdomainNodes, knownNode)
		}
	}
	node.KnownNodesMutex.RUnlock()

	return subdomainNodes
}

// updateDomainRegistry updates the domain-to-nodes registry for a node.
// This maintains a reverse index for efficient subdomain lookups.
func (node *QuidnugNode) updateDomainRegistry(nodeID string, domains []string) {
	node.DomainRegistryMutex.Lock()
	defer node.DomainRegistryMutex.Unlock()

	// Remove node from all existing domain entries
	for domain, nodeIDs := range node.DomainRegistry {
		filtered := make([]string, 0, len(nodeIDs))
		for _, id := range nodeIDs {
			if id != nodeID {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) > 0 {
			node.DomainRegistry[domain] = filtered
		} else {
			delete(node.DomainRegistry, domain)
		}
	}

	// Add node to its current domains
	for _, domain := range domains {
		node.DomainRegistry[domain] = append(node.DomainRegistry[domain], nodeID)
	}
}

// runDomainGossip periodically broadcasts domain information to known nodes
func (node *QuidnugNode) runDomainGossip(ctx context.Context, interval time.Duration) {
	logger.Info("Starting domain gossip loop", "interval", interval)

	// Initial broadcast after a short delay
	select {
	case <-ctx.Done():
		logger.Info("Domain gossip loop stopped before initial broadcast")
		return
	case <-time.After(5 * time.Second):
		node.BroadcastDomainInfo()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Periodically clean up old gossip seen entries
	cleanupTicker := time.NewTicker(10 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Domain gossip loop stopped")
			return
		case <-ticker.C:
			node.BroadcastDomainInfo()
		case <-cleanupTicker.C:
			node.cleanupGossipSeen()
		}
	}
}

// BroadcastDomainInfo creates and sends a domain gossip message to all known nodes
func (node *QuidnugNode) BroadcastDomainInfo() {
	gossip := node.createDomainGossip()
	if gossip == nil {
		return
	}

	node.KnownNodesMutex.RLock()
	nodes := make([]Node, 0, len(node.KnownNodes))
	for _, n := range node.KnownNodes {
		if n.ID != node.NodeID {
			nodes = append(nodes, n)
		}
	}
	node.KnownNodesMutex.RUnlock()

	if len(nodes) == 0 {
		logger.Debug("No known nodes to broadcast domain gossip to")
		return
	}

	// Mark as seen before broadcasting
	node.markGossipSeen(gossip.MessageID)

	gossipJSON, err := json.Marshal(gossip)
	if err != nil {
		logger.Error("Failed to marshal domain gossip", "error", err)
		return
	}

	logger.Debug("Broadcasting domain info", "domains", gossip.Domains, "ttl", gossip.TTL, "targetNodes", len(nodes))

	for _, targetNode := range nodes {
		go node.sendDomainGossip(targetNode, gossipJSON)
	}
}

// createDomainGossip creates a new domain gossip message for this node
func (node *QuidnugNode) createDomainGossip() *DomainGossip {
	domains := node.SupportedDomains
	if len(domains) == 0 {
		// If no explicit supported domains, include managed trust domains
		node.TrustDomainsMutex.RLock()
		for domain := range node.TrustDomains {
			domains = append(domains, domain)
		}
		node.TrustDomainsMutex.RUnlock()
	}

	if len(domains) == 0 {
		logger.Debug("No domains to gossip about")
		return nil
	}

	timestamp := time.Now().Unix()
	messageID := fmt.Sprintf("%s:%d:%x", node.NodeID, timestamp, time.Now().UnixNano())

	return &DomainGossip{
		NodeID:    node.NodeID,
		Domains:   domains,
		Timestamp: timestamp,
		TTL:       node.GossipTTL,
		HopCount:  0,
		MessageID: messageID,
	}
}

// sendDomainGossip sends a gossip message to a single node
func (node *QuidnugNode) sendDomainGossip(targetNode Node, gossipJSON []byte) {
	path := "/api/v1/gossip/domains"
	endpoint := fmt.Sprintf("http://%s%s", targetNode.Address, path)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(gossipJSON))
	if err != nil {
		logger.Debug("Failed to create gossip request", "targetNodeId", targetNode.ID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Add authentication headers if secret is configured
	if secret := GetNodeAuthSecret(); secret != "" {
		timestamp := time.Now().Unix()
		signature := SignRequest("POST", path, gossipJSON, secret, timestamp)
		req.Header.Set(NodeSignatureHeader, signature)
		req.Header.Set(NodeTimestampHeader, strconv.FormatInt(timestamp, 10))
	}

	resp, err := node.httpClient.Do(req)
	if err != nil {
		logger.Debug("Failed to send domain gossip", "targetNodeId", targetNode.ID, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Debug("Successfully sent domain gossip", "targetNodeId", targetNode.ID)
	} else {
		logger.Debug("Node rejected domain gossip", "targetNodeId", targetNode.ID, "status", resp.StatusCode)
	}
}

// ReceiveDomainGossip processes an incoming domain gossip message
func (node *QuidnugNode) ReceiveDomainGossip(gossip DomainGossip) error {
	// Check if we've already seen this message
	if node.hasSeenGossip(gossip.MessageID) {
		logger.Debug("Ignoring duplicate gossip", "messageId", gossip.MessageID, "fromNode", gossip.NodeID)
		return nil
	}

	// Mark as seen
	node.markGossipSeen(gossip.MessageID)

	// Validate gossip
	if gossip.NodeID == "" || gossip.NodeID == node.NodeID {
		return fmt.Errorf("invalid gossip: empty or self nodeId")
	}

	if gossip.TTL < 0 {
		return fmt.Errorf("invalid gossip: negative TTL")
	}

	// Process the domain information
	node.processDomainGossip(gossip)

	// Forward if TTL allows
	if gossip.TTL > 0 {
		node.forwardDomainGossip(gossip)
	}

	return nil
}

// processDomainGossip updates the domain registry with gossip information
func (node *QuidnugNode) processDomainGossip(gossip DomainGossip) {
	// Update known node with domain information
	node.KnownNodesMutex.Lock()
	if existingNode, exists := node.KnownNodes[gossip.NodeID]; exists {
		existingNode.TrustDomains = gossip.Domains
		existingNode.LastSeen = time.Now().Unix()
		node.KnownNodes[gossip.NodeID] = existingNode
	} else {
		// Add new node entry (we don't know the address yet)
		node.KnownNodes[gossip.NodeID] = Node{
			ID:               gossip.NodeID,
			TrustDomains:     gossip.Domains,
			LastSeen:         time.Now().Unix(),
			ConnectionStatus: "discovered-via-gossip",
		}
	}
	node.KnownNodesMutex.Unlock()

	// Update domain registry for efficient subdomain lookups
	node.updateDomainRegistry(gossip.NodeID, gossip.Domains)

	logger.Debug("Processed domain gossip",
		"fromNode", gossip.NodeID,
		"domains", gossip.Domains,
		"hopCount", gossip.HopCount)
}

// forwardDomainGossip forwards a gossip message to other known nodes
func (node *QuidnugNode) forwardDomainGossip(gossip DomainGossip) {
	// Decrement TTL and increment hop count
	forwardGossip := DomainGossip{
		NodeID:    gossip.NodeID,
		Domains:   gossip.Domains,
		Timestamp: gossip.Timestamp,
		TTL:       gossip.TTL - 1,
		HopCount:  gossip.HopCount + 1,
		MessageID: gossip.MessageID,
	}

	node.KnownNodesMutex.RLock()
	nodes := make([]Node, 0, len(node.KnownNodes))
	for _, n := range node.KnownNodes {
		// Don't forward back to the originating node or to self
		if n.ID != node.NodeID && n.ID != gossip.NodeID && n.Address != "" {
			nodes = append(nodes, n)
		}
	}
	node.KnownNodesMutex.RUnlock()

	if len(nodes) == 0 {
		return
	}

	gossipJSON, err := json.Marshal(forwardGossip)
	if err != nil {
		logger.Error("Failed to marshal forwarded gossip", "error", err)
		return
	}

	logger.Debug("Forwarding domain gossip",
		"originalNode", gossip.NodeID,
		"ttl", forwardGossip.TTL,
		"hopCount", forwardGossip.HopCount,
		"targetNodes", len(nodes))

	for _, targetNode := range nodes {
		go node.sendDomainGossip(targetNode, gossipJSON)
	}
}

// hasSeenGossip checks if a gossip message has already been processed
func (node *QuidnugNode) hasSeenGossip(messageID string) bool {
	node.GossipSeenMutex.RLock()
	defer node.GossipSeenMutex.RUnlock()
	_, seen := node.GossipSeen[messageID]
	return seen
}

// markGossipSeen marks a gossip message as seen
func (node *QuidnugNode) markGossipSeen(messageID string) {
	node.GossipSeenMutex.Lock()
	defer node.GossipSeenMutex.Unlock()
	node.GossipSeen[messageID] = time.Now().Unix()
}

// cleanupGossipSeen removes old entries from the gossip seen map
func (node *QuidnugNode) cleanupGossipSeen() {
	node.GossipSeenMutex.Lock()
	defer node.GossipSeenMutex.Unlock()

	// Remove entries older than 30 minutes
	cutoff := time.Now().Add(-30 * time.Minute).Unix()
	for messageID, timestamp := range node.GossipSeen {
		if timestamp < cutoff {
			delete(node.GossipSeen, messageID)
		}
	}

	logger.Debug("Cleaned up gossip seen map", "remainingEntries", len(node.GossipSeen))
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
