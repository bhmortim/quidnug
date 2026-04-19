//go:build pkcs11

package hsm

import (
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/miekg/pkcs11"

	"github.com/quidnug/quidnug/pkg/signer"
)

// Open initializes a PKCS#11 session against cfg.ModulePath / TokenLabel
// and locates the P-256 private key by cfg.KeyLabel or cfg.KeyID.
func Open(cfg Config) (signer.Signer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	ctx := pkcs11.New(cfg.ModulePath)
	if ctx == nil {
		return nil, errf("pkcs11.New: module not loadable: " + cfg.ModulePath)
	}
	if err := ctx.Initialize(); err != nil {
		return nil, fmt.Errorf("pkcs11.Initialize: %w", err)
	}

	slots, err := ctx.GetSlotList(true)
	if err != nil {
		_ = ctx.Finalize()
		return nil, fmt.Errorf("GetSlotList: %w", err)
	}

	var slot uint
	if cfg.Slot != 0 {
		slot = cfg.Slot
	} else {
		found := false
		for _, s := range slots {
			ti, err := ctx.GetTokenInfo(s)
			if err == nil && ti.Label == cfg.TokenLabel {
				slot = s
				found = true
				break
			}
		}
		if !found {
			_ = ctx.Finalize()
			return nil, errf("token label not found: " + cfg.TokenLabel)
		}
	}

	session, err := ctx.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		_ = ctx.Finalize()
		return nil, fmt.Errorf("OpenSession: %w", err)
	}
	if cfg.PIN != "" {
		if err := ctx.Login(session, pkcs11.CKU_USER, cfg.PIN); err != nil {
			_ = ctx.CloseSession(session)
			_ = ctx.Finalize()
			return nil, fmt.Errorf("Login: %w", err)
		}
	}

	privHandle, pubHandle, pubBytes, err := findKey(ctx, session, cfg)
	if err != nil {
		_ = ctx.Logout(session)
		_ = ctx.CloseSession(session)
		_ = ctx.Finalize()
		return nil, err
	}

	sum := sha256.Sum256(pubBytes)
	return &hsmSigner{
		ctx:          ctx,
		session:      session,
		privHandle:   privHandle,
		pubHandle:    pubHandle,
		publicKeyHex: hex.EncodeToString(pubBytes),
		quidID:       hex.EncodeToString(sum[:8]),
	}, nil
}

func findKey(ctx *pkcs11.Ctx, session pkcs11.SessionHandle, cfg Config) (
	pkcs11.ObjectHandle, pkcs11.ObjectHandle, []byte, error,
) {
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
	}
	if cfg.KeyLabel != "" {
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_LABEL, cfg.KeyLabel))
	}
	if cfg.KeyID != "" {
		id, err := hex.DecodeString(cfg.KeyID)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("KeyID hex: %w", err)
		}
		template = append(template, pkcs11.NewAttribute(pkcs11.CKA_ID, id))
	}

	if err := ctx.FindObjectsInit(session, template); err != nil {
		return 0, 0, nil, fmt.Errorf("FindObjectsInit(priv): %w", err)
	}
	objs, _, err := ctx.FindObjects(session, 1)
	_ = ctx.FindObjectsFinal(session)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("FindObjects: %w", err)
	}
	if len(objs) == 0 {
		return 0, 0, nil, errf("no matching P-256 private key on token")
	}
	privHandle := objs[0]

	// Matching public key on the same token.
	pubTmpl := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PUBLIC_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_EC),
	}
	if cfg.KeyLabel != "" {
		pubTmpl = append(pubTmpl, pkcs11.NewAttribute(pkcs11.CKA_LABEL, cfg.KeyLabel))
	}
	if cfg.KeyID != "" {
		id, _ := hex.DecodeString(cfg.KeyID)
		pubTmpl = append(pubTmpl, pkcs11.NewAttribute(pkcs11.CKA_ID, id))
	}
	if err := ctx.FindObjectsInit(session, pubTmpl); err != nil {
		return 0, 0, nil, fmt.Errorf("FindObjectsInit(pub): %w", err)
	}
	pubObjs, _, err := ctx.FindObjects(session, 1)
	_ = ctx.FindObjectsFinal(session)
	if err != nil || len(pubObjs) == 0 {
		return 0, 0, nil, errf("matching public key not found")
	}
	pubHandle := pubObjs[0]

	// Extract the EC point (SEC1 uncompressed 04||X||Y).
	attrs, err := ctx.GetAttributeValue(session, pubHandle,
		[]*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_EC_POINT, nil)})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("GetAttributeValue(EC_POINT): %w", err)
	}
	if len(attrs) == 0 || len(attrs[0].Value) == 0 {
		return 0, 0, nil, errf("EC_POINT empty on public key")
	}
	// PKCS#11 wraps the point in an ASN.1 OCTET STRING.
	var rawPoint []byte
	if _, err := asn1.Unmarshal(attrs[0].Value, &rawPoint); err != nil {
		// Some HSMs return the naked point.
		rawPoint = attrs[0].Value
	}
	// Sanity-check: length 65 with leading 0x04 (uncompressed).
	if len(rawPoint) != 65 || rawPoint[0] != 0x04 {
		return 0, 0, nil, errf("EC_POINT is not an uncompressed P-256 point")
	}
	return privHandle, pubHandle, rawPoint, nil
}

type hsmSigner struct {
	mu           sync.Mutex
	ctx          *pkcs11.Ctx
	session      pkcs11.SessionHandle
	privHandle   pkcs11.ObjectHandle
	pubHandle    pkcs11.ObjectHandle
	publicKeyHex string
	quidID       string
	closed       bool
}

func (h *hsmSigner) PublicKeyHex() string { return h.publicKeyHex }
func (h *hsmSigner) QuidID() string       { return h.quidID }

func (h *hsmSigner) Sign(data []byte) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return "", errf("hsm signer is closed")
	}
	digest := sha256.Sum256(data)
	if err := h.ctx.SignInit(h.session,
		[]*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_ECDSA, nil)},
		h.privHandle,
	); err != nil {
		return "", fmt.Errorf("SignInit: %w", err)
	}
	raw, err := h.ctx.Sign(h.session, digest[:])
	if err != nil {
		return "", fmt.Errorf("Sign: %w", err)
	}
	// PKCS#11 returns r||s (each 32 bytes for P-256). Convert to DER.
	if len(raw) != 64 {
		return "", fmt.Errorf("unexpected signature length: %d", len(raw))
	}
	r := new(big.Int).SetBytes(raw[:32])
	s := new(big.Int).SetBytes(raw[32:])
	// Constrain s to low-S (BIP-146 style) for deterministic verify
	// behavior across implementations that enforce low-S.
	n := elliptic.P256().Params().N
	halfN := new(big.Int).Rsh(n, 1)
	if s.Cmp(halfN) == 1 {
		s = new(big.Int).Sub(n, s)
	}
	type ecSig struct{ R, S *big.Int }
	der, err := asn1.Marshal(ecSig{R: r, S: s})
	if err != nil {
		return "", fmt.Errorf("DER marshal: %w", err)
	}
	return hex.EncodeToString(der), nil
}

func (h *hsmSigner) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true
	_ = h.ctx.Logout(h.session)
	_ = h.ctx.CloseSession(h.session)
	_ = h.ctx.Finalize()
	h.ctx.Destroy()
	return nil
}
