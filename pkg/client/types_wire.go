package client

// types_wire.go — client-side typed mirror structs for every
// v1.0 transaction type.
//
// WHY THIS FILE EXISTS
//
// The reference node in internal/core validates signatures by
// calling json.Marshal on the typed tx struct with Signature
// cleared. encoding/json emits fields in struct-declaration
// order, so the SDK MUST mirror that exact layout (same field
// order, same json tags, same omitempty behavior) for signed
// payloads to round-trip.
//
// Historically pkg/client used map[string]any + CanonicalBytes
// (alphabetical round-trip) for TRUST/IDENTITY/TITLE/EVENT
// submissions. That path is broken end-to-end: the client signs
// one byte sequence, the server re-marshals to a different one,
// verification fails. See docs/test-vectors/v1.0/README.md
// § Known divergences for the history.
//
// Every wire struct here is the authoritative mirror of its
// counterpart in internal/core. If you change a field order
// there, you MUST change it here in lockstep and regenerate
// the test vectors. The vectors_test.go suite catches drift.

// ---------------------------------------------------------------
// TRUST
// ---------------------------------------------------------------

// trustTxWire mirrors core.TrustTransaction.
type trustTxWire struct {
	// BaseTransaction fields (inlined for field-order control).
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`
	// TrustTransaction-specific.
	Truster     string  `json:"truster"`
	Trustee     string  `json:"trustee"`
	TrustLevel  float64 `json:"trustLevel"`
	Nonce       int64   `json:"nonce"`
	Description string  `json:"description,omitempty"`
	ValidUntil  int64   `json:"validUntil,omitempty"`
}

// ---------------------------------------------------------------
// IDENTITY
// ---------------------------------------------------------------

// identityTxWire mirrors core.IdentityTransaction.
type identityTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	QuidID      string                 `json:"quidId"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	Creator     string                 `json:"creator"`
	UpdateNonce int64                  `json:"updateNonce"`
	HomeDomain  string                 `json:"homeDomain,omitempty"`
}

// ---------------------------------------------------------------
// TITLE
// ---------------------------------------------------------------

// ownershipStakeWire mirrors core.OwnershipStake.
type ownershipStakeWire struct {
	OwnerID    string  `json:"ownerId"`
	Percentage float64 `json:"percentage"`
	StakeType  string  `json:"stakeType,omitempty"`
}

// titleTxWire mirrors core.TitleTransaction.
type titleTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	AssetID        string               `json:"assetId"`
	Owners         []ownershipStakeWire `json:"owners"`
	PreviousOwners []ownershipStakeWire `json:"previousOwners,omitempty"`
	Signatures     map[string]string    `json:"signatures"`
	ExpiryDate     int64                `json:"expiryDate,omitempty"`
	TitleType      string               `json:"titleType,omitempty"`
}

// ---------------------------------------------------------------
// EVENT
// ---------------------------------------------------------------

// eventTxWire mirrors core.EventTransaction.
type eventTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	SubjectID       string                 `json:"subjectId"`
	SubjectType     string                 `json:"subjectType"`
	Sequence        int64                  `json:"sequence"`
	EventType       string                 `json:"eventType"`
	Payload         map[string]interface{} `json:"payload,omitempty"`
	PayloadCID      string                 `json:"payloadCid,omitempty"`
	PreviousEventID string                 `json:"previousEventId,omitempty"`
}

// ---------------------------------------------------------------
// MODERATION_ACTION (QDP-0015)
// ---------------------------------------------------------------

// moderationActionTxWire mirrors core.ModerationActionTransaction.
type moderationActionTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	ModeratorQuid  string `json:"moderatorQuid"`
	TargetType     string `json:"targetType"`
	TargetID       string `json:"targetId"`
	Scope          string `json:"scope"`
	ReasonCode     string `json:"reasonCode"`
	EvidenceURL    string `json:"evidenceUrl,omitempty"`
	AnnotationText string `json:"annotationText,omitempty"`
	SupersedesTxID string `json:"supersedesTxId,omitempty"`
	EffectiveFrom  int64  `json:"effectiveFrom,omitempty"`
	EffectiveUntil int64  `json:"effectiveUntil,omitempty"`
	Nonce          int64  `json:"nonce"`
	DoNotFederate  bool   `json:"doNotFederate,omitempty"`
}

// ---------------------------------------------------------------
// DATA_SUBJECT_REQUEST (QDP-0017)
// ---------------------------------------------------------------

// dsrRequestTxWire mirrors core.DataSubjectRequestTransaction.
type dsrRequestTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	SubjectQuid    string `json:"subjectQuid"`
	ControllerQuid string `json:"controllerQuid,omitempty"`
	RequestType    string `json:"requestType"`
	RequestDetails string `json:"requestDetails,omitempty"`
	ContactEmail   string `json:"contactEmail,omitempty"`
	Jurisdiction   string `json:"jurisdiction,omitempty"`
	Nonce          int64  `json:"nonce"`
}

// ---------------------------------------------------------------
// CONSENT_GRANT (QDP-0017)
// ---------------------------------------------------------------

// consentGrantTxWire mirrors core.ConsentGrantTransaction.
type consentGrantTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	SubjectQuid    string   `json:"subjectQuid"`
	ControllerQuid string   `json:"controllerQuid"`
	Scope          []string `json:"scope"`
	PolicyURL      string   `json:"policyUrl,omitempty"`
	PolicyHash     string   `json:"policyHash,omitempty"`
	EffectiveUntil int64    `json:"effectiveUntil,omitempty"`
	Nonce          int64    `json:"nonce"`
}

// ---------------------------------------------------------------
// CONSENT_WITHDRAW (QDP-0017)
// ---------------------------------------------------------------

// consentWithdrawTxWire mirrors core.ConsentWithdrawTransaction.
type consentWithdrawTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	SubjectQuid        string `json:"subjectQuid"`
	WithdrawsGrantTxID string `json:"withdrawsGrantTxId"`
	Reason             string `json:"reason,omitempty"`
	Nonce              int64  `json:"nonce"`
}

// ---------------------------------------------------------------
// PROCESSING_RESTRICTION (QDP-0017)
// ---------------------------------------------------------------

// processingRestrictionTxWire mirrors
// core.ProcessingRestrictionTransaction.
type processingRestrictionTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	SubjectQuid    string   `json:"subjectQuid"`
	ControllerQuid string   `json:"controllerQuid,omitempty"`
	RestrictedUses []string `json:"restrictedUses"`
	Reason         string   `json:"reason,omitempty"`
	EffectiveUntil int64    `json:"effectiveUntil,omitempty"`
	Nonce          int64    `json:"nonce"`
}

// ---------------------------------------------------------------
// DSR_COMPLIANCE (QDP-0017)
// ---------------------------------------------------------------

// dsrComplianceTxWire mirrors core.DSRComplianceTransaction.
// Field order matches core; omitempty mirrors core tags.
type dsrComplianceTxWire struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	TrustDomain string `json:"trustDomain"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"publicKey"`

	RequestTxID      string   `json:"requestTxId"`
	RequestType      string   `json:"requestType"`
	OperatorQuid     string   `json:"operatorQuid"`
	CompletedAt      int64    `json:"completedAt"`
	ActionsCategory  string   `json:"actionsCategory"`
	CarveOutsApplied []string `json:"carveOutsApplied,omitempty"`
	ManifestURL      string   `json:"manifestUrl,omitempty"`
	Nonce            int64    `json:"nonce"`
}
