// Command quidnug-oidc runs the OIDC → Quidnug bridge as a standalone
// HTTP service.
//
// It exposes:
//
//	POST /resolve            { issuer, subject, email, name }  → { quidId, bound: bool }
//	GET  /binding/{quidId}                                     → Binding record
//	GET  /healthz
//
// The real production path layers this on top of coreos/go-oidc:
//
//	1. Receive the OIDC auth-code callback at /callback.
//	2. Exchange for an ID token via the standard oauth2 flow.
//	3. Verify the token with go-oidc's Verifier.
//	4. Call /resolve with the verified claims.
//	5. Mint a session cookie or JWT that references the resolved quid.
//
// For now this scaffold demonstrates the Bridge wiring with an
// in-memory binding store and a stub signer factory.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/quidnug/quidnug/internal/oidc"
)

func main() {
	addr := flag.String("listen", ":8089", "HTTP listen address")
	domain := flag.String("domain", "default", "default Quidnug trust domain for new bindings")
	flag.Parse()

	store := oidc.NewMemoryBindingStore()
	fac := &inProcFactory{
		keys: make(map[string]*ecdsa.PrivateKey),
	}
	bridge, err := oidc.New(oidc.Options{
		Store:         store,
		Signer:        fac,
		DefaultDomain: *domain,
	})
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/resolve", handleResolve(bridge))
	mux.HandleFunc("/binding/", handleBinding(store))

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("quidnug-oidc listening on %s (domain=%s)", *addr, *domain)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

type resolveRequest struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

type resolveResponse struct {
	QuidID  string `json:"quidId"`
	Bound   bool   `json:"bound"`
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

func handleResolve(br *oidc.Bridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req resolveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		bnd, _, err := br.Resolve(r.Context(), oidc.IDToken{
			Issuer:  req.Issuer,
			Subject: req.Subject,
			Email:   req.Email,
			Name:    req.Name,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resolveResponse{
			QuidID:  bnd.QuidID,
			Bound:   true,
			Issuer:  bnd.Issuer,
			Subject: bnd.Subject,
		})
	}
}

func handleBinding(store oidc.BindingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		quidID := r.URL.Path[len("/binding/"):]
		bnd, err := store.GetByQuid(quidID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if bnd == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(bnd)
	}
}

// --- inProcFactory — dev-only signer. Replace with HSM in production. ---

type inProcFactory struct {
	mu   sync.Mutex
	keys map[string]*ecdsa.PrivateKey
}

type inProcQuid struct {
	id  string
	pub string
	sk  *ecdsa.PrivateKey
}

func (q *inProcQuid) ID() string          { return q.id }
func (q *inProcQuid) PublicKeyHex() string { return q.pub }
func (q *inProcQuid) Sign(data []byte) (string, error) {
	digest := sha256.Sum256(data)
	der, err := ecdsa.SignASN1(rand.Reader, q.sk, digest[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(der), nil
}

func (f *inProcFactory) CreateNew(_ context.Context, _, _, _ string) (oidc.BoundQuid, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	pubBytes := elliptic.Marshal(elliptic.P256(), sk.PublicKey.X, sk.PublicKey.Y) //nolint:staticcheck
	sum := sha256.Sum256(pubBytes)
	q := &inProcQuid{
		id:  hex.EncodeToString(sum[:8]),
		pub: hex.EncodeToString(pubBytes),
		sk:  sk,
	}
	f.keys[q.id] = sk
	return q, nil
}

func (f *inProcFactory) Get(_ context.Context, quidID string) (oidc.BoundQuid, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sk, ok := f.keys[quidID]
	if !ok {
		return nil, fmt.Errorf("quid not found: %s", quidID)
	}
	pubBytes := elliptic.Marshal(elliptic.P256(), sk.PublicKey.X, sk.PublicKey.Y) //nolint:staticcheck
	sum := sha256.Sum256(pubBytes)
	return &inProcQuid{
		id:  hex.EncodeToString(sum[:8]),
		pub: hex.EncodeToString(pubBytes),
		sk:  sk,
	}, nil
}

var _ = x509.MarshalPKCS8PrivateKey // keep imports used if the example extends
var _ os.File
