package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
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

	// Validate nonce is present and positive
	if tx.Nonce <= 0 {
		logger.Warn("Invalid nonce: must be positive", "nonce", tx.Nonce, "txId", tx.ID)
		return false
	}

	// Check nonce against registry for replay protection
	node.TrustRegistryMutex.RLock()
	currentNonce := int64(0)
	if trusterNonces, exists := node.TrustNonceRegistry[tx.Truster]; exists {
		if nonce, found := trusterNonces[tx.Trustee]; found {
			currentNonce = nonce
		}
	}
	node.TrustRegistryMutex.RUnlock()

	if tx.Nonce <= currentNonce {
		logger.Warn("Invalid nonce: must be greater than current",
			"providedNonce", tx.Nonce,
			"currentNonce", currentNonce,
			"truster", tx.Truster,
			"trustee", tx.Trustee,
			"txId", tx.ID)
		return false
	}

	// Verify trust level is not NaN or Inf
	if math.IsNaN(tx.TrustLevel) || math.IsInf(tx.TrustLevel, 0) {
		logger.Warn("Invalid trust level: NaN or Inf", "trustLevel", tx.TrustLevel, "txId", tx.ID)
		return false
	}

	// Verify trust level is in valid range (0.0 to 1.0)
	if tx.TrustLevel < 0.0 || tx.TrustLevel > 1.0 {
		logger.Warn("Invalid trust level", "trustLevel", tx.TrustLevel, "txId", tx.ID)
		return false
	}

	// Validate quid ID formats
	if tx.Truster != "" && !IsValidQuidID(tx.Truster) {
		logger.Warn("Invalid truster quid ID format", "truster", tx.Truster, "txId", tx.ID)
		return false
	}

	if tx.Trustee != "" && !IsValidQuidID(tx.Trustee) {
		logger.Warn("Invalid trustee quid ID format", "trustee", tx.Trustee, "txId", tx.ID)
		return false
	}

	// Validate string field lengths and control characters
	if tx.TrustDomain != "" && !ValidateStringField(tx.TrustDomain, MaxDomainLength) {
		logger.Warn("Invalid trust domain: too long or contains control characters", "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	if tx.Description != "" && !ValidateStringField(tx.Description, MaxDescriptionLength) {
		logger.Warn("Invalid description: too long or contains control characters", "txId", tx.ID)
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

	// Validate quid ID formats
	if tx.QuidID != "" && !IsValidQuidID(tx.QuidID) {
		logger.Warn("Invalid quid ID format", "quidId", tx.QuidID, "txId", tx.ID)
		return false
	}

	if tx.Creator != "" && !IsValidQuidID(tx.Creator) {
		logger.Warn("Invalid creator quid ID format", "creator", tx.Creator, "txId", tx.ID)
		return false
	}

	// Validate string field lengths and control characters
	if tx.TrustDomain != "" && !ValidateStringField(tx.TrustDomain, MaxDomainLength) {
		logger.Warn("Invalid trust domain: too long or contains control characters", "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	if tx.Name != "" && !ValidateStringField(tx.Name, MaxNameLength) {
		logger.Warn("Invalid name: too long or contains control characters", "quidId", tx.QuidID, "txId", tx.ID)
		return false
	}

	if tx.Description != "" && !ValidateStringField(tx.Description, MaxDescriptionLength) {
		logger.Warn("Invalid description: too long or contains control characters", "quidId", tx.QuidID, "txId", tx.ID)
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

// MaxEventTypeLength is the maximum length for event type field
const MaxEventTypeLength = 64

// MaxPayloadSize is the maximum size in bytes for event payload (64KB)
const MaxPayloadSize = 64 * 1024

// ValidateEventTransaction validates an event transaction
func (node *QuidnugNode) ValidateEventTransaction(tx EventTransaction) bool {
	// Validate TrustDomain is not empty
	if tx.TrustDomain == "" {
		logger.Warn("Event transaction missing trust domain", "txId", tx.ID)
		return false
	}

	// Check if transaction belongs to a known trust domain
	node.TrustDomainsMutex.RLock()
	_, domainExists := node.TrustDomains[tx.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	if !domainExists {
		logger.Warn("Event transaction from unknown trust domain", "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	// Validate SubjectID is present and has valid format
	if tx.SubjectID == "" {
		logger.Warn("Event transaction missing subject ID", "txId", tx.ID)
		return false
	}

	if !IsValidQuidID(tx.SubjectID) {
		logger.Warn("Invalid subject ID format", "subjectId", tx.SubjectID, "txId", tx.ID)
		return false
	}

	// Validate SubjectType must be "QUID" or "TITLE"
	if tx.SubjectType != "QUID" && tx.SubjectType != "TITLE" {
		logger.Warn("Invalid subject type: must be 'QUID' or 'TITLE'", "subjectType", tx.SubjectType, "txId", tx.ID)
		return false
	}

	// Validate EventType (not empty, max 64 chars)
	if tx.EventType == "" {
		logger.Warn("Event type is empty", "txId", tx.ID)
		return false
	}

	if len(tx.EventType) > MaxEventTypeLength {
		logger.Warn("Event type exceeds max length", "length", len(tx.EventType), "max", MaxEventTypeLength, "txId", tx.ID)
		return false
	}

	// Validate payload - either Payload or PayloadCID must be provided
	hasPayload := len(tx.Payload) > 0
	hasPayloadCID := tx.PayloadCID != ""

	if !hasPayload && !hasPayloadCID {
		logger.Warn("Event transaction missing payload: either Payload or PayloadCID required", "txId", tx.ID)
		return false
	}

	// If PayloadCID provided, validate CID format
	if hasPayloadCID && !IsValidCID(tx.PayloadCID) {
		logger.Warn("Invalid payload CID format", "payloadCid", tx.PayloadCID, "txId", tx.ID)
		return false
	}

	// Validate Payload size (max 64KB when serialized)
	if hasPayload {
		payloadBytes, err := json.Marshal(tx.Payload)
		if err != nil {
			logger.Warn("Failed to marshal payload for size check", "txId", tx.ID, "error", err)
			return false
		}
		if len(payloadBytes) > MaxPayloadSize {
			logger.Warn("Payload exceeds max size", "size", len(payloadBytes), "max", MaxPayloadSize, "txId", tx.ID)
			return false
		}
	}

	// Validate subject exists based on SubjectType and capture for ownership check
	var subjectIdentity IdentityTransaction
	var title TitleTransaction

	if tx.SubjectType == "QUID" {
		node.IdentityRegistryMutex.RLock()
		identity, exists := node.IdentityRegistry[tx.SubjectID]
		node.IdentityRegistryMutex.RUnlock()

		if !exists {
			logger.Warn("Subject QUID not found in identity registry", "subjectId", tx.SubjectID, "txId", tx.ID)
			return false
		}
		subjectIdentity = identity
	} else {
		node.TitleRegistryMutex.RLock()
		t, exists := node.TitleRegistry[tx.SubjectID]
		node.TitleRegistryMutex.RUnlock()

		if !exists {
			logger.Warn("Subject TITLE not found in title registry", "subjectId", tx.SubjectID, "txId", tx.ID)
			return false
		}
		title = t
	}

	// Validate sequence
	node.EventStreamMutex.RLock()
	stream, streamExists := node.EventStreamRegistry[tx.SubjectID]
	node.EventStreamMutex.RUnlock()

	if streamExists {
		if tx.Sequence <= stream.LatestSequence {
			logger.Warn("Invalid sequence: must be greater than current",
				"providedSequence", tx.Sequence,
				"currentSequence", stream.LatestSequence,
				"subjectId", tx.SubjectID,
				"txId", tx.ID)
			return false
		}
	} else {
		if tx.Sequence != 0 && tx.Sequence != 1 {
			logger.Warn("Invalid sequence for new stream: must be 0 or 1",
				"providedSequence", tx.Sequence,
				"txId", tx.ID)
			return false
		}
	}

	// Verify signature
	if tx.Signature == "" || tx.PublicKey == "" {
		logger.Warn("Missing signature or public key in event transaction", "txId", tx.ID)
		return false
	}

	txCopy := tx
	txCopy.Signature = ""
	signableData, err := json.Marshal(txCopy)
	if err != nil {
		logger.Error("Failed to marshal transaction for signature verification", "txId", tx.ID, "error", err)
		return false
	}

	if !VerifySignature(tx.PublicKey, signableData, tx.Signature) {
		logger.Warn("Invalid signature in event transaction", "txId", tx.ID)
		return false
	}

	// Verify signer is the subject owner
	if tx.SubjectType == "QUID" {
		if subjectIdentity.PublicKey != tx.PublicKey {
			logger.Warn("Signer is not the subject owner",
				"txId", tx.ID,
				"subjectId", tx.SubjectID,
				"subjectType", tx.SubjectType)
			return false
		}
	} else {
		isOwner := false
		for _, stake := range title.Owners {
			node.IdentityRegistryMutex.RLock()
			ownerIdentity, exists := node.IdentityRegistry[stake.OwnerID]
			node.IdentityRegistryMutex.RUnlock()

			if exists && ownerIdentity.PublicKey == tx.PublicKey {
				isOwner = true
				break
			}
		}

		if !isOwner {
			logger.Warn("Signer is not an owner of the subject",
				"txId", tx.ID,
				"subjectId", tx.SubjectID,
				"subjectType", tx.SubjectType)
			return false
		}
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

	// Validate asset quid ID format
	if tx.AssetID != "" && !IsValidQuidID(tx.AssetID) {
		logger.Warn("Invalid asset quid ID format", "assetId", tx.AssetID, "txId", tx.ID)
		return false
	}

	// Validate owner quid ID formats
	for _, stake := range tx.Owners {
		if stake.OwnerID != "" && !IsValidQuidID(stake.OwnerID) {
			logger.Warn("Invalid owner quid ID format", "ownerId", stake.OwnerID, "txId", tx.ID)
			return false
		}
	}

	// Validate string field lengths and control characters
	if tx.TrustDomain != "" && !ValidateStringField(tx.TrustDomain, MaxDomainLength) {
		logger.Warn("Invalid trust domain: too long or contains control characters", "domain", tx.TrustDomain, "txId", tx.ID)
		return false
	}

	if tx.TitleType != "" && !ValidateStringField(tx.TitleType, MaxNameLength) {
		logger.Warn("Invalid title type: too long or contains control characters", "assetId", tx.AssetID, "txId", tx.ID)
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

		// Get signable data for owners (transaction with all signatures and issuer pubkey cleared)
		txCopyForOwners := tx
		txCopyForOwners.Signature = ""
		txCopyForOwners.PublicKey = ""
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

// ValidateBlockCryptographic validates only cryptographic aspects (hash, signatures, chain).
// This is universal - all honest nodes agree on this.
func (node *QuidnugNode) ValidateBlockCryptographic(block Block) bool {
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

	// Verify validator signature against block content
	if len(block.TrustProof.ValidatorSigs) == 0 {
		logger.Debug("Block has no validator signatures", "blockIndex", block.Index)
		return false
	}

	// Get validator public key
	validatorPubKey := block.TrustProof.ValidatorPublicKey
	if validatorPubKey == "" {
		// For backward compatibility with self-generated blocks missing public key
		if block.TrustProof.ValidatorID == node.NodeID {
			validatorPubKey = node.GetPublicKeyHex()
		} else {
			logger.Debug("Block missing validator public key", "blockIndex", block.Index, "validatorId", block.TrustProof.ValidatorID)
			return false
		}
	}

	// Verify that the public key matches the validator ID (ID is derived from pubkey hash)
	pubKeyBytes, err := hex.DecodeString(validatorPubKey)
	if err != nil {
		logger.Debug("Invalid validator public key hex", "blockIndex", block.Index, "error", err)
		return false
	}
	computedID := fmt.Sprintf("%x", sha256.Sum256(pubKeyBytes))[:16]
	if computedID != block.TrustProof.ValidatorID {
		logger.Debug("Validator public key does not match validator ID",
			"blockIndex", block.Index,
			"expectedId", block.TrustProof.ValidatorID,
			"computedId", computedID)
		return false
	}

	// Verify the primary validator signature against block content
	signableData := GetBlockSignableData(block)
	if !VerifySignature(validatorPubKey, signableData, block.TrustProof.ValidatorSigs[0]) {
		logger.Debug("Invalid validator signature", "blockIndex", block.Index, "validatorId", block.TrustProof.ValidatorID)
		return false
	}

	return true
}

// ValidateBlockTiered validates a block and returns tiered acceptance.
// Separates cryptographic validation (universal) from trust validation (subjective).
func (node *QuidnugNode) ValidateBlockTiered(block Block) BlockAcceptance {
	// First, validate cryptographic aspects (universal)
	if !node.ValidateBlockCryptographic(block) {
		return BlockInvalid
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
			return BlockInvalid
		}
	}

	// Trust validation (subjective - different nodes may have different views)
	return node.ValidateTrustProofTiered(block)
}

// ValidateBlock validates a block (backward compatibility wrapper).
// Returns true only if the block is fully trusted.
func (node *QuidnugNode) ValidateBlock(block Block) bool {
	return node.ValidateBlockTiered(block) == BlockTrusted
}

// ValidateTrustProofTiered validates a block's trust proof and returns tiered acceptance.
// Performs cryptographic signature verification against registered validator public keys.
// Returns BlockInvalid if cryptographically invalid, otherwise returns acceptance tier based on trust.
func (node *QuidnugNode) ValidateTrustProofTiered(block Block) BlockAcceptance {
	proof := block.TrustProof

	// Read domain data while holding the lock, then release before computing trust
	node.TrustDomainsMutex.RLock()
	domain, exists := node.TrustDomains[proof.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

	// Check if the trust domain exists
	if !exists {
		return BlockInvalid
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
		return BlockInvalid
	}

	// Verify validator has a registered public key in the domain
	registeredPubKey, hasRegisteredKey := domain.ValidatorPublicKeys[proof.ValidatorID]
	if !hasRegisteredKey || registeredPubKey == "" {
		logger.Debug("Validator public key not registered in domain",
			"validator", proof.ValidatorID,
			"domain", proof.TrustDomain)
		return BlockInvalid
	}

	// Validate signatures exist
	if len(proof.ValidatorSigs) == 0 {
		return BlockInvalid
	}

	// Cryptographically verify signature against registered public key
	signableData := GetBlockSignableData(block)
	if !VerifySignature(registeredPubKey, signableData, proof.ValidatorSigs[0]) {
		logger.Debug("Validator signature verification failed against registered public key",
			"validator", proof.ValidatorID,
			"domain", proof.TrustDomain)
		return BlockInvalid
	}

	// If this node IS the validator, it trusts itself fully
	if proof.ValidatorID == node.NodeID {
		return BlockTrusted
	}

	// Node-relative trust validation: compute relational trust from this node to the validator
	trustLevel, _, err := node.ComputeRelationalTrust(node.NodeID, proof.ValidatorID, DefaultTrustMaxDepth)
	if err != nil {
		logger.Warn("Trust computation exceeded resource limits during block validation",
			"validator", proof.ValidatorID,
			"domain", proof.TrustDomain,
			"error", err)
		// Use partial result - trustLevel contains best found so far
	}

	// Return tier based on trust level
	if trustLevel >= domain.TrustThreshold {
		return BlockTrusted
	}

	if trustLevel > node.DistrustThreshold {
		logger.Debug("Tentative trust in validator",
			"validator", proof.ValidatorID,
			"trustLevel", trustLevel,
			"threshold", domain.TrustThreshold,
			"distrustThreshold", node.DistrustThreshold,
			"domain", proof.TrustDomain)
		return BlockTentative
	}

	// trustLevel <= DistrustThreshold (includes trustLevel == 0)
	logger.Debug("Insufficient relational trust in validator",
		"validator", proof.ValidatorID,
		"trustLevel", trustLevel,
		"threshold", domain.TrustThreshold,
		"distrustThreshold", node.DistrustThreshold,
		"domain", proof.TrustDomain)
	return BlockUntrusted
}

// ValidateTrustProof validates a trust proof using node-relative relational trust.
// Each node validates based on its own trust assessment of the validator.
// This means validation is subjective - different nodes may accept different blocks
// based on their own trust relationships with the validator.
// This is a backward compatibility wrapper - returns true only for BlockTrusted.
// NOTE: This method does not verify cryptographic signatures. Use ValidateBlockTiered
// for full validation including signature verification.
func (node *QuidnugNode) ValidateTrustProof(proof TrustProof) bool {
	// Read domain data
	node.TrustDomainsMutex.RLock()
	domain, exists := node.TrustDomains[proof.TrustDomain]
	node.TrustDomainsMutex.RUnlock()

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

	// Check signatures exist (but cannot verify without block data)
	if len(proof.ValidatorSigs) == 0 {
		return false
	}

	// If this node IS the validator, it trusts itself fully
	if proof.ValidatorID == node.NodeID {
		return true
	}

	// Compute relational trust (error is ignored, partial result used)
	trustLevel, _, _ := node.ComputeRelationalTrust(node.NodeID, proof.ValidatorID, DefaultTrustMaxDepth)

	return trustLevel >= domain.TrustThreshold
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
