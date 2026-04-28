package core

import (
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"
)

// rootIndexTemplate is the human-readable landing page served at /.
// AI-agent friendly: semantic HTML, descriptive prose, no client-side
// rendering required, JSON variant available via Accept negotiation.
var rootIndexTemplate = template.Must(template.New("root").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="description" content="A Quidnug protocol node. Per-observer trust-weighted reputation on a public network. JSON API at /api/v1.">
<meta name="robots" content="index,follow">
<title>Quidnug node {{.NodeID}}</title>
<style>
  :root { color-scheme: light dark; }
  body { font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; line-height: 1.5; max-width: 760px; margin: 2rem auto; padding: 0 1rem; }
  h1 { font-size: 1.6rem; margin-bottom: 0.25rem; }
  h2 { font-size: 1.15rem; margin-top: 2rem; border-bottom: 1px solid #ccc8; padding-bottom: 0.25rem; }
  .lede { color: #666; margin-top: 0; }
  dl.facts { display: grid; grid-template-columns: max-content 1fr; gap: 0.4rem 1rem; }
  dl.facts dt { font-weight: 600; color: #444; }
  dl.facts dd { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; word-break: break-all; }
  ul.linklist { padding-left: 1.2rem; }
  ul.linklist li { margin-bottom: 0.4rem; }
  code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; background: #eee5; padding: 0.05rem 0.3rem; border-radius: 3px; }
  pre { background: #eee5; padding: 0.6rem; border-radius: 4px; overflow-x: auto; font-size: 0.85rem; }
  footer { margin-top: 3rem; padding-top: 1rem; border-top: 1px solid #ccc8; font-size: 0.85rem; color: #666; }
  @media (prefers-color-scheme: dark) {
    body { background: #1a1a1a; color: #e6e6e6; }
    .lede, footer, dl.facts dt { color: #aaa; }
    code, pre { background: #333; }
    h2 { border-color: #444; }
    a { color: #8ab4f8; }
  }
</style>
</head>
<body>
<h1>Quidnug node</h1>
<p class="lede">A peer on the public Quidnug trust network. This is primarily a JSON API; you have reached the human-readable landing page.</p>

<h2>This node</h2>
<dl class="facts">
  <dt>Version</dt><dd>{{.Version}}</dd>
  <dt>Node ID</dt><dd>{{.NodeID}}</dd>
  <dt>Uptime</dt><dd>{{.Uptime}}</dd>
  <dt>Block height</dt><dd>{{.BlockHeight}}</dd>
  <dt>Connected peers</dt><dd>{{.PeerCount}}</dd>
  <dt>Trust domains served</dt><dd>{{.DomainCount}}</dd>
</dl>

{{if .DomainSamples}}
<h2>Sample trust domains</h2>
<ul>
  {{range .DomainSamples}}<li><code>{{.}}</code></li>{{end}}
</ul>
{{if .HasMoreDomains}}<p>Showing {{len .DomainSamples}} of {{.DomainCount}}. Full list at <a href="/api/v1/domains"><code>/api/v1/domains</code></a>.</p>
{{else}}<p>Full list at <a href="/api/v1/domains"><code>/api/v1/domains</code></a>.</p>{{end}}
{{end}}

<h2>What you can do here</h2>
<p>The full machine-readable API lives under <code>/api/v1</code>. The most useful endpoints to start with:</p>
<ul class="linklist">
  <li><a href="/api/v1/health"><code>GET /api/v1/health</code></a> liveness probe</li>
  <li><a href="/api/v1/info"><code>GET /api/v1/info</code></a> node identity, version, domains, block height</li>
  <li><a href="/api/v1/domains"><code>GET /api/v1/domains</code></a> trust domains served by this node</li>
  <li><a href="/api/v1/nodes"><code>GET /api/v1/nodes</code></a> known peer nodes</li>
  <li><a href="/api/v1/blocks"><code>GET /api/v1/blocks</code></a> recent blocks</li>
  <li><a href="/metrics"><code>GET /metrics</code></a> Prometheus metrics</li>
</ul>

<h2>Quick start</h2>
<p>From any shell:</p>
<pre>curl {{.SelfBaseURL}}/api/v1/info | jq .
curl {{.SelfBaseURL}}/api/v1/domains | jq .
curl {{.SelfBaseURL}}/api/v1/streams/&lt;subject-id&gt;/events | jq .</pre>

<h2>Clients and SDKs</h2>
<p>Quidnug ships SDKs for Go, Python, Rust, TypeScript and JavaScript, Java, .NET, Swift, and Android, plus framework adapters for React, Vue, and Astro, plus drop-in widgets and plugins for WordPress and Shopify.</p>
<ul class="linklist">
  <li><a href="https://github.com/bhmortim/quidnug/tree/main/clients">All clients and SDKs (GitHub)</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/tree/main/integrations">Integrations: Sigstore, FHIR, Chainlink, Schema.org, Stripe Connect, more</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/tree/main/examples">Worked examples and demos</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/tree/main/UseCases">Use case dossiers (FinTech, AI, healthcare, reviews, more)</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/tree/main/docs/design">Design proposals (QDPs)</a></li>
</ul>

<h2>The reviews ecosystem</h2>
<p>If you reached this node looking for trust-weighted reviews:</p>
<ul class="linklist">
  <li><a href="https://github.com/bhmortim/quidnug/tree/main/UseCases/trust-weighted-reviews">Trust-weighted reviews use case</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/blob/main/examples/reviews-and-comments/PROTOCOL.md">QRP-0001 base protocol</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/blob/main/examples/reviews-and-comments/QRP-0002.md">QRP-0002 amendments (Draft)</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/tree/main/examples/reviews-and-comments/demo">Live multi-actor demo</a></li>
</ul>

<h2>Running your own node</h2>
<p>The home-operator playbook walks through bringing up a public-facing node on a home machine plus VPS for failover, with monitoring and backups, in roughly half a day at $0 to $6 per month:</p>
<ul class="linklist">
  <li><a href="https://github.com/bhmortim/quidnug/blob/main/deploy/public-network/home-operator-plan.md">Home-operator plan</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/blob/main/deploy/public-network/peering-protocol.md">Peering with the public network</a></li>
  <li><a href="https://github.com/bhmortim/quidnug/blob/main/CONTRIBUTING.md">Contributing</a></li>
</ul>

<footer>
<p>Quidnug node serving since {{.StartedAt}}. The protocol is Apache 2.0 open source. Send <code>Accept: application/json</code> to this URL for the same data as JSON.</p>
</footer>
</body>
</html>`))

// rootIndexData carries the dynamic fields the root template renders.
type rootIndexData struct {
	Version        string
	NodeID         string
	Uptime         string
	BlockHeight    int64
	PeerCount      int
	DomainCount    int
	DomainSamples  []string
	HasMoreDomains bool
	StartedAt      string
	SelfBaseURL    string
}

// rootSampleDomainLimit caps how many domain names appear inline on the
// landing page. Operators with many domains link out to the full list.
const rootSampleDomainLimit = 8

// RootHandler serves the human-readable landing page at /, with a
// JSON variant available via Accept: application/json. AI-agent
// friendly: the HTML uses semantic tags and descriptive prose, and
// the JSON variant carries the same facts machine-readably.
func (node *QuidnugNode) RootHandler(w http.ResponseWriter, r *http.Request) {
	node.TrustDomainsMutex.RLock()
	domainCount := len(node.TrustDomains)
	domainNames := make([]string, 0, domainCount)
	for name := range node.TrustDomains {
		domainNames = append(domainNames, name)
	}
	node.TrustDomainsMutex.RUnlock()
	sort.Strings(domainNames)

	domainSamples := domainNames
	hasMore := false
	if len(domainSamples) > rootSampleDomainLimit {
		domainSamples = domainSamples[:rootSampleDomainLimit]
		hasMore = true
	}

	node.KnownNodesMutex.RLock()
	peerCount := len(node.KnownNodes)
	node.KnownNodesMutex.RUnlock()

	node.BlockchainMutex.RLock()
	blockHeight := int64(0)
	startedAtUnix := int64(0)
	if len(node.Blockchain) > 0 {
		blockHeight = node.Blockchain[len(node.Blockchain)-1].Index
		startedAtUnix = node.Blockchain[0].Timestamp
	}
	node.BlockchainMutex.RUnlock()

	uptime := time.Duration(0)
	startedAt := "unknown"
	if startedAtUnix > 0 {
		started := time.Unix(startedAtUnix, 0).UTC()
		startedAt = started.Format("2006-01-02 15:04 UTC")
		uptime = time.Since(started).Round(time.Second)
	}

	// Best-effort base URL derived from the request, so the curl
	// snippets render with whatever hostname the visitor used. Honors
	// X-Forwarded-Proto and X-Forwarded-Host for nodes behind a reverse
	// proxy or Cloudflare Tunnel.
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	host := r.Host
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}
	selfBaseURL := scheme + "://" + host

	if wantsJSON(r) {
		WriteSuccess(w, map[string]interface{}{
			"version":       QuidnugVersion,
			"nodeId":        node.NodeID,
			"uptimeSeconds": int64(uptime.Seconds()),
			"startedAt":     startedAt,
			"blockHeight":   blockHeight,
			"peerCount":     peerCount,
			"domainCount":   domainCount,
			"domainSamples": domainSamples,
			"endpoints": map[string]string{
				"health":  "/api/v1/health",
				"info":    "/api/v1/info",
				"domains": "/api/v1/domains",
				"nodes":   "/api/v1/nodes",
				"blocks":  "/api/v1/blocks",
				"metrics": "/metrics",
			},
			"selfBaseUrl": selfBaseURL,
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_ = rootIndexTemplate.Execute(w, rootIndexData{
		Version:        QuidnugVersion,
		NodeID:         node.NodeID,
		Uptime:         uptime.String(),
		BlockHeight:    blockHeight,
		PeerCount:      peerCount,
		DomainCount:    domainCount,
		DomainSamples:  domainSamples,
		HasMoreDomains: hasMore,
		StartedAt:      startedAt,
		SelfBaseURL:    selfBaseURL,
	})
}

// RobotsHandler serves a permissive robots.txt. Quidnug node landing
// pages are crawlable on purpose: AI agents and humans alike should be
// able to discover the API surface from any node.
func (node *QuidnugNode) RobotsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write([]byte("User-agent: *\nAllow: /\n\n# Discoverable API root: /api/v1\n"))
}

// wantsJSON returns true when the request's Accept header prefers JSON
// over HTML. text/html (or absence of Accept) returns HTML;
// application/json returns JSON.
func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	if strings.Contains(accept, "text/html") {
		return false
	}
	return strings.Contains(accept, "application/json")
}
