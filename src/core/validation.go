package main

import (
	"encoding/json"
)

// ValidateTrustTransaction validates a trust transaction
func (node *QuidnugNode) ValidateTrustTransaction(tx TrustTransaction) bool {
	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists && tx.TrustDomain != "" {
		logger.Warn("Trust transaction from unknown trust domain", "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	// Verify trust level is in valid range (0.0 to 1.0)
	if tx.TrustLevel < 0.0 || tx.TrustLevel > 1.0 {
		logger.Warn("Invalid trust level", "trustLevel", tx.TrustLevel, "txId", tx.ID)
		return false
	}

	// Check if truster exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, trusterExists := node.IdentityRegistry[tx.Truster]
	node.IdentityRegistryMutex.RUnlock()

	if !trusterExists {
		logger.Debug("Truster not found in identity registry", "truster", tx.Truster, "txId", tx.ID)
	}

	// Check if trustee exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, trusteeExists := node.IdentityRegistry[tx.Trustee]
	node.IdentityRegistryMutex.RUnlock()

	if !trusteeExists {
		logger.Debug("Trustee not found in identity registry", "trustee", tx.Trustee, "txId", tx.ID)
	}

	// Verify signature
	if tx.Signature == "" || tx.PublicKey == "" {
		logger.Warn("Missing signature or public key in trust transaction", "txId", tx.ID)
		return false
	}

	// Get signable data (transaction with signature field cleared)
	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		logger.Error("Failed to marshal transaction for signature verification", "txId", tx.ID, "error", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		logger.Warn("Invalid signature in trust transaction", "txId", tx.ID)
		return false
	}

	return true
}

// ValidateIdentityTransaction validates an identity transaction
func (node *QuidnugNode) ValidateIdentityTransaction(tx IdentityTransaction) bool {
	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists && tx.TrustDomain != "" {
		logger.Warn("Identity transaction from unknown trust domain", "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	// Check if this is an update to an existing identity
	node.IdentityRegistryMutex.RLock()
	existingIdentity, exists := node.IdentityRegistry[tx.QuidID]
	node.IdentityRegistryMutex.RUnlock()

	if exists {
		// If it's an update, check that update nonce is higher than current
		if tx.UpdateNonce <= existingIdentity.UpdateNonce {
			logger.Warn("Invalid update nonce for identity",
				"quidId", tx.QuidID,
				"providedNonce", tx.UpdateNonce,
				"currentNonce", existingIdentity.UpdateNonce,
				"txId", tx.ID)
			return false
		}

		// Also verify that the creator is the same as original creator
		if tx.Creator != existingIdentity.Creator {
			logger.Warn("Identity update creator mismatch",
				"providedCreator", tx.Creator,
				"originalCreator", existingIdentity.Creator,
				"quidId", tx.QuidID,
				"txId", tx.ID)
			return false
		}
	}

	// Verify signature
	if tx.Signature == "" || tx.PublicKey == "" {
		logger.Warn("Missing signature or public key in identity transaction", "txId", tx.ID, "quidId", tx.QuidID)
		return false
	}

	// Get signable data (transaction with signature field cleared)
	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		logger.Error("Failed to marshal transaction for signature verification", "txId", tx.ID, "error", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		logger.Warn("Invalid signature in identity transaction", "txId", tx.ID, "quidId", tx.QuidID)
		return false
	}

	return true
}

// ValidateTitleTransaction validates a title transaction
func (node *QuidnugNode) ValidateTitleTransaction(tx TitleTransaction) bool {
	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists && tx.TrustDomain != "" {
		logger.Warn("Title transaction from unknown trust domain", "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	// Check if asset exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, assetExists := node.IdentityRegistry[tx.AssetID]
	node.IdentityRegistryMutex.RUnlock()

	if !assetExists {
		logger.Warn("Asset not found in identity registry", "assetId", tx.AssetID, "txId", tx.ID)
		return false
	}

	// Verify total ownership percentage adds up to 100%
	var totalPercentage float64
	for _, stake := range tx.Owners {
		totalPercentage += stake.Percentage
	}

	if totalPercentage != 100.0 {
		logger.Warn("Total ownership percentage doesn't equal 100%",
			"totalPercentage", totalPercentage,
			"assetId", tx.AssetID,
			"txId", tx.ID)
		return false
	}

	// Verify main signature from issuer
	if tx.Signature == "" || tx.PublicKey == "" {
		logger.Warn("Missing signature or public key in title transaction", "txId", tx.ID, "assetId", tx.AssetID)
		return false
	}

	// Get signable data for issuer (transaction with main signature cleared)
	txCopyForIssuer := tx
	txCopyForIssuer.Signature = ""
	issuerSignableData, err := json.Marshal(txCopyForIssuer)
	if err != nil {
		logger.Error("Failed to marshal transaction for issuer signature verification", "txId", tx.ID, "error", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, issuerSignableData, tx.Signature) {
		logger.Warn("Invalid issuer signature in title transaction", "txId", tx.ID, "assetId", tx.AssetID)
		return false
	}

	// If this is a transfer (has previous owners), verify previous owners' signatures
	if len(tx.PreviousOwners) > 0 {
		// Verify previous ownership matches current title in registry
		node.TitleRegistryMutex.RLock()
		currentTitle, exists := node.TitleRegistry[tx.AssetID]
		node.TitleRegistryMutex.RUnlock()

		if exists {
			if !areOwnershipStakesEqual(tx.PreviousOwners, currentTitle.Owners) {
				logger.Warn("Previous owners don't match current title", "assetId", tx.AssetID, "txId", tx.ID)
				return false
			}
		}

		// Get signable data for owners (transaction with all signatures cleared)
		txCopyForOwners := tx
		txCopyForOwners.Signature = ""
		txCopyForOwners.Signatures = nil
		ownerSignableData, err := json.Marshal(txCopyForOwners)
		if err != nil {
			logger.Error("Failed to marshal transaction for owner signature verification", "txId", tx.ID, "error", err)
			return false
		}

		// Verify each previous owner's signature
		for _, stake := range tx.PreviousOwners {
			// Get previous owner's public key from identity registry
			node.IdentityRegistryMutex.RLock()
			ownerIdentity, ownerExists := node.IdentityRegistry[stake.OwnerID]
			node.IdentityRegistryMutex.RUnlock()

			if !ownerExists {
				logger.Warn("Previous owner not found in identity registry", "ownerId", stake.OwnerID, "txId", tx.ID)
				return false
			}

			if ownerIdentity.PublicKey == "" {
				logger.Warn("Previous owner has no public key", "ownerId", stake.OwnerID, "txId", tx.ID)
				return false
			}

			// Get the signature for this owner
			ownerSig, hasSig := tx.Signatures[stake.OwnerID]
			if !hasSig || ownerSig == "" {
				logger.Warn("Missing signature from previous owner", "ownerId", stake.OwnerID, "txId", tx.ID)
				return false
			}

			// Verify the owner's signature
			if !VerifySignature(ownerIdentity.PublicKey, ownerSignableData, ownerSig) {
				logger.Warn("Invalid signature from previous owner", "ownerId", stake.OwnerID, "txId", tx.ID)
				return false
			}
		}
	}

	return true
}

// ValidateBlock validates a block
func (node *QuidnugNode) ValidateBlock(block Block) bool {
	node.BlockchainMutex.RLock()
	defer node.BlockchainMutex.RUnlock()

	// Check if blockchain is empty
	if len(node.Blockchain) == 0 {
		return false
	}

	prevBlock := node.Blockchain[len(node.Blockchain)-1]

	// Check block index and previous hash
	if block.Index != prevBlock.Index+1 || block.PrevHash != prevBlock.Hash {
		return false
	}

	// Verify the block hash
	if calculateBlockHash(block) != block.Hash {
		return false
	}

	// Verify trust proof (implement proof of trust validation)
	if !node.ValidateTrustProof(block.TrustProof) {
		return false
	}

	// Validate all transactions in the block
	for _, txInterface := range block.Transactions {
		txJson, _ := json.Marshal(txInterface)

		// Determine transaction type
		var baseTx BaseTransaction
		json.Unmarshal(txJson, &baseTx)

		var isValid bool

		switch baseTx.Type {
		case TxTypeTrust:
			var tx TrustTransaction
			json.Unmarshal(txJson, &tx)
			isValid = node.ValidateTrustTransaction(tx)

		case TxTypeIdentity:
			var tx IdentityTransaction
			json.Unmarshal(txJson, &tx)
			isValid = node.ValidateIdentityTransaction(tx)

		case TxTypeTitle:
			var tx TitleTransaction
			json.Unmarshal(txJson, &tx)
			isValid = node.ValidateTitleTransaction(tx)

		default:
			isValid = false
		}

		if !isValid {
			return false
		}
	}

	return true
}

// ValidateTrustProof validates a trust proof
func (node *QuidnugNode) ValidateTrustProof(proof TrustProof) bool {
	node.TrustDomainsMutex.RLock()
	defer node.TrustDomainsMutex.RUnlock()

	// Check if the trust domain exists
	domain, exists := node.TrustDomains[proof.TrustDomain]
	if !exists {
		return false
	}

	// Verify validator is part of the domain
	validatorFound := false
	for _, validatorID := range domain.ValidatorNodes {
		if validatorID == proof.ValidatorID {
			validatorFound = true
			break
		}
	}

	if !validatorFound {
		return false
	}

	// Validate signatures (in a real implementation, this would verify cryptographic signatures)
	// For simplicity, we're just checking that signatures exist
	if len(proof.ValidatorSigs) == 0 {
		return false
	}

	// Check trust score threshold
	if proof.TrustScore < domain.TrustThreshold {
		return false
	}

	return true
}

// areOwnershipStakesEqual is a helper function to compare ownership stakes
func areOwnershipStakesEqual(a, b []OwnershipStake) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for easy comparison
	mapA := make(map[string]float64)
	mapB := make(map[string]float64)

	for _, stake := range a {
		mapA[stake.OwnerID] = stake.Percentage
	}

	for _, stake := range b {
		mapB[stake.OwnerID] = stake.Percentage
	}

	// Compare maps
	for owner, percentage := range mapA {
		if mapB[owner] != percentage {
			return false
		}
	}

	return true
}
