package main

import (
	"encoding/json"
	"log"
)

// ValidateTrustTransaction validates a trust transaction
func (node *QuidnugNode) ValidateTrustTransaction(tx TrustTransaction) bool {
	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists && tx.TrustDomain != "" {
		log.Printf("Trust transaction from unknown trust domain: %s", tx.TrustDomain)
		return false
	}

	// Verify trust level is in valid range (0.0 to 1.0)
	if tx.TrustLevel < 0.0 || tx.TrustLevel > 1.0 {
		log.Printf("Invalid trust level: %f", tx.TrustLevel)
		return false
	}

	// Check if truster exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, trusterExists := node.IdentityRegistry[tx.Truster]
	node.IdentityRegistryMutex.RUnlock()

	if !trusterExists {
		log.Printf("Truster %s not found in identity registry", tx.Truster)
	}

	// Check if trustee exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, trusteeExists := node.IdentityRegistry[tx.Trustee]
	node.IdentityRegistryMutex.RUnlock()

	if !trusteeExists {
		log.Printf("Trustee %s not found in identity registry", tx.Trustee)
	}

	// Verify signature
	if tx.Signature == "" || tx.PublicKey == "" {
		log.Printf("Missing signature or public key in trust transaction")
		return false
	}

	// Get signable data (transaction with signature field cleared)
	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		log.Printf("Failed to marshal transaction for signature verification: %v", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		log.Printf("Invalid signature in trust transaction")
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
		log.Printf("Identity transaction from unknown trust domain: %s", tx.TrustDomain)
		return false
	}

	// Check if this is an update to an existing identity
	node.IdentityRegistryMutex.RLock()
	existingIdentity, exists := node.IdentityRegistry[tx.QuidID]
	node.IdentityRegistryMutex.RUnlock()

	if exists {
		// If it's an update, check that update nonce is higher than current
		if tx.UpdateNonce <= existingIdentity.UpdateNonce {
			log.Printf("Invalid update nonce for identity %s: %d <= %d",
				tx.QuidID, tx.UpdateNonce, existingIdentity.UpdateNonce)
			return false
		}

		// Also verify that the creator is the same as original creator
		if tx.Creator != existingIdentity.Creator {
			log.Printf("Identity update creator mismatch: %s != %s",
				tx.Creator, existingIdentity.Creator)
			return false
		}
	}

	// Verify signature
	if tx.Signature == "" || tx.PublicKey == "" {
		log.Printf("Missing signature or public key in identity transaction")
		return false
	}

	// Get signable data (transaction with signature field cleared)
	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		log.Printf("Failed to marshal transaction for signature verification: %v", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		log.Printf("Invalid signature in identity transaction")
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
		log.Printf("Title transaction from unknown trust domain: %s", tx.TrustDomain)
		return false
	}

	// Check if asset exists in identity registry
	node.IdentityRegistryMutex.RLock()
	_, assetExists := node.IdentityRegistry[tx.AssetID]
	node.IdentityRegistryMutex.RUnlock()

	if !assetExists {
		log.Printf("Asset %s not found in identity registry", tx.AssetID)
		return false
	}

	// Verify total ownership percentage adds up to 100%
	var totalPercentage float64
	for _, stake := range tx.Owners {
		totalPercentage += stake.Percentage
	}

	if totalPercentage != 100.0 {
		log.Printf("Total ownership percentage doesn't equal 100%%: %.2f", totalPercentage)
		return false
	}

	// Verify main signature from issuer
	if tx.Signature == "" || tx.PublicKey == "" {
		log.Printf("Missing signature or public key in title transaction")
		return false
	}

	// Get signable data for issuer (transaction with main signature cleared)
	txCopyForIssuer := tx
	txCopyForIssuer.Signature = ""
	issuerSignableData, err := json.Marshal(txCopyForIssuer)
	if err != nil {
		log.Printf("Failed to marshal transaction for issuer signature verification: %v", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, issuerSignableData, tx.Signature) {
		log.Printf("Invalid issuer signature in title transaction")
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
				log.Printf("Previous owners don't match current title")
				return false
			}
		}

		// Get signable data for owners (transaction with all signatures cleared)
		txCopyForOwners := tx
		txCopyForOwners.Signature = ""
		txCopyForOwners.Signatures = nil
		ownerSignableData, err := json.Marshal(txCopyForOwners)
		if err != nil {
			log.Printf("Failed to marshal transaction for owner signature verification: %v", err)
			return false
		}

		// Verify each previous owner's signature
		for _, stake := range tx.PreviousOwners {
			// Get previous owner's public key from identity registry
			node.IdentityRegistryMutex.RLock()
			ownerIdentity, ownerExists := node.IdentityRegistry[stake.OwnerID]
			node.IdentityRegistryMutex.RUnlock()

			if !ownerExists {
				log.Printf("Previous owner %s not found in identity registry", stake.OwnerID)
				return false
			}

			if ownerIdentity.PublicKey == "" {
				log.Printf("Previous owner %s has no public key", stake.OwnerID)
				return false
			}

			// Get the signature for this owner
			ownerSig, hasSig := tx.Signatures[stake.OwnerID]
			if !hasSig || ownerSig == "" {
				log.Printf("Missing signature from previous owner %s", stake.OwnerID)
				return false
			}

			// Verify the owner's signature
			if !VerifySignature(ownerIdentity.PublicKey, ownerSignableData, ownerSig) {
				log.Printf("Invalid signature from previous owner %s", stake.OwnerID)
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
