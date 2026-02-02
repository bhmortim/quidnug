package main

import (
	"encoding/json"
)

// processBlockTransactions processes transactions in a block to update registries
func (node *QuidnugNode) processBlockTransactions(block Block) {
	for _, txInterface := range block.Transactions {
		txJson, _ := json.Marshal(txInterface)

		// Determine transaction type
		var baseTx BaseTransaction
		json.Unmarshal(txJson, &baseTx)

		switch baseTx.Type {
		case TxTypeTrust:
			var tx TrustTransaction
			json.Unmarshal(txJson, &tx)
			node.updateTrustRegistry(tx)

		case TxTypeIdentity:
			var tx IdentityTransaction
			json.Unmarshal(txJson, &tx)
			node.updateIdentityRegistry(tx)

		case TxTypeTitle:
			var tx TitleTransaction
			json.Unmarshal(txJson, &tx)
			node.updateTitleRegistry(tx)
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

	logger.Debug("Updated trust registry",
		"truster", tx.Truster,
		"trustee", tx.Trustee,
		"trustLevel", tx.TrustLevel)
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
// Parameters:
//   - observer: the quid ID of the observer (source of trust query)
//   - target: the quid ID of the target (destination of trust query)
//   - maxDepth: maximum number of hops to explore (defaults to 5 if <= 0)
//
// Returns:
//   - float64: the maximum trust level found (0 if no path exists)
//   - []string: the path of quid IDs for the best trust path
//   - error: nil (reserved for future validation errors)
func (node *QuidnugNode) ComputeRelationalTrust(observer, target string, maxDepth int) (float64, []string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	// Same entity has full trust in itself
	if observer == target {
		return 1.0, []string{observer}, nil
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

	bestTrust := 0.0
	var bestPath []string

	for len(queue) > 0 {
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

			// Continue BFS if within depth limit
			// len(current.path) represents hops taken so far
			if len(current.path) < maxDepth {
				queue = append(queue, searchState{
					quid:  trustee,
					path:  newPath,
					trust: pathTrust,
				})
			}
		}
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

// GetAssetOwnership returns asset ownership information
func (node *QuidnugNode) GetAssetOwnership(assetID string) (TitleTransaction, bool) {
	node.TitleRegistryMutex.RLock()
	defer node.TitleRegistryMutex.RUnlock()

	title, exists := node.TitleRegistry[assetID]
	return title, exists
}
