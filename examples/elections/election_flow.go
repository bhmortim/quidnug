// End-to-end election flow on Quidnug.
//
// Simulates a small election (3 candidates, 5 voters, 2 observers)
// and prints the resulting audit log.
//
//   cd examples/elections
//   go run election_flow.go
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

const domain = "election.2026.mayor.nyc"

func main() {
	ctx := context.Background()
	c, err := client.New("http://localhost:8080")
	if err != nil {
		log.Fatal(err)
	}

	// --- 1. Election authority sets up ---------------------------------
	authority, _ := client.GenerateQuid()
	mustRegister(ctx, c, authority, "NYC Board of Elections", domain)
	electionTitleID := "election-2026-mayor-nyc"
	mustRegisterTitle(ctx, c, authority, electionTitleID, "ELECTION")
	fmt.Printf("✓ Election authority %s registered\n", authority.ID)

	// --- 2. Candidates register ----------------------------------------
	candidates := make([]*client.Quid, 3)
	for i := range candidates {
		candidates[i], _ = client.GenerateQuid()
		name := fmt.Sprintf("Candidate %c", 'A'+i)
		mustRegister(ctx, c, candidates[i], name, domain)
		// Authority vouches for each candidate (= the quid represents the named person)
		if _, err := c.GrantTrust(ctx, authority, client.TrustParams{
			Trustee: candidates[i].ID, Level: 1.0, Domain: domain,
		}); err != nil {
			log.Printf("grant trust candidate: %v", err)
		}
		fmt.Printf("✓ %s (%s)\n", name, candidates[i].ID)
	}

	// --- 3. Voters register --------------------------------------------
	voters := make([]*client.Quid, 5)
	for i := range voters {
		voters[i], _ = client.GenerateQuid()
		name := fmt.Sprintf("Voter %d", i+1)
		mustRegister(ctx, c, voters[i], name, domain)
		// Authority issues voter registration trust
		if _, err := c.GrantTrust(ctx, authority, client.TrustParams{
			Trustee: voters[i].ID, Level: 1.0, Domain: domain,
		}); err != nil {
			log.Printf("grant trust voter: %v", err)
		}
	}
	fmt.Printf("✓ %d voters registered\n", len(voters))

	// --- 4. Observers register + attest --------------------------------
	observers := make([]*client.Quid, 2)
	for i := range observers {
		observers[i], _ = client.GenerateQuid()
		name := fmt.Sprintf("Observer %d", i+1)
		mustRegister(ctx, c, observers[i], name, domain)

		// Observer attests "polling opened"
		_, _ = c.EmitEvent(ctx, observers[i], client.EventParams{
			SubjectID:   electionTitleID,
			SubjectType: "TITLE",
			EventType:   "OBSERVER_ATTEST",
			Domain:      domain,
			Payload: map[string]any{
				"observation":    "polling_station_opened",
				"pollingStation": "NYC-BRK-07",
				"observedAt":     time.Now().Unix(),
			},
		})
		fmt.Printf("✓ %s attested\n", name)
	}

	// --- 5. Voters cast ballots ----------------------------------------
	tally := map[string]int{}
	for i, voter := range voters {
		choiceIdx := i % len(candidates)
		choice := candidates[choiceIdx]
		ballotPlaintext := fmt.Sprintf("voter=%s;choice=%s;nonce=%d",
			voter.ID, choice.ID, time.Now().UnixNano())
		commit := sha256.Sum256([]byte(ballotPlaintext))

		_, err := c.EmitEvent(ctx, voter, client.EventParams{
			SubjectID:   electionTitleID,
			SubjectType: "TITLE",
			EventType:   "BALLOT_CAST",
			Domain:      domain,
			Payload: map[string]any{
				"ballotCommitment": "sha256:" + hex.EncodeToString(commit[:]),
				"ballotIndex":      i + 1,
				"pollingStation":   "NYC-BRK-07",
				"castAt":           time.Now().Unix(),
			},
		})
		if err != nil {
			log.Printf("voter %d ballot: %v", i, err)
			continue
		}
		tally[choice.ID]++
		fmt.Printf("✓ Voter %d cast ballot for %s\n", i+1, choice.ID)
	}

	// --- 6. Tabulation published as signed EVENT -----------------------
	rootBytes := sha256.Sum256([]byte(fmt.Sprintf("%v", tally)))
	_, err = c.EmitEvent(ctx, authority, client.EventParams{
		SubjectID:   electionTitleID,
		SubjectType: "TITLE",
		EventType:   "FINAL_TABULATION",
		Domain:      domain,
		Payload: map[string]any{
			"candidateVotes": tally,
			"totalBallots":   len(voters),
			"merkleRoot":     "sha256:" + hex.EncodeToString(rootBytes[:]),
			"tallyMethod":    "first-past-the-post",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n✓ Final tabulation published")

	// --- 7. Public auditor walks the stream ----------------------------
	citizen, _ := client.GenerateQuid()
	mustRegister(ctx, c, citizen, "Concerned Citizen", domain)
	_, _ = c.GrantTrust(ctx, citizen, client.TrustParams{
		Trustee: authority.ID, Level: 0.9, Domain: domain,
	})

	events, _, err := c.GetStreamEvents(ctx, electionTitleID, domain, 100, 0)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n=== Audit log: %d events ===\n", len(events))
	for _, ev := range events {
		// Compute the citizen's relational trust in the event's signer.
		tr, _ := c.GetTrust(ctx, citizen.ID, ev.Creator, domain, 5)
		accept := "✓"
		if tr.TrustLevel < 0.5 {
			accept = "✗"
		}
		fmt.Printf("  %s #%d %s by %s (trust %.2f)\n",
			accept, ev.Sequence, ev.EventType, ev.Creator[:8], tr.TrustLevel)
	}

	fmt.Println("\nFinal tally (from accepted events):")
	for qid, n := range tally {
		fmt.Printf("  %s: %d\n", qid[:8], n)
	}
}

func mustRegister(ctx context.Context, c *client.Client, q *client.Quid, name, dom string) {
	if _, err := c.RegisterIdentity(ctx, q, client.IdentityParams{
		Name: name, HomeDomain: dom,
	}); err != nil {
		log.Printf("register %s: %v", name, err)
	}
}

func mustRegisterTitle(ctx context.Context, c *client.Client, signer *client.Quid, id, ttype string) {
	if _, err := c.RegisterTitle(ctx, signer, client.TitleParams{
		AssetID:   id,
		TitleType: ttype,
		Owners: []client.OwnershipStake{
			{OwnerID: signer.ID, Percentage: 100.0},
		},
		Domain: domain,
	}); err != nil {
		log.Printf("register title %s: %v", id, err)
	}
}
