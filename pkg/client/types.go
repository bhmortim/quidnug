package client

// Wire types exposed to external Go consumers.
//
// These must stay JSON-wire-compatible with the node. The server-side
// equivalents live in internal/core (not importable from outside the
// module). The SDK converts at the HTTP boundary.

// OwnershipStake is one line of a title's ownership map.
type OwnershipStake struct {
	OwnerID    string  `json:"ownerId"`
	Percentage float64 `json:"percentage"`
	StakeType  string  `json:"stakeType,omitempty"`
}

// Title describes asset ownership at the time of query.
type Title struct {
	AssetID    string            `json:"assetId"`
	Domain     string            `json:"domain,omitempty"`
	TitleType  string            `json:"titleType,omitempty"`
	Owners     []OwnershipStake  `json:"ownershipMap"`
	Creator    string            `json:"issuerQuid,omitempty"`
	Signatures map[string]string `json:"transferSigs,omitempty"`
	Attributes map[string]any    `json:"attributes,omitempty"`
}

// IdentityRecord is the current identity snapshot for a quid.
type IdentityRecord struct {
	QuidID      string         `json:"quidId"`
	Creator     string         `json:"creator,omitempty"`
	UpdateNonce int64          `json:"updateNonce"`
	Signature   string         `json:"signature,omitempty"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Attributes  map[string]any `json:"attributes,omitempty"`
	HomeDomain  string         `json:"homeDomain,omitempty"`
	PublicKey   string         `json:"publicKey,omitempty"`
}

// TrustEdge is a direct outbound trust edge.
type TrustEdge struct {
	Truster     string         `json:"truster"`
	Trustee     string         `json:"trustee"`
	TrustLevel  float64        `json:"trustLevel"`
	Domain      string         `json:"domain"`
	Nonce       int64          `json:"nonce"`
	Signature   string         `json:"signature,omitempty"`
	ValidUntil  int64          `json:"validUntil,omitempty"`
	Description string         `json:"description,omitempty"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

// TrustResult is the output of a relational-trust query.
type TrustResult struct {
	Observer   string   `json:"observer"`
	Target     string   `json:"target"`
	TrustLevel float64  `json:"trustLevel"`
	Path       []string `json:"trustPath"`
	PathDepth  int      `json:"pathDepth"`
	Domain     string   `json:"domain"`
}

// Event is one row of a subject's event stream.
type Event struct {
	SubjectID   string         `json:"subjectId"`
	SubjectType string         `json:"subjectType"`
	EventType   string         `json:"eventType"`
	Payload     map[string]any `json:"payload,omitempty"`
	PayloadCID  string         `json:"payloadCid,omitempty"`
	Timestamp   int64          `json:"timestamp"`
	Sequence    int64          `json:"sequence"`
	Creator     string         `json:"creator,omitempty"`
	Signature   string         `json:"signature,omitempty"`
}

// GuardianRef is one guardian entry in a guardian set.
type GuardianRef struct {
	Quid          string `json:"quid"`
	Weight        int    `json:"weight"`
	Epoch         int    `json:"epoch"`
	AddedAtBlock  int64  `json:"addedAtBlock,omitempty"`
}

// GuardianSet is the current guardian configuration for a subject.
type GuardianSet struct {
	SubjectQuid              string        `json:"subjectQuid"`
	Guardians                []GuardianRef `json:"guardians"`
	Threshold                int           `json:"threshold"`
	RecoveryDelaySeconds     int64         `json:"recoveryDelaySeconds"`
	RequireGuardianRotation  bool          `json:"requireGuardianRotation,omitempty"`
	UpdatedAtBlock           int64         `json:"updatedAtBlock,omitempty"`
}

// TotalWeight sums effective (>=1) guardian weights.
func (s GuardianSet) TotalWeight() int {
	total := 0
	for _, g := range s.Guardians {
		if g.Weight <= 0 {
			total++
		} else {
			total += g.Weight
		}
	}
	return total
}

// PrimarySignature is the subject's own signature on a guardian-set
// update. Required when the subject is available.
type PrimarySignature struct {
	KeyEpoch  int    `json:"keyEpoch"`
	Signature string `json:"signature"`
}

// GuardianSignature is a per-guardian signature with its key epoch.
type GuardianSignature struct {
	GuardianQuid string `json:"guardianQuid"`
	KeyEpoch     int    `json:"keyEpoch"`
	Signature    string `json:"signature"`
}

// GuardianSetUpdate installs or rotates the guardian configuration.
type GuardianSetUpdate struct {
	SubjectQuid          string              `json:"subjectQuid"`
	NewSet               GuardianSet         `json:"newSet"`
	AnchorNonce          int64               `json:"anchorNonce"`
	ValidFrom            int64               `json:"validFrom"`
	PrimarySignature     *PrimarySignature   `json:"primarySignature,omitempty"`
	NewGuardianConsents  []GuardianSignature `json:"newGuardianConsents,omitempty"`
	CurrentGuardianSigs  []GuardianSignature `json:"currentGuardianSigs,omitempty"`
}

// GuardianRecoveryInit starts the delayed recovery flow.
type GuardianRecoveryInit struct {
	SubjectQuid          string              `json:"subjectQuid"`
	FromEpoch            int                 `json:"fromEpoch"`
	ToEpoch              int                 `json:"toEpoch"`
	NewPublicKey         string              `json:"newPublicKey"`
	MinNextNonce         int64               `json:"minNextNonce"`
	MaxAcceptedOldNonce  int64               `json:"maxAcceptedOldNonce"`
	AnchorNonce          int64               `json:"anchorNonce"`
	ValidFrom            int64               `json:"validFrom"`
	GuardianSigs         []GuardianSignature `json:"guardianSigs,omitempty"`
	ExpiresAt            int64               `json:"expiresAt,omitempty"`
}

// GuardianRecoveryVeto aborts an in-flight recovery.
type GuardianRecoveryVeto struct {
	SubjectQuid         string              `json:"subjectQuid"`
	RecoveryAnchorHash  string              `json:"recoveryAnchorHash"`
	AnchorNonce         int64               `json:"anchorNonce"`
	ValidFrom           int64               `json:"validFrom"`
	PrimarySignature    *PrimarySignature   `json:"primarySignature,omitempty"`
	GuardianSigs        []GuardianSignature `json:"guardianSigs,omitempty"`
}

// GuardianRecoveryCommit finalizes a recovery after the delay has elapsed.
type GuardianRecoveryCommit struct {
	SubjectQuid        string `json:"subjectQuid"`
	RecoveryAnchorHash string `json:"recoveryAnchorHash"`
	AnchorNonce        int64  `json:"anchorNonce"`
	ValidFrom          int64  `json:"validFrom"`
	CommitterQuid      string `json:"committerQuid"`
	CommitterSig       string `json:"committerSig"`
}

// GuardianResignation lets a guardian leave the set.
type GuardianResignation struct {
	GuardianQuid     string `json:"guardianQuid"`
	SubjectQuid      string `json:"subjectQuid"`
	GuardianSetHash  string `json:"guardianSetHash"`
	ResignationNonce int64  `json:"resignationNonce"`
	EffectiveAt      int64  `json:"effectiveAt"`
	Signature        string `json:"signature,omitempty"`
}

// DomainFingerprint is a signed summary of a domain's latest block.
type DomainFingerprint struct {
	Domain        string `json:"domain"`
	BlockHeight   int64  `json:"blockHeight"`
	BlockHash     string `json:"blockHash"`
	ProducerQuid  string `json:"producerQuid"`
	Timestamp     int64  `json:"timestamp"`
	Signature     string `json:"signature,omitempty"`
	SchemaVersion int    `json:"schemaVersion,omitempty"`
}

// MerkleProofFrame is one sibling hash + side in an inclusion proof.
type MerkleProofFrame struct {
	Hash string `json:"hash"`
	Side string `json:"side"` // "left" or "right"
}

// AnchorGossipMessage is one cross-domain anchor delivery.
type AnchorGossipMessage struct {
	MessageID           string             `json:"messageId"`
	OriginDomain        string             `json:"originDomain"`
	OriginBlockHeight   int64              `json:"originBlockHeight"`
	OriginBlock         map[string]any     `json:"originBlock"`
	AnchorTxIndex       int                `json:"anchorTxIndex"`
	DomainFingerprint   DomainFingerprint  `json:"domainFingerprint"`
	Timestamp           int64              `json:"timestamp"`
	GossipProducerQuid  string             `json:"gossipProducerQuid"`
	GossipSignature     string             `json:"gossipSignature,omitempty"`
	SchemaVersion       int                `json:"schemaVersion,omitempty"`
	MerkleProof         []MerkleProofFrame `json:"merkleProof,omitempty"`
}

// NonceSnapshotEntry is one row in a K-of-K bootstrap snapshot.
type NonceSnapshotEntry struct {
	Quid     string `json:"quid"`
	Epoch    int    `json:"epoch"`
	MaxNonce int64  `json:"maxNonce"`
}

// NonceSnapshot is a signed K-of-K bootstrap packet (QDP-0008).
type NonceSnapshot struct {
	BlockHeight   int64                `json:"blockHeight"`
	BlockHash     string               `json:"blockHash"`
	Timestamp     int64                `json:"timestamp"`
	TrustDomain   string               `json:"trustDomain"`
	Entries       []NonceSnapshotEntry `json:"entries"`
	ProducerQuid  string               `json:"producerQuid"`
	Signature     string               `json:"signature,omitempty"`
	SchemaVersion int                  `json:"schemaVersion,omitempty"`
}

// ForkSig is one validator's signature on a fork-activation block.
type ForkSig struct {
	ValidatorQuid string `json:"validatorQuid"`
	KeyEpoch      int    `json:"keyEpoch"`
	Signature     string `json:"signature"`
}

// ForkBlock is a signed activation block for a protocol feature (QDP-0009).
type ForkBlock struct {
	TrustDomain string    `json:"trustDomain"`
	Feature     string    `json:"feature"`
	ForkHeight  int64     `json:"forkHeight"`
	ForkNonce   int64     `json:"forkNonce"`
	ProposedAt  int64     `json:"proposedAt"`
	Signatures  []ForkSig `json:"signatures,omitempty"`
	ExpiresAt   int64     `json:"expiresAt,omitempty"`
}

// Pagination is the standard pagination envelope.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// IdentityParams are the writable fields for RegisterIdentity.
type IdentityParams struct {
	SubjectQuid string         // defaults to signer.ID
	Domain      string         // defaults to "default"
	Name        string         // optional
	Description string         // optional
	Attributes  map[string]any // optional
	HomeDomain  string         // optional — QDP-0007
	UpdateNonce int64          // default 1
}

// TrustParams are the writable fields for GrantTrust.
type TrustParams struct {
	Trustee     string
	Level       float64
	Domain      string // defaults to "default"
	Nonce       int64  // default 1
	ValidUntil  int64  // optional
	Description string // optional
}

// TitleParams are the writable fields for RegisterTitle.
type TitleParams struct {
	AssetID        string
	Owners         []OwnershipStake
	Domain         string // defaults to "default"
	TitleType      string
	PrevTitleTxID  string
}

// EventParams are the writable fields for EmitEvent.
//
// Exactly one of Payload or PayloadCID must be set.
type EventParams struct {
	SubjectID   string
	SubjectType string // "QUID" or "TITLE"
	EventType   string
	Domain      string         // defaults to "default"
	Payload     map[string]any // mutually exclusive with PayloadCID
	PayloadCID  string         // mutually exclusive with Payload
	Sequence    int64          // 0 = auto-detect by querying the stream
}
