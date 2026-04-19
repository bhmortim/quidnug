// Benchmarks for the hot paths in the Go SDK.
//
//   cd tests/benchmarks
//   go test -bench=. -benchmem -run=^$
//
// These numbers are the "floor" performance — an HTTP-free
// measurement of the crypto + canonical-bytes work every
// transaction does. Add a real node round-trip and the network is
// dominant.
package benchmarks

import (
	"testing"

	"github.com/quidnug/quidnug/pkg/client"
)

func BenchmarkQuidGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = client.GenerateQuid()
	}
}

func BenchmarkCanonicalBytesSmall(b *testing.B) {
	tx := map[string]any{
		"type":        "TRUST",
		"timestamp":   int64(1_700_000_000),
		"trustDomain": "bench",
		"signerQuid":  "aaaa1111bbbb2222",
		"truster":     "aaaa1111bbbb2222",
		"trustee":     "cccc3333dddd4444",
		"trustLevel":  0.9,
		"nonce":       int64(1),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = client.CanonicalBytes(tx, "signature", "txId")
	}
}

func BenchmarkCanonicalBytesNested(b *testing.B) {
	tx := map[string]any{
		"type":      "EVENT",
		"timestamp": int64(1_700_000_000),
		"payload": map[string]any{
			"model":    "claude-3.5",
			"tokensIn": 1024,
			"nested": map[string]any{
				"a": 1, "b": 2, "c": 3, "d": map[string]any{"x": "y"},
			},
			"list": []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = client.CanonicalBytes(tx, "signature")
	}
}

func BenchmarkSign(b *testing.B) {
	q, _ := client.GenerateQuid()
	data := []byte(`{"type":"TRUST","trustee":"bob","level":0.9}`)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = q.Sign(data)
	}
}

func BenchmarkVerify(b *testing.B) {
	q, _ := client.GenerateQuid()
	data := []byte(`{"type":"TRUST","trustee":"bob","level":0.9}`)
	sig, _ := q.Sign(data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.Verify(data, sig)
	}
}

func BenchmarkSignWithCanonicalize(b *testing.B) {
	q, _ := client.GenerateQuid()
	tx := map[string]any{
		"type":        "TRUST",
		"timestamp":   int64(1_700_000_000),
		"trustDomain": "bench",
		"signerQuid":  q.ID,
		"truster":     q.ID,
		"trustee":     "cccc3333dddd4444",
		"trustLevel":  0.9,
		"nonce":       int64(1),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cb, _ := client.CanonicalBytes(tx, "signature", "txId")
		_, _ = q.Sign(cb)
	}
}
