// QDP-0023 CLI commands for the quidnug-cli:
//
//	quidnug-cli dns claim      — initiate a DNS attestation claim against a root
//	quidnug-cli dns challenge  — fetch the challenge the root issued; emit the
//	                             TXT record + well-known file content the
//	                             owner must publish
//	quidnug-cli dns verify     — submit the challenge response to the root and
//	                             poll for attestation issuance
//	quidnug-cli dns renew      — renew an existing attestation before expiry
//	quidnug-cli dns status     — show attestation status (remaining validity,
//	                             issuing root, etc.)
//	quidnug-cli dns revoke     — revoke an attestation (owner or root-initiated)
//
// See docs/design/0023-dns-anchored-attestation.md for the
// protocol specification behind each command.
//
// NOTE: This is a scaffold. Several commands print
// "not-yet-wired" hints where they depend on node-side handlers
// that land in Phase 1 of QDP-0023 implementation. The shape
// of the CLI surface is stable; wire-up happens as the
// reference node adds the five event types + associated
// endpoints.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// printJSON is a local helper so dns commands can render
// payload dictionaries without constructing full commonFlags.
func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// --- top-level dispatch ---------------------------------------------------

func cmdDNS(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("dns: subcommand required (claim | challenge | verify | renew | status | revoke)")
	}
	switch args[0] {
	case "claim":
		return cmdDNSClaim(args[1:])
	case "challenge":
		return cmdDNSChallenge(args[1:])
	case "verify":
		return cmdDNSVerify(args[1:])
	case "renew":
		return cmdDNSRenew(args[1:])
	case "status":
		return cmdDNSStatus(args[1:])
	case "revoke":
		return cmdDNSRevoke(args[1:])
	default:
		return fmt.Errorf("dns: unknown subcommand %q", args[0])
	}
}

// --- dns claim ------------------------------------------------------------

func cmdDNSClaim(args []string) error {
	fs := flag.NewFlagSet("dns claim", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)

	var (
		domain       string
		signerPath   string
		rootQuid     string
		rootURL      string
		paymentRail  string
		paymentRef   string
		validUntilStr string
		contactEmail string
	)
	fs.StringVar(&domain, "domain", "", "DNS domain being attested (required)")
	fs.StringVar(&signerPath, "signer", "", "owner quid file (required; holds pubkey + privkey)")
	fs.StringVar(&rootQuid, "root-quid", "", "quid of the attestation root being asked (required)")
	fs.StringVar(&rootURL, "root-url", "", "root's API base URL (required; discovered via .well-known if empty)")
	fs.StringVar(&paymentRail, "payment-method", "stripe", "stripe | crypto | waiver (free tier)")
	fs.StringVar(&paymentRef, "payment-ref", "", "off-chain payment receipt id (required unless waiver tier)")
	fs.StringVar(&validUntilStr, "valid-until", "", "requested expiry (RFC3339); default now+1y")
	fs.StringVar(&contactEmail, "contact-email", "", "email for challenge-delivery fallback (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if domain == "" || signerPath == "" || rootQuid == "" {
		return errors.New("dns claim: --domain, --signer, --root-quid all required")
	}

	var validUntil time.Time
	if validUntilStr != "" {
		t, err := time.Parse(time.RFC3339, validUntilStr)
		if err != nil {
			return fmt.Errorf("--valid-until: %w", err)
		}
		validUntil = t
	} else {
		validUntil = time.Now().AddDate(1, 0, 0)
	}

	qf, err := loadQuidFile(signerPath)
	if err != nil {
		return fmt.Errorf("load owner quid: %w", err)
	}

	payload := map[string]any{
		"domain":              domain,
		"ownerQuid":           qf.ID,
		"rootQuid":            rootQuid,
		"requestedValidUntil": validUntil.UnixNano(),
		"paymentMethod":       paymentRail,
		"paymentReference":    paymentRef,
	}
	if contactEmail != "" {
		payload["contactEmail"] = contactEmail
	}

	fmt.Println("# dns claim — scaffold output")
	fmt.Printf("# domain:        %s\n", domain)
	fmt.Printf("# owner quid:    %s\n", qf.ID)
	fmt.Printf("# root quid:     %s\n", rootQuid)
	fmt.Printf("# valid-until:   %s\n", validUntil.Format(time.RFC3339))
	fmt.Println("# payload:")
	printJSON(payload)
	if rootURL == "" {
		fmt.Fprintln(os.Stderr, "note: --root-url not provided; real implementation would discover via QDP-0014 well-known")
	} else {
		fmt.Fprintln(os.Stderr, "note: scaffold does not yet POST to root; target would be:",
			strings.TrimRight(rootURL, "/")+"/api/v2/dns/claims")
	}
	fmt.Println("# next: run `quidnug-cli dns challenge --claim-id <id>` to retrieve the challenge.")
	return nil
}

// --- dns challenge --------------------------------------------------------

func cmdDNSChallenge(args []string) error {
	fs := flag.NewFlagSet("dns challenge", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)

	var (
		domain     string
		claimID    string
		rootQuid   string
		signerPath string
	)
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	fs.StringVar(&claimID, "claim-id", "", "DNS_CLAIM event id (required)")
	fs.StringVar(&rootQuid, "root-quid", "", "attestation root quid (required)")
	fs.StringVar(&signerPath, "signer", "", "owner quid file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if domain == "" || claimID == "" || rootQuid == "" || signerPath == "" {
		return errors.New("dns challenge: --domain, --claim-id, --root-quid, --signer all required")
	}

	qf, err := loadQuidFile(signerPath)
	if err != nil {
		return fmt.Errorf("load owner quid: %w", err)
	}

	// Generate a fresh 32-byte nonce. In real flow this comes
	// from the root's DNS_CHALLENGE event; scaffold generates
	// one locally so the user can see what content they'd
	// publish.
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return err
	}
	nonce := hex.EncodeToString(nonceBytes)

	signableBytes := []byte("quidnug-dns-attest-v1\n" +
		domain + "\n" +
		qf.ID + "\n" +
		nonce + "\n" +
		rootQuid + "\n" +
		claimID)

	// Scaffold: real implementation signs with the owner's
	// private key via pkg/client.Sign.
	sigHex := "(scaffold) " + hex.EncodeToString(signableBytes[:16]) + "..."
	_ = sigHex

	txtValue := fmt.Sprintf("v=1; quid=%s; nonce=%s; sig=<owner-ecdsa-sig-hex>",
		qf.ID, nonce)

	fmt.Println("# DNS TXT record to publish:")
	fmt.Printf("# _quidnug-attest.%s.\tIN\tTXT\t%q\n", domain, txtValue)
	fmt.Println()
	fmt.Println("# Well-known file to publish at:")
	fmt.Printf("#   https://%s/.well-known/quidnug-domain-attestation.txt\n", domain)
	fmt.Println("# Body (identical to TXT value):")
	fmt.Println(txtValue)
	fmt.Println()
	fmt.Println("# Once both are live + reachable, run:")
	fmt.Printf("#   quidnug-cli dns verify --domain %s --claim-id %s --signer %s\n",
		domain, claimID, signerPath)
	return nil
}

// --- dns verify -----------------------------------------------------------

func cmdDNSVerify(args []string) error {
	fs := flag.NewFlagSet("dns verify", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)

	var (
		domain    string
		claimID   string
		pollEvery time.Duration
		maxWait   time.Duration
	)
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	fs.StringVar(&claimID, "claim-id", "", "DNS_CLAIM event id (required)")
	fs.DurationVar(&pollEvery, "poll", 10*time.Second, "poll interval while waiting for attestation")
	fs.DurationVar(&maxWait, "max-wait", 10*time.Minute, "give up after this long")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if domain == "" || claimID == "" {
		return errors.New("dns verify: --domain and --claim-id required")
	}

	fmt.Printf("# waiting for attestation on %s (claim %s)\n", domain, claimID)
	fmt.Printf("# poll interval: %s; max wait: %s\n", pollEvery, maxWait)
	fmt.Fprintln(os.Stderr, "note: scaffold does not yet query node for attestation; run `quidnug-dns-verifier verify ...` locally to simulate.")
	return nil
}

// --- dns renew ------------------------------------------------------------

func cmdDNSRenew(args []string) error {
	fs := flag.NewFlagSet("dns renew", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)

	var (
		domain     string
		signerPath string
		rootQuid   string
		paymentRef string
	)
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	fs.StringVar(&signerPath, "signer", "", "owner quid file (required)")
	fs.StringVar(&rootQuid, "root-quid", "", "attestation root quid (required; pass multiple --root-quid for multi-root renew)")
	fs.StringVar(&paymentRef, "payment-ref", "", "payment receipt id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if domain == "" || signerPath == "" || rootQuid == "" {
		return errors.New("dns renew: --domain, --signer, --root-quid all required")
	}

	qf, err := loadQuidFile(signerPath)
	if err != nil {
		return err
	}
	fmt.Printf("# dns renew: owner=%s domain=%s root=%s\n", qf.ID, domain, rootQuid)
	fmt.Fprintln(os.Stderr, "note: scaffold does not yet submit renewal; node-side handler lands in QDP-0023 Phase 1.")
	return nil
}

// --- dns status -----------------------------------------------------------

func cmdDNSStatus(args []string) error {
	fs := flag.NewFlagSet("dns status", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)

	var (
		domain         string
		exitZeroValid  string
	)
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	fs.StringVar(&exitZeroValid, "exit-zero-if-valid-for", "",
		"exit 0 if current attestation remains valid for at least this duration (e.g. 90d); nonzero otherwise")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if domain == "" {
		return errors.New("dns status: --domain required")
	}

	fmt.Printf("# dns status for %s\n", domain)
	fmt.Fprintln(os.Stderr, "note: scaffold does not yet consult node for attestation state; real output lands in QDP-0023 Phase 1 CLI wire-up.")
	if exitZeroValid != "" {
		fmt.Printf("# exit-zero-if-valid-for=%s (CI-gate mode)\n", exitZeroValid)
		os.Exit(1) // scaffold: no real data to compare against
	}
	return nil
}

// --- dns revoke -----------------------------------------------------------

func cmdDNSRevoke(args []string) error {
	fs := flag.NewFlagSet("dns revoke", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)

	var (
		attestationRef string
		signerPath     string
		role           string
		reason         string
	)
	fs.StringVar(&attestationRef, "attestation-ref", "", "event id of the DNS_ATTESTATION to revoke (required)")
	fs.StringVar(&signerPath, "signer", "", "revoker quid file (required)")
	fs.StringVar(&role, "role", "owner", "revoker role: owner | root | governor-quorum")
	fs.StringVar(&reason, "reason", "", "reason code (fraud-detected | owner-request | transfer | malfeasance | ...)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if attestationRef == "" || signerPath == "" {
		return errors.New("dns revoke: --attestation-ref and --signer required")
	}
	qf, err := loadQuidFile(signerPath)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"attestationRef": attestationRef,
		"revokerQuid":    qf.ID,
		"revokerRole":    role,
		"reason":         reason,
		"revokedAt":      time.Now().UnixNano(),
	}
	fmt.Println("# dns revoke payload:")
	printJSON(payload)
	fmt.Fprintln(os.Stderr, "note: scaffold emits payload only; submission wires up in QDP-0023 Phase 1.")
	return nil
}

// --- helpers --------------------------------------------------------------

// loadQuidFile is a thin shim over readQuidFile in main.go; we
// keep a separate name so dns commands can later pick up
// additional fields (X25519 pubkey for QDP-0024 groups).
func loadQuidFile(path string) (*quidFile, error) {
	return readQuidFile(path)
}
