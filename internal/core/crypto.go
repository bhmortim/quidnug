package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math/big"
)

// ---------------------------------------------------------------
// RFC 6979 deterministic ECDSA
// ---------------------------------------------------------------
//
// Per v1.0 (protocol-v1.0.md §2.3), the reference node MUST sign
// using RFC 6979 deterministic-k ECDSA with SHA-256 + low-s
// normalization. SDKs SHOULD adopt the same over time; until they
// do, verification remains lenient so non-deterministic signatures
// from other implementations still verify.
//
// Implementation is a direct translation of RFC 6979 §3.2 for
// ECDSA P-256 + SHA-256 (the only suite v1.0 uses).

// rfc6979K computes the deterministic nonce k per RFC 6979 §3.2
// for curve N = order, private scalar x, and hashed message h1.
// Output is in [1, N-1].
func rfc6979K(n, x *big.Int, h1 []byte) *big.Int {
	qlen := n.BitLen()
	hashLen := sha256.Size
	// bits2octets(h1) per §2.3.4: take leftmost qlen bits, reduce mod n.
	bits2int := func(b []byte) *big.Int {
		v := new(big.Int).SetBytes(b)
		blen := len(b) * 8
		if blen > qlen {
			v.Rsh(v, uint(blen-qlen))
		}
		return v
	}
	bits2octets := func(b []byte) []byte {
		z1 := bits2int(b)
		z2 := new(big.Int).Mod(z1, n)
		return int2octets(z2, qlen)
	}

	V := bytes1Repeated(0x01, hashLen)
	K := make([]byte, hashLen) // all zeros

	x1 := int2octets(x, qlen)
	h1o := bits2octets(h1)

	// K = HMAC(K, V || 0x00 || int2octets(x) || bits2octets(h1))
	K = hmacSum(sha256.New, K, concat(V, []byte{0x00}, x1, h1o))
	V = hmacSum(sha256.New, K, V)
	// K = HMAC(K, V || 0x01 || int2octets(x) || bits2octets(h1))
	K = hmacSum(sha256.New, K, concat(V, []byte{0x01}, x1, h1o))
	V = hmacSum(sha256.New, K, V)

	for {
		T := []byte{}
		for len(T)*8 < qlen {
			V = hmacSum(sha256.New, K, V)
			T = append(T, V...)
		}
		k := bits2int(T)
		if k.Sign() > 0 && k.Cmp(n) < 0 {
			return k
		}
		// k not in range; re-seed and retry
		K = hmacSum(sha256.New, K, concat(V, []byte{0x00}))
		V = hmacSum(sha256.New, K, V)
	}
}

// int2octets encodes an int to a big-endian byte string of
// exactly rlen bits (rounded up to bytes), per RFC 6979 §2.3.3.
func int2octets(v *big.Int, rlen int) []byte {
	out := v.Bytes()
	byteLen := (rlen + 7) / 8
	if len(out) < byteLen {
		pad := make([]byte, byteLen-len(out))
		out = append(pad, out...)
	}
	if len(out) > byteLen {
		out = out[len(out)-byteLen:]
	}
	return out
}

func bytes1Repeated(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func hmacSum(newHash func() hash.Hash, key, data []byte) []byte {
	m := hmac.New(newHash, key)
	m.Write(data)
	return m.Sum(nil)
}

func concat(parts ...[]byte) []byte {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	out := make([]byte, 0, total)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// SignRFC6979 produces a deterministic ECDSA P-256 + SHA-256
// signature per RFC 6979, with low-s normalization per BIP-62.
// Returns (r, s) where s <= n/2.
//
// The returned pair serializes to 64 bytes as r||s padded to
// 32 bytes each; that's the on-wire IEEE-1363 form.
func SignRFC6979(priv *ecdsa.PrivateKey, digest []byte) (r, s *big.Int) {
	curve := priv.Curve
	params := curve.Params()
	n := params.N

	k := rfc6979K(n, priv.D, digest)
	kInv := new(big.Int).ModInverse(k, n)

	x, _ := curve.ScalarBaseMult(k.Bytes())
	r = new(big.Int).Mod(x, n)

	// e (message hash trimmed to the curve order's bit length).
	e := new(big.Int).SetBytes(digest)
	if n.BitLen() < e.BitLen() {
		e.Rsh(e, uint(e.BitLen()-n.BitLen()))
	}

	// s = k^-1 * (e + r*d) mod n
	s = new(big.Int).Mul(r, priv.D)
	s.Add(s, e)
	s.Mul(s, kInv)
	s.Mod(s, n)

	// Low-s normalization: if s > n/2, replace with n - s.
	// Ensures a canonical, non-malleable signature.
	halfN := new(big.Int).Rsh(n, 1)
	if s.Cmp(halfN) > 0 {
		s.Sub(n, s)
	}
	return r, s
}

// GetBlockSignableData returns canonical bytes for signing a block.
// Excludes the Hash field (not set during signing) and ValidatorSigs
// (signatures must not sign themselves).
//
// Canonical form: the bytes are JSON with all object keys sorted
// alphabetically at every level of nesting. The canonicalization is
// achieved by round-tripping the typed structure through
// interface{} / map[string]interface{}, which forces every sub-value
// to alphabetical key order regardless of whether the input field
// was a typed struct (struct-declaration order) or a generic map
// (already alphabetical). This must match calculateBlockHash's
// canonicalization because the two operate on the same logical
// envelope; if they diverged, a block whose hash you trust could
// fail signature verification, or vice versa.
//
// ENG-82: without the round-trip, signing and verification diverge
// silently across the wire. A block signed locally has typed
// transactions in []interface{} (TrustTransaction, IdentityTransaction,
// etc.) which json.Marshal emits in struct-declaration order. When a
// peer receives the block over JSON HTTP, those same transactions
// arrive as map[string]interface{} and json.Marshal emits them in
// alphabetical order. The pubkey, validator ID, and signature bytes
// all match, but SHA-256 of the message diverges, so ECDSA verify
// rejects the signature. Round-tripping through generic interface{}
// here forces both paths to the same canonical bytes.
//
// NOTE for QDP-0001: Block.NonceCheckpoints is deliberately NOT
// included in the signable envelope in the foundation / Phase-0
// deployment. Including it is a hard-fork boundary and will be done
// by a coordinated network-wide switch-over (QDP-0001 §10.2). Until
// then, checkpoints are populated for local ledger bookkeeping only.
//
// Returns nil if either marshal step fails. Verification against nil
// fails by design; we'd rather refuse to verify than produce a
// silently-incorrect digest.
func GetBlockSignableData(block Block) []byte {
	// Create a copy of TrustProof without signatures for signing.
	// ValidatorSigs intentionally excluded so signatures don't sign
	// themselves.
	trustProofForSigning := TrustProof{
		TrustDomain:             block.TrustProof.TrustDomain,
		ValidatorID:             block.TrustProof.ValidatorID,
		ValidatorPublicKey:      block.TrustProof.ValidatorPublicKey,
		ValidatorTrustInCreator: block.TrustProof.ValidatorTrustInCreator,
		ConsensusData:           block.TrustProof.ConsensusData,
		ValidationTime:          block.TrustProof.ValidationTime,
	}

	// Stage 1: marshal the typed envelope.
	typed, err := json.Marshal(struct {
		Index        int64
		Timestamp    int64
		Transactions []interface{}
		TrustProof   TrustProof
		PrevHash     string
	}{
		Index:        block.Index,
		Timestamp:    block.Timestamp,
		Transactions: block.Transactions,
		TrustProof:   trustProofForSigning,
		PrevHash:     block.PrevHash,
	})
	if err != nil {
		return nil
	}

	// Stage 2: round-trip through generic interface{} / map so every
	// sub-value is normalized to alphabetical key ordering. Without
	// this, signing-time bytes (typed structs in declaration order)
	// diverge from verify-time bytes (maps in alphabetical order)
	// and signatures fail across the wire even when keys match. See
	// the function comment for ENG-82 details.
	var generic interface{}
	if err := json.Unmarshal(typed, &generic); err != nil {
		return nil
	}
	canonical, err := json.Marshal(generic)
	if err != nil {
		return nil
	}
	return canonical
}

// calculateBlockHash calculates the hash for a block.
//
// Block.Transactions is typed as []interface{}, so json.Marshal
// produces struct-declaration-order bytes for typed wrapper structs
// (TrustTransaction, AnchorTransaction, etc.) but alphabetical order
// for the map[string]interface{} values that result from a JSON
// round-trip. The hash must be stable across both shapes — otherwise
// any process that serializes, transmits, and re-hashes a block
// (cross-node block sync, QDP-0003 anchor gossip, any future replay
// diagnostic) would compute a different hash for the same logical
// block.
//
// We canonicalize by round-tripping through map[string]interface{}
// ourselves. The resulting bytes are always alphabetical-order JSON
// regardless of the input's typed shape. This is a behavior change
// vs. the pre-canonicalization impl but is internal (no on-wire
// hash format change — block hashes are stored AND verified with
// the same function).
func calculateBlockHash(block Block) string {
	blockData, _ := canonicalBlockBytes(block)
	hash := sha256.Sum256(blockData)
	return hex.EncodeToString(hash[:])
}

// canonicalBlockBytes marshals the block's hashable fields in a form
// that's stable under JSON round-tripping. See calculateBlockHash for
// the rationale.
func canonicalBlockBytes(block Block) ([]byte, error) {
	// Stage 1: marshal the typed structure.
	typed, err := json.Marshal(struct {
		Index        int64
		Timestamp    int64
		Transactions []interface{}
		TrustProof   TrustProof
		PrevHash     string
	}{
		Index:        block.Index,
		Timestamp:    block.Timestamp,
		Transactions: block.Transactions,
		TrustProof:   block.TrustProof,
		PrevHash:     block.PrevHash,
	})
	if err != nil {
		return nil, err
	}
	// Stage 2: round-trip through interface{} / map so every sub-
	// value is normalized to alphabetical key ordering. Re-marshaling
	// the resulting value produces canonical bytes.
	var generic interface{}
	if err := json.Unmarshal(typed, &generic); err != nil {
		return nil, err
	}
	return json.Marshal(generic)
}

// SignData signs data with the node's private key.
//
// v1.0 canonical form: deterministic RFC 6979 k selection +
// low-s normalization. Output is 64-byte IEEE-1363 raw
// (r||s, each zero-padded to 32 bytes). Signatures are
// bit-stable across runs for the same (key, message) pair,
// enabling reproducible test vectors and eliminating the
// class of bugs caused by weak RNGs leaking private keys via
// k-reuse.
//
// Non-RFC-6979 signers (other SDKs) still produce valid ECDSA
// signatures that this verifier accepts; determinism is a
// one-way guarantee the reference node provides, not a
// requirement on incoming signatures.
func (node *QuidnugNode) SignData(data []byte) ([]byte, error) {
	digest := sha256.Sum256(data)

	r, s := SignRFC6979(node.PrivateKey, digest[:])

	// Pad r and s to 32 bytes each for P-256 (64 bytes total).
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// Keep rand import alive for any future non-deterministic path
// (e.g., future protocol additions requiring fresh entropy).
var _ = rand.Reader

// GetPublicKeyHex returns the hex-encoded public key in uncompressed format.
// Returns an empty string when the node has no public key set (e.g. test nodes
// constructed without NewQuidnugNode).
func (node *QuidnugNode) GetPublicKeyHex() string {
	if node == nil || node.PublicKey == nil {
		return ""
	}
	publicKeyBytes := elliptic.Marshal(node.PublicKey.Curve, node.PublicKey.X, node.PublicKey.Y)
	return hex.EncodeToString(publicKeyBytes)
}

// VerifySignature verifies an ECDSA P-256 signature.
// publicKeyHex: hex-encoded public key in uncompressed format (65 bytes: 0x04 || X || Y).
// data: the data that was signed.
// signatureHex: hex-encoded signature (64 bytes: r || s, each padded to 32 bytes).
//
// This is the boolean entry point used throughout the codebase. For
// callers that need to distinguish "wrong key", "wrong signature",
// "malformed input", and "verify-rejected" (e.g. block validation
// debug paths under ENG-83), use VerifySignatureDetailed.
func VerifySignature(publicKeyHex string, data []byte, signatureHex string) bool {
	ok, _ := VerifySignatureDetailed(publicKeyHex, data, signatureHex)
	return ok
}

// VerifySignatureDetailed verifies an ECDSA P-256 signature and
// returns a non-nil error describing why verification failed when it
// does. The error categories (in order of check) are:
//
//   - missing inputs      empty pubkey or signature hex
//   - pubkey decode       hex decode of the pubkey failed
//   - pubkey unmarshal    bytes are not a valid P-256 uncompressed point
//   - signature decode    hex decode of the signature failed
//   - signature length    decoded signature is not exactly 64 bytes
//   - ecdsa rejected      signature does not validate against sha256(data)
//
// ENG-83: distinguishing "signature really does not match this data"
// from "data we hashed differently" requires knowing whether ECDSA
// verify or an earlier decode step was the failure point. Callers
// (block-validation debug logs) thread the returned error into their
// structured log so the operator can tell, from a single log line,
// whether they have a canonical-form drift or a key-vs-key mismatch.
func VerifySignatureDetailed(publicKeyHex string, data []byte, signatureHex string) (bool, error) {
	if publicKeyHex == "" {
		return false, errors.New("missing public key")
	}
	if signatureHex == "" {
		return false, errors.New("missing signature")
	}

	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return false, fmt.Errorf("public key hex decode failed: %w", err)
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), publicKeyBytes)
	if x == nil {
		return false, errors.New("public key unmarshal failed: bytes are not a valid P-256 uncompressed point")
	}

	publicKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("signature hex decode failed: %w", err)
	}

	if len(signatureBytes) != 64 {
		return false, fmt.Errorf("signature length is %d, expected 64 (IEEE-1363 r||s, each 32 bytes)", len(signatureBytes))
	}

	r := new(big.Int).SetBytes(signatureBytes[:32])
	s := new(big.Int).SetBytes(signatureBytes[32:])

	digest := sha256.Sum256(data)
	if !ecdsa.Verify(publicKey, digest[:], r, s) {
		return false, errors.New("ecdsa verify rejected: signature does not validate against sha256(data)")
	}
	return true, nil
}
