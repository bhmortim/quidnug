// sign_helper.go — stdin-to-stdout signing helper.
//
// Reads JSON-lines from stdin, each line describing a tx the caller
// wants signed with a private-key hex they supply. Uses the exact
// same json.Marshal path the Go node's validators do, so the
// produced signatures are byte-compatible.
//
// Line format (JSON):
//   { "priv": "<pkcs8-hex>", "tx": { ... Go-shaped struct ... }, "kind": "IDENTITY"|"TITLE"|"TRUST"|"EVENT" }
//
// Output (one JSON line per input):
//   { "ok": true, "signed": { ... tx with .signature filled ... } }
//   { "ok": false, "error": "..." }
//
// Usage from Python:
//   p = subprocess.Popen(['./sign_helper'], stdin=PIPE, stdout=PIPE)
//   p.stdin.write(b'{"priv":"...","tx":{...},"kind":"IDENTITY"}\n')
//   p.stdin.flush()
//   resp = json.loads(p.stdout.readline())
package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/quidnug/quidnug/internal/core"
)

type request struct {
	PrivHex string          `json:"priv"`
	Kind    string          `json:"kind"`
	Tx      json.RawMessage `json:"tx"`
}

type response struct {
	OK     bool            `json:"ok"`
	Error  string          `json:"error,omitempty"`
	Signed json.RawMessage `json:"signed,omitempty"`
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			resp := handle(line)
			b, _ := json.Marshal(resp)
			_, _ = writer.Write(b)
			_, _ = writer.Write([]byte("\n"))
			_ = writer.Flush()
		}
		if err != nil {
			return
		}
	}
}

func handle(line []byte) response {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return response{Error: "parse request: " + err.Error()}
	}
	priv, err := loadPrivHex(req.PrivHex)
	if err != nil {
		return response{Error: "parse priv: " + err.Error()}
	}
	switch req.Kind {
	case "IDENTITY":
		return signIdentity(priv, req.Tx)
	case "TITLE":
		return signTitle(priv, req.Tx)
	case "TRUST":
		return signTrust(priv, req.Tx)
	case "EVENT":
		return signEvent(priv, req.Tx)
	default:
		return response{Error: "unknown kind: " + req.Kind}
	}
}

func signIdentity(priv *ecdsa.PrivateKey, raw []byte) response {
	var tx core.IdentityTransaction
	if err := json.Unmarshal(raw, &tx); err != nil {
		return response{Error: "unmarshal: " + err.Error()}
	}
	tx.PublicKey = pubKeyHex(priv)
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signHex(priv, signable)
	b, _ := json.Marshal(tx)
	return response{OK: true, Signed: b}
}

func signTitle(priv *ecdsa.PrivateKey, raw []byte) response {
	var tx core.TitleTransaction
	if err := json.Unmarshal(raw, &tx); err != nil {
		return response{Error: "unmarshal: " + err.Error()}
	}
	tx.PublicKey = pubKeyHex(priv)
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signHex(priv, signable)
	b, _ := json.Marshal(tx)
	return response{OK: true, Signed: b}
}

func signTrust(priv *ecdsa.PrivateKey, raw []byte) response {
	var tx core.TrustTransaction
	if err := json.Unmarshal(raw, &tx); err != nil {
		return response{Error: "unmarshal: " + err.Error()}
	}
	tx.PublicKey = pubKeyHex(priv)
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signHex(priv, signable)
	b, _ := json.Marshal(tx)
	return response{OK: true, Signed: b}
}

func signEvent(priv *ecdsa.PrivateKey, raw []byte) response {
	var tx core.EventTransaction
	if err := json.Unmarshal(raw, &tx); err != nil {
		return response{Error: "unmarshal: " + err.Error()}
	}
	tx.PublicKey = pubKeyHex(priv)
	tx.Signature = ""
	signable, _ := json.Marshal(tx)
	tx.Signature = signHex(priv, signable)
	b, _ := json.Marshal(tx)
	return response{OK: true, Signed: b}
}

// --- crypto helpers ---------------------------------------------------------

func loadPrivHex(h string) (*ecdsa.PrivateKey, error) {
	der, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	p, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not ECDSA")
	}
	return p, nil
}

// pubKeyHex encodes the public key the way the Go node's
// VerifySignature expects: SEC1 uncompressed `04 || X || Y`.
func pubKeyHex(priv *ecdsa.PrivateKey) string {
	raw := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck
	return hex.EncodeToString(raw)
}

// signHex produces an IEEE-1363 r||s signature (64 bytes, hex).
// The Go node's VerifySignature parses exactly this format.
func signHex(priv *ecdsa.PrivateKey, data []byte) string {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return ""
	}
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return hex.EncodeToString(sig)
}
