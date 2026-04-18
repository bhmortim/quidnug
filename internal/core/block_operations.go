// Package core. block_operations.go — block generation, acceptance, tentative storage.
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func (node *QuidnugNode) runBlockGeneration(ctx context.Context, interval time.Duration) {
	logger.Info("Starting block generation loop", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Block generation loop stopped")
			return
		case <-ticker.C:
			node.TrustDomainsMutex.RLock()
			managedDomains := make([]string, 0, len(node.TrustDomains))
			for domain := range node.TrustDomains {
				managedDomains = append(managedDomains, domain)
			}
			node.TrustDomainsMutex.RUnlock()

			for _, domain := range managedDomains {
				select {
				case <-ctx.Done():
					return
				default:
				}

				block, err := node.GenerateBlock(domain)
				if err != nil {
					logger.Debug("Failed to generate block", "domain", domain, "error", err)
					continue
				}

				if err := node.AddBlock(*block); err != nil {
					logger.Error("Failed to add generated block", "domain", domain, "error", err)
				}
			}
		}
	}
}

// Shutdown performs graceful shutdown of the node

func (node *QuidnugNode) FilterTransactionsForBlock(txs []interface{}, domain string) []interface{} {
	var filtered []interface{}

	for _, tx := range txs {
		var creatorQuid string
		var txID string

		// Extract creator quid based on transaction type
		switch t := tx.(type) {
		case TrustTransaction:
			creatorQuid = t.Truster
			txID = t.ID
		case IdentityTransaction:
			creatorQuid = t.Creator
			txID = t.ID
		case TitleTransaction:
			// For title transactions, use first owner as creator
			if len(t.Owners) > 0 {
				creatorQuid = t.Owners[0].OwnerID
			}
			txID = t.ID
		case EventTransaction:
			// For event transactions, derive creator quid from signer's public key
			if t.PublicKey != "" {
				pubKeyBytes, err := hex.DecodeString(t.PublicKey)
				if err == nil {
					creatorQuid = fmt.Sprintf("%x", sha256.Sum256(pubKeyBytes))[:16]
				}
			}
			txID = t.ID
		default:
			// Unknown transaction type, skip
			logger.Debug("Skipping unknown transaction type in trust filter")
			continue
		}

		// If no creator quid, skip this transaction
		if creatorQuid == "" {
			logger.Debug("Skipping transaction with empty creator",
				"txId", txID,
				"domain", domain)
			continue
		}

		// Compute relational trust from this node to the creator
		trustLevel, _, err := node.ComputeRelationalTrust(node.NodeQuidID, creatorQuid, DefaultTrustMaxDepth)
		if err != nil {
			logger.Warn("Trust computation exceeded resource limits",
				"txId", txID,
				"creator", creatorQuid,
				"error", err,
				"domain", domain)
			// Use partial result (trustLevel contains best found so far)
		}

		// Include if trust meets threshold
		if trustLevel >= node.TransactionTrustThreshold {
			filtered = append(filtered, tx)
		} else {
			logger.Debug("Filtered out transaction due to insufficient trust",
				"txId", txID,
				"creator", creatorQuid,
				"trustLevel", trustLevel,
				"threshold", node.TransactionTrustThreshold,
				"domain", domain)
		}
	}

	return filtered
}

// GenerateBlock generates a new block with pending transactions
func (node *QuidnugNode) GenerateBlock(trustDomain string) (*Block, error) {
	node.BlockchainMutex.RLock()
	prevBlock := node.Blockchain[len(node.Blockchain)-1]
	node.BlockchainMutex.RUnlock()

	// Get validator's participation weight for this domain
	node.TrustDomainsMutex.RLock()
	validatorWeight := 1.0 // default for self-owned domains
	if domain, exists := node.TrustDomains[trustDomain]; exists {
		if weight, found := domain.Validators[node.NodeID]; found {
			validatorWeight = weight
		}
	}
	node.TrustDomainsMutex.RUnlock()

	node.PendingTxsMutex.Lock()
	defer node.PendingTxsMutex.Unlock()

	if len(node.PendingTxs) == 0 {
		return nil, fmt.Errorf("no pending transactions to include in block")
	}

	// Filter transactions for this trust domain
	var domainTxs []interface{}
	var remainingTxs []interface{}

	for _, tx := range node.PendingTxs {
		var txDomain string

		// Extract trust domain based on transaction type
		switch t := tx.(type) {
		case TrustTransaction:
			txDomain = t.TrustDomain
		case IdentityTransaction:
			txDomain = t.TrustDomain
		case TitleTransaction:
			txDomain = t.TrustDomain
		default:
			// Unknown transaction type, skip
			continue
		}

		if txDomain == trustDomain || (trustDomain == "default" && txDomain == "") {
			domainTxs = append(domainTxs, tx)
		} else {
			remainingTxs = append(remainingTxs, tx)
		}
	}

	// Apply trust-based filtering to domain transactions
	domainTxs = node.FilterTransactionsForBlock(domainTxs, trustDomain)

	if len(domainTxs) == 0 {
		return nil, fmt.Errorf("no pending transactions for trust domain: %s", trustDomain)
	}

	// Create a new block
	newBlock := Block{
		Index:        prevBlock.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: domainTxs,
		TrustProof: TrustProof{
			TrustDomain:             trustDomain,
			ValidatorID:             node.NodeID,
			ValidatorPublicKey:      node.GetPublicKeyHex(),
			ValidatorTrustInCreator: validatorWeight,
			ValidatorSigs:           []string{},
			ValidationTime:          time.Now().Unix(),
		},
		PrevHash: prevBlock.Hash,
	}

	// QDP-0001 §6.3: compute per-signer nonce checkpoints at seal time.
	// Populated unconditionally so the receive path can rebuild the
	// ledger from blocks. Not yet included in the signable envelope —
	// see GetBlockSignableData for the rationale.
	newBlock.NonceCheckpoints = computeNonceCheckpoints(domainTxs, trustDomain)

	// Sign the full block content (not just PrevHash) to prevent transaction tampering
	signableData := GetBlockSignableData(newBlock)
	signature, err := node.SignData(signableData)
	if err == nil {
		newBlock.TrustProof.ValidatorSigs = append(newBlock.TrustProof.ValidatorSigs, hex.EncodeToString(signature))
	}

	// Calculate the hash of the new block
	newBlock.Hash = calculateBlockHash(newBlock)

	// Update pending transactions (remove the ones included in this block)
	node.PendingTxs = remainingTxs

	logger.Info("Generated new block",
		"blockIndex", newBlock.Index,
		"domain", trustDomain,
		"txCount", len(domainTxs),
		"hash", newBlock.Hash)

	RecordBlockGenerated(trustDomain)

	return &newBlock, nil
}

// AddBlock adds a block to the blockchain after validation.
// Only accepts fully trusted blocks. For tiered processing, use ReceiveBlock.
func (node *QuidnugNode) AddBlock(block Block) error {
	acceptance, err := node.ReceiveBlock(block)
	if err != nil {
		return err
	}
	if acceptance != BlockTrusted {
		return fmt.Errorf("block not trusted (acceptance tier: %d)", acceptance)
	}
	return nil
}

// ReceiveBlock processes an incoming block with tiered acceptance.
// Extracts trust graph data from all cryptographically valid blocks.
//
// The policy check (IsDomainSupported) runs before the cryptographic
// check. Rationale: a block from a domain this node has no business
// validating is returned as BlockUntrusted so operators can
// distinguish "foreign block" from "malformed block" in logs and
// metrics. The cryptographic test would also fail on an unknown
// domain (validator public keys wouldn't resolve), but BlockInvalid
// incorrectly suggests the block is corrupt — policy-reject first.
func (node *QuidnugNode) ReceiveBlock(block Block) (BlockAcceptance, error) {
	// 0. Domain policy check — cheap, conservative, happens first.
	domain := block.TrustProof.TrustDomain
	if !node.IsDomainSupported(domain) {
		return BlockUntrusted, fmt.Errorf("block rejected: domain %q is not supported by this node", domain)
	}

	// 1. Cryptographic validation
	if !node.ValidateBlockCryptographic(block) {
		return BlockInvalid, fmt.Errorf("block failed cryptographic validation")
	}

	// 2. Extract trust edges from ALL cryptographically valid blocks
	edges := node.ExtractTrustEdgesFromBlock(block, false)
	for _, edge := range edges {
		node.AddUnverifiedTrustEdge(edge)
	}

	// 3. Get tiered acceptance
	acceptance := node.ValidateBlockTiered(block)

	// Record block reception metrics
	RecordBlockReceived(block.TrustProof.TrustDomain, acceptance)

	// 4. Process based on tier
	switch acceptance {
	case BlockTrusted:
		// Add to main chain
		node.BlockchainMutex.Lock()
		node.Blockchain = append(node.Blockchain, block)
		node.BlockchainMutex.Unlock()

		// QDP-0001 §6.4: Trusted tier advances both accepted and
		// tentative per the ledger's tier table.
		if node.NonceLedger != nil {
			node.NonceLedger.ApplyCheckpoints(block.NonceCheckpoints, true)
		}

		// Process transactions
		node.processBlockTransactions(block)

		// Update domain head
		node.TrustDomainsMutex.Lock()
		if domain, exists := node.TrustDomains[block.TrustProof.TrustDomain]; exists {
			domain.BlockchainHead = block.Hash
			node.TrustDomains[block.TrustProof.TrustDomain] = domain
		}
		node.TrustDomainsMutex.Unlock()

		// Promote extracted edges to verified
		for _, edge := range edges {
			node.PromoteTrustEdge(edge.Truster, edge.Trustee)
		}

		if logger != nil {
			logger.Info("Received trusted block",
				"blockIndex", block.Index,
				"hash", block.Hash,
				"domain", block.TrustProof.TrustDomain)
		}

	case BlockTentative:
		if err := node.StoreTentativeBlock(block); err != nil {
			return acceptance, err
		}
		// QDP-0001 §6.4: Tentative tier reserves but does not commit.
		// This is the invariant that prevents partition-split nonce
		// ambiguity (§3.3).
		if node.NonceLedger != nil {
			node.NonceLedger.ApplyCheckpoints(block.NonceCheckpoints, false)
		}
		if logger != nil {
			logger.Info("Received tentative block",
				"blockIndex", block.Index,
				"hash", block.Hash,
				"domain", block.TrustProof.TrustDomain)
		}

	case BlockUntrusted:
		// Edges already added as unverified, don't store block
		if logger != nil {
			logger.Info("Received untrusted block - extracted edges only",
				"blockIndex", block.Index,
				"hash", block.Hash,
				"domain", block.TrustProof.TrustDomain)
		}

	case BlockInvalid:
		return BlockInvalid, fmt.Errorf("block validation failed")
	}

	return acceptance, nil
}

// StoreTentativeBlock stores a block in the tentative storage.
func (node *QuidnugNode) StoreTentativeBlock(block Block) error {
	node.TentativeBlocksMutex.Lock()
	defer node.TentativeBlocksMutex.Unlock()

	domain := block.TrustProof.TrustDomain
	if _, exists := node.TentativeBlocks[domain]; !exists {
		node.TentativeBlocks[domain] = make([]Block, 0)
	}

	// Check if block already exists (by hash)
	for _, existing := range node.TentativeBlocks[domain] {
		if existing.Hash == block.Hash {
			return fmt.Errorf("block %s already in tentative storage", block.Hash)
		}
	}

	node.TentativeBlocks[domain] = append(node.TentativeBlocks[domain], block)
	return nil
}

// GetTentativeBlocks returns tentative blocks for a domain.
func (node *QuidnugNode) GetTentativeBlocks(domain string) []Block {
	node.TentativeBlocksMutex.RLock()
	defer node.TentativeBlocksMutex.RUnlock()

	if blocks, exists := node.TentativeBlocks[domain]; exists {
		result := make([]Block, len(blocks))
		copy(result, blocks)
		return result
	}
	return nil
}

// ReEvaluateTentativeBlocks checks if any tentative blocks can now be promoted.
func (node *QuidnugNode) ReEvaluateTentativeBlocks(domain string) error {
	node.TentativeBlocksMutex.Lock()
	blocks, exists := node.TentativeBlocks[domain]
	if !exists || len(blocks) == 0 {
		node.TentativeBlocksMutex.Unlock()
		return nil
	}
	// Take ownership and clear
	node.TentativeBlocks[domain] = nil
	node.TentativeBlocksMutex.Unlock()

	var remaining []Block
	for _, block := range blocks {
		acceptance := node.ValidateBlockTiered(block)
		switch acceptance {
		case BlockTrusted:
			node.BlockchainMutex.Lock()
			node.Blockchain = append(node.Blockchain, block)
			node.BlockchainMutex.Unlock()

			node.processBlockTransactions(block)

			node.TrustDomainsMutex.Lock()
			if d, exists := node.TrustDomains[block.TrustProof.TrustDomain]; exists {
				d.BlockchainHead = block.Hash
				node.TrustDomains[block.TrustProof.TrustDomain] = d
			}
			node.TrustDomainsMutex.Unlock()

			edges := node.ExtractTrustEdgesFromBlock(block, true)
			for _, edge := range edges {
				node.PromoteTrustEdge(edge.Truster, edge.Trustee)
			}

			if logger != nil {
				logger.Info("Promoted tentative block to trusted",
					"blockIndex", block.Index,
					"hash", block.Hash,
					"domain", domain)
			}

		case BlockTentative:
			remaining = append(remaining, block)

		case BlockUntrusted, BlockInvalid:
			if logger != nil {
				logger.Info("Removed tentative block",
					"blockIndex", block.Index,
					"hash", block.Hash,
					"domain", domain,
					"newStatus", acceptance)
			}
		}
	}

	if len(remaining) > 0 {
		node.TentativeBlocksMutex.Lock()
		node.TentativeBlocks[domain] = remaining
		node.TentativeBlocksMutex.Unlock()
	}

	return nil
}

// GetParentDomain returns the parent domain of a given domain.
// For "sub.example.com", returns "example.com".
// For root domains (no dots), returns empty string.
