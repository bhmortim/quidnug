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

    public Task<JsonNode?> NodesAsync(int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, AppendQuery("nodes",
            ("limit", limit?.ToString()), ("offset", offset?.ToString())), null, ct);

    public Task<JsonNode?> BlocksAsync(int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, AppendQuery("blocks",
            ("limit", limit?.ToString()), ("offset", offset?.ToString())), null, ct);

    /// <summary>GET /api/transactions — pending tx pool.</summary>
    public Task<JsonNode?> GetPendingTransactionsAsync(
        int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, AppendQuery("transactions",
            ("limit", limit?.ToString()), ("offset", offset?.ToString())), null, ct);

    /// <summary>GET /api/blocks/tentative/{domain} — proposed-but-not-sealed blocks.</summary>
    public Task<JsonNode?> GetTentativeBlocksAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get,
            "blocks/tentative/" + Uri.EscapeDataString(domain), null, ct);
    }

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

    /// <summary>GET /api/registry/identity — paginated identity dump / domain filter.</summary>
    public Task<JsonNode?> QueryIdentityRegistryAsync(
        int? limit = null, int? offset = null, string? domain = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, AppendQuery("registry/identity",
            ("limit", limit?.ToString()),
            ("offset", offset?.ToString()),
            ("domain", domain)), null, ct);

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

    /// <summary>POST /api/trust/query — structured relational-trust query.</summary>
    public async Task<TrustResult> QueryRelationalTrustAsync(
        string observer, string target, string domain, int maxDepth = 5,
        CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(observer) || string.IsNullOrEmpty(target))
            throw new QuidnugValidationException("observer and target are required");
        var body = new Dictionary<string, object?>
        {
            ["observer"] = observer,
            ["target"] = target,
            ["domain"] = domain,
            ["maxDepth"] = maxDepth,
        };
        var node = await RequestAsync(HttpMethod.Post, "trust/query", body, ct)
                   ?? throw new QuidnugNodeException("empty response", 200, null);
        return JsonSerializer.Deserialize<TrustResult>(node.ToJsonString())
               ?? throw new QuidnugNodeException("decode trust result failed", 200, node.ToJsonString());
    }

    /// <summary>GET /api/registry/trust — paginated trust-edge listing.</summary>
    public Task<JsonNode?> QueryTrustRegistryAsync(
        int? limit = null, int? offset = null, string? truster = null, string? trustee = null,
        CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, AppendQuery("registry/trust",
            ("truster", truster),
            ("trustee", trustee),
            ("limit", limit?.ToString()),
            ("offset", offset?.ToString())), null, ct);

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

    /// <summary>GET /api/registry/title — paginated title listing.</summary>
    public Task<JsonNode?> QueryTitleRegistryAsync(
        int? limit = null, int? offset = null, string? assetId = null, string? ownerId = null,
        CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, AppendQuery("registry/title",
            ("asset_id", assetId),
            ("owner_id", ownerId),
            ("limit", limit?.ToString()),
            ("offset", offset?.ToString())), null, ct);

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

    /// <summary>POST /api/guardian/resign — guardian leaves the set.</summary>
    public Task<JsonNode?> SubmitGuardianResignationAsync(object resignation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/resign", resignation, ct);

    /// <summary>GET /api/guardian/resignations/{quid} — pending resignations for a subject.</summary>
    public async Task<List<JsonNode>> GetGuardianResignationsAsync(string quidId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quidId))
            throw new QuidnugValidationException("quidId is required");
        var node = await RequestAsync(HttpMethod.Get,
            "guardian/resignations/" + Uri.EscapeDataString(quidId), null, ct);
        var result = new List<JsonNode>();
        if (node is not JsonObject obj) return result;
        var arr = obj["data"] as JsonArray ?? obj["resignations"] as JsonArray;
        if (arr is null) return result;
        foreach (var item in arr)
        {
            if (item is not null) result.Add(item.DeepClone());
        }
        return result;
    }

    /// <summary>GET /api/guardian/pending-recovery/{quid} — in-flight recovery state, if any.</summary>
    public async Task<JsonNode?> GetPendingRecoveryAsync(string quidId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quidId))
            throw new QuidnugValidationException("quidId is required");
        try
        {
            return await RequestAsync(HttpMethod.Get,
                "guardian/pending-recovery/" + Uri.EscapeDataString(quidId), null, ct);
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

    /// <summary>POST /api/gossip/push-anchor — push-gossip variant for anchors (QDP-0005).</summary>
    public Task<JsonNode?> PushAnchorAsync(object msg, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-anchor", msg, ct);

    /// <summary>POST /api/gossip/push-fingerprint — push-gossip variant for fingerprints (QDP-0005).</summary>
    public Task<JsonNode?> PushFingerprintAsync(object fp, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-fingerprint", fp, ct);

    public Task<JsonNode?> SubmitForkBlockAsync(object fb, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "fork-block", fb, ct);

    public Task<JsonNode?> ForkBlockStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "fork-block/status", null, ct);

    public Task<JsonNode?> BootstrapStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "bootstrap/status", null, ct);

    /// <summary>POST /api/nonce-snapshots — publish a K-of-K snapshot.</summary>
    public Task<JsonNode?> SubmitNonceSnapshotAsync(object snapshot, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "nonce-snapshots", snapshot, ct);

    /// <summary>GET /api/nonce-snapshots/{domain}/latest — latest signed snapshot for a domain.</summary>
    public async Task<NonceSnapshot?> GetLatestNonceSnapshotAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        try
        {
            var node = await RequestAsync(HttpMethod.Get,
                "nonce-snapshots/" + Uri.EscapeDataString(domain) + "/latest", null, ct);
            return node is null ? null : JsonSerializer.Deserialize<NonceSnapshot>(node.ToJsonString());
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Domains
    // =====================================================================

    /// <summary>GET /api/domains — list registered trust domains on this node.</summary>
    public Task<JsonNode?> ListDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "domains", null, ct);

    /// <summary>POST /api/domains — register a new trust domain.</summary>
    public Task<JsonNode?> RegisterDomainAsync(string name, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(name))
            throw new QuidnugValidationException("name is required");
        return RequestAsync(HttpMethod.Post, "domains", new Dictionary<string, object?> { ["name"] = name }, ct);
    }

    /// <summary>
    /// Register the domain if it does not already exist; tolerates an
    /// "already exists" rejection. Idempotent — safe for demo and
    /// bootstrap scripts.
    /// </summary>
    public async Task<JsonNode?> EnsureDomainAsync(string name, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(name))
            throw new QuidnugValidationException("name is required");
        try
        {
            return await RegisterDomainAsync(name, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Message.Contains("already exists",
                                                        StringComparison.OrdinalIgnoreCase))
        {
            var ack = new JsonObject
            {
                ["status"] = "success",
                ["domain"] = name,
                ["message"] = "trust domain already exists",
            };
            return ack;
        }
        catch (QuidnugConflictException ex) when (
            (ex.Details.TryGetValue("code", out var c) && (c as string) == "ALREADY_EXISTS")
            || ex.Message.Contains("already exists", StringComparison.OrdinalIgnoreCase))
        {
            var ack = new JsonObject
            {
                ["status"] = "success",
                ["domain"] = name,
                ["message"] = "trust domain already exists",
            };
            return ack;
        }
    }

    /// <summary>GET /api/domains/{name}/query — domain-scoped registry lookup.</summary>
    public Task<JsonNode?> QueryDomainAsync(
        string name, string type, string param, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(name))
            throw new QuidnugValidationException("name is required");
        if (string.IsNullOrEmpty(type))
            throw new QuidnugValidationException("type is required");
        return RequestAsync(HttpMethod.Get, AppendQuery(
            "domains/" + Uri.EscapeDataString(name) + "/query",
            ("type", type), ("param", param)), null, ct);
    }

    /// <summary>GET /api/node/domains — domains this node is configured to manage.</summary>
    public Task<JsonNode?> GetNodeDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "node/domains", null, ct);

    /// <summary>POST /api/node/domains — replace the managed-domains list on this node.</summary>
    public Task<JsonNode?> UpdateNodeDomainsAsync(IEnumerable<string> domains, CancellationToken ct = default)
    {
        if (domains is null)
            throw new QuidnugValidationException("domains is required");
        var body = new Dictionary<string, object?>
        {
            ["managedDomains"] = domains.ToList(),
        };
        return RequestAsync(HttpMethod.Post, "node/domains", body, ct);
    }

    // =====================================================================
    // IPFS
    // =====================================================================

    /// <summary>POST /api/ipfs/pin — pin raw bytes; returns the resulting CID.</summary>
    public async Task<string> IpfsPinAsync(byte[] content, CancellationToken ct = default)
    {
        if (content is null)
            throw new QuidnugValidationException("content is required");

        using var req = new HttpRequestMessage(HttpMethod.Post, _apiBase + "/ipfs/pin");
        req.Headers.Accept.Clear();
        req.Headers.Accept.ParseAdd("application/json");
        req.Content = new ByteArrayContent(content);
        req.Content.Headers.ContentType =
            new MediaTypeHeaderValue("application/octet-stream");

        HttpResponseMessage resp;
        try
        {
            resp = await _http.SendAsync(req, ct);
        }
        catch (Exception e) when (e is HttpRequestException or TaskCanceledException)
        {
            throw new QuidnugNodeException($"network error on POST ipfs/pin: {e.Message}", e);
        }

        var node = await ParseEnvelopeAsync(resp, ct);
        string? cid =
            (node as JsonObject)?["cid"]?.GetValue<string>()
            ?? (node as JsonObject)?["value"]?.GetValue<string>();
        if (string.IsNullOrEmpty(cid))
            throw new QuidnugNodeException("IPFS pin response missing cid", 200, node?.ToJsonString());
        return cid;
    }

    /// <summary>GET /api/ipfs/{cid} — fetch raw bytes. Returns the content body verbatim
    /// (no envelope unwrap, as the node serves IPFS responses as-is).</summary>
    public async Task<byte[]> IpfsGetAsync(string cid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(cid))
            throw new QuidnugValidationException("cid is required");
        using var req = new HttpRequestMessage(HttpMethod.Get,
            _apiBase + "/ipfs/" + Uri.EscapeDataString(cid));
        HttpResponseMessage resp;
        try
        {
            resp = await _http.SendAsync(req, ct);
        }
        catch (Exception e) when (e is HttpRequestException or TaskCanceledException)
        {
            throw new QuidnugNodeException($"network error on GET ipfs/{cid}: {e.Message}", e);
        }
        using (resp)
        {
            if ((int)resp.StatusCode >= 400)
            {
                string body = await resp.Content.ReadAsStringAsync(ct);
                throw new QuidnugNodeException(
                    $"IPFS fetch failed (HTTP {(int)resp.StatusCode})", (int)resp.StatusCode, body);
            }
            return await resp.Content.ReadAsByteArrayAsync(ct);
        }
    }

    // =====================================================================
    // Commit-wait helpers
    // =====================================================================

    /// <summary>
    /// Block until the identity with <paramref name="quidId"/> is visible
    /// in the committed registry, or throw <see cref="TimeoutException"/>.
    /// </summary>
    public async Task<IdentityRecord> WaitForIdentityAsync(
        string quidId, string? domain, TimeSpan timeout, TimeSpan pollInterval,
        CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quidId))
            throw new QuidnugValidationException("quidId is required");
        if (pollInterval <= TimeSpan.Zero)
            throw new QuidnugValidationException("pollInterval must be > 0");
        var deadline = DateTime.UtcNow + timeout;
        while (true)
        {
            var rec = await GetIdentityAsync(quidId, domain, ct);
            if (rec is not null) return rec;
            if (DateTime.UtcNow >= deadline)
                throw new TimeoutException(
                    $"identity {quidId} did not commit within {timeout.TotalSeconds}s");
            var remaining = deadline - DateTime.UtcNow;
            var sleep = remaining < pollInterval ? remaining : pollInterval;
            if (sleep > TimeSpan.Zero) await Task.Delay(sleep, ct);
        }
    }

    /// <summary>
    /// Block until every quid in <paramref name="quidIds"/> is committed.
    /// Shares a single deadline across all ids.
    /// </summary>
    public async Task WaitForIdentitiesAsync(
        IEnumerable<string> quidIds, string? domain, TimeSpan timeout, TimeSpan pollInterval,
        CancellationToken ct = default)
    {
        if (quidIds is null)
            throw new QuidnugValidationException("quidIds is required");
        var deadline = DateTime.UtcNow + timeout;
        foreach (var qid in quidIds)
        {
            var remaining = deadline - DateTime.UtcNow;
            if (remaining <= TimeSpan.Zero)
                throw new TimeoutException(
                    $"identities not all committed within {timeout.TotalSeconds}s (blocked on {qid})");
            await WaitForIdentityAsync(qid, domain, remaining, pollInterval, ct);
        }
    }

    /// <summary>
    /// Block until the title with <paramref name="assetId"/> is visible
    /// in the committed registry, or throw <see cref="TimeoutException"/>.
    /// </summary>
    public async Task<Title> WaitForTitleAsync(
        string assetId, string? domain, TimeSpan timeout, TimeSpan pollInterval,
        CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(assetId))
            throw new QuidnugValidationException("assetId is required");
        if (pollInterval <= TimeSpan.Zero)
            throw new QuidnugValidationException("pollInterval must be > 0");
        var deadline = DateTime.UtcNow + timeout;
        while (true)
        {
            var rec = await GetTitleAsync(assetId, domain, ct);
            if (rec is not null) return rec;
            if (DateTime.UtcNow >= deadline)
                throw new TimeoutException(
                    $"title {assetId} did not commit within {timeout.TotalSeconds}s");
            var remaining = deadline - DateTime.UtcNow;
            var sleep = remaining < pollInterval ? remaining : pollInterval;
            if (sleep > TimeSpan.Zero) await Task.Delay(sleep, ct);
        }
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

    /// <summary>
    /// Append URL-encoded query parameters to <paramref name="path"/>,
    /// dropping pairs whose value is null or empty (matches Python's
    /// <c>_strip_none</c> + Go's <c>omitempty</c> behavior).
    /// </summary>
    private static string AppendQuery(string path, params (string Key, string? Value)[] pairs)
    {
        var keep = pairs.Where(p => !string.IsNullOrEmpty(p.Value)).ToList();
        if (keep.Count == 0) return path;
        var sb = new System.Text.StringBuilder(path);
        sb.Append(path.Contains('?') ? '&' : '?');
        for (int i = 0; i < keep.Count; i++)
        {
            if (i > 0) sb.Append('&');
            sb.Append(Uri.EscapeDataString(keep[i].Key));
            sb.Append('=');
            sb.Append(Uri.EscapeDataString(keep[i].Value!));
        }
        return sb.ToString();
    }
}
