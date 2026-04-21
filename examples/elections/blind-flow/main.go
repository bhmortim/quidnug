// Anonymous-ballot election flow using real QDP-0021 RSA-FDH
// blind signatures (via pkg/crypto/blindrsa).
//
// This file extends the demo in election_flow.go with the
// cryptographic anonymity step the original demo skipped: the
// authority issues a blind signature on each voter's ballot
// token without learning the token itself, and the voter casts
// a ballot bearing the signature. A verifier confirms the
// ballot is authentic without being able to link it back to a
// specific voter.
//
//   cd examples/elections
//   go run election_blind_flow.go
//
// This POC is self-contained: it does not require a live
// Quidnug node. It demonstrates the blind-signature primitive
// in the shape a real election system would use it.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/quidnug/quidnug/pkg/crypto/blindrsa"
)

// ------------------------------------------------------------
// Simulated ledger: track cast ballot tokens and detect replays.
// ------------------------------------------------------------

type ledger struct {
	castTokens map[string]struct{}
	// Per-ballot, the signature and the candidate choice.
	entries []ledgerEntry
}

type ledgerEntry struct {
	token     []byte
	signature *big.Int
	candidate string
}

func newLedger() *ledger {
	return &ledger{castTokens: map[string]struct{}{}}
}

func (l *ledger) tryCast(
	authority *rsa.PublicKey, token []byte, sig *big.Int, candidate string,
) error {
	if err := blindrsa.Verify(authority, token, sig); err != nil {
		return fmt.Errorf("signature invalid: %w", err)
	}
	key := hex.EncodeToString(token)
	if _, seen := l.castTokens[key]; seen {
		return fmt.Errorf("double-vote attempt: token %s already cast", key[:16])
	}
	l.castTokens[key] = struct{}{}
	l.entries = append(l.entries, ledgerEntry{
		token: append([]byte(nil), token...), signature: sig, candidate: candidate,
	})
	return nil
}

func (l *ledger) tally() map[string]int {
	out := map[string]int{}
	for _, e := range l.entries {
		out[e.candidate]++
	}
	return out
}

// ------------------------------------------------------------
// Voter state: holds the pre-cast token and the unblinded sig.
// ------------------------------------------------------------

type voter struct {
	name      string
	token     []byte
	blindR    *big.Int
	signature *big.Int
}

func newVoter(name string) *voter {
	t := make([]byte, 32)
	if _, err := rand.Read(t); err != nil {
		log.Fatal(err)
	}
	return &voter{name: name, token: t}
}

// requestSignature does the voter side of the blind-signature flow:
//   - Encode the token via FDH to get m in [0, n).
//   - Blind m with a random r: m_blind = m * r^e mod n.
//   - Submit m_blind to the authority.
func (v *voter) requestSignature(
	authority *rsa.PublicKey,
	sendToAuthority func(blinded *big.Int) *big.Int,
) error {
	m, err := blindrsa.FDHEncode(v.token, authority.N)
	if err != nil {
		return err
	}
	blinded, r, err := blindrsa.Blind(rand.Reader, authority, m)
	if err != nil {
		return err
	}
	v.blindR = r
	sBlind := sendToAuthority(blinded)
	// Voter unblinds.
	v.signature = blindrsa.Unblind(authority, sBlind, r)
	return nil
}

func banner(msg string) {
	fmt.Println()
	fmt.Println("========================================================================")
	fmt.Println(" ", msg)
	fmt.Println("========================================================================")
}

func main() {
	banner("Step 1: Authority generates RSA-FDH blind-signature keypair")
	priv, err := blindrsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		log.Fatal(err)
	}
	fp := blindrsa.PublicKeyFingerprint(&priv.PublicKey)
	fmt.Printf("  authority public key fingerprint: %s\n", fp[:32]+"...")
	fmt.Printf("  (In production this goes on-chain as a BLIND_KEY_ATTESTATION event.)\n")

	// Authority's signing function -- in a real deployment this
	// is behind an API that also checks the voter is eligible
	// (registered, has not requested a signature before).
	signedVoters := map[string]struct{}{}
	authoritySign := func(voterID string, blinded *big.Int) *big.Int {
		if _, dup := signedVoters[voterID]; dup {
			log.Fatalf("authority: voter %s already received a ballot signature", voterID)
		}
		signedVoters[voterID] = struct{}{}
		return blindrsa.SignBlinded(priv, blinded)
	}

	banner("Step 2: Five voters each request a blind ballot signature")
	voters := []*voter{}
	for i := 1; i <= 5; i++ {
		v := newVoter(fmt.Sprintf("voter-%d", i))
		voters = append(voters, v)

		err := v.requestSignature(&priv.PublicKey, func(b *big.Int) *big.Int {
			// The authority never sees v.token -- only blinded.
			return authoritySign(v.name, b)
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  %s  token=%s...  sig=%s...\n",
			v.name, hex.EncodeToString(v.token[:8]),
			v.signature.Text(16)[:16])
	}

	banner("Step 3: Voters cast ballots on the public ledger")
	chain := newLedger()
	// Choices are made privately by the voter before casting.
	choices := []string{"candidate-A", "candidate-B", "candidate-A",
		"candidate-C", "candidate-B"}
	for i, v := range voters {
		err := chain.tryCast(&priv.PublicKey, v.token, v.signature, choices[i])
		if err != nil {
			log.Fatalf("%s ballot failed: %v", v.name, err)
		}
		fmt.Printf("  %s voted for %s -> ACCEPTED\n", v.name, choices[i])
	}

	banner("Step 4: Attacker tries to cast a ballot with a forged signature")
	fakeToken := make([]byte, 32)
	_, _ = rand.Read(fakeToken)
	// Sign with a DIFFERENT RSA key (the attacker's).
	attackerPriv, _ := blindrsa.GenerateKey(rand.Reader, 2048)
	fakeSig := blindrsa.SignBlinded(attackerPriv, new(big.Int).SetInt64(1))
	err = chain.tryCast(&priv.PublicKey, fakeToken, fakeSig, "candidate-A")
	if err != nil {
		fmt.Printf("  forged ballot REJECTED: %v\n", err)
	} else {
		log.Fatalf("forged ballot should not have been accepted")
	}

	banner("Step 5: Attacker tries to replay voter-1's ballot")
	err = chain.tryCast(&priv.PublicKey, voters[0].token, voters[0].signature,
		"candidate-C")
	if err != nil {
		fmt.Printf("  replay REJECTED: %v\n", err)
	} else {
		log.Fatalf("double-vote should not have been accepted")
	}

	banner("Step 6: Final tally")
	tally := chain.tally()
	for candidate, count := range tally {
		fmt.Printf("  %s = %d\n", candidate, count)
	}

	banner("Anonymity property check")
	fmt.Printf("  The authority signed %d blinded requests.\n", len(signedVoters))
	fmt.Printf("  The ledger has %d cast ballots.\n", len(chain.entries))
	fmt.Println("  The authority CANNOT tell which blinded request corresponds to")
	fmt.Println("  which cast ballot -- the link is the random blinding factor r,")
	fmt.Println("  which the voter never reveals. Every ledger entry is")
	fmt.Println("  authority-authorized (via signature verification) yet unlinkable")
	fmt.Println("  to the signing session.")

	banner("Done")
	_ = fp
}
