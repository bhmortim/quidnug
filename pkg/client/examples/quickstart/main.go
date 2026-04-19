// Quickstart: two-party trust end-to-end.
//
//	go run ./pkg/client/examples/quickstart
//
// Assumes a local Quidnug node at http://localhost:8080.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/quidnug/quidnug/pkg/client"
)

func main() {
	c, err := client.New("http://localhost:8080")
	if err != nil {
		log.Fatalf("client.New: %v", err)
	}
	ctx := context.Background()

	info, err := c.Info(ctx)
	if err != nil {
		log.Fatalf("node unreachable: %v", err)
	}
	fmt.Printf("connected to %v v%v\n", info["quidId"], info["version"])

	alice, _ := client.GenerateQuid()
	bob, _ := client.GenerateQuid()
	fmt.Printf("alice=%s bob=%s\n", alice.ID, bob.ID)

	if _, err := c.RegisterIdentity(ctx, alice, client.IdentityParams{
		Name: "Alice", HomeDomain: "demo.home",
	}); err != nil {
		log.Fatalf("register alice: %v", err)
	}
	if _, err := c.RegisterIdentity(ctx, bob, client.IdentityParams{
		Name: "Bob", HomeDomain: "demo.home",
	}); err != nil {
		log.Fatalf("register bob: %v", err)
	}

	if _, err := c.GrantTrust(ctx, alice, client.TrustParams{
		Trustee: bob.ID, Level: 0.9, Domain: "demo.home",
	}); err != nil {
		log.Fatalf("grant trust: %v", err)
	}

	tr, err := c.GetTrust(ctx, alice.ID, bob.ID, "demo.home", 5)
	if err != nil {
		log.Fatalf("get trust: %v", err)
	}
	fmt.Printf("relational trust = %.3f (depth %d)\n", tr.TrustLevel, tr.PathDepth)
}
