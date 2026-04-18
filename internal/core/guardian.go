package core

import (
	"time"
)

// Guardian-based recovery (QDP-0002). A signer declares a set of guardian
// quids and a threshold; future key rotations can be authorized either
// by the primary key (the current fast path) or by M-of-N guardians
// through a time-locked recovery flow. During the delay the legitimate
// owner can veto with a single primary-key signature.

// Four new anchor kinds extend QDP-0001's set. The numeric values are
// append-only so existing serialized anchors keep their meaning.
const (
	// AnchorGuardianRecoveryInit starts a time-locked recovery.
	// Carries the proposed new key and the guardian signatures.
	AnchorGuardianRecoveryInit AnchorKind = iota + 4 // = 4

	// AnchorGuardianRecoveryVeto cancels a pending recovery. Signed
	// by the primary key (fast path) OR by threshold guardians.
	AnchorGuardianRecoveryVeto

	// AnchorGuardianRecoveryCommit finalizes a mature pending recovery.
	// Carries only a committer's signature for traceability — the
	// real authorization was the Init plus elapsed delay.
	AnchorGuardianRecoveryCommit

	// AnchorGuardianSetUpdate installs or replaces a guardian set.
	// First install: primary-only. Replace: primary + threshold
	// guardians (QDP-0002 §6.4.4).
	AnchorGuardianSetUpdate
)

// String extension for the new kinds (existing String() covers 1-3).
// Implemented as a separate method to keep the two additions
// visually aligned with where the constants live.
func (k AnchorKind) guardianString() (string, bool) {
	switch k {
	case AnchorGuardianRecoveryInit:
		return "guardian_recovery_init", true
	case AnchorGuardianRecoveryVeto:
		return "guardian_recovery_veto", true
	case AnchorGuardianRecoveryCommit:
		return "guardian_recovery_commit", true
	case AnchorGuardianSetUpdate:
		return "guardian_set_update", true
	}
	return "", false
}

// Transaction-type constants for wrapping guardian anchors in blocks.
// Each guardian anchor has a distinct TransactionType so block-
// processing can dispatch without needing to inspect payloads first.
const (
	TxTypeGuardianRecoveryInit   TransactionType = "GUARDIAN_RECOVERY_INIT"
	TxTypeGuardianRecoveryVeto   TransactionType = "GUARDIAN_RECOVERY_VETO"
	TxTypeGuardianRecoveryCommit TransactionType = "GUARDIAN_RECOVERY_COMMIT"
	TxTypeGuardianSetUpdate      TransactionType = "GUARDIAN_SET_UPDATE"
)

// Guardian-recovery constants. See QDP-0002 §6.1.
const (
	MinRecoveryDelay = 1 * time.Hour
	MaxRecoveryDelay = 365 * 24 * time.Hour // 1 year
)

// GuardianRef declares one guardian's participation in a subject's
// recovery quorum. The Epoch field pins the guardian's key version at
// set-install time so a later rotation of the guardian doesn't
// silently grant recovery power to a new key until the subject
// explicitly refreshes via GuardianSetUpdate.
type GuardianRef struct {
	Quid         string `json:"quid"`
	Weight       uint16 `json:"weight,omitempty"` // defaults to 1 when zero
	Epoch        uint32 `json:"epoch"`
	AddedAtBlock int64  `json:"addedAtBlock,omitempty"`
}

// EffectiveWeight returns the guardian's weight with a default of 1
// when the stored value is zero. Letting zero mean "default 1" keeps
// the common equal-weight case concise in JSON without losing the
// ability to express weighted voting when needed.
func (g GuardianRef) EffectiveWeight() uint16 {
	if g.Weight == 0 {
		return 1
	}
	return g.Weight
}

// GuardianSet is a subject quid's declared recovery authority. Stored
// in the ledger keyed by subject quid.
type GuardianSet struct {
	Guardians               []GuardianRef `json:"guardians"`
	Threshold               uint16        `json:"threshold"`
	RecoveryDelay           time.Duration `json:"recoveryDelay"` // serialized as nanoseconds
	UpdatedAtBlock          int64         `json:"updatedAtBlock,omitempty"`
	MaxConcurrentRecoveries uint8         `json:"maxConcurrentRecoveries,omitempty"`
	RequireGuardianRotation bool          `json:"requireGuardianRotation,omitempty"`
}

// Empty reports whether the set carries no guardians. A subject with
// no guardian set declared at all is distinguishable from a set with
// zero entries by checking presence in the ledger's map.
func (s *GuardianSet) Empty() bool {
	return s == nil || len(s.Guardians) == 0
}

// TotalWeight is the sum of EffectiveWeight across all guardians —
// the ceiling against which Threshold must be ≤.
func (s *GuardianSet) TotalWeight() uint32 {
	if s == nil {
		return 0
	}
	var total uint32
	for _, g := range s.Guardians {
		total += uint32(g.EffectiveWeight())
	}
	return total
}

// GuardianSignature is one guardian's signature over a
// guardian-recovery anchor's signable data.
type GuardianSignature struct {
	GuardianQuid string `json:"guardianQuid"`
	KeyEpoch     uint32 `json:"keyEpoch"`
	Signature    string `json:"signature"`
}

// PrimarySignature is a subject's own signature over a guardian-
// recovery anchor's signable data. Only used for primary-key veto and
// for the fast-path on GuardianSetUpdate installs.
type PrimarySignature struct {
	KeyEpoch  uint32 `json:"keyEpoch"`
	Signature string `json:"signature"`
}

// ----- Anchor structs ------------------------------------------------------

// GuardianRecoveryInit starts a delayed rotation authorized by a
// guardian threshold. Signable data is the struct with GuardianSigs
// cleared (every guardian signs a canonical version that includes the
// other guardian quids, but not their signatures — mirrors how block
// signatures work).
type GuardianRecoveryInit struct {
	Kind                AnchorKind          `json:"kind"`
	SubjectQuid         string              `json:"subjectQuid"`
	FromEpoch           uint32              `json:"fromEpoch"`
	ToEpoch             uint32              `json:"toEpoch"`
	NewPublicKey        string              `json:"newPublicKey"`
	MinNextNonce        int64               `json:"minNextNonce"`
	MaxAcceptedOldNonce int64               `json:"maxAcceptedOldNonce"`
	AnchorNonce         int64               `json:"anchorNonce"`
	ValidFrom           int64               `json:"validFrom"`
	ExpiresAt           int64               `json:"expiresAt,omitempty"`
	GuardianSigs        []GuardianSignature `json:"guardianSigs"`
}

// GuardianRecoveryVeto cancels a pending GuardianRecoveryInit.
// Authorized by either a single PrimarySignature (fast path) OR by
// threshold guardians. Exactly one of the two signature paths must be
// populated; ValidateGuardianRecoveryVeto enforces that.
type GuardianRecoveryVeto struct {
	Kind               AnchorKind          `json:"kind"`
	SubjectQuid        string              `json:"subjectQuid"`
	RecoveryAnchorHash string              `json:"recoveryAnchorHash"`
	AnchorNonce        int64               `json:"anchorNonce"`
	ValidFrom          int64               `json:"validFrom"`
	PrimarySignature   *PrimarySignature   `json:"primarySignature,omitempty"`
	GuardianSigs       []GuardianSignature `json:"guardianSigs,omitempty"`
}

// GuardianRecoveryCommit finalizes a mature pending recovery. Any
// quid can publish this once the delay has elapsed; the committer's
// signature is for audit traceability, not authorization — the real
// authorization was the Init anchor plus time.
type GuardianRecoveryCommit struct {
	Kind               AnchorKind `json:"kind"`
	SubjectQuid        string     `json:"subjectQuid"`
	RecoveryAnchorHash string     `json:"recoveryAnchorHash"`
	AnchorNonce        int64      `json:"anchorNonce"`
	ValidFrom          int64      `json:"validFrom"`
	CommitterQuid      string     `json:"committerQuid"`
	CommitterSig       string     `json:"committerSig"`
}

// GuardianSetUpdate installs or replaces a GuardianSet.
//
//   - First install (no current set): primary-only signature.
//   - Replace existing set: primary + ≥ Threshold guardians of the
//     CURRENT set (not the new one — mutation requires existing
//     authority, otherwise an attacker could hijack the recovery
//     authority through a unilateral update).
type GuardianSetUpdate struct {
	Kind             AnchorKind          `json:"kind"`
	SubjectQuid      string              `json:"subjectQuid"`
	NewSet           GuardianSet         `json:"newSet"`
	AnchorNonce      int64               `json:"anchorNonce"`
	ValidFrom        int64               `json:"validFrom"`
	PrimarySignature *PrimarySignature   `json:"primarySignature"`
	GuardianSigs     []GuardianSignature `json:"guardianSigs,omitempty"` // required when replacing
}

// ----- Transaction wrappers ------------------------------------------------

// Each guardian anchor kind has its own transaction wrapper so
// processBlockTransactions can dispatch by Type without inspecting
// payload contents first.

type GuardianRecoveryInitTransaction struct {
	BaseTransaction
	Init GuardianRecoveryInit `json:"init"`
}

type GuardianRecoveryVetoTransaction struct {
	BaseTransaction
	Veto GuardianRecoveryVeto `json:"veto"`
}

type GuardianRecoveryCommitTransaction struct {
	BaseTransaction
	Commit GuardianRecoveryCommit `json:"commit"`
}

type GuardianSetUpdateTransaction struct {
	BaseTransaction
	Update GuardianSetUpdate `json:"update"`
}

// ----- Recovery state machine ---------------------------------------------

// RecoveryState is the lifecycle of a pending guardian recovery.
// Encoded as iota for internal compactness; external callers use the
// Stringer.
type RecoveryState int

const (
	RecoveryIdle       RecoveryState = iota // no pending recovery
	RecoveryPending                         // init accepted, delay elapsing
	RecoveryCommitted                       // commit accepted → rotation in effect
	RecoveryVetoed                          // canceled during delay
	RecoveryReplaced                        // superseded by a later init
)

func (s RecoveryState) String() string {
	switch s {
	case RecoveryIdle:
		return "idle"
	case RecoveryPending:
		return "pending"
	case RecoveryCommitted:
		return "committed"
	case RecoveryVetoed:
		return "vetoed"
	case RecoveryReplaced:
		return "replaced"
	default:
		return "unknown"
	}
}

// PendingRecovery tracks an Init anchor through the delay window. The
// InitHash is stored so Veto/Commit anchors can unambiguously
// reference it; the Init itself is preserved verbatim so Commit can
// apply the recorded rotation without recomputing from scratch.
type PendingRecovery struct {
	InitHash        string
	Init            GuardianRecoveryInit
	InitBlockHeight int64
	MaturityUnix    int64 // unix seconds at which commit is allowed
	State           RecoveryState
}
