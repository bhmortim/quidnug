package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const pendingTxsFilename = "pending_transactions.json"

// SavePendingTransactions saves pending transactions to a JSON file
func (node *QuidnugNode) SavePendingTransactions(dataDir string) error {
	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()

	if len(node.PendingTxs) == 0 {
		return nil
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	filePath := filepath.Join(dataDir, pendingTxsFilename)

	data, err := json.MarshalIndent(node.PendingTxs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pending transactions: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write pending transactions file: %w", err)
	}

	return nil
}

// LoadPendingTransactions loads pending transactions from a JSON file
func (node *QuidnugNode) LoadPendingTransactions(dataDir string) error {
	filePath := filepath.Join(dataDir, pendingTxsFilename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read pending transactions file: %w", err)
	}

	var pendingTxs []interface{}
	if err := json.Unmarshal(data, &pendingTxs); err != nil {
		return fmt.Errorf("failed to unmarshal pending transactions: %w", err)
	}

	node.PendingTxsMutex.Lock()
	node.PendingTxs = pendingTxs
	node.PendingTxsMutex.Unlock()

	if logger != nil {
		logger.Info("Loaded pending transactions", "count", len(pendingTxs), "file", filePath)
	}

	return nil
}

// ClearPendingTransactionsFile removes the pending transactions file after successful processing
func (node *QuidnugNode) ClearPendingTransactionsFile(dataDir string) error {
	filePath := filepath.Join(dataDir, pendingTxsFilename)

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove pending transactions file: %w", err)
	}

	return nil
}
