// Example: cross-border credit-transfer flow.
//
// Models the lifecycle of a single international payment from
// initiation through settlement and status reporting, recording each
// signed ISO 20022 message as a tamper-evident event on the same
// Quidnug Title.
//
//   pain.001 → originator bank
//   pacs.008 → settlement bank
//   pacs.002 → beneficiary bank status
//
//   go run ./integrations/iso20022/examples/cross_border_payment
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/quidnug/quidnug/integrations/iso20022"
	"github.com/quidnug/quidnug/pkg/client"
)

func main() {
	ctx := context.Background()

	c, err := client.New("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	rec, err := iso20022.New(iso20022.Options{
		Client: c,
		Domain: "bank.settlement",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Bank quids (in production: loaded from HSM via pkg/signer/hsm)
	originator, _ := client.GenerateQuid()
	settlement, _ := client.GenerateQuid()
	beneficiary, _ := client.GenerateQuid()

	for _, q := range []*client.Quid{originator, settlement, beneficiary} {
		if _, err := c.RegisterIdentity(ctx, q, client.IdentityParams{
			HomeDomain: "bank.settlement",
		}); err != nil {
			log.Printf("identity register: %v", err)
		}
	}

	// One Quidnug Title per logical payment. Its asset_id doubles as
	// the end-to-end payment reference.
	paymentRef := "E2E-REF-" + fmt.Sprint(time.Now().Unix())
	commonHeader := iso20022.Header{
		MessageID: paymentRef,
		Amount:    50_000.00,
		Currency:  "EUR",
		CreatedAt: time.Now().Unix(),
		Reference: paymentRef,
	}

	// 1. Originator initiates
	if _, err := rec.RecordMessage(ctx, originator, paymentRef, iso20022.Message{
		Family: "pain", Variant: "pain.001.001.09", Role: iso20022.RoleInitiation,
		Raw: []byte("<Document><!-- pain.001 XML --></Document>"),
		ParsedHeader: withFromTo(commonHeader, "CUSTA1X", "BANKAUSXXX"),
	}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ pain.001 recorded")

	// 2. Settlement bank issues pacs.008
	if _, err := rec.RecordMessage(ctx, settlement, paymentRef, iso20022.Message{
		Family: "pacs", Variant: "pacs.008.001.08", Role: iso20022.RoleSettlement,
		Raw: []byte("<Document><!-- pacs.008 XML --></Document>"),
		ParsedHeader: withFromTo(commonHeader, "BANKAUSXXX", "BANKBEBBXXX"),
	}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ pacs.008 recorded")

	// 3. Beneficiary bank confirms
	if _, err := rec.RecordMessage(ctx, beneficiary, paymentRef, iso20022.Message{
		Family: "pacs", Variant: "pacs.002.001.10", Role: iso20022.RoleStatus,
		Raw: []byte("<Document><!-- pacs.002 XML --></Document>"),
		ParsedHeader: withFromTo(commonHeader, "BANKBEBBXXX", "BANKAUSXXX"),
	}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ pacs.002 recorded")

	// Read the full chain back
	events, _, err := c.GetStreamEvents(ctx, paymentRef, "bank.settlement", 100, 0)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\npayment timeline (%d events):\n", len(events))
	for _, e := range events {
		fmt.Printf("  #%d %s by %s\n", e.Sequence, e.EventType, e.Creator)
	}
}

func withFromTo(h iso20022.Header, from, to string) iso20022.Header {
	h.From = from
	h.To = to
	return h
}
