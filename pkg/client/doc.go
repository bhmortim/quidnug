// Package client is the official Go client SDK for Quidnug.
//
// It provides a strongly-typed HTTP interface to every node endpoint
// (identity, trust, titles, events, anchors, guardians, gossip,
// snapshots, fork-block, Merkle proofs — QDPs 0001–0010).
//
// External Go programs consume this package via:
//
//	import "github.com/quidnug/quidnug/pkg/client"
//
// They must NOT reach into internal/core. All wire types consumers
// need are re-declared here as exported structs with identical JSON
// shapes — the client converts at the HTTP boundary.
//
// # Quick start
//
//	c, err := client.New("http://localhost:8080")
//	if err != nil { log.Fatal(err) }
//
//	alice, _ := client.GenerateQuid()
//	bob,   _ := client.GenerateQuid()
//
//	_ = c.RegisterIdentity(ctx, alice, client.IdentityParams{Name: "Alice"})
//	_ = c.RegisterIdentity(ctx, bob,   client.IdentityParams{Name: "Bob"})
//	_ = c.GrantTrust(ctx, alice, client.TrustParams{
//		Trustee: bob.ID,
//		Level:   0.9,
//		Domain:  "contractors.home",
//	})
//
//	tr, _ := c.GetTrust(ctx, alice.ID, bob.ID, "contractors.home", 5)
//	fmt.Printf("%.3f via %v\n", tr.TrustLevel, tr.Path)
//
// # Error taxonomy
//
//   - ValidationError — local precondition failures.
//   - ConflictError   — nonce replay, quorum not met, etc. (409 / select 400s).
//   - UnavailableError — 503 / feature-not-active.
//   - NodeError       — everything else: transport, 5xx, unexpected shape.
//
// All inherit from Error, so errors.As(err, new(client.Error)) catches
// anything SDK-sourced.
//
// # Retries
//
// By default, GETs retry up to 3 times on 5xx/429 with exponential
// backoff and ±100ms jitter. POSTs are not retried — callers should
// reconcile via a follow-up GET before resubmitting a write.
//
// See examples/ for end-to-end workflows.
package client
