package main

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"testing"
)

// newTestNode creates a QuidnugNode with pre-populated test data for testing
func newTestNode() *QuidnugNode {
	node, _ := NewQuidnugNode()

	// Get the node's public key for test identities
	nodePublicKey := node.GetPublicKeyHex()

	// Add test trust domains
	node.TrustDomains["test.domain.com"] = TrustDomain{
		Name:           "test.domain.com",
		ValidatorNodes: []string{node.NodeID},
		TrustThreshold: 0.75,
		Validators:     map[string]float64{node.NodeID: 1.0},
	}

	// Add test identities with public keys
	node.IdentityRegistry["quid_truster_001"] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_identity_001",
			Type:        TxTypeIdentity,
			TrustDomain: "test.domain.com",
			Timestamp:   1000000,
			PublicKey:   nodePublicKey,
		},
		QuidID:      "quid_truster_001",
		Name:        "Test Truster",
		Creator:     "quid_creator_001",
		UpdateNonce: 1,
	}

	node.IdentityRegistry["quid_trustee_001"] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_identity_002",
			Type:        TxTypeIdentity,
			TrustDomain: "test.domain.com",
			Timestamp:   1000001,
			PublicKey:   nodePublicKey,
		},
		QuidID:      "quid_trustee_001",
		Name:        "Test Trustee",
		Creator:     "quid_creator_002",
		UpdateNonce: 1,
	}

	node.IdentityRegistry["quid_asset_001"] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_identity_003",
			Type:        TxTypeIdentity,
			TrustDomain: "test.domain.com",
			Timestamp:   1000002,
			PublicKey:   nodePublicKey,
		},
		QuidID:      "quid_asset_001",
		Name:        "Test Asset",
		Creator:     "quid_creator_003",
		UpdateNonce: 1,
	}

	// Add owner identities with public keys for transfer testing
	node.IdentityRegistry["quid_owner_001"] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_identity_004",
			Type:        TxTypeIdentity,
			TrustDomain: "test.domain.com",
			Timestamp:   1000004,
			PublicKey:   nodePublicKey,
		},
		QuidID:      "quid_owner_001",
		Name:        "Test Owner 1",
		Creator:     "quid_creator_004",
		UpdateNonce: 1,
	}

	node.IdentityRegistry["quid_owner_002"] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_identity_005",
			Type:        TxTypeIdentity,
			TrustDomain: "test.domain.com",
			Timestamp:   1000005,
			PublicKey:   nodePublicKey,
		},
		QuidID:      "quid_owner_002",
		Name:        "Test Owner 2",
		Creator:     "quid_creator_005",
		UpdateNonce: 1,
	}

	// Add a test title for transfer testing
	node.TitleRegistry["quid_asset_001"] = TitleTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_title_001",
			Type:        TxTypeTitle,
			TrustDomain: "test.domain.com",
			Timestamp:   1000003,
			PublicKey:   nodePublicKey,
		},
		AssetID: "quid_asset_001",
		Owners: []OwnershipStake{
			{OwnerID: "quid_owner_001", Percentage: 60.0},
			{OwnerID: "quid_owner_002", Percentage: 40.0},
		},
		Signatures: make(map[string]string),
	}

	return node
}

// signTrustTx signs a trust transaction using the node's private key
func signTrustTx(node *QuidnugNode, tx TrustTransaction) TrustTransaction {
	tx.PublicKey = node.GetPublicKeyHex()
	tx.Signature = ""
	signableData, _ := json.Marshal(tx)
	signature, _ := node.SignData(signableData)
	tx.Signature = hex.EncodeToString(signature)
	return tx
}

// signIdentityTx signs an identity transaction using the node's private key
func signIdentityTx(node *QuidnugNode, tx IdentityTransaction) IdentityTransaction {
	tx.PublicKey = node.GetPublicKeyHex()
	tx.Signature = ""
	signableData, _ := json.Marshal(tx)
	signature, _ := node.SignData(signableData)
	tx.Signature = hex.EncodeToString(signature)
	return tx
}

// signTitleTx signs a title transaction using the node's private key
func signTitleTx(node *QuidnugNode, tx TitleTransaction) TitleTransaction {
	tx.PublicKey = node.GetPublicKeyHex()
	tx.Signature = ""
	signableData, _ := json.Marshal(tx)
	signature, _ := node.SignData(signableData)
	tx.Signature = hex.EncodeToString(signature)
	return tx
}

// signTitleTxWithOwners signs a title transaction and adds owner signatures for transfers
func signTitleTxWithOwners(node *QuidnugNode, tx TitleTransaction) TitleTransaction {
	// First, create owner signatures (before main signature is set)
	if len(tx.PreviousOwners) > 0 {
		tx.Signatures = make(map[string]string)
		txCopyForOwners := tx
		txCopyForOwners.Signature = ""
		txCopyForOwners.Signatures = nil
		ownerSignableData, _ := json.Marshal(txCopyForOwners)

		for _, stake := range tx.PreviousOwners {
			signature, _ := node.SignData(ownerSignableData)
			tx.Signatures[stake.OwnerID] = hex.EncodeToString(signature)
		}
	}

	// Then sign the main transaction (includes owner signatures)
	tx.PublicKey = node.GetPublicKeyHex()
	tx.Signature = ""
	signableData, _ := json.Marshal(tx)
	signature, _ := node.SignData(signableData)
	tx.Signature = hex.EncodeToString(signature)
	return tx
}

func TestValidateTrustTransaction(t *testing.T) {
	node := newTestNode()

	t.Run("valid signed transaction with known domain and trust level 0.5", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_valid",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.5,
		})
		if !node.ValidateTrustTransaction(tx) {
			t.Error("Expected valid transaction to pass")
		}
	})

	t.Run("invalid: unknown trust domain", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_unknown_domain",
				Type:        TxTypeTrust,
				TrustDomain: "unknown.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.5,
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected unknown domain to fail")
		}
	})

	t.Run("invalid: trust level less than 0", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_negative",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: -0.1,
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected negative trust level to fail")
		}
	})

	t.Run("invalid: trust level greater than 1", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_over_one",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 1.5,
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected trust level > 1 to fail")
		}
	})

	t.Run("valid: empty trust domain with signature", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_empty_domain",
				Type:        TxTypeTrust,
				TrustDomain: "",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.5,
		})
		if !node.ValidateTrustTransaction(tx) {
			t.Error("Expected empty domain with valid signature to pass")
		}
	})

	t.Run("valid: trust level at boundary 0.0", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_zero",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.0,
		})
		if !node.ValidateTrustTransaction(tx) {
			t.Error("Expected trust level 0.0 to pass")
		}
	})

	t.Run("valid: trust level at boundary 1.0", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_one",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 1.0,
		})
		if !node.ValidateTrustTransaction(tx) {
			t.Error("Expected trust level 1.0 to pass")
		}
	})

	t.Run("invalid: trust level NaN", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_nan",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: math.NaN(),
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected NaN trust level to fail")
		}
	})

	t.Run("invalid: trust level positive Inf", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_inf_pos",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: math.Inf(1),
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected positive Inf trust level to fail")
		}
	})

	t.Run("invalid: trust level negative Inf", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_inf_neg",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: math.Inf(-1),
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected negative Inf trust level to fail")
		}
	})

	t.Run("invalid: invalid truster quid ID format", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_invalid_truster",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "INVALID-FORMAT!",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.5,
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected invalid truster quid ID format to fail")
		}
	})

	t.Run("invalid: invalid trustee quid ID format", func(t *testing.T) {
		tx := signTrustTx(node, TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_invalid_trustee",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			Truster:    "quid_truster_001",
			Trustee:    "short",
			TrustLevel: 0.5,
		})
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected invalid trustee quid ID format to fail")
		}
	})

	t.Run("invalid: missing signature", func(t *testing.T) {
		tx := TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_no_sig",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
				PublicKey:   node.GetPublicKeyHex(),
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.5,
		}
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected missing signature to fail")
		}
	})

	t.Run("invalid: missing public key", func(t *testing.T) {
		tx := TrustTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_trust_no_pubkey",
				Type:        TxTypeTrust,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
				Signature:   "deadbeef",
			},
			Truster:    "quid_truster_001",
			Trustee:    "quid_trustee_001",
			TrustLevel: 0.5,
		}
		if node.ValidateTrustTransaction(tx) {
			t.Error("Expected missing public key to fail")
		}
	})
}

func TestValidateIdentityTransaction(t *testing.T) {
	node := newTestNode()

	t.Run("valid new identity", func(t *testing.T) {
		tx := signIdentityTx(node, IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_new",
				Type:        TxTypeIdentity,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			QuidID:      "quid_new_identity",
			Name:        "New Identity",
			Creator:     "quid_creator_new",
			UpdateNonce: 1,
		})
		if !node.ValidateIdentityTransaction(tx) {
			t.Error("Expected valid new identity to pass")
		}
	})

	t.Run("valid update with higher nonce", func(t *testing.T) {
		tx := signIdentityTx(node, IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_update",
				Type:        TxTypeIdentity,
				TrustDomain: "test.domain.com",
				Timestamp:   1000001,
			},
			QuidID:      "quid_truster_001",
			Name:        "Updated Name",
			Creator:     "quid_creator_001",
			UpdateNonce: 2,
		})
		if !node.ValidateIdentityTransaction(tx) {
			t.Error("Expected valid update with higher nonce to pass")
		}
	})

	t.Run("invalid: update with lower nonce", func(t *testing.T) {
		tx := signIdentityTx(node, IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_lower_nonce",
				Type:        TxTypeIdentity,
				TrustDomain: "test.domain.com",
				Timestamp:   1000001,
			},
			QuidID:      "quid_truster_001",
			Name:        "Updated Name",
			Creator:     "quid_creator_001",
			UpdateNonce: 0,
		})
		if node.ValidateIdentityTransaction(tx) {
			t.Error("Expected lower nonce to fail")
		}
	})

	t.Run("invalid: update with equal nonce", func(t *testing.T) {
		tx := signIdentityTx(node, IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_equal_nonce",
				Type:        TxTypeIdentity,
				TrustDomain: "test.domain.com",
				Timestamp:   1000001,
			},
			QuidID:      "quid_truster_001",
			Name:        "Updated Name",
			Creator:     "quid_creator_001",
			UpdateNonce: 1,
		})
		if node.ValidateIdentityTransaction(tx) {
			t.Error("Expected equal nonce to fail")
		}
	})

	t.Run("invalid: update with different creator", func(t *testing.T) {
		tx := signIdentityTx(node, IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_diff_creator",
				Type:        TxTypeIdentity,
				TrustDomain: "test.domain.com",
				Timestamp:   1000001,
			},
			QuidID:      "quid_truster_001",
			Name:        "Updated Name",
			Creator:     "quid_different_creator",
			UpdateNonce: 2,
		})
		if node.ValidateIdentityTransaction(tx) {
			t.Error("Expected different creator to fail")
		}
	})

	t.Run("invalid: unknown trust domain", func(t *testing.T) {
		tx := signIdentityTx(node, IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_unknown_domain",
				Type:        TxTypeIdentity,
				TrustDomain: "unknown.domain.com",
				Timestamp:   1000000,
			},
			QuidID:      "quid_new_identity",
			Name:        "New Identity",
			Creator:     "quid_creator_new",
			UpdateNonce: 1,
		})
		if node.ValidateIdentityTransaction(tx) {
			t.Error("Expected unknown domain to fail")
		}
	})

	t.Run("valid: empty trust domain with signature", func(t *testing.T) {
		tx := signIdentityTx(node, IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_empty_domain",
				Type:        TxTypeIdentity,
				TrustDomain: "",
				Timestamp:   1000000,
			},
			QuidID:      "quid_new_identity",
			Name:        "New Identity",
			Creator:     "quid_creator_new",
			UpdateNonce: 1,
		})
		if !node.ValidateIdentityTransaction(tx) {
			t.Error("Expected empty domain with valid signature to pass")
		}
	})

	t.Run("invalid: missing signature", func(t *testing.T) {
		tx := IdentityTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_identity_no_sig",
				Type:        TxTypeIdentity,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
				PublicKey:   node.GetPublicKeyHex(),
			},
			QuidID:      "quid_new_identity",
			Name:        "New Identity",
			Creator:     "quid_creator_new",
			UpdateNonce: 1,
		}
		if node.ValidateIdentityTransaction(tx) {
			t.Error("Expected missing signature to fail")
		}
	})
}

func TestValidateTitleTransaction(t *testing.T) {
	node := newTestNode()

	t.Run("valid title with 100% ownership single owner", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_valid",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 100.0},
			},
			Signatures: make(map[string]string),
		})
		if !node.ValidateTitleTransaction(tx) {
			t.Error("Expected valid title to pass")
		}
	})

	t.Run("invalid: ownership does not equal 100%", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_invalid_pct",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 50.0},
			},
			Signatures: make(map[string]string),
		})
		if node.ValidateTitleTransaction(tx) {
			t.Error("Expected ownership != 100% to fail")
		}
	})

	t.Run("invalid: asset not in identity registry", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_no_asset",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_nonexistent_asset",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 100.0},
			},
			Signatures: make(map[string]string),
		})
		if node.ValidateTitleTransaction(tx) {
			t.Error("Expected nonexistent asset to fail")
		}
	})

	t.Run("invalid: previous owners mismatch", func(t *testing.T) {
		tx := signTitleTxWithOwners(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_mismatch",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_new_owner", Percentage: 100.0},
			},
			PreviousOwners: []OwnershipStake{
				{OwnerID: "quid_wrong_owner", Percentage: 100.0},
			},
		})
		if node.ValidateTitleTransaction(tx) {
			t.Error("Expected previous owners mismatch to fail")
		}
	})

	t.Run("valid: multiple owners summing to 100%", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_multi_owner",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 60.0},
				{OwnerID: "quid_owner_002", Percentage: 25.0},
				{OwnerID: "quid_owner_003", Percentage: 15.0},
			},
			Signatures: make(map[string]string),
		})
		if !node.ValidateTitleTransaction(tx) {
			t.Error("Expected multiple owners summing to 100% to pass")
		}
	})

	t.Run("invalid: unknown trust domain", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_unknown_domain",
				Type:        TxTypeTitle,
				TrustDomain: "unknown.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 100.0},
			},
			Signatures: make(map[string]string),
		})
		if node.ValidateTitleTransaction(tx) {
			t.Error("Expected unknown domain to fail")
		}
	})

	t.Run("valid: empty trust domain with signature", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_empty_domain",
				Type:        TxTypeTitle,
				TrustDomain: "",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 100.0},
			},
			Signatures: make(map[string]string),
		})
		if !node.ValidateTitleTransaction(tx) {
			t.Error("Expected empty domain with valid signature to pass")
		}
	})

	t.Run("valid: transfer with correct previous owners and signatures", func(t *testing.T) {
		tx := signTitleTxWithOwners(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_transfer",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_new_owner", Percentage: 100.0},
			},
			PreviousOwners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 60.0},
				{OwnerID: "quid_owner_002", Percentage: 40.0},
			},
		})
		if !node.ValidateTitleTransaction(tx) {
			t.Error("Expected valid transfer with owner signatures to pass")
		}
	})

	t.Run("invalid: ownership exceeds 100%", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_over_100",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 60.0},
				{OwnerID: "quid_owner_002", Percentage: 50.0},
			},
			Signatures: make(map[string]string),
		})
		if node.ValidateTitleTransaction(tx) {
			t.Error("Expected ownership > 100% to fail")
		}
	})

	t.Run("invalid: missing signature", func(t *testing.T) {
		tx := TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_no_sig",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
				PublicKey:   node.GetPublicKeyHex(),
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 100.0},
			},
			Signatures: make(map[string]string),
		}
		if node.ValidateTitleTransaction(tx) {
			t.Error("Expected missing signature to fail")
		}
	})

	t.Run("invalid: transfer missing owner signature", func(t *testing.T) {
		tx := signTitleTx(node, TitleTransaction{
			BaseTransaction: BaseTransaction{
				ID:          "tx_title_transfer_no_owner_sig",
				Type:        TxTypeTitle,
				TrustDomain: "test.domain.com",
				Timestamp:   1000000,
			},
			AssetID: "quid_asset_001",
			Owners: []OwnershipStake{
				{OwnerID: "quid_new_owner", Percentage: 100.0},
			},
			PreviousOwners: []OwnershipStake{
				{OwnerID: "quid_owner_001", Percentage: 60.0},
				{OwnerID: "quid_owner_002", Percentage: 40.0},
			},
			Signatures: make(map[string]string),
		})
		if node.ValidateTitleTransaction(tx) {
			t.Error("Expected transfer missing owner signatures to fail")
		}
	})
}

func TestAreOwnershipStakesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []OwnershipStake
		b        []OwnershipStake
		expected bool
	}{
		{
			name: "equal single owner",
			a: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 100.0},
			},
			b: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 100.0},
			},
			expected: true,
		},
		{
			name: "equal multiple owners",
			a: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 60.0},
				{OwnerID: "owner2", Percentage: 40.0},
			},
			b: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 60.0},
				{OwnerID: "owner2", Percentage: 40.0},
			},
			expected: true,
		},
		{
			name: "equal multiple owners different order",
			a: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 60.0},
				{OwnerID: "owner2", Percentage: 40.0},
			},
			b: []OwnershipStake{
				{OwnerID: "owner2", Percentage: 40.0},
				{OwnerID: "owner1", Percentage: 60.0},
			},
			expected: true,
		},
		{
			name: "different lengths",
			a: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 100.0},
			},
			b: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 60.0},
				{OwnerID: "owner2", Percentage: 40.0},
			},
			expected: false,
		},
		{
			name: "different percentages",
			a: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 50.0},
				{OwnerID: "owner2", Percentage: 50.0},
			},
			b: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 60.0},
				{OwnerID: "owner2", Percentage: 40.0},
			},
			expected: false,
		},
		{
			name: "different owners",
			a: []OwnershipStake{
				{OwnerID: "owner1", Percentage: 100.0},
			},
			b: []OwnershipStake{
				{OwnerID: "owner2", Percentage: 100.0},
			},
			expected: false,
		},
		{
			name: "empty slices",
			a:        []OwnershipStake{},
			b:        []OwnershipStake{},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := areOwnershipStakesEqual(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("areOwnershipStakesEqual() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	node := newTestNode()

	t.Run("valid signature verification", func(t *testing.T) {
		data := []byte("test data to sign")
		signature, err := node.SignData(data)
		if err != nil {
			t.Fatalf("Failed to sign data: %v", err)
		}

		publicKeyHex := node.GetPublicKeyHex()
		signatureHex := hex.EncodeToString(signature)

		if !VerifySignature(publicKeyHex, data, signatureHex) {
			t.Error("Expected valid signature to verify")
		}
	})

	t.Run("invalid signature rejected", func(t *testing.T) {
		data := []byte("test data to sign")
		publicKeyHex := node.GetPublicKeyHex()
		invalidSignatureHex := hex.EncodeToString(make([]byte, 64))

		if VerifySignature(publicKeyHex, data, invalidSignatureHex) {
			t.Error("Expected invalid signature to be rejected")
		}
	})

	t.Run("tampered data rejected", func(t *testing.T) {
		originalData := []byte("original data")
		signature, err := node.SignData(originalData)
		if err != nil {
			t.Fatalf("Failed to sign data: %v", err)
		}

		publicKeyHex := node.GetPublicKeyHex()
		signatureHex := hex.EncodeToString(signature)
		tamperedData := []byte("tampered data")

		if VerifySignature(publicKeyHex, tamperedData, signatureHex) {
			t.Error("Expected tampered data to be rejected")
		}
	})

	t.Run("wrong public key rejected", func(t *testing.T) {
		data := []byte("test data")
		signature, err := node.SignData(data)
		if err != nil {
			t.Fatalf("Failed to sign data: %v", err)
		}

		otherNode, _ := NewQuidnugNode()
		wrongPublicKeyHex := otherNode.GetPublicKeyHex()
		signatureHex := hex.EncodeToString(signature)

		if VerifySignature(wrongPublicKeyHex, data, signatureHex) {
			t.Error("Expected wrong public key to be rejected")
		}
	})

	t.Run("empty public key rejected", func(t *testing.T) {
		if VerifySignature("", []byte("data"), "deadbeef") {
			t.Error("Expected empty public key to be rejected")
		}
	})

	t.Run("empty signature rejected", func(t *testing.T) {
		publicKeyHex := node.GetPublicKeyHex()
		if VerifySignature(publicKeyHex, []byte("data"), "") {
			t.Error("Expected empty signature to be rejected")
		}
	})

	t.Run("invalid hex public key rejected", func(t *testing.T) {
		if VerifySignature("not-valid-hex", []byte("data"), hex.EncodeToString(make([]byte, 64))) {
			t.Error("Expected invalid hex public key to be rejected")
		}
	})

	t.Run("invalid hex signature rejected", func(t *testing.T) {
		publicKeyHex := node.GetPublicKeyHex()
		if VerifySignature(publicKeyHex, []byte("data"), "not-valid-hex") {
			t.Error("Expected invalid hex signature to be rejected")
		}
	})

	t.Run("wrong signature length rejected", func(t *testing.T) {
		publicKeyHex := node.GetPublicKeyHex()
		shortSignature := hex.EncodeToString(make([]byte, 32))
		if VerifySignature(publicKeyHex, []byte("data"), shortSignature) {
			t.Error("Expected wrong signature length to be rejected")
		}
	})
}

func TestSignDataProduces64ByteSignature(t *testing.T) {
	node := newTestNode()
	data := []byte("test data")

	signature, err := node.SignData(data)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}

	if len(signature) != 64 {
		t.Errorf("Expected signature length 64, got %d", len(signature))
	}
}

func TestGetPublicKeyHex(t *testing.T) {
	node := newTestNode()
	publicKeyHex := node.GetPublicKeyHex()

	if publicKeyHex == "" {
		t.Error("Expected non-empty public key hex")
	}

	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		t.Fatalf("Failed to decode public key hex: %v", err)
	}

	if len(publicKeyBytes) != 65 {
		t.Errorf("Expected 65-byte uncompressed public key, got %d bytes", len(publicKeyBytes))
	}

	if publicKeyBytes[0] != 0x04 {
		t.Error("Expected uncompressed public key prefix 0x04")
	}
}

func TestNewTestNodeInitialization(t *testing.T) {
	node := newTestNode()

	t.Run("node has valid ID", func(t *testing.T) {
		if len(node.NodeID) != 16 {
			t.Errorf("NodeID should be 16 characters, got %d", len(node.NodeID))
		}
	})

	t.Run("test domain exists", func(t *testing.T) {
		if _, exists := node.TrustDomains["test.domain.com"]; !exists {
			t.Error("test.domain.com should exist in TrustDomains")
		}
	})

	t.Run("test identities exist", func(t *testing.T) {
		expectedQuids := []string{"quid_truster_001", "quid_trustee_001", "quid_asset_001"}
		for _, quid := range expectedQuids {
			if _, exists := node.IdentityRegistry[quid]; !exists {
				t.Errorf("Identity %s should exist in IdentityRegistry", quid)
			}
		}
	})

	t.Run("test title exists", func(t *testing.T) {
		if _, exists := node.TitleRegistry["quid_asset_001"]; !exists {
			t.Error("quid_asset_001 should exist in TitleRegistry")
		}
	})

	t.Run("genesis block exists", func(t *testing.T) {
		if len(node.Blockchain) == 0 {
			t.Error("Blockchain should have at least the genesis block")
		}
		if node.Blockchain[0].Index != 0 {
			t.Error("Genesis block should have index 0")
		}
	})
}
