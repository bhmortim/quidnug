package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrTrustGraphTooLarge is returned when trust computation exceeds resource limits
var ErrTrustGraphTooLarge = errors.New("trust graph too large: resource limits exceeded")

// TrustCache provides thread-safe caching for trust computations
type TrustCache struct {
	entries         map[string]TrustCacheEntry
	enhancedEntries map[string]EnhancedTrustCacheEntry
	mu              sync.RWMutex
	ttl             time.Duration
}

// NewTrustCache creates a new TrustCache with the specified TTL
func NewTrustCache(ttl time.Duration) *TrustCache {
	return &TrustCache{
		entries:         make(map[string]TrustCacheEntry),
		enhancedEntries: make(map[string]EnhancedTrustCacheEntry),
		ttl:             ttl,
	}
}

// Get retrieves a cached trust computation result
func (c *TrustCache) Get(key string) (float64, []string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists || time.Now().Unix() > entry.ExpiresAt {
		return 0, nil, false
	}
	// Return a copy of the path to prevent mutation
	pathCopy := make([]string, len(entry.TrustPath))
	copy(pathCopy, entry.TrustPath)
	return entry.TrustLevel, pathCopy, true
}

// Set stores a trust computation result in the cache
func (c *TrustCache) Set(key string, trustLevel float64, trustPath []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store a copy of the path
	pathCopy := make([]string, len(trustPath))
	copy(pathCopy, trustPath)

	c.entries[key] = TrustCacheEntry{
		TrustLevel: trustLevel,
		TrustPath:  pathCopy,
		ExpiresAt:  time.Now().Add(c.ttl).Unix(),
	}
}

// GetEnhanced retrieves a cached enhanced trust computation result
func (c *TrustCache) GetEnhanced(key string) (EnhancedTrustResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.enhancedEntries[key]
	if !exists || time.Now().Unix() > entry.ExpiresAt {
		return EnhancedTrustResult{}, false
	}
	// Return a copy to prevent mutation
	result := entry.Result
	result.TrustPath = make([]string, len(entry.Result.TrustPath))
	copy(result.TrustPath, entry.Result.TrustPath)
	result.VerificationGaps = make([]VerificationGap, len(entry.Result.VerificationGaps))
	copy(result.VerificationGaps, entry.Result.VerificationGaps)
	return result, true
}

// SetEnhanced stores an enhanced trust computation result in the cache
func (c *TrustCache) SetEnhanced(key string, result EnhancedTrustResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store copies of slices
	resultCopy := result
	resultCopy.TrustPath = make([]string, len(result.TrustPath))
	copy(resultCopy.TrustPath, result.TrustPath)
	resultCopy.VerificationGaps = make([]VerificationGap, len(result.VerificationGaps))
	copy(resultCopy.VerificationGaps, result.VerificationGaps)

	c.enhancedEntries[key] = EnhancedTrustCacheEntry{
		Result:    resultCopy,
		ExpiresAt: time.Now().Add(c.ttl).Unix(),
	}
}

// Invalidate clears all cached entries
func (c *TrustCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]TrustCacheEntry)
	c.enhancedEntries = make(map[string]EnhancedTrustCacheEntry)
}

// Size returns the number of entries in both caches
func (c *TrustCache) Size() (basic int, enhanced int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries), len(c.enhancedEntries)
}

// makeTrustCacheKey creates a cache key for basic trust computation
func makeTrustCacheKey(observer, target string, maxDepth int) string {
	return fmt.Sprintf("%s:%s:%d", observer, target, maxDepth)
}

// makeEnhancedTrustCacheKey creates a cache key for enhanced trust computation
func makeEnhancedTrustCacheKey(observer, target string, maxDepth int, includeUnverified bool) string {
	return fmt.Sprintf("%s:%s:%d:%t", observer, target, maxDepth, includeUnverified)
}

// processBlockTransactions processes transactions in a block to update registries
func (node *QuidnugNode) processBlockTransactions(block Block) {
	for _, txInterface := range block.Transactions {
		txJson, err := json.Marshal(txInterface)
		if err != nil {
			logger.Error("Failed to marshal transaction in block", "blockIndex", block.Index, "error", err)
			continue
		}

		// Determine transaction type
		var baseTx BaseTransaction
		if err := json.Unmarshal(txJson, &baseTx); err != nil {
			logger.Error("Failed to unmarshal base transaction", "blockIndex", block.Index, "error", err)
			continue
		}

		switch baseTx.Type {
		case TxTypeTrust:
			var tx TrustTransaction
			if err := json.Unmarshal(txJson, &tx); err != nil {
				logger.Error("Failed to unmarshal trust transaction", "blockIndex", block.Index, "error", err)
				continue
			}
			node.updateTrustRegistry(tx)

		case TxTypeIdentity:
			var tx IdentityTransaction
			if err := json.Unmarshal(txJson, &tx); err != nil {
				logger.Error("Failed to unmarshal identity transaction", "blockIndex", block.Index, "error", err)
				continue
			}
			node.updateIdentityRegistry(tx)

		case TxTypeTitle:
			var tx TitleTransaction
			if err := json.Unmarshal(txJson, &tx); err != nil {
				logger.Error("Failed to unmarshal title transaction", "blockIndex", block.Index, "error", err)
				continue
			}
			node.updateTitleRegistry(tx)

		case TxTypeEvent:
			var tx EventTransaction
			if err := json.Unmarshal(txJson, &tx); err != nil {
				logger.Error("Failed to unmarshal event transaction", "blockIndex", block.Index, "error", err)
				continue
			}
			node.updateEventStreamRegistry(tx)
		}
	}
}

// updateTrustRegistry updates the trust registry with a trust transaction
func (node *QuidnugNode) updateTrustRegistry(tx TrustTransaction) {
	node.TrustRegistryMutex.Lock()
	defer node.TrustRegistryMutex.Unlock()

	// Initialize map for truster if it doesn't exist
	if _, exists := node.TrustRegistry[tx.Truster]; !exists {
		node.TrustRegistry[tx.Truster] = make(map[string]float64)
	}

	// Update trust level
	node.TrustRegistry[tx.Truster][tx.Trustee] = tx.TrustLevel

	// Update nonce registry for replay protection
	if _, exists := node.TrustNonceRegistry[tx.Truster]; !exists {
		node.TrustNonceRegistry[tx.Truster] = make(map[string]int64)
	}
	node.TrustNonceRegistry[tx.Truster][tx.Trustee] = tx.Nonce

	// Invalidate trust cache since trust graph has changed
	if node.TrustCache != nil {
		node.TrustCache.Invalidate()
	}

	logger.Debug("Updated trust registry",
		"truster", tx.Truster,
		"trustee", tx.Trustee,
		"trustLevel", tx.TrustLevel,
		"nonce", tx.Nonce)
}

// updateIdentityRegistry updates the identity registry with an identity transaction
func (node *QuidnugNode) updateIdentityRegistry(tx IdentityTransaction) {
	node.IdentityRegistryMutex.Lock()
	defer node.IdentityRegistryMutex.Unlock()

	// Add or update identity
	node.IdentityRegistry[tx.QuidID] = tx

	logger.Debug("Updated identity registry", "quidId", tx.QuidID, "name", tx.Name)
}

// updateTitleRegistry updates the title registry with a title transaction
func (node *QuidnugNode) updateTitleRegistry(tx TitleTransaction) {
	node.TitleRegistryMutex.Lock()
	defer node.TitleRegistryMutex.Unlock()

	// Add or update title
	node.TitleRegistry[tx.AssetID] = tx

	logger.Debug("Updated title registry", "assetId", tx.AssetID, "ownerCount", len(tx.Owners))
}

// GetTrustLevel returns the trust level between two quids
func (node *QuidnugNode) GetTrustLevel(truster, trustee string) float64 {
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()

	// Check if truster has a trust relationship with trustee
	if trustMap, exists := node.TrustRegistry[truster]; exists {
		if trustLevel, found := trustMap[trustee]; found {
			return trustLevel
		}
	}

	// Default trust level if no explicit relationship exists
	return 0.0
}

// GetDirectTrustees returns all quids directly trusted by a given quid
func (node *QuidnugNode) GetDirectTrustees(quidID string) map[string]float64 {
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()

	result := make(map[string]float64)
	if trustMap, exists := node.TrustRegistry[quidID]; exists {
		for trustee, level := range trustMap {
			result[trustee] = level
		}
	}
	return result
}

// ComputeRelationalTrust computes transitive trust from observer to target through the trust graph.
// It uses BFS with multiplicative decay, returning the maximum trust found across all paths.
// Results are cached with TTL-based expiration for performance.
// Parameters:
//   - observer: the quid ID of the observer (source of trust query)
//   - target: the quid ID of the target (destination of trust query)
//   - maxDepth: maximum number of hops to explore (defaults to DefaultTrustMaxDepth if <= 0)
//
// Returns:
//   - float64: the maximum trust level found (0 if no path exists)
//   - []string: the path of quid IDs for the best trust path
//   - error: ErrTrustGraphTooLarge if resource limits exceeded, nil otherwise
func (node *QuidnugNode) ComputeRelationalTrust(observer, target string, maxDepth int) (float64, []string, error) {
	start := time.Now()
	defer func() {
		trustComputationDuration.Observe(time.Since(start).Seconds())
	}()

	if maxDepth <= 0 {
		maxDepth = DefaultTrustMaxDepth
	}

	// Same entity has full trust in itself
	if observer == target {
		return 1.0, []string{observer}, nil
	}

	// Check cache first
	if node.TrustCache != nil {
		cacheKey := makeTrustCacheKey(observer, target, maxDepth)
		if trustLevel, trustPath, found := node.TrustCache.Get(cacheKey); found {
			return trustLevel, trustPath, nil
		}
	}

	type searchState struct {
		quid  string
		path  []string
		trust float64
	}

	queue := []searchState{{
		quid:  observer,
		path:  []string{observer},
		trust: 1.0,
	}}

	// Track visited nodes to bound memory usage in dense graphs
	visited := make(map[string]bool)
	visited[observer] = true

	bestTrust := 0.0
	var bestPath []string

	for len(queue) > 0 {
		// Check resource limits - return best result found so far with error
		if len(queue) > MaxTrustQueueSize {
			return bestTrust, bestPath, ErrTrustGraphTooLarge
		}
		if len(visited) > MaxTrustVisitedSize {
			return bestTrust, bestPath, ErrTrustGraphTooLarge
		}

		current := queue[0]
		queue = queue[1:]

		trustees := node.GetDirectTrustees(current.quid)

		for trustee, edgeTrust := range trustees {
			// Skip if trustee is already in current path (cycle avoidance)
			inPath := false
			for _, p := range current.path {
				if p == trustee {
					inPath = true
					break
				}
			}
			if inPath {
				continue
			}

			// Calculate multiplicative trust decay
			pathTrust := current.trust * edgeTrust

			// Build new path
			newPath := make([]string, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = trustee

			// Check if we've reached the target
			if trustee == target {
				if pathTrust > bestTrust {
					bestTrust = pathTrust
					bestPath = newPath
				}
				continue
			}

			// Continue BFS if within depth limit and not already visited
			// len(current.path) represents hops taken so far
			if len(current.path) < maxDepth && !visited[trustee] {
				visited[trustee] = true
				queue = append(queue, searchState{
					quid:  trustee,
					path:  newPath,
					trust: pathTrust,
				})
			}
		}
	}

	// Cache successful result (no error)
	if node.TrustCache != nil {
		cacheKey := makeTrustCacheKey(observer, target, maxDepth)
		node.TrustCache.Set(cacheKey, bestTrust, bestPath)
	}

	return bestTrust, bestPath, nil
}

// GetQuidIdentity returns quid identity information
func (node *QuidnugNode) GetQuidIdentity(quidID string) (IdentityTransaction, bool) {
	node.IdentityRegistryMutex.RLock()
	defer node.IdentityRegistryMutex.RUnlock()

	identity, exists := node.IdentityRegistry[quidID]
	return identity, exists
}

// updateEventStreamRegistry updates the event stream registry with an event transaction
func (node *QuidnugNode) updateEventStreamRegistry(tx EventTransaction) {
	node.EventStreamMutex.Lock()
	defer node.EventStreamMutex.Unlock()

	subjectID := tx.SubjectID

	// Initialize EventRegistry for subject if it doesn't exist
	if _, exists := node.EventRegistry[subjectID]; !exists {
		node.EventRegistry[subjectID] = make([]EventTransaction, 0)
	}

	// Append event to registry
	node.EventRegistry[subjectID] = append(node.EventRegistry[subjectID], tx)

	// Initialize EventStreamRegistry for subject if it doesn't exist
	if _, exists := node.EventStreamRegistry[subjectID]; !exists {
		node.EventStreamRegistry[subjectID] = &EventStream{
			SubjectID:   subjectID,
			SubjectType: tx.SubjectType,
			CreatedAt:   tx.Timestamp,
		}
	}

	// Update stream metadata
	stream := node.EventStreamRegistry[subjectID]
	stream.LatestSequence = tx.Sequence
	stream.EventCount = int64(len(node.EventRegistry[subjectID]))
	stream.UpdatedAt = tx.Timestamp
	stream.LatestEventID = tx.ID

	logger.Debug("Updated event stream registry",
		"subjectId", subjectID,
		"sequence", tx.Sequence,
		"eventCount", stream.EventCount)
}

// GetEventStream returns event stream metadata for a subject
func (node *QuidnugNode) GetEventStream(subjectID string) (*EventStream, bool) {
	node.EventStreamMutex.RLock()
	defer node.EventStreamMutex.RUnlock()

	stream, exists := node.EventStreamRegistry[subjectID]
	if !exists {
		return nil, false
	}
	return stream, true
}

// GetStreamEvents returns paginated events for a stream, ordered by sequence (ascending)
func (node *QuidnugNode) GetStreamEvents(subjectID string, limit, offset int) ([]EventTransaction, int) {
	node.EventStreamMutex.RLock()
	defer node.EventStreamMutex.RUnlock()

	events, exists := node.EventRegistry[subjectID]
	if !exists {
		return []EventTransaction{}, 0
	}

	total := len(events)

	// Handle offset beyond range
	if offset >= total {
		return []EventTransaction{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	// Return copy of slice (events are ordered by sequence as they're appended in order)
	result := make([]EventTransaction, end-offset)
	copy(result, events[offset:end])

	return result, total
}

// GetEventByID searches for an event by its ID across all streams
func (node *QuidnugNode) GetEventByID(eventID string) (*EventTransaction, bool) {
	node.EventStreamMutex.RLock()
	defer node.EventStreamMutex.RUnlock()

	for _, events := range node.EventRegistry {
		for i := range events {
			if events[i].ID == eventID {
				return &events[i], true
			}
		}
	}
	return nil, false
}

// GetAssetOwnership returns asset ownership information
func (node *QuidnugNode) GetAssetOwnership(assetID string) (TitleTransaction, bool) {
	node.TitleRegistryMutex.RLock()
	defer node.TitleRegistryMutex.RUnlock()

	title, exists := node.TitleRegistry[assetID]
	return title, exists
}

// AddVerifiedTrustEdge adds an edge to the verified registry (from trusted validator)
func (node *QuidnugNode) AddVerifiedTrustEdge(edge TrustEdge) {
	node.TrustRegistryMutex.Lock()
	defer node.TrustRegistryMutex.Unlock()

	// Update simple TrustRegistry for backward compatibility
	if _, exists := node.TrustRegistry[edge.Truster]; !exists {
		node.TrustRegistry[edge.Truster] = make(map[string]float64)
	}
	node.TrustRegistry[edge.Truster][edge.Trustee] = edge.TrustLevel

	// Store full TrustEdge with verified flag
	if _, exists := node.VerifiedTrustEdges[edge.Truster]; !exists {
		node.VerifiedTrustEdges[edge.Truster] = make(map[string]TrustEdge)
	}
	edge.Verified = true
	node.VerifiedTrustEdges[edge.Truster][edge.Trustee] = edge

	logger.Debug("Added verified trust edge",
		"truster", edge.Truster,
		"trustee", edge.Trustee,
		"trustLevel", edge.TrustLevel,
		"sourceBlock", edge.SourceBlock)
}

// AddUnverifiedTrustEdge adds an edge to the unverified registry (from any valid block)
func (node *QuidnugNode) AddUnverifiedTrustEdge(edge TrustEdge) {
	node.UnverifiedRegistryMutex.Lock()
	defer node.UnverifiedRegistryMutex.Unlock()

	if _, exists := node.UnverifiedTrustRegistry[edge.Truster]; !exists {
		node.UnverifiedTrustRegistry[edge.Truster] = make(map[string]TrustEdge)
	}
	edge.Verified = false
	node.UnverifiedTrustRegistry[edge.Truster][edge.Trustee] = edge

	logger.Debug("Added unverified trust edge",
		"truster", edge.Truster,
		"trustee", edge.Trustee,
		"trustLevel", edge.TrustLevel,
		"validatorQuid", edge.ValidatorQuid)
}

// PromoteTrustEdge moves an edge from unverified to verified (when validator becomes trusted)
func (node *QuidnugNode) PromoteTrustEdge(truster, trustee string) {
	// Extract edge from unverified registry
	node.UnverifiedRegistryMutex.Lock()
	var edge TrustEdge
	var found bool
	if trusterEdges, exists := node.UnverifiedTrustRegistry[truster]; exists {
		if e, ok := trusterEdges[trustee]; ok {
			edge = e
			found = true
			delete(trusterEdges, trustee)
			if len(trusterEdges) == 0 {
				delete(node.UnverifiedTrustRegistry, truster)
			}
		}
	}
	node.UnverifiedRegistryMutex.Unlock()

	if !found {
		return
	}

	// Add to verified registry
	node.AddVerifiedTrustEdge(edge)

	logger.Debug("Promoted trust edge to verified",
		"truster", truster,
		"trustee", trustee)
}

// DemoteTrustEdge moves an edge from verified to unverified (when validator becomes distrusted)
func (node *QuidnugNode) DemoteTrustEdge(truster, trustee string) {
	// Extract edge from verified registry
	node.TrustRegistryMutex.Lock()
	var edge TrustEdge
	var found bool
	if trusterEdges, exists := node.VerifiedTrustEdges[truster]; exists {
		if e, ok := trusterEdges[trustee]; ok {
			edge = e
			found = true
			delete(trusterEdges, trustee)
			if len(trusterEdges) == 0 {
				delete(node.VerifiedTrustEdges, truster)
			}
		}
	}
	// Also remove from simple TrustRegistry
	if trusterMap, exists := node.TrustRegistry[truster]; exists {
		delete(trusterMap, trustee)
		if len(trusterMap) == 0 {
			delete(node.TrustRegistry, truster)
		}
	}
	node.TrustRegistryMutex.Unlock()

	if !found {
		return
	}

	// Add to unverified registry
	node.AddUnverifiedTrustEdge(edge)

	logger.Debug("Demoted trust edge to unverified",
		"truster", truster,
		"trustee", trustee)
}

// GetTrustEdges returns trust edges for a quid, optionally including unverified
func (node *QuidnugNode) GetTrustEdges(quidID string, includeUnverified bool) map[string]TrustEdge {
	result := make(map[string]TrustEdge)

	// Get verified edges first (they take precedence)
	node.TrustRegistryMutex.RLock()
	if trusterEdges, exists := node.VerifiedTrustEdges[quidID]; exists {
		for trustee, edge := range trusterEdges {
			result[trustee] = edge
		}
	}
	node.TrustRegistryMutex.RUnlock()

	// Add unverified edges if requested (verified takes precedence for same truster/trustee)
	if includeUnverified {
		node.UnverifiedRegistryMutex.RLock()
		if trusterEdges, exists := node.UnverifiedTrustRegistry[quidID]; exists {
			for trustee, edge := range trusterEdges {
				if _, hasVerified := result[trustee]; !hasVerified {
					result[trustee] = edge
				}
			}
		}
		node.UnverifiedRegistryMutex.RUnlock()
	}

	return result
}

// ComputeRelationalTrustEnhanced computes transitive trust with optional unverified edge inclusion.
// When includeUnverified=true, unverified edges are discounted by the observer's trust in the recording validator.
// Returns enhanced result with provenance information.
// Results are cached with TTL-based expiration for performance.
// Returns ErrTrustGraphTooLarge if resource limits are exceeded.
func (node *QuidnugNode) ComputeRelationalTrustEnhanced(
	observer, target string,
	maxDepth int,
	includeUnverified bool,
) (EnhancedTrustResult, error) {
	if maxDepth <= 0 {
		maxDepth = DefaultTrustMaxDepth
	}

	// Same entity has full trust in itself
	if observer == target {
		return EnhancedTrustResult{
			RelationalTrustResult: RelationalTrustResult{
				Observer:   observer,
				Target:     target,
				TrustLevel: 1.0,
				TrustPath:  []string{observer},
				PathDepth:  0,
			},
			Confidence:     "high",
			UnverifiedHops: 0,
		}, nil
	}

	// Check cache first
	if node.TrustCache != nil {
		cacheKey := makeEnhancedTrustCacheKey(observer, target, maxDepth, includeUnverified)
		if result, found := node.TrustCache.GetEnhanced(cacheKey); found {
			return result, nil
		}
	}

	type searchState struct {
		quid           string
		path           []string
		trust          float64
		unverifiedHops int
		gaps           []VerificationGap
	}

	queue := []searchState{{
		quid:           observer,
		path:           []string{observer},
		trust:          1.0,
		unverifiedHops: 0,
		gaps:           nil,
	}}

	// Track visited nodes to bound memory usage in dense graphs
	visited := make(map[string]bool)
	visited[observer] = true

	bestTrust := 0.0
	var bestPath []string
	bestUnverifiedHops := 0
	var bestGaps []VerificationGap

	for len(queue) > 0 {
		// Check resource limits - return best result found so far with error
		if len(queue) > MaxTrustQueueSize {
			return buildEnhancedResult(observer, target, bestTrust, bestPath, bestUnverifiedHops, bestGaps), ErrTrustGraphTooLarge
		}
		if len(visited) > MaxTrustVisitedSize {
			return buildEnhancedResult(observer, target, bestTrust, bestPath, bestUnverifiedHops, bestGaps), ErrTrustGraphTooLarge
		}

		current := queue[0]
		queue = queue[1:]

		edges := node.GetTrustEdges(current.quid, includeUnverified)

		for trustee, edge := range edges {
			// Skip if trustee is already in current path (cycle avoidance)
			inPath := false
			for _, p := range current.path {
				if p == trustee {
					inPath = true
					break
				}
			}
			if inPath {
				continue
			}

			var effectiveTrust float64
			var newUnverifiedHops int
			var newGaps []VerificationGap

			if edge.Verified {
				effectiveTrust = current.trust * edge.TrustLevel
				newUnverifiedHops = current.unverifiedHops
				newGaps = current.gaps
			} else {
				// Discount by validator trust (use reduced depth to limit recursion)
				validatorTrust, _, err := node.ComputeRelationalTrust(observer, edge.ValidatorQuid, DefaultTrustMaxDepth)
				if err != nil {
					// On resource exhaustion in nested call, use zero trust for this edge
					validatorTrust = 0
				}
				effectiveTrust = current.trust * edge.TrustLevel * validatorTrust
				newUnverifiedHops = current.unverifiedHops + 1

				// Create verification gap entry
				gap := VerificationGap{
					From:           current.quid,
					To:             trustee,
					ValidatorQuid:  edge.ValidatorQuid,
					ValidatorTrust: validatorTrust,
				}
				newGaps = make([]VerificationGap, len(current.gaps)+1)
				copy(newGaps, current.gaps)
				newGaps[len(current.gaps)] = gap
			}

			// Build new path
			newPath := make([]string, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = trustee

			// Check if we've reached the target
			if trustee == target {
				if effectiveTrust > bestTrust {
					bestTrust = effectiveTrust
					bestPath = newPath
					bestUnverifiedHops = newUnverifiedHops
					bestGaps = newGaps
				}
				continue
			}

			// Continue BFS if within depth limit and not already visited
			if len(current.path) < maxDepth && !visited[trustee] {
				visited[trustee] = true
				queue = append(queue, searchState{
					quid:           trustee,
					path:           newPath,
					trust:          effectiveTrust,
					unverifiedHops: newUnverifiedHops,
					gaps:           newGaps,
				})
			}
		}
	}

	result := buildEnhancedResult(observer, target, bestTrust, bestPath, bestUnverifiedHops, bestGaps)

	// Cache successful result (no error)
	if node.TrustCache != nil {
		cacheKey := makeEnhancedTrustCacheKey(observer, target, maxDepth, includeUnverified)
		node.TrustCache.SetEnhanced(cacheKey, result)
	}

	return result, nil
}

// buildEnhancedResult constructs an EnhancedTrustResult from components
func buildEnhancedResult(observer, target string, bestTrust float64, bestPath []string, bestUnverifiedHops int, bestGaps []VerificationGap) EnhancedTrustResult {
	// Determine confidence level based on unverified hop count
	var confidence string
	switch {
	case bestUnverifiedHops == 0:
		confidence = "high"
	case bestUnverifiedHops == 1:
		confidence = "medium"
	default:
		confidence = "low"
	}

	pathDepth := 0
	if len(bestPath) > 0 {
		pathDepth = len(bestPath) - 1
	}

	// Ensure VerificationGaps is never nil for JSON serialization
	gaps := bestGaps
	if gaps == nil {
		gaps = []VerificationGap{}
	}

	return EnhancedTrustResult{
		RelationalTrustResult: RelationalTrustResult{
			Observer:   observer,
			Target:     target,
			TrustLevel: bestTrust,
			TrustPath:  bestPath,
			PathDepth:  pathDepth,
		},
		Confidence:       confidence,
		UnverifiedHops:   bestUnverifiedHops,
		VerificationGaps: gaps,
	}
}

// ExtractTrustEdgesFromBlock extracts all trust transaction edges from a block
func (node *QuidnugNode) ExtractTrustEdgesFromBlock(block Block, verified bool) []TrustEdge {
	var edges []TrustEdge

	for _, txInterface := range block.Transactions {
		txJson, err := json.Marshal(txInterface)
		if err != nil {
			continue
		}

		var baseTx BaseTransaction
		if err := json.Unmarshal(txJson, &baseTx); err != nil {
			continue
		}

		if baseTx.Type != TxTypeTrust {
			continue
		}

		var tx TrustTransaction
		if err := json.Unmarshal(txJson, &tx); err != nil {
			continue
		}

		edge := TrustEdge{
			Truster:       tx.Truster,
			Trustee:       tx.Trustee,
			TrustLevel:    tx.TrustLevel,
			SourceBlock:   block.Hash,
			ValidatorQuid: block.TrustProof.ValidatorID,
			Verified:      verified,
			Timestamp:     tx.Timestamp,
		}
		edges = append(edges, edge)
	}

	return edges
}
