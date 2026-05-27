using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace Quidnug.Client;

/// <summary>
/// Strongly-typed HTTP client for a Quidnug node.
///
/// <para>Covers the full v2 protocol surface (QDPs 0001–0010) plus the v3
/// extensions (peers, audit, moderation, privacy, discovery, DNS
/// attestation; QDPs 0011, 0014, 0015, 0017, 0018, 0023). Thread-safe;
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
    // Guardians (QDP-0002) — under /api/v2/guardian/*
    // =====================================================================

    public Task<JsonNode?> SubmitGuardianSetUpdateAsync(object update, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/guardian/set-update", update, ct);

    public Task<JsonNode?> SubmitRecoveryInitAsync(object init, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/guardian/recovery/init", init, ct);

    public Task<JsonNode?> SubmitRecoveryVetoAsync(object veto, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/guardian/recovery/veto", veto, ct);

    public Task<JsonNode?> SubmitRecoveryCommitAsync(object commit, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/guardian/recovery/commit", commit, ct);

    public async Task<GuardianSet?> GetGuardianSetAsync(string quidId, CancellationToken ct = default)
    {
        try
        {
            var node = await RequestAsync(HttpMethod.Get, $"v2/guardian/set/{Uri.EscapeDataString(quidId)}", null, ct);
            return node is null ? null : JsonSerializer.Deserialize<GuardianSet>(node.ToJsonString());
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Gossip / bootstrap / fork-block — under /api/v2/*
    // =====================================================================

    public Task<JsonNode?> SubmitDomainFingerprintAsync(object fp, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/domain-fingerprints", fp, ct);

    public async Task<DomainFingerprint?> GetLatestDomainFingerprintAsync(string domain, CancellationToken ct = default)
    {
        try
        {
            var node = await RequestAsync(HttpMethod.Get,
                $"v2/domain-fingerprints/{Uri.EscapeDataString(domain)}/latest", null, ct);
            return node is null ? null : JsonSerializer.Deserialize<DomainFingerprint>(node.ToJsonString());
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public Task<JsonNode?> SubmitAnchorGossipAsync(object msg, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/anchor-gossip", msg, ct);

    public Task<JsonNode?> SubmitForkBlockAsync(object fb, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "v2/fork-block", fb, ct);

    public Task<JsonNode?> ForkBlockStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "v2/fork-block/status", null, ct);

    public Task<JsonNode?> BootstrapStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "v2/bootstrap/status", null, ct);

    // =====================================================================
    // v3 — Peers (QDP-0011 scoreboard) — /api/peers
    // =====================================================================

    /// <summary>GET /api/peers — peer scoreboard.</summary>
    public Task<JsonNode?> GetPeersAsync(int? limit = null, int? offset = null, CancellationToken ct = default)
    {
        var path = "peers" + BuildQueryString(("limit", limit), ("offset", offset));
        return RequestAsync(HttpMethod.Get, path, null, ct);
    }

    /// <summary>GET /api/peers/{nodeQuid} — per-peer score breakdown. Returns null on NOT_FOUND.</summary>
    public async Task<JsonNode?> GetPeerAsync(string nodeQuid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(nodeQuid))
            throw new QuidnugValidationException("nodeQuid is required");
        try
        {
            return await RequestAsync(HttpMethod.Get, $"peers/{Uri.EscapeDataString(nodeQuid)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // v3 — Node advertisements — /api/node-advertisements
    // =====================================================================

    /// <summary>POST /api/node-advertisements — submit a signed node advertisement.</summary>
    public Task<JsonNode?> SubmitNodeAdvertisementAsync(object ad, CancellationToken ct = default)
    {
        if (ad is null) throw new QuidnugValidationException("advertisement is required");
        return RequestAsync(HttpMethod.Post, "node-advertisements", ad, ct);
    }

    // =====================================================================
    // v3 — Domain registry extras — /api/domains, /api/gossip/domains, /api/blocks
    // =====================================================================

    /// <summary>GET /api/domains/top — top domains by activity.</summary>
    public Task<JsonNode?> GetTopDomainsAsync(int? limit = null, int? offset = null, CancellationToken ct = default)
    {
        var path = "domains/top" + BuildQueryString(("limit", limit), ("offset", offset));
        return RequestAsync(HttpMethod.Get, path, null, ct);
    }

    /// <summary>POST /api/gossip/domains — submit a domain gossip message.</summary>
    public Task<JsonNode?> SubmitDomainGossipAsync(object msg, CancellationToken ct = default)
    {
        if (msg is null) throw new QuidnugValidationException("message is required");
        return RequestAsync(HttpMethod.Post, "gossip/domains", msg, ct);
    }

    /// <summary>GET /api/blocks/tentative/{domain} — tentative blocks pending finalization.</summary>
    public Task<JsonNode?> GetTentativeBlocksAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get, $"blocks/tentative/{Uri.EscapeDataString(domain)}", null, ct);
    }

    // =====================================================================
    // v3 — Moderation (QDP-0015) — /api/moderation/*
    // =====================================================================

    /// <summary>POST /api/moderation/actions — submit a moderation action.</summary>
    public Task<JsonNode?> SubmitModerationActionAsync(object action, CancellationToken ct = default)
    {
        if (action is null) throw new QuidnugValidationException("action is required");
        return RequestAsync(HttpMethod.Post, "moderation/actions", action, ct);
    }

    /// <summary>GET /api/moderation/actions/{targetType}/{targetId} — moderation history.</summary>
    public Task<JsonNode?> GetModerationActionsAsync(string targetType, string targetId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(targetType) || string.IsNullOrEmpty(targetId))
            throw new QuidnugValidationException("targetType and targetId are required");
        return RequestAsync(HttpMethod.Get,
            $"moderation/actions/{Uri.EscapeDataString(targetType)}/{Uri.EscapeDataString(targetId)}", null, ct);
    }

    // =====================================================================
    // v3 — Audit (QDP-0018) — /api/audit/*
    // =====================================================================

    /// <summary>GET /api/audit/head — latest audit log head.</summary>
    public Task<JsonNode?> GetAuditHeadAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "audit/head", null, ct);

    /// <summary>GET /api/audit/entries — paginated audit entries.</summary>
    public Task<JsonNode?> GetAuditEntriesAsync(long? since = null, int? limit = null, CancellationToken ct = default)
    {
        var path = "audit/entries" + BuildQueryString(("since", since), ("limit", limit));
        return RequestAsync(HttpMethod.Get, path, null, ct);
    }

    /// <summary>GET /api/audit/entry/{sequence} — single audit entry. Returns null on NOT_FOUND.</summary>
    public async Task<JsonNode?> GetAuditEntryAsync(long sequence, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get, $"audit/entry/{sequence}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // v3 — Privacy / DSR (QDP-0017) — /api/privacy/*
    // =====================================================================

    /// <summary>POST /api/privacy/dsr — submit a data subject rights request.</summary>
    public Task<JsonNode?> SubmitDSRAsync(object request, CancellationToken ct = default)
    {
        if (request is null) throw new QuidnugValidationException("request is required");
        return RequestAsync(HttpMethod.Post, "privacy/dsr", request, ct);
    }

    /// <summary>GET /api/privacy/dsr/{requestTxId} — DSR status. Returns null on NOT_FOUND.</summary>
    public async Task<JsonNode?> GetDSRStatusAsync(string requestTxId, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(requestTxId))
            throw new QuidnugValidationException("requestTxId is required");
        try
        {
            return await RequestAsync(HttpMethod.Get,
                $"privacy/dsr/{Uri.EscapeDataString(requestTxId)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    /// <summary>POST /api/privacy/consent/grants — record a consent grant.</summary>
    public Task<JsonNode?> GrantConsentAsync(object grant, CancellationToken ct = default)
    {
        if (grant is null) throw new QuidnugValidationException("grant is required");
        return RequestAsync(HttpMethod.Post, "privacy/consent/grants", grant, ct);
    }

    /// <summary>POST /api/privacy/consent/withdraws — record a consent withdrawal.</summary>
    public Task<JsonNode?> WithdrawConsentAsync(object withdraw, CancellationToken ct = default)
    {
        if (withdraw is null) throw new QuidnugValidationException("withdraw is required");
        return RequestAsync(HttpMethod.Post, "privacy/consent/withdraws", withdraw, ct);
    }

    /// <summary>GET /api/privacy/consent/history — paginated consent history.</summary>
    public Task<JsonNode?> GetConsentHistoryAsync(
        int? limit = null, int? offset = null, string? subjectQuid = null,
        CancellationToken ct = default)
    {
        var path = "privacy/consent/history"
                   + BuildQueryString(("limit", limit), ("offset", offset), ("subjectQuid", subjectQuid));
        return RequestAsync(HttpMethod.Get, path, null, ct);
    }

    /// <summary>POST /api/privacy/restrictions — create a processing restriction.</summary>
    public Task<JsonNode?> CreateProcessingRestrictionAsync(object restriction, CancellationToken ct = default)
    {
        if (restriction is null) throw new QuidnugValidationException("restriction is required");
        return RequestAsync(HttpMethod.Post, "privacy/restrictions", restriction, ct);
    }

    /// <summary>GET /api/privacy/restrictions/{subjectQuid} — list restrictions for a subject.</summary>
    public Task<JsonNode?> GetProcessingRestrictionsAsync(string subjectQuid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(subjectQuid))
            throw new QuidnugValidationException("subjectQuid is required");
        return RequestAsync(HttpMethod.Get,
            $"privacy/restrictions/{Uri.EscapeDataString(subjectQuid)}", null, ct);
    }

    /// <summary>POST /api/privacy/compliance — submit a DSR compliance attestation.</summary>
    public Task<JsonNode?> SubmitDSRComplianceAsync(object compliance, CancellationToken ct = default)
    {
        if (compliance is null) throw new QuidnugValidationException("compliance is required");
        return RequestAsync(HttpMethod.Post, "privacy/compliance", compliance, ct);
    }

    // =====================================================================
    // v3 — Discovery (QDP-0014) — /api/v2/discovery/*
    // =====================================================================

    /// <summary>GET /api/v2/discovery/domain/{name}. Returns null on NOT_FOUND.</summary>
    public async Task<JsonNode?> GetDiscoveryDomainAsync(string name, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(name))
            throw new QuidnugValidationException("name is required");
        try
        {
            return await RequestAsync(HttpMethod.Get,
                $"v2/discovery/domain/{Uri.EscapeDataString(name)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    /// <summary>GET /api/v2/discovery/node/{quid}. Returns null on NOT_FOUND.</summary>
    public async Task<JsonNode?> GetDiscoveryNodeAsync(string quid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quid))
            throw new QuidnugValidationException("quid is required");
        try
        {
            return await RequestAsync(HttpMethod.Get,
                $"v2/discovery/node/{Uri.EscapeDataString(quid)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    /// <summary>GET /api/v2/discovery/operator/{quid}. Returns null on NOT_FOUND.</summary>
    public async Task<JsonNode?> GetDiscoveryOperatorAsync(string quid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quid))
            throw new QuidnugValidationException("quid is required");
        try
        {
            return await RequestAsync(HttpMethod.Get,
                $"v2/discovery/operator/{Uri.EscapeDataString(quid)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    /// <summary>GET /api/v2/discovery/quids — discoverable QUIDs filtered by arbitrary query params.</summary>
    public Task<JsonNode?> DiscoveryQuidsAsync(
        IDictionary<string, string>? parameters = null, CancellationToken ct = default)
    {
        var path = "v2/discovery/quids" + BuildQueryString(parameters);
        return RequestAsync(HttpMethod.Get, path, null, ct);
    }

    /// <summary>GET /api/v2/discovery/trusted-quids — discoverable QUIDs filtered by trust.</summary>
    public Task<JsonNode?> DiscoveryTrustedQuidsAsync(
        IDictionary<string, string>? parameters = null, CancellationToken ct = default)
    {
        var path = "v2/discovery/trusted-quids" + BuildQueryString(parameters);
        return RequestAsync(HttpMethod.Get, path, null, ct);
    }

    // =====================================================================
    // v3 — DNS attestation (QDP-0023) — /api/v2/dns/*
    // =====================================================================

    /// <summary>POST /api/v2/dns/claim.</summary>
    public Task<JsonNode?> SubmitDNSClaimAsync(object claim, CancellationToken ct = default)
    {
        if (claim is null) throw new QuidnugValidationException("claim is required");
        return RequestAsync(HttpMethod.Post, "v2/dns/claim", claim, ct);
    }

    /// <summary>POST /api/v2/dns/challenge.</summary>
    public Task<JsonNode?> SubmitDNSChallengeAsync(object challenge, CancellationToken ct = default)
    {
        if (challenge is null) throw new QuidnugValidationException("challenge is required");
        return RequestAsync(HttpMethod.Post, "v2/dns/challenge", challenge, ct);
    }

    /// <summary>POST /api/v2/dns/attestation.</summary>
    public Task<JsonNode?> SubmitDNSAttestationAsync(object attestation, CancellationToken ct = default)
    {
        if (attestation is null) throw new QuidnugValidationException("attestation is required");
        return RequestAsync(HttpMethod.Post, "v2/dns/attestation", attestation, ct);
    }

    /// <summary>POST /api/v2/dns/renewal.</summary>
    public Task<JsonNode?> SubmitDNSRenewalAsync(object renewal, CancellationToken ct = default)
    {
        if (renewal is null) throw new QuidnugValidationException("renewal is required");
        return RequestAsync(HttpMethod.Post, "v2/dns/renewal", renewal, ct);
    }

    /// <summary>POST /api/v2/dns/revocation.</summary>
    public Task<JsonNode?> SubmitDNSRevocationAsync(object revocation, CancellationToken ct = default)
    {
        if (revocation is null) throw new QuidnugValidationException("revocation is required");
        return RequestAsync(HttpMethod.Post, "v2/dns/revocation", revocation, ct);
    }

    /// <summary>POST /api/v2/dns/delegate.</summary>
    public Task<JsonNode?> SubmitDNSDelegateAsync(object delegation, CancellationToken ct = default)
    {
        if (delegation is null) throw new QuidnugValidationException("delegate is required");
        return RequestAsync(HttpMethod.Post, "v2/dns/delegate", delegation, ct);
    }

    /// <summary>POST /api/v2/dns/delegate-revocation.</summary>
    public Task<JsonNode?> SubmitDNSDelegateRevocationAsync(object revocation, CancellationToken ct = default)
    {
        if (revocation is null) throw new QuidnugValidationException("revocation is required");
        return RequestAsync(HttpMethod.Post, "v2/dns/delegate-revocation", revocation, ct);
    }

    /// <summary>GET /api/v2/dns/attestations/{domain} — raw attestations for a domain.</summary>
    public Task<JsonNode?> GetDNSAttestationsAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get,
            $"v2/dns/attestations/{Uri.EscapeDataString(domain)}", null, ct);
    }

    /// <summary>GET /api/v2/dns/attestations/{domain}/weighted — trust-weighted attestations.</summary>
    public Task<JsonNode?> GetDNSAttestationsWeightedAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get,
            $"v2/dns/attestations/{Uri.EscapeDataString(domain)}/weighted", null, ct);
    }

    /// <summary>GET /api/v2/dns/resolve/{domain}/{recordType} — resolved DNS records.</summary>
    public Task<JsonNode?> ResolveDNSAsync(string domain, string recordType, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain) || string.IsNullOrEmpty(recordType))
            throw new QuidnugValidationException("domain and recordType are required");
        return RequestAsync(HttpMethod.Get,
            $"v2/dns/resolve/{Uri.EscapeDataString(domain)}/{Uri.EscapeDataString(recordType)}", null, ct);
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

    private static string BuildQueryString(params (string key, object? value)[] parameters)
    {
        var parts = new List<string>();
        foreach (var (k, v) in parameters)
        {
            if (v is null) continue;
            parts.Add($"{Uri.EscapeDataString(k)}={Uri.EscapeDataString(Convert.ToString(v, System.Globalization.CultureInfo.InvariantCulture) ?? "")}");
        }
        return parts.Count == 0 ? "" : "?" + string.Join("&", parts);
    }

    private static string BuildQueryString(IDictionary<string, string>? parameters)
    {
        if (parameters is null || parameters.Count == 0) return "";
        var parts = new List<string>(parameters.Count);
        foreach (var kv in parameters)
        {
            if (kv.Value is null) continue;
            parts.Add($"{Uri.EscapeDataString(kv.Key)}={Uri.EscapeDataString(kv.Value)}");
        }
        return parts.Count == 0 ? "" : "?" + string.Join("&", parts);
    }
}
