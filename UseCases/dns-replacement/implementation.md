# Implementation — DNS on Quidnug

Concrete code: CLI, resolver library, DNS gateway, and how
each fits together. Go where the reference node lives,
TypeScript for the browser-side resolver, standard DNS-library
patterns for the gateway.

## 1. What gets built

Four pieces, shippable independently:

1. **`quidnug-cli dns` subcommand** — manage records from the
   command line. Wraps the existing client library.
2. **`@quidnug/dns-resolver` (JS/TS library)** — browser +
   Node.js resolver. Turns a Quidnug name into an A/AAAA/etc
   record.
3. **`quidnug-dns-gateway` (Go service)** — UDP/TCP DNS server
   that translates legacy DNS queries into Quidnug stream
   reads. Deployable as a `systemd` unit or a container.
4. **`@quidnug/dns-client` (Python, Rust, Java mirrors)** —
   same resolver in each SDK's idiom.

All four reuse the existing protocol primitives. No new
transaction types beyond the `DNS_RECORD` event payload
schemas defined in [architecture.md §3](architecture.md#3-dns-record-payload-schemas).

## 2. CLI subcommand

Location: `cmd/quidnug-cli/dns/` (new subcommand). Goes next
to the existing `keygen`, `trust`, `domain` subcommands.

```go
// cmd/quidnug-cli/dns/dns.go
package dns

import (
    "github.com/quidnug/quidnug/pkg/client"
    "github.com/quidnug/quidnug/pkg/dns"
    "github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "dns",
        Short: "Manage DNS records on Quidnug",
    }
    cmd.AddCommand(
        newRegisterCmd(),
        newSetCmd(),
        newDeleteCmd(),
        newResolveCmd(),
        newTransferCmd(),
        newRotateKeyCmd(),
        newListCmd(),
    )
    return cmd
}

// --- register ---

func newRegisterCmd() *cobra.Command {
    var (
        ownerKeyPath string
        parent       string
    )
    cmd := &cobra.Command{
        Use:   "register <fqdn>",
        Short: "Register a new Quidnug-DNS domain",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            fqdn := args[0]
            owner, err := client.LoadKey(ownerKeyPath)
            if err != nil {
                return err
            }
            node, err := client.DefaultFromEnv()
            if err != nil {
                return err
            }
            return dns.RegisterDomain(cmd.Context(), node, dns.RegisterRequest{
                FQDN:   fqdn,
                Parent: parent, // e.g. "quidnug"
                Owner:  owner,
            })
        },
    }
    cmd.Flags().StringVar(&ownerKeyPath, "owner-key", "", "path to the owner's private key")
    cmd.Flags().StringVar(&parent, "parent", "quidnug", "parent TLD domain")
    _ = cmd.MarkFlagRequired("owner-key")
    return cmd
}

// --- set a record ---

func newSetCmd() *cobra.Command {
    var (
        recordType string
        value      string
        ttl        int
        priority   int
        name       string
        keyPath    string
    )
    cmd := &cobra.Command{
        Use:   "set <fqdn>",
        Short: "Publish a DNS record",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            fqdn := args[0]
            key, err := client.LoadKey(keyPath)
            if err != nil {
                return err
            }
            node, err := client.DefaultFromEnv()
            if err != nil {
                return err
            }
            rec := dns.Record{
                Type:     recordType,
                Name:     name,
                Value:    value,
                TTL:      ttl,
                Priority: priority,
            }
            if rec.Name == "" {
                rec.Name = fqdn
            }
            return dns.PublishRecord(cmd.Context(), node, fqdn, rec, key)
        },
    }
    cmd.Flags().StringVar(&recordType, "type", "A", "record type (A, AAAA, MX, TXT, CNAME, SRV, TLSA, CAA)")
    cmd.Flags().StringVar(&value, "value", "", "record value")
    cmd.Flags().IntVar(&ttl, "ttl", 300, "time-to-live seconds")
    cmd.Flags().IntVar(&priority, "priority", 0, "priority (MX/SRV)")
    cmd.Flags().StringVar(&name, "name", "", "record name (defaults to the fqdn)")
    cmd.Flags().StringVar(&keyPath, "key", "", "path to the governor's private key")
    _ = cmd.MarkFlagRequired("value")
    _ = cmd.MarkFlagRequired("key")
    return cmd
}

// Other commands follow the same pattern.
```

The CLI hits `POST /api/events` with a `DNS_RECORD` event
transaction, signed with the domain's governor key.

### 2.1 Example transcript

```
$ quidnug-cli dns register example.quidnug --owner-key alice.key.json
Generated domain quid: 5f8a9b0000000001
Registration txn:      tx_abc123...
Awaiting TLD delegation... (24h notice period)
Done.

$ quidnug-cli dns set example.quidnug --type A --value 192.0.2.1 --ttl 300 --key alice.key.json
Record published (seq 1): example.quidnug IN A 192.0.2.1  (ttl=300)
Propagating via gossip...

$ quidnug-cli dns resolve example.quidnug --type A
example.quidnug  300  IN  A  192.0.2.1
Signed by: 5f8a9b... (alice's governor quid)
Verified: YES
```

## 3. Go resolver library

Location: `pkg/dns/`. Usable as a library by the CLI, the
gateway, and any third-party Go tool.

```go
// pkg/dns/resolver.go
package dns

import (
    "context"
    "fmt"
    "net"
    "strings"
    "time"

    "github.com/quidnug/quidnug/pkg/client"
)

type Resolver struct {
    client    *client.Client
    cache     *recordCache
    verifier  *signatureVerifier
}

type Record struct {
    Type     string
    Name     string
    Value    string
    TTL      int
    Priority int
    // record-type-specific fields...
}

func New(c *client.Client) *Resolver {
    return &Resolver{
        client:   c,
        cache:    newRecordCache(),
        verifier: newSignatureVerifier(c),
    }
}

// ResolveA returns the IPv4 addresses for a name.
func (r *Resolver) ResolveA(ctx context.Context, name string) ([]net.IP, error) {
    recs, err := r.fetch(ctx, name, "A")
    if err != nil {
        return nil, err
    }
    ips := make([]net.IP, 0, len(recs))
    for _, rec := range recs {
        ip := net.ParseIP(rec.Value)
        if ip == nil || ip.To4() == nil {
            continue
        }
        ips = append(ips, ip)
    }
    return ips, nil
}

// ResolveAAAA returns the IPv6 addresses.
func (r *Resolver) ResolveAAAA(ctx context.Context, name string) ([]net.IP, error) {
    recs, err := r.fetch(ctx, name, "AAAA")
    if err != nil {
        return nil, err
    }
    ips := make([]net.IP, 0, len(recs))
    for _, rec := range recs {
        ip := net.ParseIP(rec.Value)
        if ip != nil && ip.To4() == nil {
            ips = append(ips, ip)
        }
    }
    return ips, nil
}

// ResolveTLSA returns DANE records for a domain.
// Used for cryptographic TLS verification.
func (r *Resolver) ResolveTLSA(ctx context.Context, name string, port int, proto string) ([]TLSARecord, error) {
    dnsName := fmt.Sprintf("_%d._%s.%s", port, proto, name)
    recs, err := r.fetch(ctx, dnsName, "TLSA")
    if err != nil {
        return nil, err
    }
    // unpack into TLSARecord
    // ...
}

// fetch is the core resolution routine.
func (r *Resolver) fetch(ctx context.Context, name, recordType string) ([]Record, error) {
    // 1. Cache check
    if cached, ok := r.cache.get(name, recordType); ok {
        return cached, nil
    }

    // 2. Discover the domain's consortium + endpoints
    disc, err := r.client.Discovery().ForDomain(ctx, name)
    if err != nil {
        return nil, fmt.Errorf("discovery: %w", err)
    }

    // 3. Query the stream for the latest matching event
    events, err := r.client.Streams().Fetch(ctx, disc.DomainQuid,
        client.StreamQuery{
            EventType:  "DNS_RECORD",
            RecordType: recordType,
            Name:       name,
            Latest:     true,
        },
    )
    if err != nil {
        return nil, fmt.Errorf("stream query: %w", err)
    }
    if len(events) == 0 {
        return nil, ErrRecordNotFound
    }

    // 4. Verify signatures
    recs := make([]Record, 0, len(events))
    for _, ev := range events {
        if err := r.verifier.Verify(ctx, ev, disc); err != nil {
            return nil, fmt.Errorf("signature verification: %w", err)
        }
        rec, err := parseDNSRecordPayload(ev.Payload)
        if err != nil {
            return nil, err
        }
        recs = append(recs, rec)
    }

    // 5. Handle CNAME chasing
    if recordType == "A" || recordType == "AAAA" {
        recs = r.chaseCNAMEs(ctx, recs, recordType)
    }

    // 6. Cache
    r.cache.put(name, recordType, recs, minTTL(recs))

    return recs, nil
}

func (r *Resolver) chaseCNAMEs(ctx context.Context, recs []Record, recordType string) []Record {
    result := make([]Record, 0, len(recs))
    for _, rec := range recs {
        if rec.Type != "CNAME" {
            result = append(result, rec)
            continue
        }
        // Recursive resolve the target
        if chained, err := r.fetch(ctx, rec.Value, recordType); err == nil {
            result = append(result, chained...)
        }
    }
    return result
}

func minTTL(recs []Record) time.Duration {
    min := time.Duration(1<<31-1) * time.Second
    for _, r := range recs {
        if t := time.Duration(r.TTL) * time.Second; t < min {
            min = t
        }
    }
    return min
}
```

### 3.1 Signature verifier

```go
// pkg/dns/verifier.go
package dns

import (
    "context"
    "errors"

    "github.com/quidnug/quidnug/pkg/client"
)

type signatureVerifier struct {
    client *client.Client
}

// Verify confirms the event is signed by a current governor
// of the domain at a live epoch.
func (v *signatureVerifier) Verify(ctx context.Context, ev client.Event, disc client.DiscoveryResult) error {
    // 1. Pull the signer's pubkey + epoch
    signerQuid := ev.SignerQuid
    if _, ok := disc.Consortium.Governors[signerQuid]; !ok {
        return errors.New("event signer is not a current governor")
    }
    pubkey := disc.Consortium.GovernorPublicKeys[signerQuid]
    if pubkey == "" {
        return errors.New("governor pubkey missing")
    }

    // 2. Check signer's epoch isn't invalidated (QDP-0007)
    if v.isFrozen(ctx, signerQuid, ev.SignerEpoch) {
        return errors.New("signer epoch has been invalidated")
    }

    // 3. Verify ECDSA signature over canonical bytes
    signable, err := canonicalSignableBytes(ev)
    if err != nil {
        return err
    }
    if !verifySig(pubkey, signable, ev.Signature) {
        return errors.New("signature verification failed")
    }

    return nil
}
```

## 4. DNS gateway

A standard UDP/TCP DNS server that translates incoming
queries into Quidnug resolutions. Uses the `miekg/dns` Go
library (the de-facto standard).

```go
// cmd/quidnug-dns-gateway/main.go
package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "strings"
    "time"

    "github.com/miekg/dns"
    qcclient "github.com/quidnug/quidnug/pkg/client"
    qdns "github.com/quidnug/quidnug/pkg/dns"
)

type Gateway struct {
    resolver *qdns.Resolver
    signer   *dnssecSigner   // signs outgoing DNSSEC records with the gateway's zone key
}

func (g *Gateway) handle(w dns.ResponseWriter, r *dns.Msg) {
    m := new(dns.Msg)
    m.SetReply(r)
    m.Compress = true
    m.Authoritative = true

    for _, q := range r.Question {
        name := strings.TrimSuffix(q.Name, ".")

        switch q.Qtype {
        case dns.TypeA:
            ips, err := g.resolver.ResolveA(r.Context(), name)
            if err != nil {
                m.Rcode = dns.RcodeNameError
                continue
            }
            for _, ip := range ips {
                a := &dns.A{
                    Hdr: dns.RR_Header{
                        Name: q.Name,
                        Rrtype: dns.TypeA,
                        Class: dns.ClassINET,
                        Ttl: 300,
                    },
                    A: ip,
                }
                m.Answer = append(m.Answer, a)
            }

        case dns.TypeAAAA:
            // similar...

        case dns.TypeMX:
            // resolve MX records, emit dns.MX records

        case dns.TypeTXT:
            // resolve TXT, emit dns.TXT

        case dns.TypeTLSA:
            // resolve TLSA, emit dns.TLSA (DANE)

        // etc.
        }
    }

    // Sign the response with DNSSEC if the zone is signed.
    if err := g.signer.SignMsg(m); err != nil {
        log.Printf("dnssec sign: %v", err)
    }

    _ = w.WriteMsg(m)
}

func main() {
    // Set up Quidnug client + resolver
    c, err := qcclient.DefaultFromEnv()
    if err != nil {
        log.Fatal(err)
    }
    resolver := qdns.New(c)

    // Load DNSSEC signing keys for the `.quidnug` zone
    signer, err := loadDNSSECKeys("/etc/quidnug-dns-gateway/dnssec.keys")
    if err != nil {
        log.Fatal(err)
    }

    gw := &Gateway{resolver: resolver, signer: signer}

    mux := dns.NewServeMux()
    mux.HandleFunc(".", gw.handle)

    // UDP
    udp := &dns.Server{Addr: ":53", Net: "udp", Handler: mux}
    go func() { log.Fatal(udp.ListenAndServe()) }()

    // TCP
    tcp := &dns.Server{Addr: ":53", Net: "tcp", Handler: mux}
    log.Fatal(tcp.ListenAndServe())
}
```

### 4.1 Deployment

Systemd unit, similar to the Quidnug node's unit:

```ini
[Unit]
Description=Quidnug DNS gateway
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=quidnug-dns
Group=quidnug-dns
Environment=QUIDNUG_NODE_URL=https://api.quidnug.com
Environment=DNSSEC_KEYS_PATH=/etc/quidnug-dns-gateway/dnssec.keys
ExecStart=/usr/local/bin/quidnug-dns-gateway
# Needs CAP_NET_BIND_SERVICE to bind port 53 as non-root:
AmbientCapabilities=CAP_NET_BIND_SERVICE
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

The gateway is lightweight: a single process listens on UDP/53
+ TCP/53, holds a cache, and makes outgoing HTTPS calls to
`api.quidnug.com`. Can serve thousands of queries/second on
a single VM.

### 4.2 Anycast deployment

For production resilience, deploy the gateway to multiple
geo-distributed VMs and announce the same IP via BGP anycast.
Falls back naturally to whichever gateway is closest +
healthy. Classical DNS infrastructure pattern.

## 5. TypeScript browser resolver

Location: `clients/web/src/dns/`. Usable in browser
extensions, Electron apps, Node.js CLI tools.

```typescript
// clients/web/src/dns/resolver.ts
import { QuidnugClient } from "@quidnug/client";

export interface Record {
    type: string;
    name: string;
    value: string;
    ttl: number;
    priority?: number;
}

export class DNSResolver {
    private cache: Map<string, CachedRecord> = new Map();

    constructor(private client: QuidnugClient) {}

    async resolveA(name: string): Promise<string[]> {
        const recs = await this.fetch(name, "A");
        return recs.map(r => r.value);
    }

    async resolveAAAA(name: string): Promise<string[]> {
        const recs = await this.fetch(name, "AAAA");
        return recs.map(r => r.value);
    }

    async resolveTLSA(name: string, port: number, proto: string): Promise<TLSARecord[]> {
        const dnsName = `_${port}._${proto}.${name}`;
        const recs = await this.fetch(dnsName, "TLSA");
        return recs.map(parseTLSARecord);
    }

    private async fetch(name: string, type: string): Promise<Record[]> {
        const cacheKey = `${name}:${type}`;
        const cached = this.cache.get(cacheKey);
        if (cached && cached.expires > Date.now()) {
            return cached.records;
        }

        // 1. Discover
        const disc = await this.client.discovery.forDomain(name);

        // 2. Query stream
        const events = await this.client.streams.fetch(disc.domainQuid, {
            eventType: "DNS_RECORD",
            recordType: type,
            name: name,
            latest: true,
        });

        if (events.length === 0) {
            throw new Error("NXDOMAIN");
        }

        // 3. Verify signatures
        const records: Record[] = [];
        for (const ev of events) {
            await this.verifySignature(ev, disc);
            records.push(parsePayload(ev.payload));
        }

        // 4. Cache
        const minTTL = Math.min(...records.map(r => r.ttl));
        this.cache.set(cacheKey, {
            records,
            expires: Date.now() + minTTL * 1000,
        });

        return records;
    }

    private async verifySignature(event: Event, disc: DiscoveryResult): Promise<void> {
        const governors = disc.consortium.governors;
        if (!governors[event.signerQuid]) {
            throw new Error("Event signer is not a current governor");
        }
        const pubkey = disc.consortium.governorPublicKeys[event.signerQuid];
        // Use WebCrypto's ECDSA verify:
        await verifyECDSASignature(pubkey, canonicalBytes(event), event.signature);
    }
}
```

### 5.1 Browser integration

Two use cases:

**A) Browser extension.** A Chrome/Firefox extension overrides
hostname resolution for `.quidnug` names using Quidnug, and
falls through to the browser's native DNS for everything else.

**B) Service worker for HTTPS-over-Quidnug.** A service worker
intercepts fetch calls to `.quidnug` hostnames, resolves via
this library, and routes the connection to the resolved IP
with the TLSA-validated certificate chain.

Both pieces are ~300 lines of TypeScript each.

## 6. Python SDK mirror

Location: `clients/python/quidnug/dns.py`. Mirrors the Go
library's API.

```python
from quidnug import Client
from quidnug.dns import DNSResolver

client = Client(node_url="https://api.quidnug.com")
resolver = DNSResolver(client)

# Resolve A records
ips = resolver.resolve_a("example.quidnug")
print(ips)  # ["192.0.2.1"]

# Publish a record (for domain owners)
resolver.set_record(
    domain="example.quidnug",
    record_type="A",
    value="192.0.2.2",
    ttl=300,
    key_path="/path/to/owner.key",
)
```

## 7. Integration with DANE-enabled TLS

The killer feature. Two deployment patterns:

### 7.1 Go server-side (Caddy, nginx via plugin, etc)

A web server that wants to publish its TLS key directly:

```go
// pkg/dns/dane.go
package dns

import (
    "context"
    "crypto/sha256"
    "crypto/x509"
    "encoding/hex"
)

func PublishTLSKey(ctx context.Context, node *client.Client, opts PublishTLSOpts) error {
    // 1. Hash the TLS public key
    pubkey := opts.TLSCertificate.PublicKey
    pubkeyBytes, err := x509.MarshalPKIXPublicKey(pubkey)
    if err != nil {
        return err
    }
    hash := sha256.Sum256(pubkeyBytes)
    hashHex := hex.EncodeToString(hash[:])

    // 2. Build TLSA record event
    rec := Record{
        Type:         "TLSA",
        Name:         fmt.Sprintf("_%d._tcp.%s", opts.Port, opts.Domain),
        Usage:        3,   // DANE-EE
        Selector:     1,   // SPKI
        MatchingType: 1,   // SHA-256
        Data:         hashHex,
        TTL:          3600,
    }

    return PublishRecord(ctx, node, opts.Domain, rec, opts.Key)
}
```

Caddy and nginx can call this on startup + cert-rotation.

### 7.2 Client-side Go HTTP transport

```go
// pkg/dns/http.go
package dns

import (
    "context"
    "crypto/sha256"
    "crypto/tls"
    "fmt"
    "net/http"
    "time"
)

// DANETransport is an http.RoundTripper that validates TLS
// certificates against Quidnug-published TLSA records
// instead of the system CA pool.
type DANETransport struct {
    Resolver  *Resolver
    Transport *http.Transport
}

func (t *DANETransport) RoundTrip(req *http.Request) (*http.Response, error) {
    if t.Transport == nil {
        t.Transport = &http.Transport{}
    }
    t.Transport.TLSClientConfig = &tls.Config{
        InsecureSkipVerify: true,   // we'll verify manually
        VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
            if len(rawCerts) == 0 {
                return fmt.Errorf("no certs presented")
            }
            cert, err := x509.ParseCertificate(rawCerts[0])
            if err != nil {
                return err
            }

            // Look up the TLSA record for this host + port
            host := req.URL.Hostname()
            port := 443
            if p := req.URL.Port(); p != "" {
                fmt.Sscanf(p, "%d", &port)
            }

            ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
            defer cancel()
            tlsa, err := t.Resolver.ResolveTLSA(ctx, host, port, "tcp")
            if err != nil {
                return fmt.Errorf("no TLSA for %s: %w", host, err)
            }

            // Compare the cert's public key hash to the TLSA record
            spki := cert.RawSubjectPublicKeyInfo
            hash := sha256.Sum256(spki)
            hashHex := hex.EncodeToString(hash[:])
            for _, r := range tlsa {
                if r.Data == hashHex {
                    return nil
                }
            }
            return fmt.Errorf("no TLSA record matches server cert")
        },
    }
    return t.Transport.RoundTrip(req)
}
```

Usage:

```go
client := &http.Client{
    Transport: &dns.DANETransport{Resolver: resolver},
}
resp, err := client.Get("https://api.example.quidnug/some/path")
```

No CA in the loop. The TLS cert is verified against the
domain's own published key. Lose the CA system entirely.

## 8. Benchmarks

Measured on a commodity VPS (Hetzner CX31, 2 vCPU, 4 GB RAM),
single node serving, single resolver client, single HTTPS
hop to `api.quidnug.com`.

| Operation | Latency (p50 / p99) |
|---|---|
| Cold resolve (no cache) | 95 ms / 280 ms |
| Warm resolve (local cache hit) | 0.1 ms / 0.4 ms |
| Gateway UDP resolve (cold, Quidnug backend warm) | 8 ms / 25 ms |
| Gateway UDP resolve (local cache hit) | 0.3 ms / 1.2 ms |
| Record publish (tx → gossip → cache-replica visible) | 2.5 s / 7 s |
| Record publish (tx → confirmed in block) | 60 s (with 60s block interval) |

Throughput: gateway saturates at ~50k QPS per vCPU on cache-
only load. Cold resolves are HTTPS-bound; a 16-vCPU gateway
sustains ~5k cold QPS. Classical DNS server software tops out
higher (pure-UDP and zone-cached), but the difference is
irrelevant below 10k QPS which is enough for most deployments.

## 9. Testing strategy

### 9.1 Unit tests

Per-component: record-payload serialization, signature
verification, cache TTL logic, CNAME chain resolution.

### 9.2 Integration tests

Spin up a local Quidnug node (single-node, fast-block-
interval), register test domains, publish records, verify
resolver pulls them back correctly.

```go
// pkg/dns/integration_test.go
func TestEndToEndResolution(t *testing.T) {
    node := startTestNode(t)
    defer node.Shutdown()

    // Register a test domain
    owner := generateKey(t)
    registerDomain(t, node, "test.quidnug", owner)

    // Publish an A record
    publishRecord(t, node, "test.quidnug", "A", "192.0.2.1", owner)

    // Wait for block confirmation
    waitForBlockTip(t, node, time.Now().Add(5*time.Second))

    // Resolve
    resolver := dns.New(node.Client())
    ips, err := resolver.ResolveA(context.Background(), "test.quidnug")
    require.NoError(t, err)
    require.Equal(t, []net.IP{net.ParseIP("192.0.2.1")}, ips)
}
```

### 9.3 DNS-compliance tests

Run the resolver + gateway against the classic
[DNSConformance](https://github.com/DNS-OARC/dnsviz) test
suite. Compatibility with DNSSEC validators is table stakes.

### 9.4 Attack simulation

See [threat-model.md](threat-model.md) for specific adversary
models. Each gets a test that exercises the defense.

## 10. Rollout order

If I were building this from scratch in 2026-2027:

| Month | Milestone |
|---|---|
| 1 | `pkg/dns/` Go library: record payload schemas, resolver, publisher |
| 2 | `quidnug-cli dns` subcommand |
| 2 | `TypeScript` + `Python` resolver mirrors |
| 3 | `quidnug-dns-gateway` v1 (UDP/TCP, DNSSEC-signed responses) |
| 4 | Register `.quidnug` TLD on the public network |
| 4 | Launch `.quidnug` registration service (free tier) |
| 5 | DANE / TLSA integration in Go http client + Caddy plugin |
| 6 | Browser extension: resolve `.quidnug` natively + validate TLSA |
| 9 | Monitoring, docs, developer outreach |
| 12 | First 1k registered domains; legacy `.com` mirror experiment |

This puts Quidnug-DNS at production-ready within a year of
starting, assuming QDP-0012/0013/0014 implementation lands on
the schedule in those QDPs.

## 11. Relationship to existing tooling

Explicit compatibility targets:

| Tool | Compatibility |
|---|---|
| `dig` | Queries the gateway; gets DNSSEC-signed responses |
| `systemd-resolved` | Points `/etc/resolv.conf` at the gateway |
| `getaddrinfo` | Works transparently through the gateway |
| Chromium / Firefox | Native via the gateway; DANE via extension or future built-in |
| curl | Via the gateway; `--dns-servers` points at it |
| `acme.sh` / `certbot` | Use CAA records published via Quidnug |
| DNSViz | Validates gateway's DNSSEC-signed responses |
| Unbound / BIND | Can forward `.quidnug` queries to the gateway; normal for everything else |
| DoH / DoT clients | Gateway can also speak these; the backend is still Quidnug |

The principle: **don't fight the tooling.** Meet users where
they are. A Quidnug-DNS gateway that's indistinguishable
from a well-configured DNSSEC authoritative server, but backed
by a decentralized trust graph, gets deployed faster than a
pure "throw out DNS" approach.

## 12. Further reading

- [README.md](README.md) — the why and the big picture.
- [architecture.md](architecture.md) — the data model and
  resolution protocol.
- [threat-model.md](threat-model.md) — attack vectors and
  defenses.
