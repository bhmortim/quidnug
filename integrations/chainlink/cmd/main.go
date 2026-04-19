// Command quidnug-chainlink-adapter serves the Chainlink External
// Adapter for Quidnug relational-trust queries.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/quidnug/quidnug/integrations/chainlink"
	"github.com/quidnug/quidnug/pkg/client"
)

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
	log.Printf("quidnug-chainlink-adapter: listening on %s, node=%s", addr, node)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
