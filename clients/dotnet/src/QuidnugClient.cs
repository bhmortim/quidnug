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

    public Task<JsonNode?> PushAnchorAsync(object msg, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-anchor", msg, ct);

    public Task<JsonNode?> PushFingerprintAsync(object fp, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-fingerprint", fp, ct);

    public Task<JsonNode?> SubmitGossipDomainsAsync(object gossip, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/domains", gossip, ct);

    public Task<JsonNode?> SubmitNonceSnapshotAsync(object snapshot, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "nonce-snapshots", snapshot, ct);

    public async Task<JsonNode?> GetLatestNonceSnapshotAsync(string domain, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get,
                $"nonce-snapshots/{Uri.EscapeDataString(domain)}/latest", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public Task<JsonNode?> SubmitGuardianResignationAsync(object resignation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/resign", resignation, ct);

    public async Task<JsonNode?> GetPendingRecoveryAsync(string quidId, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get,
                $"guardian/pending-recovery/{Uri.EscapeDataString(quidId)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public Task<JsonNode?> GetGuardianResignationsAsync(string quidId, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, $"guardian/resignations/{Uri.EscapeDataString(quidId)}", null, ct);

    public Task<JsonNode?> SubmitForkBlockAsync(object fb, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "fork-block", fb, ct);

    public Task<JsonNode?> ForkBlockStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "fork-block/status", null, ct);

    public Task<JsonNode?> BootstrapStatusAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "bootstrap/status", null, ct);

    // =====================================================================
    // Peers (Phase 4e)
    // =====================================================================

    public Task<JsonNode?> GetPeersAsync(int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "peers" + QueryString(("limit", limit), ("offset", offset)), null, ct);

    public async Task<JsonNode?> GetPeerAsync(string nodeQuid, CancellationToken ct = default)
    {
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
    // Discovery (QDP-0014)
    // =====================================================================

    public async Task<JsonNode?> DiscoverDomainAsync(string name, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get, $"discovery/domain/{Uri.EscapeDataString(name)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public async Task<JsonNode?> DiscoverNodeAsync(string quid, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get, $"discovery/node/{Uri.EscapeDataString(quid)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public Task<JsonNode?> DiscoverOperatorAsync(string quid, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, $"discovery/operator/{Uri.EscapeDataString(quid)}", null, ct);

    public Task<JsonNode?> DiscoverQuidsAsync(
        string? domain = null, string? sort = null, int? limit = null, int? offset = null,
        CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            "discovery/quids" + QueryString(("domain", domain), ("sort", sort), ("limit", limit), ("offset", offset)),
            null, ct);

    public Task<JsonNode?> DiscoverTrustedQuidsAsync(
        string? domain = null, int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            "discovery/trusted-quids" + QueryString(("domain", domain), ("limit", limit), ("offset", offset)),
            null, ct);

    // =====================================================================
    // Moderation (QDP-0015)
    // =====================================================================

    public Task<JsonNode?> SubmitModerationActionAsync(object action, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "moderation/actions", action, ct);

    public Task<JsonNode?> GetModerationActionsAsync(
        string targetType, string targetId, int? limit = null, int? offset = null,
        CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            $"moderation/actions/{Uri.EscapeDataString(targetType)}/{Uri.EscapeDataString(targetId)}"
                + QueryString(("limit", limit), ("offset", offset)),
            null, ct);

    // =====================================================================
    // Audit (QDP-0018)
    // =====================================================================

    public Task<JsonNode?> GetAuditHeadAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "audit/head", null, ct);

    public Task<JsonNode?> GetAuditEntriesAsync(
        long? fromSequence = null, int? limit = null, int? offset = null,
        CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            "audit/entries" + QueryString(("fromSequence", fromSequence), ("limit", limit), ("offset", offset)),
            null, ct);

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
    // Privacy / DSR (QDP-0017)
    // =====================================================================

    public Task<JsonNode?> SubmitDSRAsync(object request, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/dsr", request, ct);

    public async Task<JsonNode?> GetDSRStatusAsync(string requestTxId, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get, $"privacy/dsr/{Uri.EscapeDataString(requestTxId)}", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public Task<JsonNode?> SubmitConsentGrantAsync(object grant, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/consent/grants", grant, ct);

    public Task<JsonNode?> SubmitConsentWithdrawAsync(object withdraw, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/consent/withdraws", withdraw, ct);

    public Task<JsonNode?> GetConsentHistoryAsync(
        string? subjectQuid = null, string? processorQuid = null,
        int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            "privacy/consent/history" + QueryString(
                ("subjectQuid", subjectQuid),
                ("processorQuid", processorQuid),
                ("limit", limit),
                ("offset", offset)),
            null, ct);

    public Task<JsonNode?> SubmitProcessingRestrictionAsync(object restriction, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/restrictions", restriction, ct);

    public Task<JsonNode?> GetRestrictionsForSubjectAsync(string subjectQuid, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            $"privacy/restrictions/{Uri.EscapeDataString(subjectQuid)}", null, ct);

    public Task<JsonNode?> SubmitDSRComplianceAsync(object compliance, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "privacy/compliance", compliance, ct);

    // =====================================================================
    // Domains
    // =====================================================================

    public Task<JsonNode?> ListDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "domains", null, ct);

    public Task<JsonNode?> RegisterDomainAsync(string name, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "domains", new Dictionary<string, object?> { ["name"] = name }, ct);

    public Task<JsonNode?> GetTopDomainsAsync(int? limit = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "domains/top" + QueryString(("limit", limit)), null, ct);

    public async Task<JsonNode?> QueryDomainAsync(string name, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get, $"domains/{Uri.EscapeDataString(name)}/query", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    public Task<JsonNode?> GetNodeDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "node/domains", null, ct);

    public Task<JsonNode?> UpdateNodeDomainsAsync(IEnumerable<string> domains, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "node/domains",
            new Dictionary<string, object?> { ["managedDomains"] = domains.ToArray() }, ct);

    public Task<JsonNode?> CreateQuidAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "quids", new Dictionary<string, object?>(), ct);

    public Task<JsonNode?> SubmitNodeAdvertisementAsync(object advertisement, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "node-advertisements", advertisement, ct);

    public Task<JsonNode?> GetPendingTransactionsAsync(
        int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "transactions" + QueryString(("limit", limit), ("offset", offset)), null, ct);

    public Task<JsonNode?> GetTentativeBlocksAsync(string domain, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, $"blocks/tentative/{Uri.EscapeDataString(domain)}", null, ct);

    // =====================================================================
    // Registry queries
    // =====================================================================

    public Task<JsonNode?> QueryTrustRegistryAsync(
        string? truster = null, string? trustee = null, int? limit = null, int? offset = null,
        CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            "registry/trust" + QueryString(
                ("truster", truster), ("trustee", trustee),
                ("limit", limit), ("offset", offset)),
            null, ct);

    public Task<JsonNode?> QueryIdentityRegistryAsync(
        string? quidId = null, int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            "registry/identity" + QueryString(("quid_id", quidId), ("limit", limit), ("offset", offset)),
            null, ct);

    public Task<JsonNode?> QueryTitleRegistryAsync(
        string? assetId = null, string? ownerId = null,
        int? limit = null, int? offset = null, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            "registry/title" + QueryString(
                ("asset_id", assetId), ("owner_id", ownerId),
                ("limit", limit), ("offset", offset)),
            null, ct);

    // =====================================================================
    // IPFS
    // =====================================================================

    public async Task<string> IpfsPinAsync(byte[] content, CancellationToken ct = default)
    {
        using var req = new HttpRequestMessage(HttpMethod.Post, _apiBase + "/ipfs/pin");
        req.Content = new ByteArrayContent(content);
        req.Content.Headers.ContentType = new MediaTypeHeaderValue("application/octet-stream");
        var resp = await _http.SendAsync(req, ct);
        var node = await ParseEnvelopeAsync(resp, ct);
        return node?["cid"]?.GetValue<string>()
               ?? node?["value"]?.GetValue<string>()
               ?? throw new QuidnugNodeException("IPFS pin response missing cid", 200, null);
    }

    public async Task<byte[]> IpfsGetAsync(string cid, CancellationToken ct = default)
    {
        using var req = new HttpRequestMessage(HttpMethod.Get, _apiBase + $"/ipfs/{Uri.EscapeDataString(cid)}");
        var resp = await _http.SendAsync(req, ct);
        if (!resp.IsSuccessStatusCode)
            throw new QuidnugNodeException($"IPFS fetch failed (HTTP {(int)resp.StatusCode})",
                (int)resp.StatusCode, null);
        return await resp.Content.ReadAsByteArrayAsync(ct);
    }

    // =====================================================================
    // DNS attestation (QDP-0016)
    // =====================================================================

    public Task<JsonNode?> SubmitDnsClaimAsync(object claim, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "dns/claim", claim, ct);

    public Task<JsonNode?> SubmitDnsChallengeAsync(object challenge, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "dns/challenge", challenge, ct);

    public Task<JsonNode?> SubmitDnsAttestationAsync(object attestation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "dns/attestation", attestation, ct);

    public Task<JsonNode?> SubmitDnsRenewalAsync(object renewal, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "dns/renewal", renewal, ct);

    public Task<JsonNode?> SubmitDnsRevocationAsync(object revocation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "dns/revocation", revocation, ct);

    public Task<JsonNode?> SubmitDnsDelegateAsync(object delegation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "dns/delegate", delegation, ct);

    public Task<JsonNode?> SubmitDnsDelegateRevocationAsync(object revocation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "dns/delegate-revocation", revocation, ct);

    public Task<JsonNode?> GetDnsAttestationsAsync(string domain, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, $"dns/attestations/{Uri.EscapeDataString(domain)}", null, ct);

    public Task<JsonNode?> GetDnsWeightedAttestationsAsync(string domain, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            $"dns/attestations/{Uri.EscapeDataString(domain)}/weighted", null, ct);

    public Task<JsonNode?> ResolveDnsAsync(string domain, string recordType, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get,
            $"dns/resolve/{Uri.EscapeDataString(domain)}/{Uri.EscapeDataString(recordType)}",
            null, ct);

    /// <summary>
    /// Build a `?k=v&amp;k=v` query string from the supplied tuples, omitting null/empty values.
    /// </summary>
    private static string QueryString(params (string Key, object? Value)[] pairs)
    {
        var parts = new List<string>();
        foreach (var (key, value) in pairs)
        {
            if (value is null) continue;
            string? s = value switch
            {
                string str when string.IsNullOrEmpty(str) => null,
                string str => str,
                _ => value.ToString(),
            };
            if (s is null) continue;
            parts.Add($"{Uri.EscapeDataString(key)}={Uri.EscapeDataString(s)}");
        }
        return parts.Count == 0 ? string.Empty : "?" + string.Join('&', parts);
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
