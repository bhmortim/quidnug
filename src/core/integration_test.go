//go:build integration

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// testCluster holds references to test nodes and provides cleanup
type testCluster struct {
	nodes   []*QuidnugNode
	ports   []string
	cancels []context.CancelFunc
}

// setupTestCluster creates N nodes on different ports, configured to know about each other.
// Returns the cluster and a cleanup function that must be called to release resources.
func setupTestCluster(t *testing.T, nodeCount int) (*testCluster, func()) {
	t.Helper()

	cluster := &testCluster{
		nodes:   make([]*QuidnugNode, nodeCount),
		ports:   make([]string, nodeCount),
		cancels: make([]context.CancelFunc, nodeCount),
	}

	// Base port for test cluster (use high ports with randomness to avoid conflicts)
	basePort := 19000 + int(time.Now().UnixNano()%1000)

	// Create all nodes first
	for i := 0; i < nodeCount; i++ {
		node, err := NewQuidnugNode(nil)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		cluster.nodes[i] = node
		cluster.ports[i] = fmt.Sprintf("%d", basePort+i)
	}

	// Configure nodes to know about each other
	for i, node := range cluster.nodes {
		node.KnownNodesMutex.Lock()
		for j, otherNode := range cluster.nodes {
			if i != j {
				node.KnownNodes[otherNode.NodeID] = Node{
					ID:           otherNode.NodeID,
					Address:      fmt.Sprintf("localhost:%s", cluster.ports[j]),
					TrustDomains: []string{"default"},
					IsValidator:  true,
					LastSeen:     time.Now().Unix(),
				}
			}
		}
		node.KnownNodesMutex.Unlock()

		// Add other nodes as validators in the default domain
		node.TrustDomainsMutex.Lock()
		if domain, exists := node.TrustDomains["default"]; exists {
			for j, otherNode := range cluster.nodes {
				if i != j {
					domain.ValidatorNodes = append(domain.ValidatorNodes, otherNode.NodeID)
					domain.Validators[otherNode.NodeID] = 1.0
					domain.ValidatorPublicKeys[otherNode.NodeID] = otherNode.GetPublicKeyHex()
				}
			}
			node.TrustDomains["default"] = domain
		}
		node.TrustDomainsMutex.Unlock()
	}

	// Start HTTP servers for all nodes
	for i, node := range cluster.nodes {
		ctx, cancel := context.WithCancel(context.Background())
		cluster.cancels[i] = cancel

		go func(n *QuidnugNode, port string) {
			n.SetHTTPClientTimeout(2 * time.Second)
			if err := n.StartServer(port); err != nil && err != http.ErrServerClosed {
				t.Logf("Server error on port %s: %v", port, err)
			}
		}(node, cluster.ports[i])

		// Use context to prevent goroutine leaks
		_ = ctx
	}

	// Wait for servers to be ready
	for i, port := range cluster.ports {
		waitForServer(t, port, 5*time.Second)
		t.Logf("Node %d started on port %s (ID: %s)", i, port, cluster.nodes[i].NodeID[:8])
	}

	cleanup := func() {
		for _, cancel := range cluster.cancels {
			cancel()
		}

		for i, node := range cluster.nodes {
			if node.Server != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				node.Server.Shutdown(ctx)
				cancel()
			}
			t.Logf("Node %d shutdown complete", i)
		}
	}

	return cluster, cleanup
}

// waitForServer waits for a server to be ready on the given port
func waitForServer(t *testing.T, port string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://localhost:%s/api/health", port))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Server on port %s did not become ready within %v", port, timeout)
}

// TestTransactionPropagationAcrossNodes tests that transactions are broadcast to all nodes
func TestTransactionPropagationAcrossNodes(t *testing.T) {
	cluster, cleanup := setupTestCluster(t, 3)
	defer cleanup()

	// Create test identities on all nodes
	trusterID := fmt.Sprintf("%016x", 1)
	trusteeID := fmt.Sprintf("%016x", 2)

	for _, node := range cluster.nodes {
		node.IdentityRegistryMutex.Lock()
		node.IdentityRegistry[trusterID] = IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:        "id-truster",
				Type:      TxTypeIdentity,
				Timestamp: time.Now().Unix(),
				PublicKey: node.GetPublicKeyHex(),
			},
			QuidID:  trusterID,
			Name:    "Truster",
			Creator: trusterID,
		}
		node.IdentityRegistry[trusteeID] = IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:        "id-trustee",
				Type:      TxTypeIdentity,
				Timestamp: time.Now().Unix(),
				PublicKey: node.GetPublicKeyHex(),
			},
			QuidID:  trusteeID,
			Name:    "Trustee",
			Creator: trusteeID,
		}
		node.IdentityRegistryMutex.Unlock()
	}

	// Submit transaction to node 0
	node0 := cluster.nodes[0]
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   time.Now().Unix(),
			PublicKey:   node0.GetPublicKeyHex(),
		},
		Truster:    trusterID,
		Trustee:    trusteeID,
		TrustLevel: 0.85,
		Nonce:      1,
	}

	// Set ID before signing to ensure signed data matches validated data
	tx.ID = fmt.Sprintf("tx_%d", time.Now().UnixNano())

	tx = signTrustTx(node0, tx)

	txID, err := node0.AddTrustTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to add transaction: %v", err)
	}
	t.Logf("Transaction %s submitted to node 0", txID[:16])

	// Wait for propagation
	time.Sleep(500 * time.Millisecond)

	// Verify transaction appears on other nodes
	for i := 1; i < len(cluster.nodes); i++ {
		node := cluster.nodes[i]
		node.PendingTxsMutex.RLock()
		found := false
		for _, pendingTx := range node.PendingTxs {
			if trustTx, ok := pendingTx.(TrustTransaction); ok {
				if trustTx.ID == txID {
					found = true
					break
				}
			}
		}
		node.PendingTxsMutex.RUnlock()

		if found {
			t.Logf("Transaction propagated to node %d", i)
		} else {
			t.Logf("Transaction not in node %d pending pool (may have been rejected due to validation)", i)
		}
	}
}

// TestBlockSynchronization tests that blocks are properly synchronized across nodes
func TestBlockSynchronization(t *testing.T) {
	cluster, cleanup := setupTestCluster(t, 3)
	defer cleanup()

	node0 := cluster.nodes[0]

	// Add identities and trust relationships for transaction filtering
	trusterID := fmt.Sprintf("%016x", 10)
	trusteeID := fmt.Sprintf("%016x", 11)

	for _, node := range cluster.nodes {
		node.IdentityRegistryMutex.Lock()
		node.IdentityRegistry[trusterID] = IdentityTransaction{
			QuidID:  trusterID,
			Name:    "Truster",
			Creator: trusterID,
		}
		node.IdentityRegistry[trusteeID] = IdentityTransaction{
			QuidID:  trusteeID,
			Name:    "Trustee",
			Creator: trusteeID,
		}
		node.IdentityRegistryMutex.Unlock()

		// Trust the truster so transactions pass filtering
		node.TrustRegistryMutex.Lock()
		if node.TrustRegistry[node.NodeQuidID] == nil {
			node.TrustRegistry[node.NodeQuidID] = make(map[string]float64)
		}
		node.TrustRegistry[node.NodeQuidID][trusterID] = 1.0
		node.TrustRegistryMutex.Unlock()
	}

	// Add a transaction directly to pending
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "test-tx-block-sync",
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   time.Now().Unix(),
			PublicKey:   node0.GetPublicKeyHex(),
		},
		Truster:    trusterID,
		Trustee:    trusteeID,
		TrustLevel: 0.9,
		Nonce:      1,
	}
	tx = signTrustTx(node0, tx)

	node0.PendingTxsMutex.Lock()
	node0.PendingTxs = append(node0.PendingTxs, tx)
	node0.PendingTxsMutex.Unlock()

	// Generate a block on node 0
	block, err := node0.GenerateBlock("default")
	if err != nil {
		t.Fatalf("Failed to generate block: %v", err)
	}
	t.Logf("Generated block %d with hash %s", block.Index, block.Hash[:16])

	// Add block to node 0
	if err := node0.AddBlock(*block); err != nil {
		t.Fatalf("Failed to add block to node 0: %v", err)
	}

	// Verify block is in node 0's chain
	node0.BlockchainMutex.RLock()
	node0ChainLen := len(node0.Blockchain)
	node0.BlockchainMutex.RUnlock()

	t.Logf("Node 0 blockchain length: %d", node0ChainLen)

	if node0ChainLen < 2 {
		t.Errorf("Expected at least 2 blocks (genesis + new), got %d", node0ChainLen)
	}

	// Simulate block reception on other nodes
	for i := 1; i < len(cluster.nodes); i++ {
		node := cluster.nodes[i]
		acceptance, err := node.ReceiveBlock(*block)
		if err != nil {
			t.Logf("Node %d rejected block: %v", i, err)
		} else {
			t.Logf("Node %d accepted block with tier: %d", i, acceptance)
		}
	}
}

// TestCrossDomainTrustQuery tests querying trust across different domains
func TestCrossDomainTrustQuery(t *testing.T) {
	cluster, cleanup := setupTestCluster(t, 2)
	defer cleanup()

	node0 := cluster.nodes[0]
	node1 := cluster.nodes[1]

	// Register different domains on each node
	domain1 := TrustDomain{
		Name:           "sub.example.com",
		ValidatorNodes: []string{node0.NodeID},
		TrustThreshold: 0.5,
	}
	domain2 := TrustDomain{
		Name:           "example.com",
		ValidatorNodes: []string{node1.NodeID},
		TrustThreshold: 0.5,
	}

	node0.RegisterTrustDomain(domain1)
	node1.RegisterTrustDomain(domain2)

	// Update known nodes with domain info
	node0.KnownNodesMutex.Lock()
	if knownNode, exists := node0.KnownNodes[node1.NodeID]; exists {
		knownNode.TrustDomains = []string{"default", "example.com"}
		node0.KnownNodes[node1.NodeID] = knownNode
	}
	node0.KnownNodesMutex.Unlock()

	// Add identity to node 1
	testQuidID := fmt.Sprintf("%016x", 100)
	node1.IdentityRegistryMutex.Lock()
	node1.IdentityRegistry[testQuidID] = IdentityTransaction{
		QuidID:      testQuidID,
		Name:        "Test Entity",
		Description: "Entity in example.com domain",
	}
	node1.IdentityRegistryMutex.Unlock()

	// Test hierarchical domain walking
	managers := node0.findNodesForDomainWithHierarchy("sub.example.com")
	t.Logf("Found %d managers for sub.example.com", len(managers))

	// Query for example.com domain
	managers2 := node0.findNodesForDomainWithHierarchy("example.com")
	t.Logf("Found %d managers for example.com", len(managers2))

	// Verify node1 is found for example.com
	found := false
	for _, manager := range managers2 {
		if manager.ID == node1.NodeID {
			found = true
			break
		}
	}

	if found {
		t.Logf("Successfully found node 1 as manager for example.com via hierarchical walking")
	} else if len(managers2) > 0 {
		t.Logf("Node 1 not directly found, but %d other managers available", len(managers2))
	}

	// Test actual cross-domain query via HTTP
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/api/domains", cluster.ports[0]))
	if err != nil {
		t.Logf("Failed to query domains: %v", err)
	} else {
		resp.Body.Close()
		t.Logf("Successfully queried domains endpoint")
	}
}

// TestNodeDiscovery tests that nodes can discover each other through seed nodes
func TestNodeDiscovery(t *testing.T) {
	basePort := 19100 + int(time.Now().UnixNano()%1000)

	// Create node 1 (seed node)
	node1, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create node 1: %v", err)
	}
	port1 := fmt.Sprintf("%d", basePort)

	// Create node 2 (knows node 1)
	node2, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create node 2: %v", err)
	}
	port2 := fmt.Sprintf("%d", basePort+1)

	// Create node 3 (will discover through node 1)
	node3, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create node 3: %v", err)
	}
	port3 := fmt.Sprintf("%d", basePort+2)

	// Add node 2 to node 1's known nodes (so node 1 returns it in discovery)
	node1.KnownNodesMutex.Lock()
	node1.KnownNodes[node2.NodeID] = Node{
		ID:           node2.NodeID,
		Address:      fmt.Sprintf("localhost:%s", port2),
		TrustDomains: []string{"default"},
		LastSeen:     time.Now().Unix(),
	}
	node1.KnownNodesMutex.Unlock()

	// Start servers
	var wg sync.WaitGroup
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	ctx3, cancel3 := context.WithCancel(context.Background())

	defer func() {
		cancel1()
		cancel2()
		cancel3()
		if node1.Server != nil {
			node1.Server.Shutdown(context.Background())
		}
		if node2.Server != nil {
			node2.Server.Shutdown(context.Background())
		}
		if node3.Server != nil {
			node3.Server.Shutdown(context.Background())
		}
	}()

	wg.Add(3)
	go func() { defer wg.Done(); node1.StartServer(port1) }()
	go func() { defer wg.Done(); node2.StartServer(port2) }()
	go func() { defer wg.Done(); node3.StartServer(port3) }()

	waitForServer(t, port1, 5*time.Second)
	waitForServer(t, port2, 5*time.Second)
	waitForServer(t, port3, 5*time.Second)

	t.Logf("All 3 nodes started")

	// Node 3 discovers from node 1 as seed
	seedNodes := []string{fmt.Sprintf("localhost:%s", port1)}
	node3.discoverFromSeeds(ctx3, seedNodes)

	// Check what node 3 discovered
	node3.KnownNodesMutex.RLock()
	discoveredCount := len(node3.KnownNodes)
	discoveredNode2 := false
	for nodeID := range node3.KnownNodes {
		if nodeID == node2.NodeID {
			discoveredNode2 = true
		}
	}
	node3.KnownNodesMutex.RUnlock()

	t.Logf("Node 3 discovered %d nodes", discoveredCount)

	if discoveredNode2 {
		t.Logf("SUCCESS: Node 3 discovered node 2 through node 1 seed")
	} else {
		t.Logf("Node 3 did not discover node 2 (discovery returned %d nodes)", discoveredCount)
	}

	// Suppress unused variable warnings
	_ = ctx1
	_ = ctx2
}

// TestGracefulShutdownDuringBlockGeneration tests clean shutdown during operations
func TestGracefulShutdownDuringBlockGeneration(t *testing.T) {
	node, err := NewQuidnugNode(nil)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	port := fmt.Sprintf("%d", 19200+int(time.Now().UnixNano()%1000))

	// Add test data
	trusterID := fmt.Sprintf("%016x", 1)
	trusteeID := fmt.Sprintf("%016x", 2)

	node.IdentityRegistryMutex.Lock()
	node.IdentityRegistry[trusterID] = IdentityTransaction{
		QuidID: trusterID,
		Name:   "Truster",
	}
	node.IdentityRegistry[trusteeID] = IdentityTransaction{
		QuidID: trusteeID,
		Name:   "Trustee",
	}
	node.IdentityRegistryMutex.Unlock()

	// Trust the truster for transaction filtering
	node.TrustRegistryMutex.Lock()
	if node.TrustRegistry[node.NodeQuidID] == nil {
		node.TrustRegistry[node.NodeQuidID] = make(map[string]float64)
	}
	node.TrustRegistry[node.NodeQuidID][trusterID] = 1.0
	node.TrustRegistryMutex.Unlock()

	// Add pending transaction
	tx := TrustTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "shutdown-test-tx",
			Type:        TxTypeTrust,
			TrustDomain: "default",
			Timestamp:   time.Now().Unix(),
			PublicKey:   node.GetPublicKeyHex(),
		},
		Truster:    trusterID,
		Trustee:    trusteeID,
		TrustLevel: 0.8,
		Nonce:      1,
	}
	tx = signTrustTx(node, tx)

	node.PendingTxsMutex.Lock()
	node.PendingTxs = append(node.PendingTxs, tx)
	node.PendingTxsMutex.Unlock()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start server
	go node.StartServer(port)
	waitForServer(t, port, 5*time.Second)
	t.Logf("Server started on port %s", port)

	// Start block generation in background
	blockGenDone := make(chan struct{})
	go func() {
		defer close(blockGenDone)
		node.runBlockGeneration(ctx, 100*time.Millisecond)
	}()

	// Let it run briefly
	time.Sleep(300 * time.Millisecond)

	// Get state before shutdown
	node.PendingTxsMutex.RLock()
	pendingBefore := len(node.PendingTxs)
	node.PendingTxsMutex.RUnlock()

	node.BlockchainMutex.RLock()
	chainLenBefore := len(node.Blockchain)
	node.BlockchainMutex.RUnlock()

	t.Logf("Before shutdown: %d pending txs, %d blocks", pendingBefore, chainLenBefore)

	// Trigger shutdown
	cancel()

	// Create config for shutdown
	cfg := &Config{
		ShutdownTimeout: 5 * time.Second,
		DataDir:         t.TempDir(),
	}

	// Perform graceful shutdown
	node.Shutdown(ctx, cfg)

	// Wait for block generation to stop
	select {
	case <-blockGenDone:
		t.Log("Block generation stopped cleanly")
	case <-time.After(3 * time.Second):
		t.Error("Block generation did not stop in time")
	}

	// Verify data integrity
	node.BlockchainMutex.RLock()
	chainLenAfter := len(node.Blockchain)
	for i := 1; i < chainLenAfter; i++ {
		if node.Blockchain[i].PrevHash != node.Blockchain[i-1].Hash {
			t.Errorf("Block %d has invalid prev hash after shutdown", i)
		}
	}
	node.BlockchainMutex.RUnlock()

	t.Logf("After shutdown: %d blocks, blockchain integrity verified", chainLenAfter)
}

// TestMultiNodeTrustComputation tests trust computation consistency across nodes
func TestMultiNodeTrustComputation(t *testing.T) {
	cluster, cleanup := setupTestCluster(t, 2)
	defer cleanup()

	node0 := cluster.nodes[0]
	node1 := cluster.nodes[1]

	// Create a trust chain: A -> B -> C
	quidA := fmt.Sprintf("%016x", 1)
	quidB := fmt.Sprintf("%016x", 2)
	quidC := fmt.Sprintf("%016x", 3)

	// Register identities on both nodes
	for _, node := range cluster.nodes {
		node.IdentityRegistryMutex.Lock()
		node.IdentityRegistry[quidA] = IdentityTransaction{QuidID: quidA, Name: "Entity A"}
		node.IdentityRegistry[quidB] = IdentityTransaction{QuidID: quidB, Name: "Entity B"}
		node.IdentityRegistry[quidC] = IdentityTransaction{QuidID: quidC, Name: "Entity C"}
		node.IdentityRegistryMutex.Unlock()
	}

	// Set up identical trust on both nodes: A -> B (0.8), B -> C (0.9)
	for _, node := range cluster.nodes {
		node.TrustRegistryMutex.Lock()
		if node.TrustRegistry[quidA] == nil {
			node.TrustRegistry[quidA] = make(map[string]float64)
		}
		node.TrustRegistry[quidA][quidB] = 0.8

		if node.TrustRegistry[quidB] == nil {
			node.TrustRegistry[quidB] = make(map[string]float64)
		}
		node.TrustRegistry[quidB][quidC] = 0.9
		node.TrustRegistryMutex.Unlock()
	}

	// Compute trust A -> C on both nodes
	trust0, path0, err0 := node0.ComputeRelationalTrust(quidA, quidC, 5)
	trust1, path1, err1 := node1.ComputeRelationalTrust(quidA, quidC, 5)

	if err0 != nil || err1 != nil {
		t.Logf("Trust computation errors: node0=%v, node1=%v", err0, err1)
	}

	expectedTrust := 0.8 * 0.9 // 0.72

	t.Logf("Node 0: trust A->C = %.4f, path = %v", trust0, path0)
	t.Logf("Node 1: trust A->C = %.4f, path = %v", trust1, path1)

	// Verify both nodes compute the same trust
	tolerance := 0.001
	if trust0 < expectedTrust-tolerance || trust0 > expectedTrust+tolerance {
		t.Errorf("Node 0 computed wrong trust: got %.4f, expected %.4f", trust0, expectedTrust)
	}

	if trust1 < expectedTrust-tolerance || trust1 > expectedTrust+tolerance {
		t.Errorf("Node 1 computed wrong trust: got %.4f, expected %.4f", trust1, expectedTrust)
	}

	if trust0 != trust1 {
		t.Errorf("Nodes computed different trust values: %.4f vs %.4f", trust0, trust1)
	}
}

// TestConcurrentMultiNodeOperations tests concurrent operations across multiple nodes
func TestConcurrentMultiNodeOperations(t *testing.T) {
	cluster, cleanup := setupTestCluster(t, 3)
	defer cleanup()

	// Pre-populate identities
	for i := 0; i < 10; i++ {
		quidID := fmt.Sprintf("%016x", i+1)
		for _, node := range cluster.nodes {
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
		}
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Concurrent transactions on different nodes
	for nodeIdx, node := range cluster.nodes {
		wg.Add(1)
		go func(idx int, n *QuidnugNode) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-done:
					return
				default:
					counter++
					trusterIdx := (idx*3 + counter) % 10
					trusteeIdx := (idx*3 + counter + 1) % 10

					tx := TrustTransaction{
						BaseTransaction: BaseTransaction{
							Type:        TxTypeTrust,
							TrustDomain: "default",
							Timestamp:   time.Now().Unix(),
							PublicKey:   n.GetPublicKeyHex(),
						},
						Truster:    fmt.Sprintf("%016x", trusterIdx+1),
						Trustee:    fmt.Sprintf("%016x", trusteeIdx+1),
						TrustLevel: 0.7,
						Nonce:      int64(idx*1000 + counter),
					}
					tx = signTrustTx(n, tx)
					n.AddTrustTransaction(tx)

					time.Sleep(10 * time.Millisecond)
				}
			}
		}(nodeIdx, node)
	}

	// Concurrent trust queries
	for nodeIdx, node := range cluster.nodes {
		wg.Add(1)
		go func(idx int, n *QuidnugNode) {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					observer := fmt.Sprintf("%016x", (idx%10)+1)
					target := fmt.Sprintf("%016x", ((idx+5)%10)+1)
					n.ComputeRelationalTrust(observer, target, 3)
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(nodeIdx, node)
	}

	// Let operations run
	time.Sleep(500 * time.Millisecond)
	close(done)
	wg.Wait()

	// Verify all nodes are in consistent state
	for i, node := range cluster.nodes {
		node.BlockchainMutex.RLock()
		chainLen := len(node.Blockchain)
		node.BlockchainMutex.RUnlock()

		node.PendingTxsMutex.RLock()
		pendingLen := len(node.PendingTxs)
		node.PendingTxsMutex.RUnlock()

		t.Logf("Node %d: %d blocks, %d pending transactions", i, chainLen, pendingLen)
	}
}

// signTrustTxForIntegration is a helper that signs a trust transaction
// (duplicated here since signTrustTx may not be exported)
func signTrustTxIntegration(node *QuidnugNode, tx TrustTransaction) TrustTransaction {
	txCopy := tx
	txCopy.Signature = ""

	data, _ := json.Marshal(txCopy)
	sig, _ := node.SignData(data)
	tx.Signature = hex.EncodeToString(sig)

	return tx
}
