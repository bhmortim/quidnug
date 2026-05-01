// Command quidnug-cli is the operator-facing CLI for Quidnug nodes.
//
// It wraps github.com/quidnug/quidnug/pkg/client so any capability of
// the Go SDK is reachable from a shell: health checks, identity
// registration, trust grants, event emission, guardian queries,
// gossip publication, Merkle proof verification.
//
// Common invocations:
//
//	quidnug-cli health
//	quidnug-cli quid generate --out alice.quid.json
//	quidnug-cli identity register --quid alice.quid.json --name Alice
//	quidnug-cli trust grant --signer alice.quid.json --trustee $BOB_ID --level 0.9
//	quidnug-cli trust get $ALICE_ID $BOB_ID --domain contractors.home
//	quidnug-cli merkle verify --tx tx.json --proof proof.json --root $ROOT_HEX
//
// All commands honor:
//
//	--node URL      (default from QUIDNUG_NODE, else http://localhost:8080)
//	--timeout DURATION
//	--token STRING  (passed as Authorization: Bearer <token>)
//	--json           emit JSON instead of the default key-value format
//	--verbose / -v
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/quidnug/quidnug/internal/safeio"
	"github.com/quidnug/quidnug/pkg/client"
)

const version = "2.0.0"

// --- Top-level ------------------------------------------------------------

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "quidnug-cli: "+err.Error())
		var ce *client.ConflictError
		var ue *client.UnavailableError
		var ve *client.ValidationError
		var ne *client.NodeError
		switch {
		case errors.As(err, &ve):
			os.Exit(2) // usage / validation
		case errors.As(err, &ce):
			os.Exit(3) // node rejected
		case errors.As(err, &ue):
			os.Exit(4) // bootstrap / gated
		case errors.As(err, &ne):
			os.Exit(5) // transport / 5xx
		default:
			os.Exit(1)
		}
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printHelp(os.Stdout)
		return nil
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Println("quidnug-cli", version)
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "health":
		return doSimple(cmd, rest, func(c *client.Client, ctx context.Context) (any, error) { return c.Health(ctx) })
	case "info":
		return doSimple(cmd, rest, func(c *client.Client, ctx context.Context) (any, error) { return c.Info(ctx) })
	case "quid":
		return cmdQuid(rest)
	case "identity":
		return cmdIdentity(rest)
	case "trust":
		return cmdTrust(rest)
	case "title":
		return cmdTitle(rest)
	case "event":
		return cmdEvent(rest)
	case "stream":
		return cmdStream(rest)
	case "guardian":
		return cmdGuardian(rest)
	case "gossip":
		return cmdGossip(rest)
	case "bootstrap":
		return cmdBootstrap(rest)
	case "fork-block":
		return cmdForkBlock(rest)
	case "merkle":
		return cmdMerkle(rest)
	case "blocks":
		return cmdBlocks(rest)
	case "node":
		return cmdNode(rest)
	case "peer":
		return cmdPeer(rest)
	case "discover":
		return cmdDiscover(rest)
	case "well-known":
		return cmdWellKnown(rest)
	case "dns":
		return cmdDNS(rest)
	default:
		return fmt.Errorf("unknown command %q (try `quidnug-cli help`)", cmd)
	}
}

// --- Shared flag parsing --------------------------------------------------

type commonFlags struct {
	Node    string
	Timeout time.Duration
	Token   string
	JSON    bool
	Verbose bool
}

func (cf *commonFlags) register(fs *flag.FlagSet) {
	def := os.Getenv("QUIDNUG_NODE")
	if def == "" {
		def = "http://localhost:8080"
	}
	fs.StringVar(&cf.Node, "node", def, "Quidnug node base URL (env: QUIDNUG_NODE)")
	fs.DurationVar(&cf.Timeout, "timeout", 30*time.Second, "per-request timeout")
	fs.StringVar(&cf.Token, "token", os.Getenv("QUIDNUG_TOKEN"), "bearer auth token (env: QUIDNUG_TOKEN)")
	fs.BoolVar(&cf.JSON, "json", false, "emit JSON output")
	fs.BoolVar(&cf.Verbose, "verbose", false, "verbose output")
	fs.BoolVar(&cf.Verbose, "v", false, "verbose output (shorthand)")
}

func (cf *commonFlags) client() (*client.Client, error) {
	opts := []client.Option{client.WithTimeout(cf.Timeout)}
	if cf.Token != "" {
		opts = append(opts, client.WithAuthToken(cf.Token))
	}
	return client.New(cf.Node, opts...)
}

// doSimple handles commands with no arguments of their own.
func doSimple(
	name string,
	args []string,
	fn func(*client.Client, context.Context) (any, error),
) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	result, err := fn(c, ctx)
	if err != nil {
		return err
	}
	return emit(cf, result)
}

// --- quid: generate + show ------------------------------------------------

type quidFile struct {
	ID            string `json:"id"`
	PublicKeyHex  string `json:"publicKeyHex"`
	PrivateKeyHex string `json:"privateKeyHex,omitempty"`
}

func cmdQuid(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("quid: subcommand required (generate | show)")
	}
	switch args[0] {
	case "generate":
		return cmdQuidGenerate(args[1:])
	case "show":
		return cmdQuidShow(args[1:])
	default:
		return fmt.Errorf("quid: unknown subcommand %q", args[0])
	}
}

func cmdQuidGenerate(args []string) error {
	fs := flag.NewFlagSet("quid generate", flag.ContinueOnError)
	var out string
	fs.StringVar(&out, "out", "", "write JSON keypair to this path (stdout if empty)")
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "emit JSON output to stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}

	q, err := client.GenerateQuid()
	if err != nil {
		return err
	}
	qf := quidFile{ID: q.ID, PublicKeyHex: q.PublicKeyHex, PrivateKeyHex: q.PrivateKeyHex}
	b, _ := json.MarshalIndent(qf, "", "  ")

	if out == "" {
		// stdout write failure here means the parent process
		// closed the pipe; nothing useful to do besides return.
		if _, err := os.Stdout.Write(b); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		fmt.Println()
		return nil
	}
	if err := os.WriteFile(out, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	if jsonOut {
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("wrote %s (quid id=%s)\n", out, q.ID)
	return nil
}

func cmdQuidShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("quid show: path to quid file required")
	}
	qf, err := readQuidFile(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("id=%s\npublicKey=%s\nhasPrivateKey=%v\n",
		qf.ID, qf.PublicKeyHex, qf.PrivateKeyHex != "")
	return nil
}

func readQuidFile(path string) (*quidFile, error) {
	// Path comes from operator CLI flag and is treated as untrusted:
	// safeio.ReadFile rejects path traversal, NUL injection, symlinks,
	// and non-regular files before issuing the read.
	raw, err := safeio.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var qf quidFile
	if err := json.Unmarshal(raw, &qf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &qf, nil
}

func loadQuid(path string) (*client.Quid, error) {
	qf, err := readQuidFile(path)
	if err != nil {
		return nil, err
	}
	if qf.PrivateKeyHex == "" {
		return client.QuidFromPublicHex(qf.PublicKeyHex)
	}
	return client.QuidFromPrivateHex(qf.PrivateKeyHex)
}

// --- identity -------------------------------------------------------------

func cmdIdentity(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("identity: subcommand required (register | get)")
	}
	switch args[0] {
	case "register":
		return cmdIdentityRegister(args[1:])
	case "get":
		return cmdIdentityGet(args[1:])
	default:
		return fmt.Errorf("identity: unknown subcommand %q", args[0])
	}
}

func cmdIdentityRegister(args []string) error {
	fs := flag.NewFlagSet("identity register", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var quidPath, name, description, homeDomain, attrJSON, domain string
	fs.StringVar(&quidPath, "quid", "", "path to signer quid file (required)")
	fs.StringVar(&name, "name", "", "human-readable name")
	fs.StringVar(&description, "description", "", "description")
	fs.StringVar(&homeDomain, "home-domain", "", "home domain (QDP-0007)")
	fs.StringVar(&domain, "domain", "default", "trust domain")
	fs.StringVar(&attrJSON, "attributes", "", "JSON object of additional attributes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if quidPath == "" {
		return &client.ValidationError{}
	}
	signer, err := loadQuid(quidPath)
	if err != nil {
		return err
	}
	params := client.IdentityParams{
		Domain: domain, Name: name, Description: description, HomeDomain: homeDomain,
	}
	if attrJSON != "" {
		if err := json.Unmarshal([]byte(attrJSON), &params.Attributes); err != nil {
			return fmt.Errorf("--attributes: %w", err)
		}
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.RegisterIdentity(ctx, signer, params)
	if err != nil {
		return err
	}
	return emit(cf, r)
}

func cmdIdentityGet(args []string) error {
	fs := flag.NewFlagSet("identity get", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var domain string
	fs.StringVar(&domain, "domain", "", "trust domain")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("identity get: quid id required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.GetIdentity(ctx, fs.Arg(0), domain)
	if err != nil {
		return err
	}
	if r == nil {
		fmt.Println("not found")
		return nil
	}
	return emit(cf, r)
}

// --- trust ----------------------------------------------------------------

func cmdTrust(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("trust: subcommand required (grant | get | edges)")
	}
	switch args[0] {
	case "grant":
		return cmdTrustGrant(args[1:])
	case "get":
		return cmdTrustGet(args[1:])
	case "edges":
		return cmdTrustEdges(args[1:])
	default:
		return fmt.Errorf("trust: unknown subcommand %q", args[0])
	}
}

func cmdTrustGrant(args []string) error {
	fs := flag.NewFlagSet("trust grant", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var signerPath, trustee, domain, description string
	var level float64
	var nonce int64
	var validUntil int64
	fs.StringVar(&signerPath, "signer", "", "path to signer quid file (required)")
	fs.StringVar(&trustee, "trustee", "", "trustee quid id (required)")
	fs.Float64Var(&level, "level", -1, "trust level in [0,1] (required)")
	fs.StringVar(&domain, "domain", "default", "trust domain")
	fs.Int64Var(&nonce, "nonce", 1, "nonce (monotonic per truster)")
	fs.Int64Var(&validUntil, "valid-until", 0, "Unix expiry timestamp (optional)")
	fs.StringVar(&description, "description", "", "description")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if signerPath == "" || trustee == "" || level < 0 || level > 1 {
		return fmt.Errorf("--signer, --trustee, --level in [0,1] are required")
	}
	signer, err := loadQuid(signerPath)
	if err != nil {
		return err
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.GrantTrust(ctx, signer, client.TrustParams{
		Trustee: trustee, Level: level, Domain: domain,
		Nonce: nonce, ValidUntil: validUntil, Description: description,
	})
	if err != nil {
		return err
	}
	return emit(cf, r)
}

func cmdTrustGet(args []string) error {
	fs := flag.NewFlagSet("trust get", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var domain string
	var maxDepth int
	fs.StringVar(&domain, "domain", "default", "trust domain")
	fs.IntVar(&maxDepth, "max-depth", 5, "maximum path depth")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("trust get: <observer> <target> required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.GetTrust(ctx, fs.Arg(0), fs.Arg(1), domain, maxDepth)
	if err != nil {
		return err
	}
	if cf.JSON {
		return emit(cf, r)
	}
	fmt.Printf("trust_level=%.6f\npath=%s\ndepth=%d\ndomain=%s\n",
		r.TrustLevel, strings.Join(r.Path, " -> "), r.PathDepth, r.Domain)
	return nil
}

func cmdTrustEdges(args []string) error {
	fs := flag.NewFlagSet("trust edges", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("trust edges: quid id required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	edges, err := c.GetTrustEdges(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	return emit(cf, edges)
}

// --- title ----------------------------------------------------------------

func cmdTitle(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("title: subcommand required (register | get)")
	}
	switch args[0] {
	case "register":
		return cmdTitleRegister(args[1:])
	case "get":
		return cmdTitleGet(args[1:])
	default:
		return fmt.Errorf("title: unknown subcommand %q", args[0])
	}
}

func cmdTitleRegister(args []string) error {
	fs := flag.NewFlagSet("title register", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var signerPath, asset, domain, titleType, prevID, ownersJSON string
	fs.StringVar(&signerPath, "signer", "", "signer quid file (required)")
	fs.StringVar(&asset, "asset", "", "asset id (required)")
	fs.StringVar(&domain, "domain", "default", "trust domain")
	fs.StringVar(&titleType, "title-type", "", "title type discriminator")
	fs.StringVar(&prevID, "prev-title", "", "prev title tx id")
	fs.StringVar(&ownersJSON, "owners", "",
		`JSON array: [{"ownerId":"...","percentage":60.0},{"ownerId":"...","percentage":40.0}]`)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if signerPath == "" || asset == "" || ownersJSON == "" {
		return fmt.Errorf("--signer, --asset, --owners are required")
	}
	var owners []client.OwnershipStake
	if err := json.Unmarshal([]byte(ownersJSON), &owners); err != nil {
		return fmt.Errorf("--owners: %w", err)
	}
	signer, err := loadQuid(signerPath)
	if err != nil {
		return err
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.RegisterTitle(ctx, signer, client.TitleParams{
		AssetID: asset, Owners: owners, Domain: domain,
		TitleType: titleType, PrevTitleTxID: prevID,
	})
	if err != nil {
		return err
	}
	return emit(cf, r)
}

func cmdTitleGet(args []string) error {
	fs := flag.NewFlagSet("title get", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var domain string
	fs.StringVar(&domain, "domain", "", "trust domain")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("title get: asset id required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.GetTitle(ctx, fs.Arg(0), domain)
	if err != nil {
		return err
	}
	if r == nil {
		fmt.Println("not found")
		return nil
	}
	return emit(cf, r)
}

// --- event / stream --------------------------------------------------------

func cmdEvent(args []string) error {
	if len(args) == 0 || args[0] != "emit" {
		return fmt.Errorf("event: only subcommand is 'emit'")
	}
	return cmdEventEmit(args[1:])
}

func cmdEventEmit(args []string) error {
	fs := flag.NewFlagSet("event emit", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var signerPath, subjectID, subjectType, eventType, domain, payloadJSON, payloadCID string
	var sequence int64
	fs.StringVar(&signerPath, "signer", "", "signer quid file (required)")
	fs.StringVar(&subjectID, "subject-id", "", "subject quid or title id (required)")
	fs.StringVar(&subjectType, "subject-type", "QUID", "QUID or TITLE")
	fs.StringVar(&eventType, "type", "", "event type name (required)")
	fs.StringVar(&domain, "domain", "default", "trust domain")
	fs.StringVar(&payloadJSON, "payload", "", "inline JSON payload")
	fs.StringVar(&payloadCID, "payload-cid", "", "IPFS CID of payload")
	fs.Int64Var(&sequence, "sequence", 0, "0 = auto-detect")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if signerPath == "" || subjectID == "" || eventType == "" {
		return fmt.Errorf("--signer, --subject-id, --type are required")
	}
	var payload map[string]any
	if payloadJSON != "" {
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			return fmt.Errorf("--payload: %w", err)
		}
	}
	signer, err := loadQuid(signerPath)
	if err != nil {
		return err
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.EmitEvent(ctx, signer, client.EventParams{
		SubjectID: subjectID, SubjectType: subjectType, EventType: eventType,
		Domain: domain, Payload: payload, PayloadCID: payloadCID, Sequence: sequence,
	})
	if err != nil {
		return err
	}
	return emit(cf, r)
}

func cmdStream(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("stream: subcommand required (get | events)")
	}
	switch args[0] {
	case "get":
		return runStreamGet(args[1:])
	case "events":
		return runStreamEvents(args[1:])
	default:
		return fmt.Errorf("stream: unknown subcommand %q", args[0])
	}
}

func runStreamGet(args []string) error {
	fs := flag.NewFlagSet("stream get", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var domain string
	fs.StringVar(&domain, "domain", "", "trust domain")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("stream get: subject id required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.GetEventStream(ctx, fs.Arg(0), domain)
	if err != nil {
		return err
	}
	if r == nil {
		fmt.Println("not found")
		return nil
	}
	return emit(cf, r)
}

func runStreamEvents(args []string) error {
	fs := flag.NewFlagSet("stream events", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var domain string
	var limit, offset int
	fs.StringVar(&domain, "domain", "", "trust domain")
	fs.IntVar(&limit, "limit", 50, "events per page")
	fs.IntVar(&offset, "offset", 0, "page offset")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("stream events: subject id required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	events, _, err := c.GetStreamEvents(ctx, fs.Arg(0), domain, limit, offset)
	if err != nil {
		return err
	}
	return emit(cf, events)
}

// --- guardian / gossip / bootstrap / fork-block ---------------------------

func cmdGuardian(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("guardian: subcommand required (get | pending-recovery)")
	}
	fs := flag.NewFlagSet("guardian", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("guardian %s: quid id required", args[0])
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	switch args[0] {
	case "get":
		r, err := c.GetGuardianSet(ctx, fs.Arg(0))
		if err != nil {
			return err
		}
		if r == nil {
			fmt.Println("no guardian set")
			return nil
		}
		return emit(cf, r)
	case "pending-recovery":
		r, err := c.GetPendingRecovery(ctx, fs.Arg(0))
		if err != nil {
			return err
		}
		if r == nil {
			fmt.Println("no pending recovery")
			return nil
		}
		return emit(cf, r)
	default:
		return fmt.Errorf("guardian: unknown subcommand %q", args[0])
	}
}

func cmdGossip(args []string) error {
	if len(args) == 0 || args[0] != "fingerprint" {
		return fmt.Errorf("gossip: only read subcommand is 'fingerprint <domain>'")
	}
	fs := flag.NewFlagSet("gossip fingerprint", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("gossip fingerprint: domain required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.GetLatestDomainFingerprint(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	if r == nil {
		fmt.Println("no fingerprint")
		return nil
	}
	return emit(cf, r)
}

func cmdBootstrap(args []string) error {
	if len(args) == 0 || args[0] != "status" {
		return fmt.Errorf("bootstrap: only subcommand is 'status'")
	}
	return doSimple("bootstrap status", args[1:],
		func(c *client.Client, ctx context.Context) (any, error) { return c.BootstrapStatus(ctx) })
}

func cmdForkBlock(args []string) error {
	if len(args) == 0 || args[0] != "status" {
		return fmt.Errorf("fork-block: only subcommand is 'status'")
	}
	return doSimple("fork-block status", args[1:],
		func(c *client.Client, ctx context.Context) (any, error) { return c.ForkBlockStatus(ctx) })
}

// --- merkle verify --------------------------------------------------------

func cmdMerkle(args []string) error {
	if len(args) == 0 || args[0] != "verify" {
		return fmt.Errorf("merkle: only subcommand is 'verify'")
	}
	fs := flag.NewFlagSet("merkle verify", flag.ContinueOnError)
	var txPath, proofPath, root string
	fs.StringVar(&txPath, "tx", "", "path to canonical tx bytes (required)")
	fs.StringVar(&proofPath, "proof", "", `path to proof JSON [{"hash":"...","side":"right"}, ...] (required)`)
	fs.StringVar(&root, "root", "", "expected hex root (required)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if txPath == "" || proofPath == "" || root == "" {
		return fmt.Errorf("--tx, --proof, --root are required")
	}
	txBytes, err := safeio.ReadFile(txPath)
	if err != nil {
		return fmt.Errorf("read tx: %w", err)
	}
	proofRaw, err := safeio.ReadFile(proofPath)
	if err != nil {
		return fmt.Errorf("read proof: %w", err)
	}
	var frames []client.MerkleProofFrame
	if err := json.Unmarshal(proofRaw, &frames); err != nil {
		return fmt.Errorf("parse proof: %w", err)
	}
	ok, err := client.VerifyInclusionProof(txBytes, frames, root)
	if err != nil {
		return err
	}
	if ok {
		fmt.Println("PASS: proof reconstructs expected root")
		return nil
	}
	fmt.Println("FAIL: proof does not match expected root")
	os.Exit(6)
	return nil
}

// --- blocks ---------------------------------------------------------------

func cmdBlocks(args []string) error {
	fs := flag.NewFlagSet("blocks", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var limit, offset int
	fs.IntVar(&limit, "limit", 50, "blocks per page")
	fs.IntVar(&offset, "offset", 0, "page offset")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	r, err := c.GetBlocks(ctx, limit, offset)
	if err != nil {
		return err
	}
	return emit(cf, r)
}

// --- Output helpers -------------------------------------------------------

func emit(cf commonFlags, v any) error {
	if cf.JSON || !isSimpleStruct(v) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	return emitKV(v)
}

func isSimpleStruct(v any) bool {
	switch v.(type) {
	case map[string]any, *map[string]any:
		return true
	default:
		return false
	}
}

func emitKV(v any) error {
	m, ok := v.(map[string]any)
	if !ok {
		if pm, ok := v.(*map[string]any); ok {
			m = *pm
		}
	}
	if m == nil {
		_, err := fmt.Fprintf(os.Stdout, "%v\n", v)
		return err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// alphabetize for stable output
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		val := m[k]
		switch vv := val.(type) {
		case string:
			fmt.Printf("%s=%s\n", k, vv)
		case float64:
			if vv == float64(int64(vv)) {
				fmt.Printf("%s=%d\n", k, int64(vv))
			} else {
				fmt.Printf("%s=%s\n", k, strconv.FormatFloat(vv, 'f', -1, 64))
			}
		case bool:
			fmt.Printf("%s=%v\n", k, vv)
		case nil:
			fmt.Printf("%s=\n", k)
		default:
			b, _ := json.Marshal(vv)
			fmt.Printf("%s=%s\n", k, string(b))
		}
	}
	return nil
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `quidnug-cli — operator CLI for Quidnug nodes

Commands:
  health                                    Check node health
  info                                      Show node info + capabilities
  quid generate [--out FILE]                Generate a quid keypair
  quid show FILE                            Print quid details

  identity register --quid FILE [--name ...] [--home-domain ...] [--domain D]
  identity get QUID [--domain D]

  trust grant --signer FILE --trustee QUID --level N [--domain D] [--nonce N]
  trust get OBSERVER TARGET [--domain D] [--max-depth 5]
  trust edges QUID

  title register --signer FILE --asset ID --owners JSON [--title-type T]
  title get ASSET [--domain D]

  event emit --signer FILE --subject-id X --subject-type QUID|TITLE \
             --type T [--payload JSON | --payload-cid CID]
  stream get SUBJECT [--domain D]
  stream events SUBJECT [--limit 50] [--offset 0] [--domain D]

  guardian get QUID
  guardian pending-recovery QUID

  gossip fingerprint DOMAIN
  bootstrap status
  fork-block status

  merkle verify --tx FILE --proof FILE --root HEX
  blocks [--limit 50] [--offset 0]

  node advertise --signer FILE --operator-quid Q --domain D \
                 --endpoints "url|protocol|region|priority|weight,..." \
                 [--supported-domains glob,...] [--capabilities ...] \
                 --nonce N                  QDP-0014 self-advertisement
  node show --quid Q                        Fetch a signed advertisement

  peer list                                 Known peers + composite scores
  peer show NODE_QUID                       Full per-peer score breakdown + recent events
  peer add ADDR [--operator-quid Q]         Append entry to local peers_file
                 [--allow-private]
                 [--file PATH]
  peer remove ADDR [--file PATH]            Remove entry from local peers_file
  peer scan-lan [--service _quidnug._tcp]   One-shot mDNS browse of the local segment
                 [--timeout 5s] [--json]

  discover domain --domain D                Consortium + endpoint hints + block tip
  discover operator --quid Q                All nodes an operator runs
  discover quids --domain D [--sort S] [--observer Q] [--event-type T] ...
  discover trusted-quids --domain D [--min-trust F]

  well-known generate --operator-key FILE --api-gateway URL --seeds-json JSON \
                      [--domains-json JSON] [--operator-name ...] [--out FILE]

Global flags (honored everywhere):
  --node URL         (env QUIDNUG_NODE, default http://localhost:8080)
  --timeout DUR      (default 30s)
  --token STRING     (env QUIDNUG_TOKEN; sent as Authorization: Bearer …)
  --json             Emit JSON output
  --verbose / -v`)
}
