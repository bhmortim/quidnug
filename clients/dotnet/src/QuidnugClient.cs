using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace Quidnug.Client;

/// <summary>
/// Strongly-typed HTTP client for a Quidnug node.
///
/// <para>Covers the full v2 protocol surface (QDPs 0001–0010). Thread-safe;
/// one instance can be shared across requests.</para>
///
/// <para><b>Retry policy.</b> GETs retry on 5xx/429 with exponential
/// backoff + ±100ms jitter. POSTs are not retried — reconcile via a
/// follow-up GET before replaying a write.</para>
/// </summary>
public sealed class QuidnugClient : IDisposable
{
    private static readonly HashSet<string> ConflictCodes = new()
    {
        "NONCE_REPLAY", "GUARDIAN_SET_MISMATCH", "QUORUM_NOT_MET", "VETOED",
        "INVALID_SIGNATURE", "FORK_ALREADY_ACTIVE", "DUPLICATE",
        "ALREADY_EXISTS", "INVALID_STATE_TRANSITION",
    };
    private static readonly HashSet<string> UnavailableCodes = new()
    {
        "FEATURE_NOT_ACTIVE", "NOT_READY", "BOOTSTRAPPING",
    };

    private readonly HttpClient _http;
    private readonly bool _ownsHttp;
    private readonly string _apiBase;
    private readonly int _maxRetries;
    private readonly TimeSpan _retryBaseDelay;

    /// <summary>Construct a client against <paramref name="baseUrl"/>.</summary>
    public QuidnugClient(
        string baseUrl,
        HttpClient? http = null,
        TimeSpan? timeout = null,
        int maxRetries = 3,
        TimeSpan? retryBaseDelay = null,
        string? authToken = null,
        string userAgent = "quidnug-dotnet-sdk/2.0.0")
    {
        if (string.IsNullOrWhiteSpace(baseUrl))
            throw new QuidnugValidationException("baseUrl is required");

        _apiBase = baseUrl.TrimEnd('/') + "/api";
        _maxRetries = maxRetries;
        _retryBaseDelay = retryBaseDelay ?? TimeSpan.FromSeconds(1);

        if (http is null)
        {
            _http = new HttpClient { Timeout = timeout ?? TimeSpan.FromSeconds(30) };
            _ownsHttp = true;
        }
        else
        {
            _http = http;
            _ownsHttp = false;
        }

        _http.DefaultRequestHeaders.UserAgent.Clear();
        _http.DefaultRequestHeaders.UserAgent.ParseAdd(userAgent);
        if (!string.IsNullOrEmpty(authToken))
        {
            _http.DefaultRequestHeaders.Authorization =
                new AuthenticationHeaderValue("Bearer", authToken);
        }
    }

    public void Dispose()
    {
        if (_ownsHttp) _http.Dispose();
    }

    // =====================================================================
    // Health / info / peers
    // =====================================================================

    public Task<JsonNode?> HealthAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "health", null, ct);

    public Task<JsonNode?> InfoAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "info", null, ct);

    public Task<JsonNode?> NodesAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "nodes", null, ct);

    public Task<JsonNode?> BlocksAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "blocks", null, ct);

    // =====================================================================
    // Identity
    // =====================================================================

    public async Task<JsonNode?> RegisterIdentityAsync(
        Quid signer,
        string? name = null,
        string? homeDomain = null,
        string domain = "default",
        string? description = null,
        Dictionary<string, object?>? attributes = null,
        long updateNonce = 1,
        CancellationToken ct = default)
    {
        RequireSigner(signer);
        var tx = new Dictionary<string, object?>
        {
            ["type"] = "IDENTITY",
            ["timestamp"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
            ["trustDomain"] = domain,
            ["signerQuid"] = signer.Id,
            ["definerQuid"] = signer.Id,
            ["subjectQuid"] = signer.Id,
            ["updateNonce"] = updateNonce,
            ["schemaVersion"] = "1.0",
            ["attributes"] = attributes ?? new Dictionary<string, object?>(),
        };
        if (name is not null) tx["name"] = name;
        if (description is not null) tx["description"] = description;
        if (homeDomain is not null) tx["homeDomain"] = homeDomain;
        SignIntoTx(signer, tx);
        return await RequestAsync(HttpMethod.Post, "transactions/identity", tx, ct);
    }

    public async Task<IdentityRecord?> GetIdentityAsync(
        string quidId, string? domain = null, CancellationToken ct = default)
    {
        var path = "identity/" + Uri.EscapeDataString(quidId);
        if (domain is not null) path += "?domain=" + Uri.EscapeDataString(domain);
        try
        {
            var node = await RequestAsync(HttpMethod.Get, path, null, ct);
            return node is null ? null : JsonSerializer.Deserialize<IdentityRecord>(node.ToJsonString());
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Trust
    // =====================================================================

    public async Task<JsonNode?> GrantTrustAsync(
        Quid signer,
        string trustee,
        double level,
        string domain = "default",
        long nonce = 1,
        long validUntil = 0,
        string? description = null,
        CancellationToken ct = default)
    {
        RequireSigner(signer);
        if (string.IsNullOrEmpty(trustee))
            throw new QuidnugValidationException("trustee is required");
        if (level < 0 || level > 1)
            throw new QuidnugValidationException("level must be in [0, 1]");

        var tx = new Dictionary<string, object?>
        {
            ["type"] = "TRUST",
            ["timestamp"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
            ["trustDomain"] = domain,
            ["signerQuid"] = signer.Id,
            ["truster"] = signer.Id,
            ["trustee"] = trustee,
            ["trustLevel"] = level,
            ["nonce"] = nonce,
        };
        if (validUntil > 0) tx["validUntil"] = validUntil;
        if (description is not null) tx["description"] = description;
        SignIntoTx(signer, tx);
        return await RequestAsync(HttpMethod.Post, "transactions/trust", tx, ct);
    }

    public async Task<TrustResult> GetTrustAsync(
        string observer, string target, string domain, int maxDepth = 5,
        CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(observer) || string.IsNullOrEmpty(target))
            throw new QuidnugValidationException("observer and target are required");
        string path = $"trust/{Uri.EscapeDataString(observer)}/{Uri.EscapeDataString(target)}"
                    + $"?domain={Uri.EscapeDataString(domain)}&maxDepth={maxDepth}";
        var node = await RequestAsync(HttpMethod.Get, path, null, ct)
                   ?? throw new QuidnugNodeException("empty response", 200, null);
        return JsonSerializer.Deserialize<TrustResult>(node.ToJsonString())
               ?? throw new QuidnugNodeException("decode trust result failed", 200, node.ToJsonString());
    }

    public async Task<List<TrustEdge>> GetTrustEdgesAsync(string quidId, CancellationToken ct = default)
    {
        var node = await RequestAsync(HttpMethod.Get, $"trust/edges/{Uri.EscapeDataString(quidId)}", null, ct);
        if (node is not JsonObject obj) return new List<TrustEdge>();
        var arr = obj["edges"] as JsonArray ?? obj["data"] as JsonArray;
        if (arr is null) return new List<TrustEdge>();
        var edges = new List<TrustEdge>(arr.Count);
        foreach (var item in arr)
        {
            if (item is null) continue;
            var edge = JsonSerializer.Deserialize<TrustEdge>(item.ToJsonString());
            if (edge is not null) edges.Add(edge);
        }
        return edges;
    }

    // =====================================================================
    // Title
    // =====================================================================

    public async Task<JsonNode?> RegisterTitleAsync(
        Quid signer,
        string assetId,
        List<OwnershipStake> owners,
        string domain = "default",
        string? titleType = null,
        string? prevTitleTxId = null,
        CancellationToken ct = default)
    {
        RequireSigner(signer);
        if (string.IsNullOrEmpty(assetId))
            throw new QuidnugValidationException("assetId is required");
        if (owners is null || owners.Count == 0)
            throw new QuidnugValidationException("owners is required");
        double total = owners.Sum(s => s.Percentage);
        if (Math.Abs(total - 100.0) > 0.001)
            throw new QuidnugValidationException($"owner percentages must sum to 100 (got {total})");

        var tx = new Dictionary<string, object?>
        {
            ["type"] = "TITLE",
            ["timestamp"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
            ["trustDomain"] = domain,
            ["signerQuid"] = signer.Id,
            ["issuerQuid"] = signer.Id,
            ["assetQuid"] = assetId,
            ["ownershipMap"] = owners,
            ["transferSigs"] = new Dictionary<string, string>(),
        };
        if (titleType is not null) tx["titleType"] = titleType;
        if (prevTitleTxId is not null) tx["prevTitleTxID"] = prevTitleTxId;
        SignIntoTx(signer, tx);
        return await RequestAsync(HttpMethod.Post, "transactions/title", tx, ct);
    }

    public async Task<Title?> GetTitleAsync(string assetId, string? domain = null, CancellationToken ct = default)
    {
        string path = "title/" + Uri.EscapeDataString(assetId);
        if (domain is not null) path += "?domain=" + Uri.EscapeDataString(domain);
        try
        {
            var node = await RequestAsync(HttpMethod.Get, path, null, ct);
            return node is null ? null : JsonSerializer.Deserialize<Title>(node.ToJsonString());
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Events + streams
    // =====================================================================

    public async Task<JsonNode?> EmitEventAsync(
        Quid signer,
        string subjectId,
        string subjectType,
        string eventType,
        string domain = "default",
        Dictionary<string, object?>? payload = null,
        string? payloadCid = null,
        long sequence = 0,
        CancellationToken ct = default)
    {
        RequireSigner(signer);
        if (subjectType is not "QUID" and not "TITLE")
            throw new QuidnugValidationException("subjectType must be 'QUID' or 'TITLE'");
        if (string.IsNullOrEmpty(eventType))
            throw new QuidnugValidationException("eventType is required");
        if ((payload is null) == (payloadCid is null))
            throw new QuidnugValidationException("exactly one of payload or payloadCid is required");

        if (sequence == 0)
        {
            try
            {
                string streamPath = "streams/" + Uri.EscapeDataString(subjectId);
                if (domain != "default") streamPath += "?domain=" + Uri.EscapeDataString(domain);
                var stream = await RequestAsync(HttpMethod.Get, streamPath, null, ct);
                sequence = (stream?["latestSequence"]?.GetValue<long>() ?? 0L) + 1L;
            }
            catch (QuidnugException)
            {
                sequence = 1;
            }
        }

        var tx = new Dictionary<string, object?>
        {
            ["type"] = "EVENT",
            ["timestamp"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
            ["trustDomain"] = domain,
            ["subjectId"] = subjectId,
            ["subjectType"] = subjectType,
            ["eventType"] = eventType,
            ["sequence"] = sequence,
        };
        if (payload is not null) tx["payload"] = payload;
        if (payloadCid is not null) tx["payloadCid"] = payloadCid;

        byte[] signable = CanonicalBytes.Of(tx, "signature", "txId", "publicKey");
        tx["signature"] = signer.Sign(signable);
        tx["publicKey"] = signer.PublicKeyHex;
        return await RequestAsync(HttpMethod.Post, "events", tx, ct);
    }

    public async Task<JsonNode?> GetEventStreamAsync(
        string subjectId, string? domain = null, CancellationToken ct = default)
    {
        try
        {
            string path = "streams/" + Uri.EscapeDataString(subjectId);
            if (domain is not null) path += "?domain=" + Uri.EscapeDataString(domain);
            return await RequestAsync(HttpMethod.Get, path, null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public async Task<List<Event>> GetStreamEventsAsync(
        string subjectId, string? domain = null, int limit = 50, int offset = 0,
        CancellationToken ct = default)
    {
        var path = $"streams/{Uri.EscapeDataString(subjectId)}/events?";
        if (domain is not null) path += $"domain={Uri.EscapeDataString(domain)}&";
        if (limit > 0) path += $"limit={limit}&";
        if (offset > 0) path += $"offset={offset}";
        var node = await RequestAsync(HttpMethod.Get, path.TrimEnd('&', '?'), null, ct);
        if (node is not JsonObject obj) return new List<Event>();
        var arr = obj["data"] as JsonArray ?? obj["events"] as JsonArray;
        if (arr is null) return new List<Event>();
        var events = new List<Event>(arr.Count);
        foreach (var item in arr)
        {
            if (item is null) continue;
            var ev = JsonSerializer.Deserialize<Event>(item.ToJsonString());
            if (ev is not null) events.Add(ev);
        }
        return events;
    }

    // =====================================================================
    // Guardians (QDP-0002)
    // =====================================================================

    public Task<JsonNode?> SubmitGuardianSetUpdateAsync(object update, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/set-update", update, ct);

    public Task<JsonNode?> SubmitRecoveryInitAsync(object init, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/recovery/init", init, ct);

    public Task<JsonNode?> SubmitRecoveryVetoAsync(object veto, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/recovery/veto", veto, ct);

    public Task<JsonNode?> SubmitRecoveryCommitAsync(object commit, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/recovery/commit", commit, ct);

    public async Task<GuardianSet?> GetGuardianSetAsync(string quidId, CancellationToken ct = default)
    {
        try
        {
            var node = await RequestAsync(HttpMethod.Get, $"guardian/set/{Uri.EscapeDataString(quidId)}", null, ct);
            return node is null ? null : JsonSerializer.Deserialize<GuardianSet>(node.ToJsonString());
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Gossip / bootstrap / fork-block
    // =====================================================================

    public Task<JsonNode?> SubmitDomainFingerprintAsync(object fp, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "domain-fingerprints", fp, ct);

    public async Task<DomainFingerprint?> GetLatestDomainFingerprintAsync(string domain, CancellationToken ct = default)
    {
        try
        {
            var node = await RequestAsync(HttpMethod.Get,
                $"domain-fingerprints/{Uri.EscapeDataString(domain)}/latest", null, ct);
            return node is null ? null : JsonSerializer.Deserialize<DomainFingerprint>(node.ToJsonString());
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public Task<JsonNode?> SubmitAnchorGossipAsync(object msg, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "anchor-gossip", msg, ct);

    public Task<JsonNode?> SubmitForkBlockAsync(object fb, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "fork-block", fb, ct);

    public Task<JsonNode?> ForkBlockStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "fork-block/status", null, ct);

    public Task<JsonNode?> BootstrapStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "bootstrap/status", null, ct);

    // =====================================================================
    // Guardian recovery + resignation (QDP-0006)
    // =====================================================================

    /// <summary>POST /api/guardian/resign — guardian leaves the set.</summary>
    public Task<JsonNode?> SubmitGuardianResignationAsync(object resignation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/resign", resignation, ct);

    /// <summary>GET /api/guardian/pending-recovery/{quid} — pending recovery for a subject or null.</summary>
    public async Task<JsonNode?> GetPendingRecoveryAsync(string quidId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quidId)) throw new QuidnugValidationException("quidId is required");
        try
        {
            return await RequestAsync(HttpMethod.Get, $"guardian/pending-recovery/{Uri.EscapeDataString(quidId)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c) && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    /// <summary>GET /api/guardian/resignations/{quid} — all resignations for a subject.</summary>
    public Task<JsonNode?> GetGuardianResignationsAsync(string quidId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quidId)) throw new QuidnugValidationException("quidId is required");
        return RequestAsync(HttpMethod.Get, $"guardian/resignations/{Uri.EscapeDataString(quidId)}", null, ct);
    }

    // =====================================================================
    // Gossip + bootstrap
    // =====================================================================

    /// <summary>POST /api/gossip/push-anchor — push-gossip variant (QDP-0005).</summary>
    public Task<JsonNode?> PushAnchorAsync(object msg, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-anchor", msg, ct);

    /// <summary>POST /api/gossip/push-fingerprint — push-gossip variant (QDP-0005).</summary>
    public Task<JsonNode?> PushFingerprintAsync(object fp, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-fingerprint", fp, ct);

    /// <summary>POST /api/nonce-snapshots — publish a K-of-K bootstrap snapshot.</summary>
    public Task<JsonNode?> SubmitNonceSnapshotAsync(object snapshot, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "nonce-snapshots", snapshot, ct);

    /// <summary>GET /api/nonce-snapshots/{domain}/latest.</summary>
    public async Task<JsonNode?> GetLatestNonceSnapshotAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain)) throw new QuidnugValidationException("domain is required");
        try
        {
            return await RequestAsync(HttpMethod.Get, $"nonce-snapshots/{Uri.EscapeDataString(domain)}/latest", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c) && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Peers
    // =====================================================================

    /// <summary>GET /api/peers — peer scoreboard snapshot.</summary>
    public Task<JsonNode?> PeersAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "peers", null, ct);

    /// <summary>GET /api/peers/{nodeQuid} — one peer's record or null if absent.</summary>
    public async Task<JsonNode?> GetPeerAsync(string nodeQuid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(nodeQuid)) throw new QuidnugValidationException("nodeQuid is required");
        try
        {
            return await RequestAsync(HttpMethod.Get, $"peers/{Uri.EscapeDataString(nodeQuid)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) is "PEER_NOT_FOUND" or "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Transactions / blocks / domains / quids
    // =====================================================================

    /// <summary>GET /api/transactions — pending transactions in the mempool.</summary>
    public Task<JsonNode?> PendingTransactionsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "transactions", null, ct);

    /// <summary>GET /api/blocks/tentative/{domain} — tentative blocks for a domain.</summary>
    public Task<JsonNode?> GetTentativeBlocksAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain)) throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get, $"blocks/tentative/{Uri.EscapeDataString(domain)}", null, ct);
    }

    /// <summary>GET /api/domains — every domain this node knows about.</summary>
    public Task<JsonNode?> ListDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "domains", null, ct);

    /// <summary>POST /api/domains — register a trust domain.</summary>
    public Task<JsonNode?> RegisterDomainAsync(string domain, IDictionary<string, object?>? attrs = null, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain)) throw new QuidnugValidationException("domain is required");
        var body = new Dictionary<string, object?>(attrs ?? new Dictionary<string, object?>())
        {
            ["name"] = domain
        };
        return RequestAsync(HttpMethod.Post, "domains", body, ct);
    }

    /// <summary>GET /api/domains/top — top-N most-active domains.</summary>
    public Task<JsonNode?> TopDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "domains/top", null, ct);

    /// <summary>GET /api/domains/{name}/query — query a domain registry directly.</summary>
    /// <param name="domain">Domain name.</param>
    /// <param name="queryType">"identity", "trust", or "title".</param>
    /// <param name="param">Lookup key (for trust queries, "observer:target").</param>
    public Task<JsonNode?> QueryDomainAsync(string domain, string queryType, string param, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain)) throw new QuidnugValidationException("domain is required");
        if (queryType != "identity" && queryType != "trust" && queryType != "title")
            throw new QuidnugValidationException("queryType must be 'identity', 'trust', or 'title'");
        return RequestAsync(HttpMethod.Get,
            $"domains/{Uri.EscapeDataString(domain)}/query?type={Uri.EscapeDataString(queryType)}&param={Uri.EscapeDataString(param ?? "")}",
            null, ct);
    }

    /// <summary>GET /api/node/domains — domains this node currently serves.</summary>
    public Task<JsonNode?> GetNodeDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "node/domains", null, ct);

    /// <summary>POST /api/node/domains — replace the node's managed-domains list.</summary>
    public Task<JsonNode?> UpdateNodeDomainsAsync(IList<string> domains, CancellationToken ct = default)
    {
        return RequestAsync(HttpMethod.Post, "node/domains",
            new Dictionary<string, object?> { ["managedDomains"] = domains }, ct);
    }

    /// <summary>POST /api/gossip/domains — push a domain-gossip message.</summary>
    public Task<JsonNode?> SendDomainGossipAsync(object gossip, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/domains", gossip, ct);

    /// <summary>POST /api/quids — server-side keygen (trusted mode only).</summary>
    public Task<JsonNode?> GenerateQuidAsync(IDictionary<string, object?>? metadata = null, CancellationToken ct = default)
    {
        var body = new Dictionary<string, object?> { ["metadata"] = metadata ?? new Dictionary<string, object?>() };
        return RequestAsync(HttpMethod.Post, "quids", body, ct);
    }

    /// <summary>POST /api/node-advertisements — publish a signed node advertisement.</summary>
    public Task<JsonNode?> CreateNodeAdvertisementAsync(object advertisement, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "node-advertisements", advertisement, ct);

    // =====================================================================
    // Registry queries
    // =====================================================================

    /// <summary>GET /api/registry/trust — paginated/filterable trust registry.</summary>
    public Task<JsonNode?> QueryTrustRegistryAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "registry/trust", null, ct);

    /// <summary>GET /api/registry/identity — paginated/filterable identity registry.</summary>
    public Task<JsonNode?> QueryIdentityRegistryAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "registry/identity", null, ct);

    /// <summary>GET /api/registry/title — paginated/filterable title registry.</summary>
    public Task<JsonNode?> QueryTitleRegistryAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "registry/title", null, ct);

    /// <summary>POST /api/trust/query — multi-quid relational trust query.</summary>
    public Task<JsonNode?> QueryRelationalTrustAsync(object query, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "trust/query", query, ct);

    // =====================================================================
    // IPFS
    // =====================================================================

    /// <summary>POST /api/ipfs/pin — pin raw content; returns the CID.</summary>
    public async Task<string> IpfsPinAsync(byte[] content, CancellationToken ct = default)
    {
        if (content == null || content.Length == 0)
            throw new QuidnugValidationException("content is required");
        using var req = new HttpRequestMessage(HttpMethod.Post, _apiBase + "/ipfs/pin");
        req.Content = new ByteArrayContent(content);
        req.Content.Headers.ContentType = new System.Net.Http.Headers.MediaTypeHeaderValue("application/octet-stream");
        using var resp = await _http.SendAsync(req, ct);
        var env = await ParseEnvelopeAsync(resp, ct);
        var cid = env?["cid"]?.GetValue<string>() ?? env?["value"]?.GetValue<string>();
        if (string.IsNullOrEmpty(cid))
            throw new QuidnugNodeException("IPFS pin response missing cid", 0, null);
        return cid;
    }

    /// <summary>GET /api/ipfs/{cid} — fetch the pinned bytes.</summary>
    public async Task<byte[]> IpfsGetAsync(string cid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(cid)) throw new QuidnugValidationException("cid is required");
        using var req = new HttpRequestMessage(HttpMethod.Get, _apiBase + "/ipfs/" + Uri.EscapeDataString(cid));
        using var resp = await _http.SendAsync(req, ct);
        if ((int)resp.StatusCode >= 400)
            throw new QuidnugNodeException($"IPFS get failed (HTTP {(int)resp.StatusCode})", (int)resp.StatusCode, null);
        return await resp.Content.ReadAsByteArrayAsync(ct);
    }

    // =====================================================================
    // Moderation (QDP-0015)
    // =====================================================================

    /// <summary>POST /api/moderation/actions — submit a signed moderation action.</summary>
    public Task<JsonNode?> CreateModerationActionAsync(object action, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "moderation/actions", action, ct);

    /// <summary>GET /api/moderation/actions/{targetType}/{targetId}.</summary>
    public Task<JsonNode?> GetModerationActionsAsync(string targetType, string targetId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(targetType) || string.IsNullOrEmpty(targetId))
            throw new QuidnugValidationException("targetType and targetId are required");
        return RequestAsync(HttpMethod.Get,
            $"moderation/actions/{Uri.EscapeDataString(targetType)}/{Uri.EscapeDataString(targetId)}",
            null, ct);
    }

    // =====================================================================
    // Audit (QDP-0018)
    // =====================================================================

    /// <summary>GET /api/audit/head — operator's current audit head.</summary>
    public Task<JsonNode?> AuditHeadAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "audit/head", null, ct);

    /// <summary>GET /api/audit/entries — entries after a cursor.</summary>
    public Task<JsonNode?> AuditEntriesAsync(long? since = null, int? limit = null, CancellationToken ct = default)
    {
        var path = "audit/entries";
        var qs = new List<string>();
        if (since.HasValue) qs.Add($"since={since.Value}");
        if (limit.HasValue) qs.Add($"limit={limit.Value}");
        if (qs.Count > 0) path += "?" + string.Join("&", qs);
        return RequestAsync(HttpMethod.Get, path, null, ct);
    }

    /// <summary>GET /api/audit/entry/{sequence} — one entry, or null on 404.</summary>
    public async Task<JsonNode?> AuditEntryAsync(long sequence, CancellationToken ct = default)
    {
        if (sequence < 0) throw new QuidnugValidationException("sequence must be non-negative");
        try
        {
            return await RequestAsync(HttpMethod.Get, $"audit/entry/{sequence}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c) && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Privacy (QDP-0017)
    // =====================================================================

    /// <summary>POST /api/privacy/dsr — submit a Data Subject Request.</summary>
    public Task<JsonNode?> CreateDSRAsync(object request, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/dsr", request, ct);

    /// <summary>GET /api/privacy/dsr/{requestTxId} — status of a DSR or null on 404.</summary>
    public async Task<JsonNode?> GetDSRStatusAsync(string requestTxId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(requestTxId)) throw new QuidnugValidationException("requestTxId is required");
        try
        {
            return await RequestAsync(HttpMethod.Get, $"privacy/dsr/{Uri.EscapeDataString(requestTxId)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c) && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    /// <summary>POST /api/privacy/consent/grants — record an opt-in.</summary>
    public Task<JsonNode?> CreateConsentGrantAsync(object grant, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/consent/grants", grant, ct);

    /// <summary>POST /api/privacy/consent/withdraws — revoke a prior grant.</summary>
    public Task<JsonNode?> CreateConsentWithdrawAsync(object withdraw, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/consent/withdraws", withdraw, ct);

    /// <summary>GET /api/privacy/consent/history?subject={quid}.</summary>
    public Task<JsonNode?> GetConsentHistoryAsync(string subjectQuid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(subjectQuid)) throw new QuidnugValidationException("subjectQuid is required");
        return RequestAsync(HttpMethod.Get,
            $"privacy/consent/history?subject={Uri.EscapeDataString(subjectQuid)}", null, ct);
    }

    /// <summary>POST /api/privacy/restrictions — narrow allowed processing.</summary>
    public Task<JsonNode?> CreateProcessingRestrictionAsync(object restriction, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/restrictions", restriction, ct);

    /// <summary>GET /api/privacy/restrictions/{subjectQuid}.</summary>
    public Task<JsonNode?> GetRestrictionsForSubjectAsync(string subjectQuid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(subjectQuid)) throw new QuidnugValidationException("subjectQuid is required");
        return RequestAsync(HttpMethod.Get,
            $"privacy/restrictions/{Uri.EscapeDataString(subjectQuid)}", null, ct);
    }

    /// <summary>POST /api/privacy/compliance — operator's compliance attestation.</summary>
    public Task<JsonNode?> CreateDSRComplianceAsync(object compliance, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/compliance", compliance, ct);

    // =====================================================================
    // Discovery (QDP-0014)
    // =====================================================================

    /// <summary>GET /api/v2/discovery/domain/{name}.</summary>
    public Task<JsonNode?> DiscoverDomainAsync(string name, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(name)) throw new QuidnugValidationException("name is required");
        return RequestAsync(HttpMethod.Get, $"v2/discovery/domain/{Uri.EscapeDataString(name)}", null, ct);
    }

    /// <summary>GET /api/v2/discovery/node/{quid}.</summary>
    public Task<JsonNode?> DiscoverNodeAsync(string quid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quid)) throw new QuidnugValidationException("quid is required");
        return RequestAsync(HttpMethod.Get, $"v2/discovery/node/{Uri.EscapeDataString(quid)}", null, ct);
    }

    /// <summary>GET /api/v2/discovery/operator/{quid}.</summary>
    public Task<JsonNode?> DiscoverOperatorAsync(string quid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quid)) throw new QuidnugValidationException("quid is required");
        return RequestAsync(HttpMethod.Get, $"v2/discovery/operator/{Uri.EscapeDataString(quid)}", null, ct);
    }

    /// <summary>GET /api/v2/discovery/quids.</summary>
    public Task<JsonNode?> DiscoverQuidsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "v2/discovery/quids", null, ct);

    /// <summary>GET /api/v2/discovery/trusted-quids.</summary>
    public Task<JsonNode?> DiscoverTrustedQuidsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "v2/discovery/trusted-quids", null, ct);

    // =====================================================================
    // DNS attestation (QDP-0023)
    // =====================================================================

    public Task<JsonNode?> SubmitDNSClaimAsync(object claim, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/dns/claim", claim, ct);

    public Task<JsonNode?> SubmitDNSChallengeAsync(object challenge, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/dns/challenge", challenge, ct);

    public Task<JsonNode?> SubmitDNSAttestationAsync(object attestation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/dns/attestation", attestation, ct);

    public Task<JsonNode?> SubmitDNSRenewalAsync(object renewal, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/dns/renewal", renewal, ct);

    public Task<JsonNode?> SubmitDNSRevocationAsync(object revocation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/dns/revocation", revocation, ct);

    public Task<JsonNode?> SubmitAuthorityDelegateAsync(object delegateTx, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/dns/delegate", delegateTx, ct);

    public Task<JsonNode?> SubmitAuthorityDelegateRevocationAsync(object revocation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/dns/delegate-revocation", revocation, ct);

    public Task<JsonNode?> GetDNSAttestationsAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain)) throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get, $"v2/dns/attestations/{Uri.EscapeDataString(domain)}", null, ct);
    }

    public Task<JsonNode?> GetDNSAttestationsWeightedAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain)) throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get, $"v2/dns/attestations/{Uri.EscapeDataString(domain)}/weighted", null, ct);
    }

    public Task<JsonNode?> ResolveDNSRecordAsync(string domain, string recordType, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain) || string.IsNullOrEmpty(recordType))
            throw new QuidnugValidationException("domain and recordType are required");
        return RequestAsync(HttpMethod.Get,
            $"v2/dns/resolve/{Uri.EscapeDataString(domain)}/{Uri.EscapeDataString(recordType)}",
            null, ct);
    }

    // =====================================================================
    // HTTP plumbing
    // =====================================================================

    private async Task<JsonNode?> RequestAsync(
        HttpMethod method, string path, object? body, CancellationToken ct)
    {
        bool retry = method == HttpMethod.Get;
        int attempts = retry ? _maxRetries + 1 : 1;
        Exception? lastError = null;

        for (int attempt = 0; attempt < attempts; attempt++)
        {
            using var req = new HttpRequestMessage(method, _apiBase + "/" + path.TrimStart('/'));
            req.Headers.Accept.Clear();
            req.Headers.Accept.ParseAdd("application/json");
            if (body is not null)
            {
                req.Content = JsonContent.Create(body);
            }

            HttpResponseMessage resp;
            try
            {
                resp = await _http.SendAsync(req, ct);
            }
            catch (Exception e) when (e is HttpRequestException or TaskCanceledException)
            {
                lastError = e;
                if (attempt < attempts - 1)
                {
                    await SleepBackoff(attempt, null, ct);
                    continue;
                }
                throw new QuidnugNodeException($"network error on {method} {path}: {e.Message}", e);
            }

            if (((int)resp.StatusCode >= 500 || resp.StatusCode == (HttpStatusCode)429) && attempt < attempts - 1)
            {
                string? retryAfter = null;
                if (resp.Headers.RetryAfter?.Delta is TimeSpan delta)
                    retryAfter = ((int)delta.TotalSeconds).ToString();
                else if (resp.Headers.TryGetValues("Retry-After", out var ra))
                    retryAfter = ra.FirstOrDefault();
                resp.Dispose();
                await SleepBackoff(attempt, retryAfter, ct);
                continue;
            }

            return await ParseEnvelopeAsync(resp, ct);
        }
        throw new QuidnugNodeException($"{method} {path}: retries exhausted",
            lastError ?? new Exception("unknown"));
    }

    private async Task<JsonNode?> ParseEnvelopeAsync(HttpResponseMessage resp, CancellationToken ct)
    {
        using (resp)
        {
            string body = await resp.Content.ReadAsStringAsync(ct);
            JsonNode? env;
            try
            {
                env = string.IsNullOrEmpty(body) ? null : JsonNode.Parse(body);
            }
            catch (JsonException)
            {
                throw new QuidnugNodeException($"non-JSON response (HTTP {(int)resp.StatusCode})",
                    (int)resp.StatusCode, body);
            }
            if (env is null)
                throw new QuidnugNodeException($"empty response (HTTP {(int)resp.StatusCode})",
                    (int)resp.StatusCode, body);

            bool ok = env["success"]?.GetValue<bool>() ?? false;
            if (ok) return env["data"]?.DeepClone();

            string code = env["error"]?["code"]?.GetValue<string>() ?? "UNKNOWN_ERROR";
            string message = env["error"]?["message"]?.GetValue<string>() ?? $"HTTP {(int)resp.StatusCode}";
            var details = new Dictionary<string, object?> { ["code"] = code };

            if (resp.StatusCode == HttpStatusCode.ServiceUnavailable || UnavailableCodes.Contains(code))
                throw new QuidnugUnavailableException(message, details);
            if (resp.StatusCode == HttpStatusCode.Conflict || ConflictCodes.Contains(code))
                throw new QuidnugConflictException(message, details);
            if ((int)resp.StatusCode >= 400 && (int)resp.StatusCode < 500)
                throw new QuidnugValidationException(message, details);
            throw new QuidnugNodeException(message, (int)resp.StatusCode, body);
        }
    }

    private async Task SleepBackoff(int attempt, string? retryAfter, CancellationToken ct)
    {
        double delayMs;
        if (retryAfter is not null && int.TryParse(retryAfter.Trim(), out int secs) && secs >= 0)
        {
            delayMs = secs * 1000.0;
        }
        else
        {
            delayMs = _retryBaseDelay.TotalMilliseconds * Math.Pow(2, attempt)
                    + Random.Shared.Next(0, 100);
        }
        await Task.Delay(TimeSpan.FromMilliseconds(Math.Min(delayMs, 60_000)), ct);
    }

    private static void SignIntoTx(Quid signer, Dictionary<string, object?> tx)
    {
        byte[] signable = CanonicalBytes.Of(tx, "signature", "txId");
        tx["signature"] = signer.Sign(signable);
    }

    private static void RequireSigner(Quid? signer)
    {
        if (signer is null || !signer.HasPrivateKey)
            throw new QuidnugValidationException("signer must have a private key");
    }
}
