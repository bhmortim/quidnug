package main

import (
	"testing"
)

// newTestNode creates a QuidnugNode with pre-populated test data for testing
func newTestNode() *QuidnugNode {
	node, _ := NewQuidnugNode()

	// Add test trust domains
	node.TrustDomains["test.domain.com"] = TrustDomain{
		Name:           "test.domain.com",
		ValidatorNodes: []string{node.NodeID},
		TrustThreshold: 0.75,
		Validators:     map[string]float64{node.NodeID: 1.0},
	}

	// Add test identities
	node.IdentityRegistry["quid_truster_001"] = IdentityTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_identity_001",
			Type:        TxTypeIdentity,
			TrustDomain: "test.domain.com",
			Timestamp:   1000000,
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
		},
		QuidID:      "quid_asset_001",
		Name:        "Test Asset",
		Creator:     "quid_creator_003",
		UpdateNonce: 1,
	}

	// Add a test title for transfer testing
	node.TitleRegistry["quid_asset_001"] = TitleTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "tx_title_001",
			Type:        TxTypeTitle,
			TrustDomain: "test.domain.com",
			Timestamp:   1000003,
		},
		AssetID: "quid_asset_001",
		Owners: []OwnershipStake{
			{OwnerID: "quid_owner_001", Percentage: 60.0},
			{OwnerID: "quid_owner_002", Percentage: 40.0},
		},
	}

	return node
}

func TestValidateTrustTransaction(t *testing.T) {
	tests := []struct {
		name     string
		tx       TrustTransaction
		expected bool
	}{
		{
			name: "valid transaction with known domain and trust level 0.5",
			tx: TrustTransaction{
				BaseTransaction: BaseTransaction{
					ID:          "tx_trust_valid",
					Type:        TxTypeTrust,
					TrustDomain: "test.domain.com",
					Timestamp:   1000000,
				},
				Truster:    "quid_truster_001",
				Trustee:    "quid_trustee_001",
				TrustLevel: 0.5,
			},
			expected: true,
		},
		{
			name: "invalid: unknown trust domain",
			tx: TrustTransaction{
				BaseTransaction: BaseTransaction{
					ID:          "tx_trust_unknown_domain",
					Type:        TxTypeTrust,
					TrustDomain: "unknown.domain.com",
					Timestamp:   1000000,
				},
				Truster:    "quid_truster_001",
				Trustee:    "quid_trustee_001",
				TrustLevel: 0.5,
			},
			expected: false,
		},
		{
			name: "invalid: trust level less than 0",
			tx: TrustTransaction{
				BaseTransaction: BaseTransaction{
					ID:          "tx_trust_negative",
					Type:        TxTypeTrust,
					TrustDomain: "test.domain.com",
					Timestamp:   1000000,
				},
				Truster:    "quid_truster_001",
				Trustee:    "quid_trustee_001",
				TrustLevel: -0.1,
			},
			expected: false,
		},
		{
			name: "invalid: trust level greater than 1",
			tx: TrustTransaction{
				BaseTransaction: BaseTransaction{
					ID:          "tx_trust_over_one",
					Type:        TxTypeTrust,
					TrustDomain: "test.domain.com",
					Timestamp:   1000000,
				},
				Truster:    "quid_truster_001",
				Trustee:    "quid_trustee_001",
				TrustLevel: 1.5,
			},
			expected: false,
		},
		{
			name: "edge case: empty trust domain (allowed)",
			tx: TrustTransaction{
				BaseTransaction: BaseTransaction{
					ID:          "tx_trust_empty_domain",
					Type:        TxTypeTrust,
					TrustDomain: "",
					Timestamp:   1000000,
				},
				Truster:    "quid_truster_001",
				Trustee:    "quid_trustee_001",
				TrustLevel: 0.5,
			},
			expected: true,
		},
		{
			name: "valid: trust level at boundary 0.0",
			tx: TrustTransaction{
				BaseTransaction: BaseTransaction{
					ID:          "tx_trust_zero",
					Type:        TxTypeTrust,
					TrustDomain: "test.domain.com",
					Timestamp:   1000000,
				},
				Truster:    "quid_truster_001",
				Trustee:    "quid_trustee_001",
				TrustLevel: 0.0,
			},
			expected: true,
		},
		{
			name: "valid: trust level at boundary 1.0",
			tx: TrustTransaction{
				BaseTransaction: BaseTransaction{
					ID:          "tx_trust_one",
					Type:        TxTypeTrust,
					TrustDomain: "test.domain.com",
					Timestamp:   1000000,
				},
				Truster:    "quid_truster_001",
				Trustee:    "quid_trustee_001",
				TrustLevel: 1.0,
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := newTestNode()
			result := node.ValidateTrustTransaction(tc.tx)
			if result != tc.expected {
				t.Errorf("ValidateTrustTransaction() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestValidateIdentityTransaction(t *testing.T) {
	tests := []struct {
		name     string
		tx       IdentityTransaction
		expected bool
	}{
		{
			name: "valid new identity",
			tx: IdentityTransaction{
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
			},
			expected: true,
		},
		{
			name: "valid update with higher nonce",
			tx: IdentityTransaction{
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
			},
			expected: true,
		},
		{
			name: "invalid: update with lower nonce",
			tx: IdentityTransaction{
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
			},
			expected: false,
		},
		{
			name: "invalid: update with equal nonce",
			tx: IdentityTransaction{
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
			},
			expected: false,
		},
		{
			name: "invalid: update with different creator",
			tx: IdentityTransaction{
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
			},
			expected: false,
		},
		{
			name: "invalid: unknown trust domain",
			tx: IdentityTransaction{
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
			},
			expected: false,
		},
		{
			name: "edge case: empty trust domain (allowed)",
			tx: IdentityTransaction{
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
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := newTestNode()
			result := node.ValidateIdentityTransaction(tc.tx)
			if result != tc.expected {
				t.Errorf("ValidateIdentityTransaction() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestValidateTitleTransaction(t *testing.T) {
	tests := []struct {
		name     string
		tx       TitleTransaction
		expected bool
	}{
		{
			name: "valid title with 100% ownership single owner",
			tx: TitleTransaction{
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
			},
			expected: true,
		},
		{
			name: "invalid: ownership does not equal 100%",
			tx: TitleTransaction{
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
			},
			expected: false,
		},
		{
			name: "invalid: asset not in identity registry",
			tx: TitleTransaction{
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
			},
			expected: false,
		},
		{
			name: "invalid: previous owners mismatch",
			tx: TitleTransaction{
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
			},
			expected: false,
		},
		{
			name: "edge case: multiple owners summing to 100%",
			tx: TitleTransaction{
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
			},
			expected: true,
		},
		{
			name: "invalid: unknown trust domain",
			tx: TitleTransaction{
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
			},
			expected: false,
		},
		{
			name: "edge case: empty trust domain (allowed)",
			tx: TitleTransaction{
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
			},
			expected: true,
		},
		{
			name: "valid: transfer with correct previous owners",
			tx: TitleTransaction{
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
			},
			expected: true,
		},
		{
			name: "invalid: ownership exceeds 100%",
			tx: TitleTransaction{
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
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := newTestNode()
			result := node.ValidateTitleTransaction(tc.tx)
			if result != tc.expected {
				t.Errorf("ValidateTitleTransaction() = %v, expected %v", result, tc.expected)
			}
		})
	}
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
