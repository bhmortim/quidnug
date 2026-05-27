using System.Net;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using Xunit;

namespace Quidnug.Client.Tests;

public class QuidnugClientTests
{
    /// <summary>Stub HttpMessageHandler that returns queued responses.</summary>
    private sealed class StubHandler : HttpMessageHandler
    {
        public readonly List<HttpRequestMessage> Requests = new();
        public readonly List<string> Bodies = new();
        public readonly Queue<(HttpStatusCode status, string body)> Responses = new();

        protected override async Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken ct)
        {
            Requests.Add(request);
            Bodies.Add(request.Content is null ? "" : await request.Content.ReadAsStringAsync(ct));
            if (Responses.Count == 0)
                throw new InvalidOperationException($"no response queued for {request.Method} {request.RequestUri}");
            var (status, body) = Responses.Dequeue();
            return new HttpResponseMessage(status)
            {
                Content = new StringContent(body, Encoding.UTF8, "application/json"),
            };
        }
    }

    private static (QuidnugClient, StubHandler) MakeClient(int maxRetries = 0)
    {
        var handler = new StubHandler();
        var http = new HttpClient(handler);
        var client = new QuidnugClient("http://node.local", http: http, maxRetries: maxRetries);
        return (client, handler);
    }

    // --- Envelope parsing -----------------------------------------------

    [Fact]
    public async Task SuccessEnvelopeUnwraps()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK,
            @"{""success"":true,""data"":{""status"":""ok""}}"));
        var data = await client.HealthAsync();
        Assert.Equal("ok", data!["status"]!.GetValue<string>());
    }

    [Fact]
    public async Task Error409RaisesConflict()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.Conflict,
            @"{""success"":false,""error"":{""code"":""NONCE_REPLAY"",""message"":""replay""}}"));
        var ex = await Assert.ThrowsAsync<QuidnugConflictException>(() => client.HealthAsync());
        Assert.Equal("replay", ex.Message);
        Assert.Equal("NONCE_REPLAY", ex.Details["code"]);
    }

    [Fact]
    public async Task Error503RaisesUnavailable()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.ServiceUnavailable,
            @"{""success"":false,""error"":{""code"":""BOOTSTRAPPING"",""message"":""warm""}}"));
        await Assert.ThrowsAsync<QuidnugUnavailableException>(() => client.HealthAsync());
    }

    [Fact]
    public async Task NonJsonResponseIsNodeError()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.InternalServerError, "<html>500</html>"));
        var ex = await Assert.ThrowsAsync<QuidnugNodeException>(() => client.HealthAsync());
        Assert.Equal(500, ex.StatusCode);
    }

    // --- Validation -----------------------------------------------------

    [Fact]
    public async Task GrantTrustValidatesLevel()
    {
        var (client, _) = MakeClient();
        using var q = Quid.Generate();
        await Assert.ThrowsAsync<QuidnugValidationException>(
            () => client.GrantTrustAsync(q, "bob", 1.5, "x"));
    }

    [Fact]
    public async Task RegisterTitleValidatesPercentageSum()
    {
        var (client, _) = MakeClient();
        using var q = Quid.Generate();
        var owners = new List<OwnershipStake>
        {
            new("a", 50.0),
            new("b", 30.0),
        };
        await Assert.ThrowsAsync<QuidnugValidationException>(
            () => client.RegisterTitleAsync(q, "asset-1", owners));
    }

    [Fact]
    public async Task EmitEventRequiresXorPayloadAndCid()
    {
        var (client, _) = MakeClient();
        using var q = Quid.Generate();
        await Assert.ThrowsAsync<QuidnugValidationException>(
            () => client.EmitEventAsync(q, "s", "QUID", "LOGIN"));
        await Assert.ThrowsAsync<QuidnugValidationException>(
            () => client.EmitEventAsync(q, "s", "QUID", "LOGIN",
                payload: new Dictionary<string, object?> { ["x"] = 1 },
                payloadCid: "Qm..."));
    }

    // --- Endpoint routing -----------------------------------------------

    [Fact]
    public async Task GrantTrustPostsCorrectEnvelope()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""txId"":""abc""}}"));
        using var alice = Quid.Generate();
        var data = await client.GrantTrustAsync(alice, "bob", 0.9, "demo.home");
        Assert.Equal("abc", data!["txId"]!.GetValue<string>());
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/transactions/trust", h.Requests[0].RequestUri!.AbsolutePath);
        var body = JsonNode.Parse(h.Bodies[0])!;
        Assert.Equal("TRUST", body["type"]!.GetValue<string>());
        Assert.Equal("bob", body["trustee"]!.GetValue<string>());
        Assert.NotNull(body["signature"]);
    }

    [Fact]
    public async Task GetIdentityReturnsNullOnNotFound()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""absent""}}"));
        var rec = await client.GetIdentityAsync("missing");
        Assert.Null(rec);
    }

    // --- Retry behavior -------------------------------------------------

    [Fact]
    public async Task RetriesTransient5xx()
    {
        var (client, h) = MakeClient(maxRetries: 3);
        h.Responses.Enqueue((HttpStatusCode.InternalServerError,
            @"{""success"":false,""error"":{""code"":""INTERNAL""}}"));
        h.Responses.Enqueue((HttpStatusCode.InternalServerError,
            @"{""success"":false,""error"":{""code"":""INTERNAL""}}"));
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}"));
        var data = await client.HealthAsync();
        Assert.True(data!["ok"]!.GetValue<bool>());
        Assert.Equal(3, h.Requests.Count);
    }

    [Fact]
    public async Task PostIsNotRetriedByDefault()
    {
        var (client, h) = MakeClient(maxRetries: 3);
        h.Responses.Enqueue((HttpStatusCode.InternalServerError,
            @"{""success"":false,""error"":{""code"":""INTERNAL""}}"));
        using var q = Quid.Generate();
        await Assert.ThrowsAsync<QuidnugNodeException>(
            () => client.GrantTrustAsync(q, "bob", 0.5, "x"));
        Assert.Single(h.Requests);
    }

    // --- v2 path routing (fix) -----------------------------------------

    [Fact]
    public async Task GuardianSetUpdateRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}"));
        await client.SubmitGuardianSetUpdateAsync(new { subjectQuid = "abc" });
        Assert.EndsWith("/api/v2/guardian/set-update", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetGuardianSetRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK,
            @"{""success"":true,""data"":{""quidId"":""abc"",""guardians"":[],""threshold"":0,""recoveryWindowSeconds"":0,""updateNonce"":0}}"));
        await client.GetGuardianSetAsync("abc");
        Assert.EndsWith("/api/v2/guardian/set/abc", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task DomainFingerprintRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}"));
        await client.SubmitDomainFingerprintAsync(new { domain = "x" });
        Assert.EndsWith("/api/v2/domain-fingerprints", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task ForkBlockStatusRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{}}"));
        await client.ForkBlockStatusAsync();
        Assert.EndsWith("/api/v2/fork-block/status", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task BootstrapStatusRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{}}"));
        await client.BootstrapStatusAsync();
        Assert.EndsWith("/api/v2/bootstrap/status", h.Requests[0].RequestUri!.AbsolutePath);
    }

    // --- v3 routing ----------------------------------------------------

    [Fact]
    public async Task GetPeersRoutesToApiPeersWithQueryString()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""peers"":[]}}"));
        await client.GetPeersAsync(limit: 25, offset: 0);
        var uri = h.Requests[0].RequestUri!;
        Assert.EndsWith("/api/peers", uri.AbsolutePath);
        Assert.Contains("limit=25", uri.Query);
        Assert.Contains("offset=0", uri.Query);
    }

    [Fact]
    public async Task GetPeerReturnsNullOnNotFound()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""absent""}}"));
        var p = await client.GetPeerAsync("missing");
        Assert.Null(p);
    }

    [Fact]
    public async Task SubmitNodeAdvertisementRoutesToApi()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{}}"));
        await client.SubmitNodeAdvertisementAsync(new { nodeQuid = "n" });
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/node-advertisements", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetAuditHeadRoutesToApi()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""sequence"":7}}"));
        var head = await client.GetAuditHeadAsync();
        Assert.EndsWith("/api/audit/head", h.Requests[0].RequestUri!.AbsolutePath);
        Assert.Equal(7, head!["sequence"]!.GetValue<int>());
    }

    [Fact]
    public async Task GetAuditEntryReturnsNullOnNotFound()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""absent""}}"));
        var entry = await client.GetAuditEntryAsync(999);
        Assert.Null(entry);
    }

    [Fact]
    public async Task ModerationRoutesToApi()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}"));
        await client.SubmitModerationActionAsync(new { action = "hide" });
        Assert.EndsWith("/api/moderation/actions", h.Requests[0].RequestUri!.AbsolutePath);

        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""actions"":[]}}"));
        await client.GetModerationActionsAsync("review", "abc 123");
        Assert.EndsWith("/api/moderation/actions/review/abc%20123",
            h.Requests[1].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task PrivacyDSRRoutesToApi()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""txId"":""tx-1""}}"));
        await client.SubmitDSRAsync(new { subjectQuid = "x", action = "delete" });
        Assert.EndsWith("/api/privacy/dsr", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task DiscoveryDomainRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""domain"":""ex.org""}}"));
        await client.GetDiscoveryDomainAsync("ex.org");
        Assert.EndsWith("/api/v2/discovery/domain/ex.org",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetDiscoveryNodeReturnsNullOnNotFound()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""absent""}}"));
        var node = await client.GetDiscoveryNodeAsync("missing");
        Assert.Null(node);
    }

    [Fact]
    public async Task DiscoveryQuidsBuildsQueryString()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""quids"":[]}}"));
        await client.DiscoveryQuidsAsync(new Dictionary<string, string>
        {
            ["domain"] = "x.home",
            ["limit"] = "10",
        });
        var uri = h.Requests[0].RequestUri!;
        Assert.EndsWith("/api/v2/discovery/quids", uri.AbsolutePath);
        Assert.Contains("domain=x.home", uri.Query);
        Assert.Contains("limit=10", uri.Query);
    }

    [Fact]
    public async Task SubmitDNSClaimRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}"));
        await client.SubmitDNSClaimAsync(new { domain = "ex.org", claimantQuid = "q" });
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/v2/dns/claim", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task ResolveDNSRoutesToV2()
    {
        var (client, h) = MakeClient();
        h.Responses.Enqueue((HttpStatusCode.OK, @"{""success"":true,""data"":{""records"":[]}}"));
        await client.ResolveDNSAsync("ex.org", "A");
        Assert.EndsWith("/api/v2/dns/resolve/ex.org/A",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetDNSAttestationsRequiresDomain()
    {
        var (client, _) = MakeClient();
        await Assert.ThrowsAsync<QuidnugValidationException>(
            () => client.GetDNSAttestationsAsync(""));
    }
}
