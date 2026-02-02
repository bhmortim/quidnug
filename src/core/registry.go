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
