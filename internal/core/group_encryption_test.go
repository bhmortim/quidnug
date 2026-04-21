// group_encryption_test.go — QDP-0024 Phase 1 in-process
// integration test. Covers the full enterprise-domain-
// authority-style flow: create group, publish member key
// packages, advance epoch (distribute wrapped secrets),
// encrypt a record, decrypt as a member, fail as non-member.

package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/quidnug/quidnug/pkg/crypto/groupenc"
)

// groupMember carries both the ECDSA identity (for signing
// events) and the X25519 keypair (for group-encryption
// membership).
type groupMember struct {
	name     string
	priv     *ecdsa.PrivateKey
	pubHex   string
	quidID   string
	x25519Kp *groupenc.KeyPair
}

func newGroupMember(t *testing.T, name string) *groupMember {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa: %v", err)
	}
	pubBytes := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck
	h := sha256.Sum256(pubBytes)
	kp, err := groupenc.GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("x25519: %v", err)
	}
	return &groupMember{
		name:     name,
		priv:     priv,
		pubHex:   hex.EncodeToString(pubBytes),
		quidID:   hex.EncodeToString(h[:8]),
		x25519Kp: kp,
	}
}

func (m *groupMember) signECDSA(data []byte) string {
	digest := sha256.Sum256(data)
	r, s := SignRFC6979(m.priv, digest[:])
	sig := make([]byte, 64)
	rb := r.Bytes()
	sb := s.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):64], sb)
	return hex.EncodeToString(sig)
}

// signMemberKeyPackage fills PublicKey/Signature + ID.
func (m *groupMember) signMemberKeyPackage(tx MemberKeyPackageTransaction) MemberKeyPackageTransaction {
	tx.PublicKey = m.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			MemberQuid      string
			X25519PublicKey string
			Nonce           int64
			Timestamp       int64
		}{tx.MemberQuid, tx.X25519PublicKey, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = m.signECDSA(b)
	return tx
}

func (m *groupMember) signGroupCreate(tx GroupCreateTransaction) GroupCreateTransaction {
	tx.PublicKey = m.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			GroupID   string
			GroupType string
			Nonce     int64
			Timestamp int64
		}{tx.GroupID, tx.GroupType, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = m.signECDSA(b)
	return tx
}

func (m *groupMember) signEpochAdvance(tx EpochAdvanceTransaction) EpochAdvanceTransaction {
	tx.PublicKey = m.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			GroupID   string
			NewEpoch  int64
			Reason    string
			Nonce     int64
			Timestamp int64
		}{tx.GroupID, tx.NewEpoch, tx.ReasonCode, tx.Nonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = m.signECDSA(b)
	return tx
}

func (m *groupMember) signEncryptedRecord(tx EncryptedRecordTransaction) EncryptedRecordTransaction {
	tx.PublicKey = m.pubHex
	tx.Signature = ""
	if tx.ID == "" {
		tx.ID = seedID(struct {
			GroupID       string
			Epoch         int64
			CiphertextHex string
			TxNonce       int64
			Timestamp     int64
		}{tx.GroupID, tx.Epoch, tx.CiphertextHex, tx.TxNonce, tx.Timestamp})
	}
	b, _ := json.Marshal(tx)
	tx.Signature = m.signECDSA(b)
	return tx
}

// --- Tests ---

func TestGroupEncryption_FullFlow(t *testing.T) {
	node := newTestNode()
	alice := newGroupMember(t, "alice")
	bob := newGroupMember(t, "bob")
	carol := newGroupMember(t, "carol") // not a member — should not decrypt

	nowSec := time.Now().Unix()
	groupID := "bank.chase.employees"

	// 1. Alice + Bob publish their X25519 key packages.
	for _, m := range []*groupMember{alice, bob} {
		pkg := m.signMemberKeyPackage(MemberKeyPackageTransaction{
			BaseTransaction: BaseTransaction{
				Type:        TxTypeMemberKeyPackage,
				TrustDomain: groupID,
				Timestamp:   nowSec,
			},
			MemberQuid:      m.quidID,
			X25519PublicKey: hex.EncodeToString(m.x25519Kp.PublicKeyBytes()),
			Ciphersuite:     "X25519-AESGCM256-HKDFSHA256",
			Nonce:           1,
		})
		if _, err := node.AddMemberKeyPackageTransaction(pkg); err != nil {
			t.Fatalf("%s key package: %v", m.name, err)
		}
	}

	// 2. Alice creates the group (static, alice + bob).
	create := alice.signGroupCreate(GroupCreateTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeGroupCreate,
			TrustDomain: groupID,
			Timestamp:   nowSec + 1,
		},
		GroupID:       groupID,
		GroupName:     "Chase Employees",
		GroupType:     "static",
		StaticMembers: []string{alice.quidID, bob.quidID},
		Policy: GroupPolicy{
			RotationIntervalSeconds: 90 * 24 * 3600,
			MaxMembers:              10000,
			HistoryRetention:        "forever",
			KeyScheme:               "direct-wrap",
		},
		Nonce: 1,
	})
	if _, err := node.AddGroupCreateTransaction(create); err != nil {
		t.Fatalf("group create: %v", err)
	}

	// 3. Alice generates an epoch secret and wraps it for
	// each member.
	epochSecret, err := groupenc.NewEpochSecret(rand.Reader)
	if err != nil {
		t.Fatalf("epoch secret: %v", err)
	}
	wrappedAliceHex := wrapMemberSecret(t, alice.x25519Kp, epochSecret)
	wrappedBobHex := wrapMemberSecret(t, bob.x25519Kp, epochSecret)

	adv := alice.signEpochAdvance(EpochAdvanceTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeEpochAdvance,
			TrustDomain: groupID,
			Timestamp:   nowSec + 2,
		},
		GroupID:       groupID,
		PreviousEpoch: 0,
		NewEpoch:      1,
		ReasonCode:    "scheduled",
		WrappedSecrets: map[string]string{
			alice.quidID: wrappedAliceHex,
			bob.quidID:   wrappedBobHex,
		},
		EffectiveAt: time.Now().UnixNano(),
		Nonce:       2,
	})
	if _, err := node.AddEpochAdvanceTransaction(adv); err != nil {
		t.Fatalf("epoch advance: %v", err)
	}

	// 4. Alice encrypts a record under the epoch secret.
	plaintext := []byte(`{"recordType":"DIRECTORY/employees","value":"..."}`)
	nonce, ct, err := groupenc.EncryptRecord(rand.Reader, epochSecret, plaintext, nil)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	rec := alice.signEncryptedRecord(EncryptedRecordTransaction{
		BaseTransaction: BaseTransaction{
			Type:        TxTypeEncryptedRecord,
			TrustDomain: groupID,
			Timestamp:   nowSec + 3,
		},
		GroupID:       groupID,
		Epoch:         1,
		ContentType:   "directory",
		Nonce12Hex:    hex.EncodeToString(nonce),
		CiphertextHex: hex.EncodeToString(ct),
		TxNonce:       3,
	})
	if _, err := node.AddEncryptedRecordTransaction(rec); err != nil {
		t.Fatalf("record: %v", err)
	}

	// 5. Bob fetches + decrypts the record.
	recs := node.GroupRegistry.GetEncryptedRecords(groupID, 0)
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}

	// Bob first unwraps his epoch secret from the EPOCH_ADVANCE.
	advance, ok := node.GroupRegistry.GetLatestEpoch(groupID)
	if !ok {
		t.Fatal("no latest epoch")
	}
	bobWrappedHex, ok := advance.WrappedSecrets[bob.quidID]
	if !ok {
		t.Fatal("bob has no wrapped secret")
	}
	bobWrappedBytes, _ := hex.DecodeString(bobWrappedHex)
	bobWrapped, err := groupenc.UnmarshalWrappedSecret(bobWrappedBytes)
	if err != nil {
		t.Fatalf("unmarshal wrapped: %v", err)
	}
	bobSecret, err := groupenc.UnwrapEpochKey(bob.x25519Kp.Private, bobWrapped)
	if err != nil {
		t.Fatalf("bob unwrap: %v", err)
	}
	if !bytes.Equal(bobSecret, epochSecret) {
		t.Error("bob's recovered secret differs from original")
	}

	// Now Bob decrypts the record.
	recNonce, _ := hex.DecodeString(recs[0].Nonce12Hex)
	recCT, _ := hex.DecodeString(recs[0].CiphertextHex)
	gotPlaintext, err := groupenc.DecryptRecord(bobSecret, recNonce, recCT, nil)
	if err != nil {
		t.Fatalf("bob decrypt: %v", err)
	}
	if !bytes.Equal(gotPlaintext, plaintext) {
		t.Errorf("bob got wrong plaintext")
	}

	// 6. Carol (not a member) has no wrapped secret; she can't
	// recover the epoch secret; decryption with her own random
	// key fails.
	if _, ok := advance.WrappedSecrets[carol.quidID]; ok {
		t.Error("carol should not have a wrapped secret")
	}
	randomKey := make([]byte, 32)
	_, _ = rand.Read(randomKey)
	if _, err := groupenc.DecryptRecord(randomKey, recNonce, recCT, nil); err == nil {
		t.Error("carol decrypted with random key — catastrophic")
	}
}

func TestGroupEncryption_RemoveMemberRevokesFutureAccess(t *testing.T) {
	node := newTestNode()
	alice := newGroupMember(t, "alice")
	bob := newGroupMember(t, "bob")
	carol := newGroupMember(t, "carol")

	nowSec := time.Now().Unix()
	groupID := "board.bigcorp"

	// Publish key packages for all three.
	for _, m := range []*groupMember{alice, bob, carol} {
		pkg := m.signMemberKeyPackage(MemberKeyPackageTransaction{
			BaseTransaction: BaseTransaction{
				Type: TxTypeMemberKeyPackage, TrustDomain: groupID, Timestamp: nowSec,
			},
			MemberQuid:      m.quidID,
			X25519PublicKey: hex.EncodeToString(m.x25519Kp.PublicKeyBytes()),
			Nonce:           1,
		})
		node.AddMemberKeyPackageTransaction(pkg)
	}

	// Create group with all three.
	create := alice.signGroupCreate(GroupCreateTransaction{
		BaseTransaction: BaseTransaction{
			Type: TxTypeGroupCreate, TrustDomain: groupID, Timestamp: nowSec + 1,
		},
		GroupID: groupID, GroupName: "Board", GroupType: "static",
		StaticMembers: []string{alice.quidID, bob.quidID, carol.quidID},
		Policy:        GroupPolicy{KeyScheme: "direct-wrap"},
		Nonce:         1,
	})
	node.AddGroupCreateTransaction(create)

	// Epoch 1: all three get wrapped secrets.
	secret1, _ := groupenc.NewEpochSecret(rand.Reader)
	adv1 := alice.signEpochAdvance(EpochAdvanceTransaction{
		BaseTransaction: BaseTransaction{
			Type: TxTypeEpochAdvance, TrustDomain: groupID, Timestamp: nowSec + 2,
		},
		GroupID: groupID, PreviousEpoch: 0, NewEpoch: 1, ReasonCode: "scheduled",
		WrappedSecrets: map[string]string{
			alice.quidID: wrapMemberSecret(t, alice.x25519Kp, secret1),
			bob.quidID:   wrapMemberSecret(t, bob.x25519Kp, secret1),
			carol.quidID: wrapMemberSecret(t, carol.x25519Kp, secret1),
		},
		EffectiveAt: time.Now().UnixNano(),
		Nonce:       2,
	})
	node.AddEpochAdvanceTransaction(adv1)

	// Epoch 2: remove carol. New secret wrapped only for
	// alice + bob.
	secret2, _ := groupenc.NewEpochSecret(rand.Reader)
	adv2 := alice.signEpochAdvance(EpochAdvanceTransaction{
		BaseTransaction: BaseTransaction{
			Type: TxTypeEpochAdvance, TrustDomain: groupID, Timestamp: nowSec + 3,
		},
		GroupID: groupID, PreviousEpoch: 1, NewEpoch: 2,
		ReasonCode:     "member-removed",
		RemovedMembers: []string{carol.quidID},
		WrappedSecrets: map[string]string{
			alice.quidID: wrapMemberSecret(t, alice.x25519Kp, secret2),
			bob.quidID:   wrapMemberSecret(t, bob.x25519Kp, secret2),
		},
		EffectiveAt: time.Now().UnixNano(),
		Nonce:       3,
	})
	if _, err := node.AddEpochAdvanceTransaction(adv2); err != nil {
		t.Fatalf("advance: %v", err)
	}

	// Carol is NOT in the epoch-2 wrapped secrets.
	latest, _ := node.GroupRegistry.GetLatestEpoch(groupID)
	if _, ok := latest.WrappedSecrets[carol.quidID]; ok {
		t.Error("carol still has a secret in epoch 2 — revocation failed")
	}

	// Alice encrypts a record under epoch 2.
	plaintext := []byte("secret board memo")
	recNonce, recCT, _ := groupenc.EncryptRecord(rand.Reader, secret2, plaintext, nil)

	// Carol only has epoch 1 secret; trying to decrypt the
	// epoch-2 record with it fails.
	carolEpoch1WrappedHex := adv1.WrappedSecrets[carol.quidID]
	carolEpoch1WrappedBytes, _ := hex.DecodeString(carolEpoch1WrappedHex)
	carolEpoch1Wrapped, _ := groupenc.UnmarshalWrappedSecret(carolEpoch1WrappedBytes)
	carolEpoch1Secret, _ := groupenc.UnwrapEpochKey(carol.x25519Kp.Private, carolEpoch1Wrapped)
	if _, err := groupenc.DecryptRecord(carolEpoch1Secret, recNonce, recCT, nil); err == nil {
		t.Error("carol decrypted epoch-2 record with epoch-1 secret — bug")
	}

	// But alice still can.
	aliceEpoch2WrappedHex := latest.WrappedSecrets[alice.quidID]
	aliceEpoch2WrappedBytes, _ := hex.DecodeString(aliceEpoch2WrappedHex)
	aliceEpoch2Wrapped, _ := groupenc.UnmarshalWrappedSecret(aliceEpoch2WrappedBytes)
	aliceEpoch2Secret, _ := groupenc.UnwrapEpochKey(alice.x25519Kp.Private, aliceEpoch2Wrapped)
	if got, err := groupenc.DecryptRecord(aliceEpoch2Secret, recNonce, recCT, nil); err != nil {
		t.Errorf("alice decrypt: %v", err)
	} else if !bytes.Equal(got, plaintext) {
		t.Error("alice got wrong plaintext")
	}
}

// wrapMemberSecret wraps an epoch secret for a member + hex
// encodes the result.
func wrapMemberSecret(t *testing.T, kp *groupenc.KeyPair, secret []byte) string {
	t.Helper()
	w, err := groupenc.WrapEpochKey(rand.Reader, kp.Public, secret)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	return hex.EncodeToString(w.Marshal())
}
