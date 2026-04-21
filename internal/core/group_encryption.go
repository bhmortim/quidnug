// group_encryption.go — QDP-0024 Phase 1: group-keyed
// encryption protocol layer.
//
// Phase 1 scope: six event types (GROUP_CREATE,
// EPOCH_ADVANCE, MEMBER_KEY_PACKAGE, ENCRYPTED_RECORD,
// MEMBER_INVITE, MEMBER_KEY_RECOVERY) + their admission +
// registry. Crypto primitives live in
// `pkg/crypto/groupenc`.
//
// Out of scope Phase 1: full TreeKEM (RFC 9420); we use the
// simpler "direct-wrap" key-distribution for groups under
// ~16 members. Phase 2 adds TreeKEM for scaling + post-
// compromise security via tree-path updates.

package core

import (
	"sync"
	"time"
)

// GroupRegistry tracks every group the node has seen.
type GroupRegistry struct {
	mu sync.RWMutex

	// groups stores GroupCreateTransaction by GroupID.
	groups map[string]GroupCreateTransaction

	// latestEpoch: GroupID -> EpochAdvanceTransaction for the
	// highest-numbered epoch observed. Records encrypted at
	// older epochs are still decryptable by members who held
	// the corresponding wrapped secret at that time — the
	// node retains all EPOCH_ADVANCE events below for audit.
	latestEpoch map[string]EpochAdvanceTransaction

	// epochHistory: GroupID -> ordered list of EPOCH_ADVANCE
	// events (oldest first). For audit + member recovery.
	epochHistory map[string][]EpochAdvanceTransaction

	// keyPackages: memberQuid -> latest MemberKeyPackage.
	keyPackages map[string]MemberKeyPackageTransaction

	// encryptedRecords: GroupID -> list of records, ordered by
	// timestamp. Phase 1 keeps them all in memory; Phase 2+
	// paginates + offloads to disk via IPFS.
	encryptedRecords map[string][]EncryptedRecordTransaction

	// invites: targetMemberQuid -> list of pending
	// MemberInvite messages for that member to pick up.
	invites map[string][]MemberInviteTransaction

	// recoveries: list of MemberKeyRecoveryTransaction events
	// keyed by MemberQuid for audit + lookup.
	recoveries map[string][]MemberKeyRecoveryTransaction

	// nonces per signer.
	nonces map[string]int64
}

// NewGroupRegistry returns an empty registry.
func NewGroupRegistry() *GroupRegistry {
	return &GroupRegistry{
		groups:           make(map[string]GroupCreateTransaction),
		latestEpoch:      make(map[string]EpochAdvanceTransaction),
		epochHistory:     make(map[string][]EpochAdvanceTransaction),
		keyPackages:      make(map[string]MemberKeyPackageTransaction),
		encryptedRecords: make(map[string][]EncryptedRecordTransaction),
		invites:          make(map[string][]MemberInviteTransaction),
		recoveries:       make(map[string][]MemberKeyRecoveryTransaction),
		nonces:           make(map[string]int64),
	}
}

func (r *GroupRegistry) currentNonce(signer string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nonces[signer]
}

func (r *GroupRegistry) bumpNonce(signer string, n int64) {
	if cur, ok := r.nonces[signer]; !ok || n > cur {
		r.nonces[signer] = n
	}
}

// --- Read helpers ---

// GetGroup returns the group creation record or false.
func (r *GroupRegistry) GetGroup(groupID string) (GroupCreateTransaction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.groups[groupID]
	return g, ok
}

// GetLatestEpoch returns the most recent EPOCH_ADVANCE for
// the group or false if the group has only the initial
// create state.
func (r *GroupRegistry) GetLatestEpoch(groupID string) (EpochAdvanceTransaction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.latestEpoch[groupID]
	return e, ok
}

// GetMemberKeyPackage returns the current published key
// package for a member (for senders wrapping to them).
func (r *GroupRegistry) GetMemberKeyPackage(memberQuid string) (MemberKeyPackageTransaction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k, ok := r.keyPackages[memberQuid]
	return k, ok
}

// GetEncryptedRecords returns all records for a group since
// the given timestamp. Returns the full list if sinceUnixSec
// == 0.
func (r *GroupRegistry) GetEncryptedRecords(groupID string, sinceUnixSec int64) []EncryptedRecordTransaction {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := r.encryptedRecords[groupID]
	if sinceUnixSec == 0 {
		out := make([]EncryptedRecordTransaction, len(all))
		copy(out, all)
		return out
	}
	var out []EncryptedRecordTransaction
	for _, rec := range all {
		if rec.Timestamp >= sinceUnixSec {
			out = append(out, rec)
		}
	}
	return out
}

// GetPendingInvites returns queued invites for the member,
// then clears them (they're one-shot).
func (r *GroupRegistry) GetPendingInvites(memberQuid string) []MemberInviteTransaction {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.invites[memberQuid]
	delete(r.invites, memberQuid)
	return out
}

// --- Admission ---

// AddGroupCreateTransaction admits a GROUP_CREATE.
func (node *QuidnugNode) AddGroupCreateTransaction(tx GroupCreateTransaction) (string, error) {
	if node.GroupRegistry == nil {
		return "", ErrTxTypeUnsupported("GROUP_CREATE: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeGroupCreate
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			GroupID   string
			GroupType string
			Nonce     int64
			Timestamp int64
		}{tx.GroupID, tx.GroupType, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateGroupCreate(tx) {
		return "", ErrInvalidTx("GROUP_CREATE")
	}
	node.GroupRegistry.mu.Lock()
	node.GroupRegistry.groups[tx.GroupID] = tx
	node.GroupRegistry.bumpNonce(getSigner(tx.BaseTransaction), tx.Nonce)
	node.GroupRegistry.mu.Unlock()
	return tx.ID, nil
}

// AddEpochAdvanceTransaction admits an EPOCH_ADVANCE.
func (node *QuidnugNode) AddEpochAdvanceTransaction(tx EpochAdvanceTransaction) (string, error) {
	if node.GroupRegistry == nil {
		return "", ErrTxTypeUnsupported("EPOCH_ADVANCE: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeEpochAdvance
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			GroupID  string
			NewEpoch int64
			Reason   string
			Nonce    int64
			Timestamp int64
		}{tx.GroupID, tx.NewEpoch, tx.ReasonCode, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateEpochAdvance(tx) {
		return "", ErrInvalidTx("EPOCH_ADVANCE")
	}
	node.GroupRegistry.mu.Lock()
	node.GroupRegistry.latestEpoch[tx.GroupID] = tx
	node.GroupRegistry.epochHistory[tx.GroupID] = append(
		node.GroupRegistry.epochHistory[tx.GroupID], tx)
	node.GroupRegistry.bumpNonce(getSigner(tx.BaseTransaction), tx.Nonce)
	node.GroupRegistry.mu.Unlock()
	return tx.ID, nil
}

// AddMemberKeyPackageTransaction admits a MEMBER_KEY_PACKAGE.
func (node *QuidnugNode) AddMemberKeyPackageTransaction(tx MemberKeyPackageTransaction) (string, error) {
	if node.GroupRegistry == nil {
		return "", ErrTxTypeUnsupported("MEMBER_KEY_PACKAGE: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeMemberKeyPackage
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			MemberQuid      string
			X25519PublicKey string
			Nonce           int64
			Timestamp       int64
		}{tx.MemberQuid, tx.X25519PublicKey, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateMemberKeyPackage(tx) {
		return "", ErrInvalidTx("MEMBER_KEY_PACKAGE")
	}
	node.GroupRegistry.mu.Lock()
	node.GroupRegistry.keyPackages[tx.MemberQuid] = tx
	node.GroupRegistry.bumpNonce(tx.MemberQuid, tx.Nonce)
	node.GroupRegistry.mu.Unlock()
	return tx.ID, nil
}

// AddEncryptedRecordTransaction admits an ENCRYPTED_RECORD.
func (node *QuidnugNode) AddEncryptedRecordTransaction(tx EncryptedRecordTransaction) (string, error) {
	if node.GroupRegistry == nil {
		return "", ErrTxTypeUnsupported("ENCRYPTED_RECORD: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeEncryptedRecord
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			GroupID       string
			Epoch         int64
			CiphertextHex string
			TxNonce       int64
			Timestamp     int64
		}{tx.GroupID, tx.Epoch, tx.CiphertextHex, tx.TxNonce, tx.Timestamp})
	}
	if !node.ValidateEncryptedRecord(tx) {
		return "", ErrInvalidTx("ENCRYPTED_RECORD")
	}
	node.GroupRegistry.mu.Lock()
	node.GroupRegistry.encryptedRecords[tx.GroupID] = append(
		node.GroupRegistry.encryptedRecords[tx.GroupID], tx)
	node.GroupRegistry.bumpNonce(getSigner(tx.BaseTransaction), tx.TxNonce)
	node.GroupRegistry.mu.Unlock()
	return tx.ID, nil
}

// AddMemberInviteTransaction admits a MEMBER_INVITE.
func (node *QuidnugNode) AddMemberInviteTransaction(tx MemberInviteTransaction) (string, error) {
	if node.GroupRegistry == nil {
		return "", ErrTxTypeUnsupported("MEMBER_INVITE: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeMemberInvite
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			GroupID           string
			InvitedMemberQuid string
			EpochSnapshot     int64
			Nonce             int64
			Timestamp         int64
		}{tx.GroupID, tx.InvitedMemberQuid, tx.EpochSnapshot, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateMemberInvite(tx) {
		return "", ErrInvalidTx("MEMBER_INVITE")
	}
	node.GroupRegistry.mu.Lock()
	node.GroupRegistry.invites[tx.InvitedMemberQuid] = append(
		node.GroupRegistry.invites[tx.InvitedMemberQuid], tx)
	node.GroupRegistry.bumpNonce(tx.InvitingMemberQuid, tx.Nonce)
	node.GroupRegistry.mu.Unlock()
	return tx.ID, nil
}

// AddMemberKeyRecoveryTransaction admits a MEMBER_KEY_RECOVERY.
func (node *QuidnugNode) AddMemberKeyRecoveryTransaction(tx MemberKeyRecoveryTransaction) (string, error) {
	if node.GroupRegistry == nil {
		return "", ErrTxTypeUnsupported("MEMBER_KEY_RECOVERY: registry not initialized")
	}
	signed := tx.Signature != ""
	if !signed && tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}
	if !signed {
		tx.Type = TxTypeMemberKeyRecovery
	}
	if !signed && tx.ID == "" {
		tx.ID = seedID(struct {
			MemberQuid         string
			NewX25519PublicKey string
			Nonce              int64
			Timestamp          int64
		}{tx.MemberQuid, tx.NewX25519PublicKey, tx.Nonce, tx.Timestamp})
	}
	if !node.ValidateMemberKeyRecovery(tx) {
		return "", ErrInvalidTx("MEMBER_KEY_RECOVERY")
	}
	node.GroupRegistry.mu.Lock()
	node.GroupRegistry.recoveries[tx.MemberQuid] = append(
		node.GroupRegistry.recoveries[tx.MemberQuid], tx)
	// Replace key package if caller indicated the prior key is
	// revoked and the new one is provided.
	if tx.PriorKeyRevoked && tx.NewX25519PublicKey != "" {
		if pkg, ok := node.GroupRegistry.keyPackages[tx.MemberQuid]; ok {
			pkg.X25519PublicKey = tx.NewX25519PublicKey
			pkg.Nonce = tx.Nonce
			pkg.Timestamp = tx.Timestamp
			node.GroupRegistry.keyPackages[tx.MemberQuid] = pkg
		}
	}
	node.GroupRegistry.bumpNonce(tx.MemberQuid, tx.Nonce)
	node.GroupRegistry.mu.Unlock()
	return tx.ID, nil
}

// getSigner returns a stable handle for the tx signer when
// we don't want to derive the full quid ID (e.g., for nonce
// bookkeeping). We use the first 16 hex chars of PublicKey
// which matches the quid-ID derivation prefix; sufficient as
// a counter key.
func getSigner(base BaseTransaction) string {
	if len(base.PublicKey) >= 16 {
		return base.PublicKey[:16]
	}
	return base.PublicKey
}
