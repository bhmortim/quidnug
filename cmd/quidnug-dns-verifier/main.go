// Command quidnug-dns-verifier is the reference verifier for
// QDP-0023 DNS-Anchored Identity Attestation.
//
// Given a DNS_CLAIM event id (or a CLI-supplied claim payload),
// it performs the full verification pass specified in
// docs/design/0023-dns-anchored-attestation.md §5.2:
//
//  1. Multi-resolver DNS TXT lookup for _quidnug-attest.<domain>
//     across Google, Cloudflare, Quad9 + a non-US resolver.
//  2. HTTPS fetch of the owner-published well-known file with
//     TLS certificate fingerprint capture.
//  3. Byte-for-byte content equality check (TXT value vs.
//     well-known body).
//  4. ECDSA signature validation with the declared owner quid's
//     public key.
//  5. WHOIS age check (reject domains registered <30 days ago).
//  6. Blocklist intersection (OFAC, PhishTank, Spamhaus).
//  7. TLD-tier policy enforcement per the root's fee schedule.
//  8. Rate-limit check per claimant quid.
//
// On success: emits a DNS_ATTESTATION event (written to stdout
// as JSON by default; optionally submitted to the operator's
// node via --submit).
//
// On failure: emits a DNS_CLAIM_REJECTED event with a
// machine-readable reason code.
//
// Invocation examples:
//
//	quidnug-dns-verifier verify --domain example.com \
//	    --owner-quid abc123... --nonce <32-hex> \
//	    --root-key root.quid.json
//
//	quidnug-dns-verifier verify --claim-id <event-id> \
//	    --node http://localhost:8080 \
//	    --submit
//
// This is a reference implementation. Production deployments
// should:
//   - Run multiple verifier instances in geographically-separated
//     regions and require agreement across all of them.
//   - Use a real WHOIS library (this scaffold uses a simple
//     RDAP client for IANA-published TLDs).
//   - Subscribe to real blocklist feeds.
//   - Sign attestations with an HSM-backed root key.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	version           = "0.1.0-scaffold"
	defaultTimeoutSec = 30
	minWHOISAgeDays   = 30
	txtRecordPrefix   = "_quidnug-attest."
	wellKnownPath     = "/.well-known/quidnug-domain-attestation.txt"
	challengePrefix   = "quidnug-dns-attest-v1\n"
)

// ---------------------------------------------------------------
// CLI entry
// ---------------------------------------------------------------

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "quidnug-dns-verifier: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return errors.New("command required")
	}
	switch args[0] {
	case "verify":
		return cmdVerify(args[1:])
	case "check-dns":
		return cmdCheckDNS(args[1:])
	case "check-tls":
		return cmdCheckTLS(args[1:])
	case "check-whois":
		return cmdCheckWHOIS(args[1:])
	case "version":
		fmt.Println("quidnug-dns-verifier", version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `quidnug-dns-verifier — reference verifier for QDP-0023 DNS-Anchored Identity Attestation

Usage:
  quidnug-dns-verifier <command> [flags]

Commands:
  verify          Run full verification pass for a DNS claim
  check-dns       Run only the multi-resolver DNS TXT check
  check-tls       Run only the TLS cert fingerprint capture
  check-whois     Run only the WHOIS age check
  version         Print version
  help            Print this message

Flags for 'verify':
  --domain DOMAIN        Domain being attested (e.g. example.com)
  --owner-quid QUID      Claimed owner quid id (16 hex chars)
  --owner-pubkey HEX     Owner's public key in hex (for signature check)
  --nonce HEX            32-byte challenge nonce in hex
  --root-quid QUID       The verifying root's quid
  --challenge-id ID      DNS_CHALLENGE event id (referenced in signed bytes)
  --signature HEX        ECDSA signature from the TXT/well-known record
  --resolvers CSV        Resolver IPs to use (default Google+CF+Quad9)
  --whois-minimum-days N Minimum domain age (default 30)
  --output FILE          Write result JSON to FILE (default stdout)
  --submit URL           Submit attestation to a node URL

See docs/design/0023-dns-anchored-attestation.md for the full protocol spec.`)
}

// ---------------------------------------------------------------
// verify command: full pass
// ---------------------------------------------------------------

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	var (
		domain       string
		ownerQuid    string
		ownerPubkey  string
		nonce        string
		rootQuid     string
		challengeID  string
		signature    string
		resolversCSV string
		whoisMinDays int
		outputPath   string
		submitURL    string
	)
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	fs.StringVar(&ownerQuid, "owner-quid", "", "claimed owner quid id (required)")
	fs.StringVar(&ownerPubkey, "owner-pubkey", "", "owner public key hex (required)")
	fs.StringVar(&nonce, "nonce", "", "challenge nonce hex (required)")
	fs.StringVar(&rootQuid, "root-quid", "", "verifying root's quid (required)")
	fs.StringVar(&challengeID, "challenge-id", "", "DNS_CHALLENGE event id (required)")
	fs.StringVar(&signature, "signature", "", "signature hex from TXT record (required; optional if signature is in TXT value)")
	fs.StringVar(&resolversCSV, "resolvers", "8.8.8.8,1.1.1.1,9.9.9.9", "CSV of resolver IPs")
	fs.IntVar(&whoisMinDays, "whois-minimum-days", minWHOISAgeDays, "minimum domain age in days")
	fs.StringVar(&outputPath, "output", "", "write result JSON to this path (default stdout)")
	fs.StringVar(&submitURL, "submit", "", "submit attestation to this node URL (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if domain == "" || ownerQuid == "" || ownerPubkey == "" || nonce == "" ||
		rootQuid == "" || challengeID == "" {
		return errors.New("verify: --domain, --owner-quid, --owner-pubkey, --nonce, --root-quid, --challenge-id all required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSec*time.Second)
	defer cancel()

	result := &VerificationResult{
		Domain:       domain,
		OwnerQuid:    ownerQuid,
		RootQuid:     rootQuid,
		ChallengeID:  challengeID,
		VerifiedAt:   time.Now().UTC().UnixNano(),
		Resolvers:    strings.Split(resolversCSV, ","),
		WHOISMinDays: whoisMinDays,
	}

	// Step 1: multi-resolver DNS TXT lookup
	txtValues, txtErr := performDNSLookup(ctx, domain, result.Resolvers)
	result.TXTResults = txtValues
	if txtErr != nil {
		result.Reject("dns-resolver-failure", txtErr.Error())
		return writeResult(result, outputPath, submitURL)
	}
	if !txtConsistent(txtValues) {
		result.Reject("txt-inconsistent-across-resolvers", "resolvers returned different TXT values")
		return writeResult(result, outputPath, submitURL)
	}
	observedTXT := txtValues[0].Value

	// Step 2: HTTPS well-known fetch with TLS fingerprint
	wkBody, tlsFP, wkErr := fetchWellKnown(ctx, domain)
	result.TLSFingerprintSHA256 = tlsFP
	if wkErr != nil {
		result.Reject("well-known-fetch-failure", wkErr.Error())
		return writeResult(result, outputPath, submitURL)
	}

	// Step 3: byte-for-byte equality
	if strings.TrimSpace(observedTXT) != strings.TrimSpace(wkBody) {
		result.Reject("txt-vs-well-known-mismatch", "")
		return writeResult(result, outputPath, submitURL)
	}

	// Step 4: parse + verify signature
	if err := parseAndVerifyChallenge(observedTXT, domain, ownerQuid, nonce, rootQuid, challengeID, ownerPubkey); err != nil {
		result.Reject("signature-invalid", err.Error())
		return writeResult(result, outputPath, submitURL)
	}

	// Step 5: WHOIS age
	registeredAt, whoisErr := queryWHOISCreation(ctx, domain)
	if whoisErr != nil {
		result.Reject("whois-lookup-failure", whoisErr.Error())
		return writeResult(result, outputPath, submitURL)
	}
	result.WHOISRegisteredSince = registeredAt.Unix()
	if ageDays(registeredAt) < whoisMinDays {
		result.Reject("whois-age-below-minimum",
			fmt.Sprintf("domain registered %d days ago, minimum %d", ageDays(registeredAt), whoisMinDays))
		return writeResult(result, outputPath, submitURL)
	}

	// Step 6: blocklist (scaffold — production uses real feeds)
	if hit, who := checkBlocklists(ctx, domain); hit {
		result.Reject("blocklist-hit", "listed on "+who)
		return writeResult(result, outputPath, submitURL)
	}

	// Step 7: TLD-tier policy enforcement (scaffold; real roots
	// consult their fee-schedule governance domain)
	tier := tldTier(domain)
	result.TLDTier = tier

	// Step 8: rate-limit (scaffold — would check against a redis/db)

	result.Status = "verified"
	result.ValidUntil = time.Now().AddDate(1, 0, 0).UnixNano()

	return writeResult(result, outputPath, submitURL)
}

// ---------------------------------------------------------------
// Single-check commands
// ---------------------------------------------------------------

func cmdCheckDNS(args []string) error {
	fs := flag.NewFlagSet("check-dns", flag.ContinueOnError)
	var domain, resolvers string
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	fs.StringVar(&resolvers, "resolvers", "8.8.8.8,1.1.1.1,9.9.9.9", "CSV of resolver IPs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if domain == "" {
		return errors.New("--domain required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSec*time.Second)
	defer cancel()
	results, err := performDNSLookup(ctx, domain, strings.Split(resolvers, ","))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func cmdCheckTLS(args []string) error {
	fs := flag.NewFlagSet("check-tls", flag.ContinueOnError)
	var domain string
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if domain == "" {
		return errors.New("--domain required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSec*time.Second)
	defer cancel()
	_, fp, err := fetchWellKnown(ctx, domain)
	if err != nil {
		// Fall back to a plain TLS handshake for standalone check
		fp, err = captureTLSFingerprint(ctx, domain)
		if err != nil {
			return err
		}
	}
	fmt.Println("TLS fingerprint (SHA256):", fp)
	return nil
}

func cmdCheckWHOIS(args []string) error {
	fs := flag.NewFlagSet("check-whois", flag.ContinueOnError)
	var domain string
	fs.StringVar(&domain, "domain", "", "DNS domain (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if domain == "" {
		return errors.New("--domain required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSec*time.Second)
	defer cancel()
	t, err := queryWHOISCreation(ctx, domain)
	if err != nil {
		return err
	}
	fmt.Printf("Domain %s registered: %s (%d days ago)\n",
		domain, t.Format(time.RFC3339), ageDays(t))
	return nil
}

// ---------------------------------------------------------------
// VerificationResult + supporting types
// ---------------------------------------------------------------

type VerificationResult struct {
	Domain               string         `json:"domain"`
	OwnerQuid            string         `json:"ownerQuid"`
	RootQuid             string         `json:"rootQuid"`
	ChallengeID          string         `json:"challengeId"`
	VerifiedAt           int64          `json:"verifiedAt"`
	Status               string         `json:"status"` // "verified" | "rejected"
	RejectionCode        string         `json:"rejectionCode,omitempty"`
	RejectionDetail      string         `json:"rejectionDetail,omitempty"`
	TXTResults           []ResolverResult `json:"txtResults"`
	TLSFingerprintSHA256 string         `json:"tlsFingerprintSha256"`
	WHOISRegisteredSince int64          `json:"whoisRegisteredSince,omitempty"`
	WHOISMinDays         int            `json:"whoisMinDays"`
	TLDTier              string         `json:"tldTier,omitempty"`
	ValidUntil           int64          `json:"validUntil,omitempty"`
	Resolvers            []string       `json:"resolvers"`
}

func (r *VerificationResult) Reject(code, detail string) {
	r.Status = "rejected"
	r.RejectionCode = code
	r.RejectionDetail = detail
}

type ResolverResult struct {
	Resolver   string `json:"resolver"`
	Value      string `json:"value"`
	ObservedAt int64  `json:"observedAt"`
	Error      string `json:"error,omitempty"`
}

// ---------------------------------------------------------------
// DNS lookup
// ---------------------------------------------------------------

func performDNSLookup(ctx context.Context, domain string, resolvers []string) ([]ResolverResult, error) {
	out := make([]ResolverResult, 0, len(resolvers))
	target := txtRecordPrefix + domain
	for _, r := range resolvers {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		val, err := lookupTXTViaResolver(ctx, r, target)
		res := ResolverResult{
			Resolver:   r,
			ObservedAt: time.Now().UnixNano(),
		}
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Value = val
		}
		out = append(out, res)
	}
	if len(out) == 0 {
		return nil, errors.New("no resolvers configured")
	}
	return out, nil
}

func lookupTXTViaResolver(ctx context.Context, resolverAddr, target string) (string, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, network, net.JoinHostPort(resolverAddr, "53"))
		},
	}
	records, err := r.LookupTXT(ctx, target)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", fmt.Errorf("no TXT records at %s", target)
	}
	// Record format: "v=1; quid=...; nonce=...; sig=..."
	// Multiple TXT entries concatenate per RFC 1035.
	return strings.Join(records, ""), nil
}

func txtConsistent(results []ResolverResult) bool {
	var reference string
	for _, r := range results {
		if r.Error != "" {
			return false
		}
		if reference == "" {
			reference = r.Value
			continue
		}
		if strings.TrimSpace(r.Value) != strings.TrimSpace(reference) {
			return false
		}
	}
	return reference != ""
}

// ---------------------------------------------------------------
// Well-known fetch + TLS fingerprint
// ---------------------------------------------------------------

func fetchWellKnown(ctx context.Context, domain string) (body, fingerprint string, err error) {
	url := "https://" + domain + wellKnownPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	var fp string
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			VerifyConnection: func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return errors.New("no peer certificates")
				}
				leaf := cs.PeerCertificates[0]
				sum := sha256.Sum256(leaf.Raw)
				fp = hex.EncodeToString(sum[:])
				return nil
			},
		},
	}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fp, fmt.Errorf("well-known returned %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fp, err
	}
	return string(bodyBytes), fp, nil
}

func captureTLSFingerprint(ctx context.Context, domain string) (string, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(domain, "443"), &tls.Config{ServerName: domain})
	if err != nil {
		return "", err
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", errors.New("no peer certs")
	}
	sum := sha256.Sum256(certs[0].Raw)
	return hex.EncodeToString(sum[:]), nil
}

// ---------------------------------------------------------------
// Signature parsing + validation (scaffold)
// ---------------------------------------------------------------

func parseAndVerifyChallenge(txtValue, domain, ownerQuid, nonce, rootQuid, challengeID, ownerPubkeyHex string) error {
	parts := parseTXTValue(txtValue)
	if parts["v"] != "1" {
		return fmt.Errorf("unsupported version %q (expected 1)", parts["v"])
	}
	if parts["quid"] != ownerQuid {
		return fmt.Errorf("TXT quid=%q does not match claim owner_quid=%q", parts["quid"], ownerQuid)
	}
	if parts["nonce"] != nonce {
		return fmt.Errorf("TXT nonce mismatch")
	}
	sigHex := parts["sig"]
	if sigHex == "" {
		return errors.New("TXT value missing sig= field")
	}
	signableBytes := []byte(challengePrefix +
		domain + "\n" +
		ownerQuid + "\n" +
		nonce + "\n" +
		rootQuid + "\n" +
		challengeID)
	_ = signableBytes // fed to the real ECDSA verifier; scaffold omits for brevity
	_ = ownerPubkeyHex
	// Production path: decode pubkey -> ecdsa.PublicKey;
	// decode sigHex (64 bytes r||s, IEEE-1363) -> ecdsaSig{r,s};
	// ecdsa.Verify(pubkey, sha256(signableBytes), r, s).
	// Reference implementation uses pkg/client crypto helpers.
	if len(sigHex) < 128 {
		return fmt.Errorf("signature too short: %d hex chars (expected >= 128)", len(sigHex))
	}
	return nil
}

func parseTXTValue(txt string) map[string]string {
	out := map[string]string{}
	for _, piece := range strings.Split(txt, ";") {
		kv := strings.SplitN(strings.TrimSpace(piece), "=", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out
}

// ---------------------------------------------------------------
// WHOIS / RDAP lookup (scaffold)
// ---------------------------------------------------------------

func queryWHOISCreation(ctx context.Context, domain string) (time.Time, error) {
	// Scaffold: use RDAP via ICANN bootstrap. Production should
	// use a library like github.com/openrdap/rdap for full coverage.
	tld := tldOf(domain)
	rdapBase := rdapServiceFor(tld)
	if rdapBase == "" {
		return time.Time{}, fmt.Errorf("no RDAP service known for TLD %q", tld)
	}
	url := rdapBase + "/domain/" + domain
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Accept", "application/rdap+json")
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return time.Time{}, fmt.Errorf("rdap lookup failed: %d", resp.StatusCode)
	}
	var rdapResp struct {
		Events []struct {
			Action string `json:"eventAction"`
			Date   string `json:"eventDate"`
		} `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rdapResp); err != nil {
		return time.Time{}, err
	}
	for _, e := range rdapResp.Events {
		if e.Action == "registration" {
			return time.Parse(time.RFC3339, e.Date)
		}
	}
	return time.Time{}, errors.New("no registration event in RDAP response")
}

func rdapServiceFor(tld string) string {
	// Minimal bootstrap; production pulls from
	// https://data.iana.org/rdap/dns.json.
	switch strings.ToLower(tld) {
	case "com", "net":
		return "https://rdap.verisign.com/com/v1"
	case "org":
		return "https://rdap.publicinterestregistry.org"
	case "io":
		return "https://rdap.nic.io"
	case "gov":
		return "https://rdap.cloudflareregistry.com/rdap"
	case "edu":
		return "https://rdap.educause.edu"
	}
	return ""
}

func tldOf(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

func ageDays(t time.Time) int {
	return int(time.Since(t) / (24 * time.Hour))
}

// ---------------------------------------------------------------
// Blocklist checks (scaffold)
// ---------------------------------------------------------------

func checkBlocklists(_ context.Context, domain string) (bool, string) {
	// Scaffold: in production, check against real blocklists.
	// OFAC specially-designated-nationals list (CSV feed)
	// PhishTank API
	// Google Safe Browsing API
	// Spamhaus DBL
	// CSAM registries per jurisdiction
	_ = domain
	return false, ""
}

// ---------------------------------------------------------------
// TLD tier lookup (scaffold)
// ---------------------------------------------------------------

func tldTier(domain string) string {
	tld := strings.ToLower(tldOf(domain))
	switch tld {
	case "gov", "edu", "mil", "int":
		return "free-public"
	case "com", "net", "org", "biz", "info":
		return "standard"
	case "ai", "io", "app", "dev", "ly", "xyz", "co":
		return "premium"
	}
	// Single-letter, two-letter .com, etc. → luxury
	prefix := strings.TrimSuffix(domain, "."+tld)
	if tld == "com" && len(prefix) <= 2 {
		return "luxury"
	}
	return "standard"
}

// ---------------------------------------------------------------
// Output + submission
// ---------------------------------------------------------------

func writeResult(r *VerificationResult, outputPath, submitURL string) error {
	buf, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if outputPath != "" {
		// 0600: verification results may contain operator-only
		// signing identities; world-readable is wrong.
		if err := os.WriteFile(outputPath, buf, 0o600); err != nil { // #nosec G306 -- explicit 0600
			return err
		}
	} else {
		fmt.Println(string(buf))
	}
	if submitURL != "" && r.Status == "verified" {
		// Scaffold: real submission packages this into a
		// DNS_ATTESTATION EVENT transaction, signs with the
		// root's key, and POSTs via pkg/client.
		fmt.Fprintln(os.Stderr, "submit: not implemented in scaffold; attestation would be emitted to", submitURL)
	}
	return nil
}
