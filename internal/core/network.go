package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DiscoverNodes is the main discovery loop. ENG-74 + ENG-76:
//
//   - The initial cycle runs immediately at boot. If it doesn't
//     reach any seed (typical when Docker's embedded DNS hasn't
//     registered the other containers' service names yet), we
//     retry with exponential backoff (1s, 2s, 4s, 8s, 16s, 30s)
//     instead of waiting the full 5-minute steady-state interval.
//     Once a cycle reaches at least one seed, we drop into the
//     5-minute cadence.
//   - Every cycle emits a single INFO summary line with seed
//     reachability, new-peer count, total-known count, and a
//     cap-5 list of failure tags. Per-seed WARN logs continue
//     to fire too.
func (node *QuidnugNode) DiscoverNodes(ctx context.Context, seedNodes []string) {
	logger.Info("Starting node discovery", "seedNodes", len(seedNodes))
	if len(seedNodes) == 0 {
		logger.Info("Node discovery idle (no seeds configured)")
		<-ctx.Done()
		logger.Info("Node discovery stopped")
		return
	}

	// Phase 1: aggressive early-boot retry until we reach at
	// least one seed. ENG-74.
	earlyBackoff := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
	}
	reachedAny := false
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			logger.Info("Node discovery stopped")
			return
		default:
		}
		res := node.discoverFromSeeds(ctx, seedNodes)
		logDiscoveryCycle(res, i == 0)
		if res.SeedsReachable > 0 {
			reachedAny = true
			break
		}
		// Cap at the slowest backoff, then keep retrying
		// at that cadence until we reach a seed or hit the
		// steady-state ticker. Operators almost never want
		// to wait 5 full minutes for a recovery, so we
		// stay in the fast loop until we succeed.
		var d time.Duration
		if i < len(earlyBackoff) {
			d = earlyBackoff[i]
		} else {
			d = earlyBackoff[len(earlyBackoff)-1]
		}
		select {
		case <-ctx.Done():
			logger.Info("Node discovery stopped")
			return
		case <-time.After(d):
		}
	}
	_ = reachedAny

	// Phase 2: steady-state re-discovery every 5 minutes.
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Node discovery stopped")
			return
		case <-ticker.C:
			res := node.discoverFromSeeds(ctx, seedNodes)
			logDiscoveryCycle(res, false)
		}
	}
}

// logDiscoveryCycle emits one structured INFO line summarizing
// a discovery cycle's outcome. ENG-76. We log on success too
// (not just the per-seed WARN on failure) so operators can
// confirm peering is actually working.
func logDiscoveryCycle(res discoverCycleResult, initial bool) {
	tag := "Discovery cycle complete"
	if initial {
		tag = "Initial discovery cycle complete"
	}
	fields := []any{
		"seedsTried", res.SeedsTried,
		"seedsReachable", res.SeedsReachable,
		"newPeers", res.NewPeers,
		"totalKnown", res.TotalKnown,
	}
	if len(res.Errors) > 0 {
		fields = append(fields, "errors", strings.Join(res.Errors, "; "))
	}
	if res.SeedsTried > 0 && res.SeedsReachable == 0 {
		// All seeds failed — still INFO (the per-seed WARNs
		// already shouted), but the cycle summary is the
		// canonical "did peering work" signal.
		logger.Info(tag+" (no seeds reachable)", fields...)
	} else {
		logger.Info(tag, fields...)
	}
}

// discoverCycleResult is the per-round outcome that DiscoverNodes
// uses for observability + retry decisions. Returned from
// discoverFromSeeds so the caller can log a single
// human-readable summary line per cycle (ENG-76) and decide
// whether to back off rapidly (ENG-74).
type discoverCycleResult struct {
	SeedsTried     int
	SeedsReachable int
	NewPeers       int
	TotalKnown     int
	Errors         []string // one-line summaries; capped to a few
}

// discoverFromSeeds performs a single discovery round from seed nodes
func (node *QuidnugNode) discoverFromSeeds(ctx context.Context, seedNodes []string) discoverCycleResult {
	res := discoverCycleResult{SeedsTried: len(seedNodes)}
	addErr := func(s string) {
		// Cap the error list so a hundred dead seeds don't
		// flood the log line. The full per-seed warns above
		// still go through the logger.
		if len(res.Errors) < 5 {
			res.Errors = append(res.Errors, s)
		}
	}
	for _, seedAddress := range seedNodes {
		select {
		case <-ctx.Done():
			return res
		default:
		}

		// SSRF gate: seedNodes is operator-supplied config and is
		// generally trusted, but we run the same sanitization as
		// the gossip-discovered peers so the dial cannot be aimed
		// at internal infrastructure even if config is wrong or
		// poisoned. ENG-79: use the node-method variant so seed
		// addresses listed with allow_private (e.g. LAN seeds)
		// don't get rejected by the global filter.
		safeAddr, err := node.validatePeerAddress(seedAddress)
		if err != nil {
			logger.Warn("Refusing seed with invalid address", "seedAddress", seedAddress, "error", err)
			addErr(seedAddress + ": invalid")
			continue
		}

		// Create request with context for cancellation
		// ENG-56: discovery must use the v1 API path served by the current router.
		reqURL := fmt.Sprintf("http://%s/api/v1/nodes", safeAddr.String())
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil) // #nosec -- reqURL built from sanitized address (see ValidatePeerAddress + safeDialContext)
		if err != nil {
			logger.Warn("Failed to create request for seed node", "seedAddress", seedAddress, "error", err)
			addErr(seedAddress + ": req-build")
			continue
		}

		resp, err := node.httpClient.Do(req) // #nosec -- request URL built from sanitized address; transport also enforces safeDialContext
		if err != nil {
			logger.Warn("Failed to connect to seed node", "seedAddress", seedAddress, "error", err)
			addErr(seedAddress + ": " + classifyDialError(err))
			continue
		}

		var nodesResponse struct {
			Nodes []Node `json:"nodes"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&nodesResponse); err != nil {
			_ = resp.Body.Close()
			logger.Warn("Failed to decode node list", "seedAddress", seedAddress, "error", err)
			addErr(seedAddress + ": decode")
			continue
		}
		_ = resp.Body.Close()
		res.SeedsReachable++

		// Add discovered nodes to our known nodes list, after
		// each has passed the admit pipeline (handshake +
		// optional NodeAdvertisement check + operator
		// attestation). Peers that fail admission are dropped
		// with a debug-level log; we don't escalate because a
		// single bad peer in a gossip response shouldn't taint
		// the whole batch.
		for _, discoveredNode := range nodesResponse.Nodes {
			select {
			case <-ctx.Done():
				return res
			default:
			}

			candidate := PeerCandidate{
				Address:  discoveredNode.Address,
				NodeQuid: discoveredNode.ID,
				Source:   PeerSourceGossip,
				// Gossip-source peers do NOT get allow_private
				// overrides — that's the whole point of the
				// per-source distinction.
			}
			verdict, err := node.AdmitPeer(ctx, candidate, node.PeerAdmit)
			if err != nil {
				logger.Debug("Refusing gossip-discovered peer",
					"nodeId", discoveredNode.ID,
					"address", discoveredNode.Address,
					"error", err)
				continue
			}

			// Fetch domain info for this node now that we've
			// admitted it.
			domains, err := node.fetchNodeDomains(ctx, discoveredNode.Address)
			if err != nil {
				logger.Debug("Failed to fetch domains from node",
					"nodeId", discoveredNode.ID,
					"address", discoveredNode.Address,
					"error", err)
			} else {
				discoveredNode.TrustDomains = domains
			}

			node.KnownNodesMutex.Lock()
			_, existed := node.KnownNodes[discoveredNode.ID]
			// Don't clobber a static-source entry with a
			// gossip-source entry; the operator's explicit
			// listing wins.
			if existing, ok := node.KnownNodes[discoveredNode.ID]; ok && existing.ConnectionStatus == "static" {
				node.KnownNodesMutex.Unlock()
			} else {
				discoveredNode.LastSeen = time.Now().Unix()
				if discoveredNode.ConnectionStatus == "" {
					discoveredNode.ConnectionStatus = "gossip"
				}
				node.KnownNodes[discoveredNode.ID] = discoveredNode
				node.KnownNodesMutex.Unlock()
			}
			if !existed {
				res.NewPeers++
			}

			// Update domain registry for efficient subdomain lookups
			node.updateDomainRegistry(discoveredNode.ID, discoveredNode.TrustDomains)

			logger.Info("Discovered node",
				"nodeId", discoveredNode.ID,
				"address", discoveredNode.Address,
				"operatorQuid", verdict.OperatorQuid,
				"hasAd", verdict.HasAd,
				"trustEdge", verdict.OpTrustEdge,
				"domains", discoveredNode.TrustDomains)
		}
	}

	// Refresh domain info for all known nodes
	node.refreshKnownNodeDomains(ctx)
	node.KnownNodesMutex.RLock()
	res.TotalKnown = len(node.KnownNodes)
	node.KnownNodesMutex.RUnlock()
	return res
}

// classifyDialError returns a short tag for the most common
// failure modes a discovery dial can hit. Used in the cycle-
// summary log line so operators see "node-2: dns" or "node-3:
// connection-refused" without scrolling through full stack
// traces. Falls through to the raw error.Error() when no
// match.
func classifyDialError(err error) string {
	if err == nil {
		return "ok"
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "no such host"):
		return "dns"
	case strings.Contains(s, "connection refused"):
		return "connection-refused"
	case strings.Contains(s, "i/o timeout"), strings.Contains(s, "deadline exceeded"):
		return "timeout"
	case strings.Contains(s, "safedial: refused"):
		return "blocked-range"
	case strings.Contains(s, "no route to host"):
		return "no-route"
	}
	return "other"
}

// fetchNodeDomains fetches the supported domains from a remote node
func (node *QuidnugNode) fetchNodeDomains(ctx context.Context, nodeAddress string) ([]string, error) {
	// SSRF gate: nodeAddress flows from peer-advertised data via
	// discovery; sanitize before composing the URL. ENG-79: use
	// node-method variant so admitted-with-allow_private peers
	// don't get rejected here.
	safeAddr, err := node.validatePeerAddress(nodeAddress)
	if err != nil {
		return nil, fmt.Errorf("fetchNodeDomains: %w", err)
	}

	reqURL := fmt.Sprintf("http://%s/api/v1/node/domains", safeAddr.String())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil) // #nosec -- reqURL built from sanitized address (see ValidatePeerAddress + safeDialContext)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := node.httpClient.Do(req) // #nosec -- request URL built from sanitized address; transport also enforces safeDialContext
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
//
// ENG-77: this switch must include every concrete transaction type
// the rest of the codebase emits, otherwise transactions of unlisted
// types fall through to the default branch and are silently dropped
// from the broadcast pipeline (every load-gen heartbeat fires the
// warning when EVENTs are dropped). The companion switches in
// GenerateBlock (block_operations.go) and ValidateBlockTiered
// (validation.go) were updated previously to know about EventTransaction;
// this is the third site that was missed at the time.
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
	case EventTransaction:
		domainName = t.TrustDomain
		txType = "event"
	case NodeAdvertisementTransaction:
		domainName = t.TrustDomain
		txType = "node-advertisement"
	case ModerationActionTransaction:
		domainName = t.TrustDomain
		txType = "moderation"
	case DataSubjectRequestTransaction:
		domainName = t.TrustDomain
		txType = "dsr"
	case ConsentGrantTransaction:
		domainName = t.TrustDomain
		txType = "consent-grant"
	case ConsentWithdrawTransaction:
		domainName = t.TrustDomain
		txType = "consent-withdraw"
	case ProcessingRestrictionTransaction:
		domainName = t.TrustDomain
		txType = "processing-restriction"
	case DSRComplianceTransaction:
		domainName = t.TrustDomain
		txType = "dsr-compliance"
	default:
		logger.Warn("Cannot broadcast unknown transaction type",
			"type", fmt.Sprintf("%T", tx))
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
	// SSRF gate: same pattern as queryNode. safeAddr is a distinct
	// type so the taint flow shows the sanitization step.
	// ENG-79: use node-method variant so admitted-with-allow_private
	// peers (static peer with allow_private:true, or mDNS peer
	// admitted on a private subnet) don't get rejected at this dial.
	safeAddr, err := node.validatePeerAddress(targetNode.Address)
	if err != nil {
		logger.Warn("Refusing broadcast to invalid peer address",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"error", err)
		return
	}

	path := fmt.Sprintf("/api/transactions/%s", txType)
	endpoint := fmt.Sprintf("http://%s%s", safeAddr.String(), path)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(txJSON)) // #nosec -- endpoint built from sanitized address (see ValidatePeerAddress + safeDialContext)
	if err != nil {
		logger.Warn("Failed to create broadcast request",
			"targetNodeId", targetNode.ID,
			"targetAddress", safeAddr.String(),
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

	resp, err := node.httpClient.Do(req) // #nosec -- request URL built from sanitized address; transport also enforces safeDialContext
	if err != nil {
		logger.Warn("Failed to broadcast transaction to node",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"error", err)
		node.recordPeerScore(targetNode.ID, EventClassBroadcast, false, "dial: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Debug("Successfully broadcast transaction to node",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"status", resp.StatusCode)
		node.recordPeerScore(targetNode.ID, EventClassBroadcast, true, "")
	} else {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("Node rejected broadcast transaction",
			"targetNodeId", targetNode.ID,
			"targetAddress", targetNode.Address,
			"status", resp.StatusCode,
			"response", string(body))
		node.recordPeerScore(targetNode.ID, EventClassBroadcast, false,
			fmt.Sprintf("status %d", resp.StatusCode))
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

	// Phase 4d routing preference: filter out quarantined
	// peers and sort the rest by composite score descending.
	// Highest-quality peers get tried first; quarantined peers
	// don't get tried at all unless every non-quarantined one
	// has already failed (then we fall back).
	domainManagers = node.preferByScore(domainManagers)

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

// preferByScore returns the same nodes filtered + sorted by
// peer-quality score: quarantined peers excluded, remaining
// sorted by composite descending (ties by ID). When the
// scoreboard is nil (typical in tests) returns the input
// unchanged. Phase 4d.
func (node *QuidnugNode) preferByScore(nodes []Node) []Node {
	if node.PeerScoreboard == nil {
		return nodes
	}
	type rankedPeer struct {
		node  Node
		score float64
	}
	ranked := make([]rankedPeer, 0, len(nodes))
	for _, n := range nodes {
		if node.PeerScoreboard.IsQuarantined(n.ID) {
			continue
		}
		ranked = append(ranked, rankedPeer{
			node:  n,
			score: node.PeerScoreboard.Composite(n.ID),
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].node.ID < ranked[j].node.ID
	})
	out := make([]Node, len(ranked))
	for i, r := range ranked {
		out[i] = r.node
	}
	return out
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

	sort.Slice(domainManagers, func(i, j int) bool {
		return domainManagers[i].ID < domainManagers[j].ID
	})

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
	sortedIDs := make([]string, 0, len(nodeIDs))
	for id := range nodeIDs {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	var subdomainNodes []Node
	node.KnownNodesMutex.RLock()
	for _, nodeID := range sortedIDs {
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
	// SSRF gate (same pattern as queryNode/broadcastToNode).
	// ENG-79: use node-method variant so the per-peer allow_private
	// override is honored here too.
	safeAddr, err := node.validatePeerAddress(targetNode.Address)
	if err != nil {
		logger.Debug("Refusing gossip to invalid peer address",
			"targetNodeId", targetNode.ID, "error", err)
		return
	}

	path := "/api/v1/gossip/domains"
	endpoint := fmt.Sprintf("http://%s%s", safeAddr.String(), path)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(gossipJSON)) // #nosec -- endpoint built from sanitized address (see ValidatePeerAddress + safeDialContext)
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

	resp, err := node.httpClient.Do(req) // #nosec -- request URL built from sanitized address; transport also enforces safeDialContext
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
	// SSRF gate: validate the peer-advertised address before
	// composing a URL with it. The httpClient's safeDialContext
	// is the authoritative defense, but we sanitize here too so
	// taint-analyzing scanners (gosec G107 / CodeQL ssrf) see
	// the gate, and so a malformed address fails fast with a
	// clear error rather than a dial failure.
	//
	// safeAddr.String() is used in URL composition below in place
	// of targetNode.Address; the distinct return type makes the
	// sanitization step visible to taint analysis.
	// ENG-79: use node-method variant so admitted-with-allow_private
	// peers don't get rejected at this dial when on a private subnet.
	safeAddr, err := node.validatePeerAddress(targetNode.Address)
	if err != nil {
		return nil, fmt.Errorf("queryNode: %w", err)
	}

	path := fmt.Sprintf("/api/domains/%s/query", url.PathEscape(domainName))
	queryString := fmt.Sprintf("type=%s&param=%s", url.QueryEscape(queryType), url.QueryEscape(queryParam))
	endpoint := fmt.Sprintf("http://%s%s?%s", safeAddr.String(), path, queryString)

	logger.Debug("Querying node for domain",
		"targetNodeId", targetNode.ID,
		"targetAddress", safeAddr.String(),
		"domain", domainName,
		"queryType", queryType,
		"queryParam", queryParam)

	req, err := http.NewRequest("GET", endpoint, nil) // #nosec -- endpoint built from sanitized address (see ValidatePeerAddress + safeDialContext)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for node %s: %w", safeAddr.String(), err)
	}

	// Add authentication headers if secret is configured
	if secret := GetNodeAuthSecret(); secret != "" {
		timestamp := time.Now().Unix()
		// For GET requests, body is empty
		signature := SignRequest("GET", path, nil, secret, timestamp)
		req.Header.Set(NodeSignatureHeader, signature)
		req.Header.Set(NodeTimestampHeader, strconv.FormatInt(timestamp, 10))
	}

	resp, err := node.httpClient.Do(req) // #nosec -- request URL built from sanitized address; transport also enforces safeDialContext
	if err != nil {
		node.recordPeerScore(targetNode.ID, EventClassQuery, false, "dial: "+err.Error())
		return nil, fmt.Errorf("failed to connect to node %s: %w", safeAddr.String(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		node.recordPeerScore(targetNode.ID, EventClassQuery, false, "read body: "+err.Error())
		return nil, fmt.Errorf("failed to read response from node %s: %w", targetNode.Address, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		node.recordPeerScore(targetNode.ID, EventClassQuery, false,
			fmt.Sprintf("status %d", resp.StatusCode))
		return nil, fmt.Errorf("node %s returned status %d: %s", targetNode.Address, resp.StatusCode, string(body))
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		node.recordPeerScore(targetNode.ID, EventClassQuery, false, "decode: "+err.Error())
		return nil, fmt.Errorf("failed to parse response from node %s: %w", targetNode.Address, err)
	}

	node.recordPeerScore(targetNode.ID, EventClassQuery, true, "")
	return result, nil
}
