// QDP-0014 CLI commands for the quidnug-cli:
//
//   quidnug-cli node advertise     — sign + submit a NODE_ADVERTISEMENT tx
//   quidnug-cli node show          — fetch an advertisement from a node
//   quidnug-cli discover domain    — list consortium + endpoint hints
//   quidnug-cli discover quids     — query the per-domain quid index
//   quidnug-cli discover trusted-quids — consortium-blessed quids only
//   quidnug-cli well-known generate — emit a signed quidnug-network.json
//
// See docs/design/0014-node-discovery-and-sharding.md for the
// protocol specification behind each command.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

// --- node (advertise + show) ----------------------------------------------

func cmdNode(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("node: subcommand required (advertise | show)")
	}
	switch args[0] {
	case "advertise":
		return cmdNodeAdvertise(args[1:])
	case "show":
		return cmdNodeShow(args[1:])
	default:
		return fmt.Errorf("node: unknown subcommand %q (expected advertise | show)", args[0])
	}
}

func cmdNodeAdvertise(args []string) error {
	fs := flag.NewFlagSet("node advertise", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)

	var (
		signerPath       string
		operatorQuid     string
		domain           string
		endpointsRaw     string
		supportedRaw     string
		capabilitiesRaw  string
		protocolVersion  string
		ttlString        string
		advertNonce      int64
	)
	fs.StringVar(&signerPath, "signer", "", "node quid file (required)")
	fs.StringVar(&operatorQuid, "operator-quid", "", "operator quid attesting this node (required)")
	fs.StringVar(&domain, "domain", "", "trust domain (required, typically operators.network.<your-domain>)")
	fs.StringVar(&endpointsRaw, "endpoints", "",
		"comma-separated endpoints; each 'url|protocol|region|priority|weight' (required)")
	fs.StringVar(&supportedRaw, "supported-domains", "",
		"comma-separated domain glob patterns served by this node (optional)")
	fs.StringVar(&capabilitiesRaw, "capabilities", "cache",
		"comma-separated caps: validator,cache,archive,bootstrap,gossipSink,ipfsGateway")
	fs.StringVar(&protocolVersion, "protocol-version", "1.0", "semver-ish")
	fs.StringVar(&ttlString, "ttl", "6h", "how long this advertisement stays valid; max 168h (7d)")
	fs.Int64Var(&advertNonce, "nonce", 0, "strictly monotonic per-node (required, > previous)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if signerPath == "" {
		return fmt.Errorf("--signer is required")
	}
	if operatorQuid == "" {
		return fmt.Errorf("--operator-quid is required")
	}
	if domain == "" {
		return fmt.Errorf("--domain is required")
	}
	if endpointsRaw == "" {
		return fmt.Errorf("--endpoints is required (comma-separated 'url|protocol|region|priority|weight')")
	}
	if advertNonce <= 0 {
		return fmt.Errorf("--nonce is required and must be > 0; publish a fresh nonce per advertisement")
	}

	endpoints, err := parseEndpointsArg(endpointsRaw)
	if err != nil {
		return err
	}
	var supported []string
	if supportedRaw != "" {
		for _, s := range strings.Split(supportedRaw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				supported = append(supported, s)
			}
		}
	}

	caps, err := parseCapabilitiesArg(capabilitiesRaw)
	if err != nil {
		return err
	}

	ttl, err := time.ParseDuration(ttlString)
	if err != nil {
		return fmt.Errorf("--ttl: %w", err)
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

	out, err := c.PublishNodeAdvertisement(ctx, signer, client.NodeAdvertisementParams{
		OperatorQuid:       operatorQuid,
		Domain:             domain,
		Endpoints:          endpoints,
		SupportedDomains:   supported,
		Capabilities:       caps,
		ProtocolVersion:    protocolVersion,
		TTL:                ttl,
		AdvertisementNonce: advertNonce,
	})
	if err != nil {
		return err
	}
	return emit(cf, out)
}

func cmdNodeShow(args []string) error {
	fs := flag.NewFlagSet("node show", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var quid string
	fs.StringVar(&quid, "quid", "", "node quid to fetch (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if quid == "" {
		return fmt.Errorf("--quid is required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	out, err := c.DiscoverNode(ctx, quid)
	if err != nil {
		return err
	}
	return emit(cf, out)
}

// --- discover (domain | quids | trusted-quids) ----------------------------

func cmdDiscover(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("discover: subcommand required (domain | operator | quids | trusted-quids)")
	}
	switch args[0] {
	case "domain":
		return cmdDiscoverDomain(args[1:])
	case "operator":
		return cmdDiscoverOperator(args[1:])
	case "quids":
		return cmdDiscoverQuids(args[1:])
	case "trusted-quids":
		return cmdDiscoverTrustedQuids(args[1:])
	default:
		return fmt.Errorf("discover: unknown subcommand %q", args[0])
	}
}

func cmdDiscoverDomain(args []string) error {
	fs := flag.NewFlagSet("discover domain", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var domain string
	fs.StringVar(&domain, "domain", "", "domain name to query (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if domain == "" {
		return fmt.Errorf("--domain is required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	out, err := c.DiscoverDomain(ctx, domain)
	if err != nil {
		return err
	}
	return emit(cf, out)
}

func cmdDiscoverOperator(args []string) error {
	fs := flag.NewFlagSet("discover operator", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var quid string
	fs.StringVar(&quid, "quid", "", "operator quid to query (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if quid == "" {
		return fmt.Errorf("--quid is required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	out, err := c.DiscoverOperator(ctx, quid)
	if err != nil {
		return err
	}
	return emit(cf, out)
}

func cmdDiscoverQuids(args []string) error {
	fs := flag.NewFlagSet("discover quids", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var (
		domain         string
		sortMode       string
		observer       string
		eventType      string
		minTrustWeight float64
		since          int64
		limit          int
		offset         int
		excludedRaw    string
	)
	fs.StringVar(&domain, "domain", "", "domain to query (required)")
	fs.StringVar(&sortMode, "sort", "", "activity | last-seen | first-seen | trust-weight")
	fs.StringVar(&observer, "observer", "",
		"quid for trust-weight sort + populated trustWeight in output")
	fs.StringVar(&eventType, "event-type", "", "filter to signers of this event type")
	fs.Float64Var(&minTrustWeight, "min-trust-weight", 0, "requires --observer")
	fs.Int64Var(&since, "since", 0, "UnixNano filter; only quids last-seen >= this")
	fs.IntVar(&limit, "limit", 50, "max 500")
	fs.IntVar(&offset, "offset", 0, "pagination")
	fs.StringVar(&excludedRaw, "exclude-quid", "", "comma-separated quids to omit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if domain == "" {
		return fmt.Errorf("--domain is required")
	}
	p := client.DiscoverQuidsParams{
		Domain:         domain,
		Since:          since,
		Sort:           sortMode,
		Observer:       observer,
		EventType:      eventType,
		MinTrustWeight: minTrustWeight,
		Limit:          limit,
		Offset:         offset,
	}
	if excludedRaw != "" {
		for _, s := range strings.Split(excludedRaw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				p.ExcludeQuids = append(p.ExcludeQuids, s)
			}
		}
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	out, err := c.DiscoverQuids(ctx, p)
	if err != nil {
		return err
	}
	return emit(cf, out)
}

func cmdDiscoverTrustedQuids(args []string) error {
	fs := flag.NewFlagSet("discover trusted-quids", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	var (
		domain   string
		minTrust float64
		limit    int
	)
	fs.StringVar(&domain, "domain", "", "domain to query (required)")
	fs.Float64Var(&minTrust, "min-trust", 0.5, "minimum trust level in [0, 1]")
	fs.IntVar(&limit, "limit", 200, "max 1000")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if domain == "" {
		return fmt.Errorf("--domain is required")
	}
	c, err := cf.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.Timeout+5*time.Second)
	defer cancel()
	out, err := c.DiscoverTrustedQuids(ctx, domain, minTrust, limit)
	if err != nil {
		return err
	}
	return emit(cf, out)
}

// --- well-known generator -------------------------------------------------

// WellKnownDoc mirrors schemas/json/quidnug-network.schema.json
// and is signed with an operator key before publication.
type WellKnownDoc struct {
	Version             int                    `json:"version"`
	Operator            WellKnownOperator      `json:"operator"`
	APIGateway          string                 `json:"apiGateway"`
	Seeds               []WellKnownSeed        `json:"seeds"`
	Domains             []WellKnownDomain      `json:"domains,omitempty"`
	Governance          map[string]interface{} `json:"governance,omitempty"`
	FederationAvailable bool                   `json:"federationAvailable,omitempty"`
	LastUpdated         int64                  `json:"lastUpdated"`
	Signature           string                 `json:"signature"`
}

type WellKnownOperator struct {
	Quid      string `json:"quid"`
	Name      string `json:"name,omitempty"`
	PublicKey string `json:"publicKey"`
	Contact   string `json:"contact,omitempty"`
}

type WellKnownSeed struct {
	NodeQuid     string   `json:"nodeQuid"`
	URL          string   `json:"url"`
	Region       string   `json:"region,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type WellKnownDomain struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Tree        string `json:"tree,omitempty"`
}

func cmdWellKnown(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("well-known: subcommand required (generate)")
	}
	switch args[0] {
	case "generate":
		return cmdWellKnownGenerate(args[1:])
	default:
		return fmt.Errorf("well-known: unknown subcommand %q", args[0])
	}
}

func cmdWellKnownGenerate(args []string) error {
	fs := flag.NewFlagSet("well-known generate", flag.ContinueOnError)
	var (
		operatorKeyPath string
		apiGateway      string
		seedsJSON       string
		domainsJSON     string
		operatorName    string
		operatorContact string
		federation      bool
		outPath         string
	)
	fs.StringVar(&operatorKeyPath, "operator-key", "",
		"operator quid file to sign with (required)")
	fs.StringVar(&apiGateway, "api-gateway", "",
		"e.g. https://api.quidnug.com (required)")
	fs.StringVar(&seedsJSON, "seeds-json", "",
		"JSON array of {nodeQuid,url,region?,capabilities?} (required)")
	fs.StringVar(&domainsJSON, "domains-json", "",
		"optional JSON array of {name,description?,tree?}")
	fs.StringVar(&operatorName, "operator-name", "", "human-readable operator name")
	fs.StringVar(&operatorContact, "operator-contact", "", "email / URL etc.")
	fs.BoolVar(&federation, "federation-available", false,
		"set true if api.* exposes the /api/v2/federation/* surface")
	fs.StringVar(&outPath, "out", "", "output path (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if operatorKeyPath == "" || apiGateway == "" || seedsJSON == "" {
		return fmt.Errorf("--operator-key, --api-gateway, and --seeds-json are required")
	}

	signer, err := loadQuid(operatorKeyPath)
	if err != nil {
		return err
	}
	if !signer.HasPrivateKey() {
		return errors.New("operator key has no private-key component")
	}

	var seeds []WellKnownSeed
	if err := json.Unmarshal([]byte(seedsJSON), &seeds); err != nil {
		return fmt.Errorf("--seeds-json: %w", err)
	}
	if len(seeds) == 0 {
		return fmt.Errorf("at least one seed required")
	}

	var domains []WellKnownDomain
	if domainsJSON != "" {
		if err := json.Unmarshal([]byte(domainsJSON), &domains); err != nil {
			return fmt.Errorf("--domains-json: %w", err)
		}
	}

	doc := WellKnownDoc{
		Version: 1,
		Operator: WellKnownOperator{
			Quid:      signer.ID,
			Name:      operatorName,
			PublicKey: signer.PublicKeyHex,
			Contact:   operatorContact,
		},
		APIGateway:          apiGateway,
		Seeds:               seeds,
		Domains:             domains,
		FederationAvailable: federation,
		LastUpdated:         time.Now().Unix(),
	}

	// Sign the doc with Signature cleared. json.Marshal on the
	// struct gives a deterministic field order.
	signable, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	sig, err := signer.Sign(signable)
	if err != nil {
		return err
	}
	doc.Signature = sig

	final, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return err
	}
	if outPath == "" {
		_, err := os.Stdout.Write(final)
		return err
	}
	if err := os.WriteFile(outPath, final, 0644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// --- helpers --------------------------------------------------------------

// parseEndpointsArg accepts comma-separated endpoints, each of
// the form "url|protocol|region|priority|weight". Only url is
// required; other fields default to empty/zero.
func parseEndpointsArg(raw string) ([]client.NodeAdvertEndpoint, error) {
	var out []client.NodeAdvertEndpoint
	for _, group := range strings.Split(raw, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		parts := strings.Split(group, "|")
		ep := client.NodeAdvertEndpoint{
			URL: strings.TrimSpace(parts[0]),
		}
		if len(parts) > 1 {
			ep.Protocol = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			ep.Region = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 {
			var p int
			if _, err := fmt.Sscanf(parts[3], "%d", &p); err == nil {
				ep.Priority = p
			}
		}
		if len(parts) > 4 {
			var w int
			if _, err := fmt.Sscanf(parts[4], "%d", &w); err == nil {
				ep.Weight = w
			}
		}
		out = append(out, ep)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no endpoints parsed from %q", raw)
	}
	return out, nil
}

func parseCapabilitiesArg(raw string) (client.NodeAdvertCapabilities, error) {
	var caps client.NodeAdvertCapabilities
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		switch tok {
		case "":
			continue
		case "validator":
			caps.Validator = true
		case "cache":
			caps.Cache = true
		case "archive":
			caps.Archive = true
		case "bootstrap":
			caps.Bootstrap = true
		case "gossipSink", "gossip-sink":
			caps.GossipSink = true
		case "ipfsGateway", "ipfs-gateway", "ipfs":
			caps.IPFSGateway = true
		default:
			return caps, fmt.Errorf("unknown capability %q", tok)
		}
	}
	return caps, nil
}
