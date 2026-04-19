// Go cross-SDK interop vector producer.
//
// Usage:
//
//	go run ./tests/interop/produce_go.go > tests/interop/vectors-go.json
//
// Emits the same four canonical test cases as the Python/other
// producers, signed with a stable keypair persisted at
// tests/interop/.keypair-go on first run.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/quidnug/quidnug/pkg/client"
)

const keypairFile = "tests/interop/.keypair-go"

func deterministicQuid() (*client.Quid, error) {
	abs, err := filepath.Abs(keypairFile)
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(abs); err == nil {
		return client.QuidFromPrivateHex(string(data))
	}
	q, err := client.GenerateQuid()
	if err != nil {
		return nil, err
	}
	_ = os.WriteFile(abs, []byte(q.PrivateKeyHex), 0o600)
	return q, nil
}

type vectorCase struct {
	Name              string                 `json:"name"`
	Tx                map[string]any         `json:"tx"`
	ExcludeFields     []string               `json:"excludeFields"`
	CanonicalBytesHex string                 `json:"canonicalBytesHex"`
	SignatureHex      string                 `json:"signatureHex"`
}

type vectors struct {
	SDK     string         `json:"sdk"`
	Version string         `json:"version"`
	Quid    map[string]any `json:"quid"`
	Cases   []vectorCase   `json:"cases"`
}

func makeCase(name string, tx map[string]any, q *client.Quid, exclude []string) (vectorCase, error) {
	cb, err := client.CanonicalBytes(tx, exclude...)
	if err != nil {
		return vectorCase{}, err
	}
	sig, err := q.Sign(cb)
	if err != nil {
		return vectorCase{}, err
	}
	return vectorCase{
		Name:              name,
		Tx:                tx,
		ExcludeFields:     exclude,
		CanonicalBytesHex: hex.EncodeToString(cb),
		SignatureHex:      sig,
	}, nil
}

func main() {
	q, err := deterministicQuid()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var cases []vectorCase

	// Case 1 — simple trust
	trust := map[string]any{
		"type":        "TRUST",
		"timestamp":   int64(1_700_000_000),
		"trustDomain": "interop.test",
		"signerQuid":  q.ID,
		"truster":     q.ID,
		"trustee":     "abc0123456789def",
		"trustLevel":  0.9,
		"nonce":       int64(1),
	}
	if c, err := makeCase("trust-basic", trust, q, []string{"signature", "txId"}); err == nil {
		cases = append(cases, c)
	}

	// Case 2 — identity with attributes
	ident := map[string]any{
		"type":          "IDENTITY",
		"timestamp":     int64(1_700_000_000),
		"trustDomain":   "interop.test",
		"signerQuid":    q.ID,
		"definerQuid":   q.ID,
		"subjectQuid":   q.ID,
		"updateNonce":   int64(1),
		"schemaVersion": "1.0",
		"name":          "Alice",
		"homeDomain":    "interop.home",
		"attributes":    map[string]any{"role": "admin", "tier": 3},
	}
	if c, err := makeCase("identity-with-attrs", ident, q, []string{"signature", "txId"}); err == nil {
		cases = append(cases, c)
	}

	// Case 3 — event with unicode + nested
	evtU := map[string]any{
		"type":        "EVENT",
		"timestamp":   int64(1_700_000_000),
		"trustDomain": "interop.test",
		"subjectId":   q.ID,
		"subjectType": "QUID",
		"eventType":   "NOTE",
		"sequence":    int64(1),
		"payload": map[string]any{
			"message": "hello 世界 🌍",
			"nested": map[string]any{
				"z": 1,
				"a": "x",
				"m": []any{1, 2, 3},
			},
		},
	}
	if c, err := makeCase("event-unicode-nested", evtU, q, []string{"signature", "txId", "publicKey"}); err == nil {
		cases = append(cases, c)
	}

	// Case 4 — numerical edges
	evtN := map[string]any{
		"type":        "EVENT",
		"timestamp":   int64(1_700_000_000),
		"trustDomain": "interop.test",
		"subjectId":   q.ID,
		"subjectType": "QUID",
		"eventType":   "MEASURE",
		"sequence":    int64(42),
		"payload": map[string]any{
			"count":  0,
			"weight": 0.5,
			"n64":    int64(9_007_199_254_740_991),
		},
	}
	if c, err := makeCase("numerical-edge", evtN, q, []string{"signature", "txId", "publicKey"}); err == nil {
		cases = append(cases, c)
	}

	out := vectors{
		SDK:     "go",
		Version: "2.0.0",
		Quid: map[string]any{
			"id":           q.ID,
			"publicKeyHex": q.PublicKeyHex,
		},
		Cases: cases,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
