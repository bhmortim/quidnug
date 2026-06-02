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
        => NodesAsync(null, null, ct);

    /// <summary>Lists known peers with optional pagination (Go parity).</summary>
    public Task<JsonNode?> NodesAsync(int? limit, int? offset, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "nodes" + PaginationQuery(limit, offset), null, ct);

    public Task<JsonNode?> BlocksAsync(CancellationToken ct = default)
        => GetBlocksAsync(null, null, ct);

    /// <summary>Paginated blocks (Go parity for <c>GetBlocks</c>).</summary>
    public Task<JsonNode?> GetBlocksAsync(int? limit, int? offset, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "blocks" + PaginationQuery(limit, offset), null, ct);

    /// <summary>Paginated pending transactions (Go parity for <c>GetPendingTransactions</c>).</summary>
    public Task<JsonNode?> GetPendingTransactionsAsync(int? limit, int? offset, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "transactions" + PaginationQuery(limit, offset), null, ct);

    /// <summary>
    /// Performs a raw GET against an arbitrary <paramref name="path"/> beneath
    /// <c>/api</c> and returns the unparsed response body. Useful for ad-hoc
    /// CLI commands without a typed wrapper. Throws <see cref="QuidnugNodeException"/>
    /// for non-2xx responses (response body still attached).
    /// </summary>
    public async Task<byte[]> RawGetAsync(string path, CancellationToken ct = default)
    {
        using var req = new HttpRequestMessage(HttpMethod.Get,
            _apiBase + "/" + path.TrimStart('/'));
        req.Headers.Accept.Clear();
        req.Headers.Accept.ParseAdd("application/json");
        using var resp = await _http.SendAsync(req, ct);
        byte[] body = await resp.Content.ReadAsByteArrayAsync(ct);
        if ((int)resp.StatusCode < 200 || (int)resp.StatusCode >= 300)
        {
            throw new QuidnugNodeException(
                $"GET {path}: status {(int)resp.StatusCode}",
                (int)resp.StatusCode,
                System.Text.Encoding.UTF8.GetString(body));
        }
        return body;
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

    private static string PaginationQuery(int? limit, int? offset)
    {
        var parts = new List<string>(2);
        if (limit.HasValue && limit.Value > 0) parts.Add($"limit={limit.Value}");
        if (offset.HasValue && offset.Value > 0) parts.Add($"offset={offset.Value}");
        return parts.Count == 0 ? "" : "?" + string.Join("&", parts);
    }

    // =====================================================================
    // QDP-0014 — Node advertisement & discovery
    // =====================================================================

    /// <summary>
    /// Builds, signs, and submits a QDP-0014 NodeAdvertisementTransaction.
    /// <paramref name="signer"/> is the node's own keypair; <c>NodeQuid</c>
    /// is derived from <c>signer.Id</c>. The OperatorQuid must hold a
    /// direct TRUST edge to the node (weight &gt;= 0.5).
    ///
    /// <para><b>Note.</b> The Go reference signs the marshaled <i>struct</i>
    /// (declaration-order JSON); this client signs canonical (sorted-key)
    /// JSON. Nodes that verify only against the canonical form will accept
    /// these; nodes that verify against struct-order JSON will not. If your
    /// node rejects the signature, sign and submit via the Go SDK or supply
    /// a pre-signed payload.</para>
    /// </summary>
    public async Task<JsonNode?> PublishNodeAdvertisementAsync(
        Quid signer, NodeAdvertisementParams p, CancellationToken ct = default)
    {
        RequireSigner(signer);
        if (p is null) throw new QuidnugValidationException("params is required");
        if (string.IsNullOrEmpty(p.OperatorQuid))
            throw new QuidnugValidationException("operatorQuid is required");
        if (p.Endpoints is null || p.Endpoints.Count == 0)
            throw new QuidnugValidationException("at least one endpoint is required");
        if (p.AdvertisementNonce <= 0)
            throw new QuidnugValidationException("advertisementNonce must be positive");
        if (string.IsNullOrEmpty(p.Domain))
            throw new QuidnugValidationException(
                "domain is required (typically operators.network.<your-domain>)");
        TimeSpan ttl = p.Ttl;
        if (ttl == TimeSpan.Zero) ttl = TimeSpan.FromHours(6);
        if (ttl > TimeSpan.FromDays(7))
            throw new QuidnugValidationException("ttl must be <= 7 days");
        string protoVer = string.IsNullOrEmpty(p.ProtocolVersion) ? "1.0" : p.ProtocolVersion;

        long nowUnix = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        long expires = DateTimeOffset.UtcNow.Add(ttl).ToUnixTimeMilliseconds() * 1_000_000L;
        // ^ UnixNano equivalent; the Go reference uses time.Now().Add(ttl).UnixNano().

        var idRaw = new byte[16];
        System.Security.Cryptography.RandomNumberGenerator.Fill(idRaw);
        string idHex = Convert.ToHexString(idRaw).ToLowerInvariant();

        // Field order MUST match the server's struct exactly; LinkedHashMap-equivalent
        // ordering via a Dictionary<string,object?> + canonical JSON is what the
        // server validates against. Use CanonicalBytes (which sorts keys) — the Go
        // reference relies on struct field order, but for a portable wire format
        // the server accepts canonical JSON. We sign the canonicalized bytes and
        // submit the same dictionary.
        var tx = new Dictionary<string, object?>
        {
            ["id"]                 = idHex,
            ["type"]               = "NODE_ADVERTISEMENT",
            ["trustDomain"]        = p.Domain,
            ["timestamp"]          = nowUnix,
            ["signature"]          = "",
            ["publicKey"]          = signer.PublicKeyHex,
            ["nodeQuid"]           = signer.Id,
            ["operatorQuid"]       = p.OperatorQuid,
            ["endpoints"]          = p.Endpoints,
            ["supportedDomains"]   = p.SupportedDomains,
            ["capabilities"]       = p.Capabilities,
            ["protocolVersion"]    = protoVer,
            ["expiresAt"]          = expires,
            ["advertisementNonce"] = p.AdvertisementNonce,
        };
        byte[] signable = CanonicalBytes.Of(tx, "signature");
        tx["signature"] = signer.Sign(signable);
        return await RequestAsync(HttpMethod.Post, "node-advertisements", tx, ct);
    }

    /// <summary>Returns the current consortium, endpoint hints, and block tip for a domain (QDP-0014).</summary>
    public Task<JsonNode?> DiscoverDomainAsync(string domain, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        return RequestAsync(HttpMethod.Get,
            "v2/discovery/domain/" + Uri.EscapeDataString(domain), null, ct);
    }

    /// <summary>Returns the raw signed advertisement for a node quid (QDP-0014).</summary>
    public Task<JsonNode?> DiscoverNodeAsync(string quid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(quid))
            throw new QuidnugValidationException("quid is required");
        return RequestAsync(HttpMethod.Get,
            "v2/discovery/node/" + Uri.EscapeDataString(quid), null, ct);
    }

    /// <summary>Lists all advertisements for a given operator quid (QDP-0014).</summary>
    public Task<JsonNode?> DiscoverOperatorAsync(string operatorQuid, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(operatorQuid))
            throw new QuidnugValidationException("operatorQuid is required");
        return RequestAsync(HttpMethod.Get,
            "v2/discovery/operator/" + Uri.EscapeDataString(operatorQuid), null, ct);
    }

    /// <summary>Queries the per-domain quid index (QDP-0014).</summary>
    public Task<JsonNode?> DiscoverQuidsAsync(DiscoverQuidsParams p, CancellationToken ct = default)
    {
        if (p is null || string.IsNullOrEmpty(p.Domain))
            throw new QuidnugValidationException("domain is required");
        var parts = new List<string> { "domain=" + Uri.EscapeDataString(p.Domain) };
        if (p.Since > 0)               parts.Add("since=" + p.Since);
        if (!string.IsNullOrEmpty(p.Sort))      parts.Add("sort=" + Uri.EscapeDataString(p.Sort));
        if (!string.IsNullOrEmpty(p.Observer))  parts.Add("observer=" + Uri.EscapeDataString(p.Observer));
        if (!string.IsNullOrEmpty(p.EventType)) parts.Add("eventType=" + Uri.EscapeDataString(p.EventType));
        if (p.MinTrustWeight > 0)
            parts.Add("min-trust-weight=" + p.MinTrustWeight.ToString(System.Globalization.CultureInfo.InvariantCulture));
        if (p.ExcludeQuids is { Count: > 0 })
            parts.Add("excludeQuid=" + Uri.EscapeDataString(string.Join(",", p.ExcludeQuids)));
        if (p.Limit > 0)  parts.Add("limit=" + p.Limit);
        if (p.Offset > 0) parts.Add("offset=" + p.Offset);
        return RequestAsync(HttpMethod.Get, "v2/discovery/quids?" + string.Join("&", parts), null, ct);
    }

    /// <summary>Returns quids the consortium directly TRUSTs above the threshold (QDP-0014).</summary>
    public Task<JsonNode?> DiscoverTrustedQuidsAsync(
        string domain, double? minTrust = null, int? limit = null, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        var parts = new List<string> { "domain=" + Uri.EscapeDataString(domain) };
        if (minTrust.HasValue && minTrust.Value > 0)
            parts.Add("min-trust=" + minTrust.Value.ToString(System.Globalization.CultureInfo.InvariantCulture));
        if (limit.HasValue && limit.Value > 0)
            parts.Add("limit=" + limit.Value);
        return RequestAsync(HttpMethod.Get,
            "v2/discovery/trusted-quids?" + string.Join("&", parts), null, ct);
    }

    // =====================================================================
    // Guardians (QDP-0002) — remaining methods
    // =====================================================================

    /// <summary>Removes a guardian from the set (QDP-0002).</summary>
    public Task<JsonNode?> SubmitGuardianResignationAsync(object resignation, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "guardian/resign", resignation, ct);

    /// <summary>Returns the pending recovery record for a quid, or null on 404.</summary>
    public async Task<JsonNode?> GetPendingRecoveryAsync(string quidId, CancellationToken ct = default)
    {
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
    // Gossip (push variants, QDP-0003/0005)
    // =====================================================================

    /// <summary>Push-mode anchor gossip (QDP-0005).</summary>
    public Task<JsonNode?> PushAnchorAsync(object message, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-anchor", message, ct);

    /// <summary>Push-mode fingerprint gossip (QDP-0003).</summary>
    public Task<JsonNode?> PushFingerprintAsync(object fingerprint, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "gossip/push-fingerprint", fingerprint, ct);

    // =====================================================================
    // Bootstrap (QDP-0008) — nonce snapshots
    // =====================================================================

    /// <summary>Publishes a K-of-K bootstrap nonce snapshot (QDP-0008).</summary>
    public Task<JsonNode?> SubmitNonceSnapshotAsync(object snapshot, CancellationToken ct = default)
        => RequestAsync(HttpMethod.Post, "nonce-snapshots", snapshot, ct);

    /// <summary>Returns the most recent nonce snapshot for a domain, or null on 404.</summary>
    public async Task<JsonNode?> GetLatestNonceSnapshotAsync(string domain, CancellationToken ct = default)
    {
        try
        {
            return await RequestAsync(HttpMethod.Get,
                "nonce-snapshots/" + Uri.EscapeDataString(domain) + "/latest", null, ct);
        }
        catch (QuidnugValidationException ex) when (ex.Details.TryGetValue("code", out var c)
                                                    && (c as string) == "NOT_FOUND")
        {
            return null;
        }
    }

    // =====================================================================
    // Domain management
    // =====================================================================

    /// <summary>Lists all known trust domains.</summary>
    public Task<JsonNode?> ListDomainsAsync(CancellationToken ct = default)
        => RequestAsync(HttpMethod.Get, "domains", null, ct);

    /// <summary>
    /// Submits a new trust domain. Fails with an "already exists" error if
    /// the domain is already known; see <see cref="EnsureDomainAsync"/> for
    /// an idempotent variant.
    /// </summary>
    public Task<JsonNode?> RegisterDomainAsync(
        string domain, Dictionary<string, object?>? attrs = null, CancellationToken ct = default)
    {
        if (string.IsNullOrEmpty(domain))
            throw new QuidnugValidationException("domain is required");
        var body = new Dictionary<string, object?> { ["name"] = domain };
        if (attrs is not null)
        {
            foreach (var kv in attrs) body[kv.Key] = kv.Value;
        }
        return RequestAsync(HttpMethod.Post, "domains", body, ct);
    }

    /// <summary>
    /// Registers a trust domain if it does not already exist. Idempotent —
    /// calling twice is cheap and does not error.
    /// </summary>
    public async Task<JsonNode?> EnsureDomainAsync(
        string domain, Dictionary<string, object?>? attrs = null, CancellationToken ct = default)
    {
        try
        {
            return await RegisterDomainAsync(domain, attrs, ct);
        }
        catch (QuidnugConflictException)
        {
            return JsonNode.Parse(JsonSerializer.Serialize(new Dictionary<string, object?>
            {
                ["status"]  = "success",
                ["domain"]  = domain,
                ["message"] = "trust domain already exists",
            }));
        }
        catch (QuidnugValidationException ex)
            when (ex.Message.Contains("already exists", StringComparison.OrdinalIgnoreCase)
                  || (ex.Details.TryGetValue("code", out var c) && (c as string) == "ALREADY_EXISTS"))
        {
            return JsonNode.Parse(JsonSerializer.Serialize(new Dictionary<string, object?>
            {
                ["status"]  = "success",
                ["domain"]  = domain,
                ["message"] = "trust domain already exists",
            }));
        }
    }

    // =====================================================================
    // Polling helpers
    // =====================================================================

    /// <summary>
    /// Blocks until the identity with the given quid ID is visible in the
    /// committed registry, or the cancellation token fires. A just-submitted
    /// identity transaction lives in the node's pending pool until the next
    /// block is sealed — call this before emitting events or titles that
    /// reference the new quid.
    /// </summary>
    public async Task<IdentityRecord?> WaitForIdentityAsync(
        string quidId, string? domain = null, TimeSpan? pollInterval = null,
        CancellationToken ct = default)
    {
        TimeSpan interval = pollInterval ?? TimeSpan.FromMilliseconds(500);
        if (interval <= TimeSpan.Zero) interval = TimeSpan.FromMilliseconds(500);
        while (true)
        {
            var rec = await GetIdentityAsync(quidId, domain, ct);
            if (rec is not null) return rec;
            await Task.Delay(interval, ct);
        }
    }

    /// <summary>Blocks until every listed quid is committed, sharing one deadline.</summary>
    public async Task WaitForIdentitiesAsync(
        IEnumerable<string> quidIds, string? domain = null, TimeSpan? pollInterval = null,
        CancellationToken ct = default)
    {
        foreach (var id in quidIds)
        {
            try
            {
                await WaitForIdentityAsync(id, domain, pollInterval, ct);
            }
            catch (Exception e) when (e is not OperationCanceledException)
            {
                throw new QuidnugNodeException($"wait for identity {id}: {e.Message}", e);
            }
        }
    }

    /// <summary>
    /// Blocks until the title with the given asset ID is visible in the
    /// committed registry. Same rationale as <see cref="WaitForIdentityAsync"/>.
    /// </summary>
    public async Task<Title?> WaitForTitleAsync(
        string assetId, string? domain = null, TimeSpan? pollInterval = null,
        CancellationToken ct = default)
    {
        TimeSpan interval = pollInterval ?? TimeSpan.FromMilliseconds(500);
        if (interval <= TimeSpan.Zero) interval = TimeSpan.FromMilliseconds(500);
        while (true)
        {
            var t = await GetTitleAsync(assetId, domain, ct);
            if (t is not null) return t;
            await Task.Delay(interval, ct);
        }
    }
}
