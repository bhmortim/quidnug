// group_encryption_validation.go — QDP-0024 Phase 1
// validation rules for the six group-encryption tx types.

package core

import (
	"encoding/hex"
)

var validGroupTypes = map[string]bool{
	"static":  true,
	"dynamic": true,
	"hybrid":  true,
}

var validEpochReasonCodes = map[string]bool{
	"member-added":        true,
	"member-removed":      true,
	"scheduled":           true,
	"compromise-response": true,
}

var validKeySchemes = map[string]bool{
	"":            true, // default allowed
	"direct-wrap": true,
	"treekem":     true,
}

// ValidateGroupCreate sanity-checks a GROUP_CREATE tx.
func (node *QuidnugNode) ValidateGroupCreate(tx GroupCreateTransaction) bool {
	if tx.GroupID == "" {
		return false
	}
	if !validGroupTypes[tx.GroupType] {
		return false
	}
	if !validKeySchemes[tx.Policy.KeyScheme] {
		return false
	}
	if tx.GroupType == "static" && len(tx.StaticMembers) == 0 {
		return false
	}
	if (tx.GroupType == "dynamic" || tx.GroupType == "hybrid") &&
		tx.DynamicTrustDomain == "" {
		return false
	}
	if tx.Policy.MaxMembers < 0 {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyStructSig(tx.PublicKey, tx.Signature, func() any {
		copy := tx
		copy.Signature = ""
		return copy
	})
}

// ValidateEpochAdvance checks an EPOCH_ADVANCE.
func (node *QuidnugNode) ValidateEpochAdvance(tx EpochAdvanceTransaction) bool {
	if tx.GroupID == "" {
		return false
	}
	if tx.NewEpoch <= tx.PreviousEpoch {
		return false
	}
	if !validEpochReasonCodes[tx.ReasonCode] {
		return false
	}
	if len(tx.WrappedSecrets) == 0 {
		return false
	}
	// Each wrapped secret value must decode as hex of the
	// expected length (32-byte ephemeral pub + 12-byte nonce
	// + 48-byte ciphertext = 92 bytes = 184 hex chars).
	for member, wrappedHex := range tx.WrappedSecrets {
		if len(wrappedHex) != 184 {
			return false
		}
		if _, err := hex.DecodeString(wrappedHex); err != nil {
			return false
		}
		if !IsValidQuidID(member) {
			return false
		}
	}
	if tx.EffectiveAt == 0 {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyStructSig(tx.PublicKey, tx.Signature, func() any {
		copy := tx
		copy.Signature = ""
		return copy
	})
}

// ValidateMemberKeyPackage checks a MEMBER_KEY_PACKAGE.
func (node *QuidnugNode) ValidateMemberKeyPackage(tx MemberKeyPackageTransaction) bool {
	if !IsValidQuidID(tx.MemberQuid) {
		return false
	}
	if len(tx.X25519PublicKey) != 64 { // 32-byte hex
		return false
	}
	if _, err := hex.DecodeString(tx.X25519PublicKey); err != nil {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyStructSig(tx.PublicKey, tx.Signature, func() any {
		copy := tx
		copy.Signature = ""
		return copy
	})
}

// ValidateEncryptedRecord checks an ENCRYPTED_RECORD.
func (node *QuidnugNode) ValidateEncryptedRecord(tx EncryptedRecordTransaction) bool {
	if tx.GroupID == "" {
		return false
	}
	if tx.Epoch <= 0 {
		return false
	}
	if tx.ContentType == "" {
		return false
	}
	if len(tx.Nonce12Hex) != 24 { // 12 bytes hex
		return false
	}
	if _, err := hex.DecodeString(tx.Nonce12Hex); err != nil {
		return false
	}
	if tx.CiphertextHex == "" {
		return false
	}
	if _, err := hex.DecodeString(tx.CiphertextHex); err != nil {
		return false
	}
	if tx.TxNonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyStructSig(tx.PublicKey, tx.Signature, func() any {
		copy := tx
		copy.Signature = ""
		return copy
	})
}

// ValidateMemberInvite checks a MEMBER_INVITE.
func (node *QuidnugNode) ValidateMemberInvite(tx MemberInviteTransaction) bool {
	if tx.GroupID == "" {
		return false
	}
	if !IsValidQuidID(tx.InvitedMemberQuid) {
		return false
	}
	if !IsValidQuidID(tx.InvitingMemberQuid) {
		return false
	}
	if len(tx.WelcomeHex) != 184 { // 92 bytes hex
		return false
	}
	if _, err := hex.DecodeString(tx.WelcomeHex); err != nil {
		return false
	}
	if tx.EpochSnapshot <= 0 {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyStructSig(tx.PublicKey, tx.Signature, func() any {
		copy := tx
		copy.Signature = ""
		return copy
	})
}

// ValidateMemberKeyRecovery checks a MEMBER_KEY_RECOVERY.
func (node *QuidnugNode) ValidateMemberKeyRecovery(tx MemberKeyRecoveryTransaction) bool {
	if !IsValidQuidID(tx.MemberQuid) {
		return false
	}
	if len(tx.GroupIDs) == 0 {
		return false
	}
	if len(tx.NewX25519PublicKey) != 64 {
		return false
	}
	if _, err := hex.DecodeString(tx.NewX25519PublicKey); err != nil {
		return false
	}
	if len(tx.GuardianSignatures) == 0 {
		return false
	}
	if tx.Nonce <= 0 {
		return false
	}
	if tx.Signature == "" || tx.PublicKey == "" {
		return false
	}
	return verifyStructSig(tx.PublicKey, tx.Signature, func() any {
		copy := tx
		copy.Signature = ""
		return copy
	})
}
