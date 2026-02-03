package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentTrustTransactionSubmission tests concurrent submission of trust transactions
// to verify no race conditions occur when multiple goroutines submit transactions simultaneously.
func TestConcurrentTrustTransactionSubmission(t *testing.T) {
	node := newTestNode()

	// Create base identities that will be trusters/trustees
	numIdentities := 10
	identities := make([]string, numIdentities)
	for i := 0; i < numIdentities; i++ {
		quidID := fmt.Sprintf("%016x", i+1)
		identities[i] = quidID

		// Register identity in registry
		node.IdentityRegistryMutex.Lock()
		node.IdentityRegistry[quidID] = IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:        fmt.Sprintf("id-tx-%d", i),
				Type:      TxTypeIdentity,
				Timestamp: time.Now().Unix(),
				PublicKey: node.GetPublicKeyHex(),
			},
			QuidID:  quidID,
			Name:    fmt.Sprintf("Test Identity %d", i),
			Creator: quidID,
		}
		node.IdentityRegistryMutex.Unlock()
	}

	// Submit 100 trust transactions concurrently
	numTransactions := 100
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	for i := 0; i < numTransactions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			trusterIdx := idx % numIdentities
			trusteeIdx := (idx + 1) % numIdentities

			tx := TrustTransaction{
				BaseTransaction: BaseTransaction{
					Type:        TxTypeTrust,
					TrustDomain: "default",
					Timestamp:   time.Now().Unix(),
					PublicKey:   node.GetPublicKeyHex(),
				},
				Truster:    identities[trusterIdx],
				Trustee:    identities[trusteeIdx],
				TrustLevel: 0.8,
				Nonce:      int64(idx + 1),
			}

			// Set ID before signing to ensure signed data matches validated data
			tx.ID = fmt.Sprintf("tx_%d_%d", time.Now().UnixNano(), idx)

			// Sign the transaction
			tx = signTrustTx(node, tx)

			_, err := node.AddTrustTransaction(tx)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// Verify some transactions were processed (nonce conflicts expected for same truster-trustee pairs)
	if successCount == 0 {
		t.Error("Expected at least some transactions to succeed")
	}

	t.Logf("Concurrent submission: %d succeeded, %d failed (nonce conflicts expected)", successCount, errorCount)

	// Verify pending transactions list is consistent
	node.PendingTxsMutex.RLock()
	pendingCount := len(node.PendingTxs)
	node.PendingTxsMutex.RUnlock()

	if pendingCount != int(successCount) {
		t.Errorf("Pending transactions count mismatch: got %d, expected %d", pendingCount, successCount)
	}
}

// TestConcurrentRegistryAccessDuringBlockProcessing tests concurrent reads from registries
// while block processing (writes) is occurring.
func TestConcurrentRegistryAccessDuringBlockProcessing(t *testing.T) {
	node := newTestNode()

	// Pre-populate registries with some data
	numEntries := 20
	for i := 0; i < numEntries; i++ {
		quidID := fmt.Sprintf("%016x", i+100)

		// Add to identity registry
		node.IdentityRegistryMutex.Lock()
		node.IdentityRegistry[quidID] = IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:        fmt.Sprintf("id-%d", i),
				Type:      TxTypeIdentity,
				Timestamp: time.Now().Unix(),
			},
			QuidID: quidID,
			Name:   fmt.Sprintf("Entity %d", i),
		}
		node.IdentityRegistryMutex.Unlock()

		// Add to trust registry
		node.TrustRegistryMutex.Lock()
		if node.TrustRegistry[quidID] == nil {
			node.TrustRegistry[quidID] = make(map[string]float64)
		}
		node.TrustRegistry[quidID][fmt.Sprintf("%016x", (i+1)%numEntries+100)] = 0.7
		node.TrustRegistryMutex.Unlock()
	}

	// Create a block with transactions to process
	assetID := fmt.Sprintf("%016x", 999)
	node.IdentityRegistryMutex.Lock()
	node.IdentityRegistry[assetID] = IdentityTransaction{
		QuidID: assetID,
		Name:   "Test Asset",
	}
	node.IdentityRegistryMutex.Unlock()

	block := Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		Transactions: []interface{}{
			TrustTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeTrust},
				Truster:         fmt.Sprintf("%016x", 100),
				Trustee:         fmt.Sprintf("%016x", 101),
				TrustLevel:      0.9,
				Nonce:           1,
			},
			IdentityTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeIdentity},
				QuidID:          fmt.Sprintf("%016x", 200),
				Name:            "New Identity",
				Creator:         fmt.Sprintf("%016x", 100),
			},
			TitleTransaction{
				BaseTransaction: BaseTransaction{Type: TxTypeTitle},
				AssetID:         assetID,
				Owners:          []OwnershipStake{{OwnerID: fmt.Sprintf("%016x", 100), Percentage: 100.0}},
			},
		},
		TrustProof: TrustProof{
			TrustDomain: "default",
			ValidatorID: node.NodeID,
		},
		PrevHash: "0",
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Start concurrent readers
	numReaders := 50
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			iterations := 0
			for {
				select {
				case <-done:
					return
				default:
					// Read from trust registry
					node.TrustRegistryMutex.RLock()
					for truster := range node.TrustRegistry {
						_ = truster
						break
					}
					node.TrustRegistryMutex.RUnlock()

					// Read from identity registry
					node.IdentityRegistryMutex.RLock()
					for quid := range node.IdentityRegistry {
						_ = quid
						break
					}
					node.IdentityRegistryMutex.RUnlock()

					// Read from title registry
					node.TitleRegistryMutex.RLock()
					for asset := range node.TitleRegistry {
						_ = asset
						break
					}
					node.TitleRegistryMutex.RUnlock()

					iterations++
				}
			}
		}(i)
	}

	// Process blocks concurrently with reads
	numBlocks := 10
	for i := 0; i < numBlocks; i++ {
		blockCopy := block
		blockCopy.Index = int64(i + 1)
		node.processBlockTransactions(blockCopy)
	}

	// Signal readers to stop
	close(done)
	wg.Wait()

	t.Log("Concurrent registry access during block processing completed without race conditions")
}

// TestConcurrentTrustComputation tests that multiple concurrent ComputeRelationalTrust calls
// return consistent results and don't cause race conditions.
func TestConcurrentTrustComputation(t *testing.T) {
	node := newTestNode()

	// Build a trust graph: A -> B -> C -> D -> E
	entities := []string{
		fmt.Sprintf("%016x", 1),
		fmt.Sprintf("%016x", 2),
		fmt.Sprintf("%016x", 3),
		fmt.Sprintf("%016x", 4),
		fmt.Sprintf("%016x", 5),
	}

	node.TrustRegistryMutex.Lock()
	for i := 0; i < len(entities)-1; i++ {
		if node.TrustRegistry[entities[i]] == nil {
			node.TrustRegistry[entities[i]] = make(map[string]float64)
		}
		node.TrustRegistry[entities[i]][entities[i+1]] = 0.8
	}
	node.TrustRegistryMutex.Unlock()

	// Expected trust from A to E: 0.8^4 = 0.4096
	expectedTrust := 0.8 * 0.8 * 0.8 * 0.8

	numGoroutines := 100
	var wg sync.WaitGroup
	results := make([]float64, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			trust, _, err := node.ComputeRelationalTrust(entities[0], entities[4], 5)
			results[idx] = trust
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all results are consistent
	for i := 0; i < numGoroutines; i++ {
		if errors[i] != nil {
			t.Errorf("Goroutine %d got unexpected error: %v", i, errors[i])
			continue
		}
		// Allow small floating point tolerance
		if results[i] < expectedTrust-0.0001 || results[i] > expectedTrust+0.0001 {
			t.Errorf("Goroutine %d got inconsistent trust: %f, expected %f", i, results[i], expectedTrust)
		}
	}

	t.Logf("All %d concurrent trust computations returned consistent results: %f", numGoroutines, expectedTrust)
}

// TestConcurrentBlockGenerationAndReception tests that block generation and reception
// can occur concurrently without race conditions.
func TestConcurrentBlockGenerationAndReception(t *testing.T) {
	node := newTestNode()

	// Pre-populate with identities for transaction validation
	for i := 0; i < 10; i++ {
		quidID := fmt.Sprintf("%016x", i+1)
		node.IdentityRegistryMutex.Lock()
		node.IdentityRegistry[quidID] = IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:        fmt.Sprintf("id-%d", i),
				Type:      TxTypeIdentity,
				Timestamp: time.Now().Unix(),
				PublicKey: node.GetPublicKeyHex(),
			},
			QuidID:  quidID,
			Name:    fmt.Sprintf("Entity %d", i),
			Creator: quidID,
		}
		node.IdentityRegistryMutex.Unlock()

		// Add trust relationship so transactions pass filtering
		node.TrustRegistryMutex.Lock()
		if node.TrustRegistry[node.NodeQuidID] == nil {
			node.TrustRegistry[node.NodeQuidID] = make(map[string]float64)
		}
		node.TrustRegistry[node.NodeQuidID][quidID] = 1.0
		node.TrustRegistryMutex.Unlock()
	}

	var wg sync.WaitGroup
	var generatedBlocks int32
	var receivedBlocks int32

	// Goroutines that add transactions and generate blocks
	numGenerators := 5
	for i := 0; i < numGenerators; i++ {
		wg.Add(1)
		go func(genID int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				// Add a transaction
				trusterIdx := (genID*3 + j) % 10
				trusteeIdx := (genID*3 + j + 1) % 10

				tx := TrustTransaction{
					BaseTransaction: BaseTransaction{
						Type:        TxTypeTrust,
						TrustDomain: "default",
						Timestamp:   time.Now().Unix(),
						PublicKey:   node.GetPublicKeyHex(),
					},
					Truster:    fmt.Sprintf("%016x", trusterIdx+1),
					Trustee:    fmt.Sprintf("%016x", trusteeIdx+1),
					TrustLevel: 0.7,
					Nonce:      int64(genID*100 + j + 1),
				}

				// Set ID before signing to ensure signed data matches validated data
				tx.ID = fmt.Sprintf("tx_%d_%d_%d", time.Now().UnixNano(), genID, j)
				tx = signTrustTx(node, tx)

				node.AddTrustTransaction(tx)

				// Try to generate a block
				block, err := node.GenerateBlock("default")
				if err == nil && block != nil {
					atomic.AddInt32(&generatedBlocks, 1)
				}

				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Goroutines that simulate receiving blocks
	numReceivers := 5
	for i := 0; i < numReceivers; i++ {
		wg.Add(1)
		go func(recvID int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				// Create a simulated received block
				node.BlockchainMutex.RLock()
				prevBlock := node.Blockchain[len(node.Blockchain)-1]
				node.BlockchainMutex.RUnlock()

				block := Block{
					Index:        prevBlock.Index + 1,
					Timestamp:    time.Now().Unix(),
					Transactions: []interface{}{},
					TrustProof: TrustProof{
						TrustDomain:        "default",
						ValidatorID:        node.NodeID,
						ValidatorPublicKey: node.GetPublicKeyHex(),
						ValidatorSigs:      []string{},
						ValidationTime:     time.Now().Unix(),
					},
					PrevHash: prevBlock.Hash,
				}

				// Sign the block
				signableData := GetBlockSignableData(block)
				sig, _ := node.SignData(signableData)
				block.TrustProof.ValidatorSigs = []string{hex.EncodeToString(sig)}
				block.Hash = calculateBlockHash(block)

				// Try to add the block (may fail due to race with other blocks)
				err := node.AddBlock(block)
				if err == nil {
					atomic.AddInt32(&receivedBlocks, 1)
				}

				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Block generation/reception completed: %d generated, %d received without race conditions",
		generatedBlocks, receivedBlocks)

	// Log final chain length (linear chain integrity not verified as concurrent operations
	// without consensus naturally create competing blocks at the same index)
	node.BlockchainMutex.RLock()
	chainLen := len(node.Blockchain)
	node.BlockchainMutex.RUnlock()

	t.Logf("Final blockchain length: %d blocks", chainLen)
}

// TestConcurrentNonceUpdates tests that concurrent transactions with auto-assigned nonces
// don't result in nonce collisions or race conditions.
func TestConcurrentNonceUpdates(t *testing.T) {
	node := newTestNode()

	// Create a single truster-trustee pair that all transactions will use
	truster := fmt.Sprintf("%016x", 1)
	trustee := fmt.Sprintf("%016x", 2)

	// Register identities
	node.IdentityRegistryMutex.Lock()
	node.IdentityRegistry[truster] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "id-truster",
			Type:      TxTypeIdentity,
			Timestamp: time.Now().Unix(),
			PublicKey: node.GetPublicKeyHex(),
		},
		QuidID:  truster,
		Name:    "Truster",
		Creator: truster,
	}
	node.IdentityRegistry[trustee] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "id-trustee",
			Type:      TxTypeIdentity,
			Timestamp: time.Now().Unix(),
			PublicKey: node.GetPublicKeyHex(),
		},
		QuidID:  trustee,
		Name:    "Trustee",
		Creator: trustee,
	}
	node.IdentityRegistryMutex.Unlock()

	// Submit transactions concurrently with nonce=0 (auto-assign)
	numTransactions := 50
	var wg sync.WaitGroup
	var successCount int32
	assignedNonces := make([]int64, numTransactions)
	var nonceMutex sync.Mutex

	for i := 0; i < numTransactions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			tx := TrustTransaction{
				BaseTransaction: BaseTransaction{
					Type:        TxTypeTrust,
					TrustDomain: "default",
					Timestamp:   time.Now().Unix(),
					PublicKey:   node.GetPublicKeyHex(),
				},
				Truster:    truster,
				Trustee:    trustee,
				TrustLevel: 0.5 + float64(idx%5)*0.1,
				Nonce:      0, // Auto-assign nonce
			}

			// Set ID before signing to ensure signed data matches validated data
			tx.ID = fmt.Sprintf("tx_%d_%d", time.Now().UnixNano(), idx)
			tx = signTrustTx(node, tx)

			txID, err := node.AddTrustTransaction(tx)
			if err == nil {
				atomic.AddInt32(&successCount, 1)

				// Record the assigned nonce by finding the transaction
				node.PendingTxsMutex.RLock()
				for _, pendingTx := range node.PendingTxs {
					if trustTx, ok := pendingTx.(TrustTransaction); ok {
						if trustTx.ID == txID {
							nonceMutex.Lock()
							assignedNonces[idx] = trustTx.Nonce
							nonceMutex.Unlock()
							break
						}
					}
				}
				node.PendingTxsMutex.RUnlock()
			}
		}(i)
	}

	wg.Wait()

	// Due to the way nonce auto-assignment works (checking current registry),
	// concurrent submissions for the same truster-trustee pair will mostly fail
	// because they'll all try to use the same next nonce
	t.Logf("Nonce auto-assignment: %d/%d succeeded (failures expected due to concurrent nonce conflicts)",
		successCount, numTransactions)

	// Verify that successful transactions have valid nonces
	node.PendingTxsMutex.RLock()
	seenNonces := make(map[int64]bool)
	for _, tx := range node.PendingTxs {
		if trustTx, ok := tx.(TrustTransaction); ok {
			if trustTx.Truster == truster && trustTx.Trustee == trustee {
				if seenNonces[trustTx.Nonce] {
					t.Errorf("Duplicate nonce detected: %d", trustTx.Nonce)
				}
				seenNonces[trustTx.Nonce] = true
			}
		}
	}
	node.PendingTxsMutex.RUnlock()

	t.Logf("Verified no duplicate nonces in pending transactions")
}

// TestConcurrentTentativeBlockStorage tests concurrent access to tentative block storage.
func TestConcurrentTentativeBlockStorage(t *testing.T) {
	node := newTestNode()

	domain := "test.domain"
	numBlocks := 50
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < numBlocks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			block := Block{
				Index:     int64(idx),
				Timestamp: time.Now().Unix(),
				Hash:      fmt.Sprintf("hash-%d", idx),
				TrustProof: TrustProof{
					TrustDomain: domain,
					ValidatorID: fmt.Sprintf("validator-%d", idx),
				},
			}

			node.StoreTentativeBlock(block)
		}(i)
	}

	// Concurrent readers
	numReaders := 20
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				blocks := node.GetTentativeBlocks(domain)
				_ = len(blocks)
				time.Sleep(time.Microsecond * 100)
			}
		}()
	}

	wg.Wait()

	// Verify all blocks were stored
	blocks := node.GetTentativeBlocks(domain)
	if len(blocks) != numBlocks {
		t.Errorf("Expected %d tentative blocks, got %d", numBlocks, len(blocks))
	}

	t.Logf("Concurrent tentative block storage completed: %d blocks stored", len(blocks))
}

// TestConcurrentTrustEdgeOperations tests concurrent add/promote/demote of trust edges.
func TestConcurrentTrustEdgeOperations(t *testing.T) {
	node := newTestNode()

	numEdges := 50
	var wg sync.WaitGroup

	// Concurrent edge additions
	for i := 0; i < numEdges; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			truster := fmt.Sprintf("%016x", idx)
			trustee := fmt.Sprintf("%016x", idx+1000)

			edge := TrustEdge{
				Truster:       truster,
				Trustee:       trustee,
				TrustLevel:    0.8,
				SourceBlock:   fmt.Sprintf("block-%d", idx),
				ValidatorQuid: "validator",
				Timestamp:     time.Now().Unix(),
			}

			// Add as unverified
			node.AddUnverifiedTrustEdge(edge)

			// Promote to verified
			node.PromoteTrustEdge(truster, trustee)

			// Get edges
			_ = node.GetTrustEdges(truster, true)
		}(i)
	}

	wg.Wait()

	// Verify edges were added correctly
	verifiedCount := 0
	node.TrustRegistryMutex.RLock()
	for truster := range node.VerifiedTrustEdges {
		verifiedCount += len(node.VerifiedTrustEdges[truster])
		_ = truster
	}
	node.TrustRegistryMutex.RUnlock()

	if verifiedCount != numEdges {
		t.Errorf("Expected %d verified edges, got %d", numEdges, verifiedCount)
	}

	t.Logf("Concurrent trust edge operations completed: %d edges processed", verifiedCount)
}

// TestConcurrentDomainAccess tests concurrent access to trust domains.
func TestConcurrentDomainAccess(t *testing.T) {
	node := newTestNode()

	numDomains := 20
	var wg sync.WaitGroup

	// Concurrent domain registration
	for i := 0; i < numDomains; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			domain := TrustDomain{
				Name:           fmt.Sprintf("domain-%d.example.com", idx),
				ValidatorNodes: []string{node.NodeID},
				TrustThreshold: 0.75,
			}

			node.RegisterTrustDomain(domain)
		}(i)
	}

	// Concurrent readers
	numReaders := 30
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				node.TrustDomainsMutex.RLock()
				for name := range node.TrustDomains {
					_ = name
				}
				node.TrustDomainsMutex.RUnlock()
				time.Sleep(time.Microsecond * 50)
			}
		}()
	}

	wg.Wait()

	// Verify domains were registered (plus the default domain)
	node.TrustDomainsMutex.RLock()
	domainCount := len(node.TrustDomains)
	node.TrustDomainsMutex.RUnlock()

	if domainCount != numDomains+2 { // +2 for default domain and test.domain.com
		t.Errorf("Expected %d domains, got %d", numDomains+2, domainCount)
	}

	t.Logf("Concurrent domain access completed: %d domains registered", domainCount)
}

// TestConcurrentMixedOperations tests a realistic mix of concurrent operations.
func TestConcurrentMixedOperations(t *testing.T) {
	node := newTestNode()

	// Pre-populate some data
	for i := 0; i < 20; i++ {
		quidID := fmt.Sprintf("%016x", i+1)
		node.IdentityRegistryMutex.Lock()
		node.IdentityRegistry[quidID] = IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:        fmt.Sprintf("id-%d", i),
				Type:      TxTypeIdentity,
				Timestamp: time.Now().Unix(),
				PublicKey: node.GetPublicKeyHex(),
			},
			QuidID:  quidID,
			Name:    fmt.Sprintf("Entity %d", i),
			Creator: quidID,
		}
		node.IdentityRegistryMutex.Unlock()

		node.TrustRegistryMutex.Lock()
		if node.TrustRegistry[quidID] == nil {
			node.TrustRegistry[quidID] = make(map[string]float64)
		}
		nextQuid := fmt.Sprintf("%016x", (i+1)%20+1)
		node.TrustRegistry[quidID][nextQuid] = 0.8
		node.TrustRegistryMutex.Unlock()
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Trust computations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				observer := fmt.Sprintf("%016x", 1)
				target := fmt.Sprintf("%016x", 10)
				node.ComputeRelationalTrust(observer, target, 5)
			}
		}
	}()

	// Registry reads
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				node.TrustRegistryMutex.RLock()
				_ = len(node.TrustRegistry)
				node.TrustRegistryMutex.RUnlock()

				node.IdentityRegistryMutex.RLock()
				_ = len(node.IdentityRegistry)
				node.IdentityRegistryMutex.RUnlock()

				node.TitleRegistryMutex.RLock()
				_ = len(node.TitleRegistry)
				node.TitleRegistryMutex.RUnlock()
			}
		}
	}()

	// Transaction submissions
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-done:
				return
			default:
				counter++
				tx := TrustTransaction{
					BaseTransaction: BaseTransaction{
						Type:        TxTypeTrust,
						TrustDomain: "default",
						Timestamp:   time.Now().Unix(),
						PublicKey:   node.GetPublicKeyHex(),
					},
					Truster:    fmt.Sprintf("%016x", counter%20+1),
					Trustee:    fmt.Sprintf("%016x", (counter+1)%20+1),
					TrustLevel: 0.7,
					Nonce:      int64(counter),
				}

				txData, _ := hex.DecodeString(tx.PublicKey)
				hash := sha256.Sum256(append([]byte(tx.Truster+tx.Trustee), txData...))
				tx.ID = hex.EncodeToString(hash[:])

				tx = signTrustTx(node, tx)
				node.AddTrustTransaction(tx)

				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Block operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				node.GenerateBlock("default")
				time.Sleep(time.Millisecond * 5)
			}
		}
	}()

	// Let operations run concurrently
	time.Sleep(time.Millisecond * 200)
	close(done)
	wg.Wait()

	t.Log("Concurrent mixed operations completed without race conditions")

	// Verify data integrity
	node.BlockchainMutex.RLock()
	chainLen := len(node.Blockchain)
	node.BlockchainMutex.RUnlock()

	node.PendingTxsMutex.RLock()
	pendingLen := len(node.PendingTxs)
	node.PendingTxsMutex.RUnlock()

	t.Logf("Final state: blockchain=%d blocks, pending=%d transactions", chainLen, pendingLen)
}
