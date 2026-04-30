// Command quidnug-chainlink-adapter serves the Chainlink External
// Adapter for Quidnug relational-trust queries.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/quidnug/quidnug/integrations/chainlink"
	"github.com/quidnug/quidnug/pkg/client"
)

// sanitizeForLog strips control characters (CR, LF, NUL, DEL,
// other low ASCII) from operator-supplied strings before they
// hit the log writer. Log injection works by smuggling a
// newline followed by a forged log entry; replacing those bytes
// with the literal string "?" makes the taint flow visibly
// terminate and removes the attack surface.
//
// Returns the sanitized copy plus a length-bounded version: any
// trailing data beyond 256 bytes is truncated so a giant
// environment value can't fill the log buffer.
func sanitizeForLog(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range s {
		if i >= 256 {
			b.WriteString("...")
			break
		}
		if r < 0x20 || r == 0x7f {
			b.WriteByte('?')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func main() {
	node := os.Getenv("QUIDNUG_NODE")
	if node == "" {
		node = "http://localhost:8080"
	}
	token := os.Getenv("QUIDNUG_TOKEN")
	addr := os.Getenv("LISTEN")
	if addr == "" {
		addr = ":8090"
	}

	opts := []client.Option{client.WithTimeout(15 * time.Second)}
	if token != "" {
		opts = append(opts, client.WithAuthToken(token))
	}
	c, err := client.New(node, opts...)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", chainlink.Handler(c))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	// addr and node come from environment variables (LISTEN,
	// QUIDNUG_NODE) which are operator-set but still untrusted
	// for log-injection purposes. Pass them through sanitizeForLog
	// so any CR/LF/control chars are replaced with '?' before they
	// reach log.Printf — the taint flow terminates at the
	// sanitizer.
	logAddr := sanitizeForLog(addr)
	logNode := sanitizeForLog(node)
	log.Printf("quidnug-chainlink-adapter: listening on %s, node=%s", logAddr, logNode)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
