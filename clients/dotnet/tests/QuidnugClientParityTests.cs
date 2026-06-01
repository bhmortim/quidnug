using System.Net;
using System.Text;
using System.Text.Json.Nodes;
using Xunit;

namespace Quidnug.Client.Tests;

/// <summary>
/// Coverage for methods added to reach Python parity:
/// registry queries, push-gossip, IPFS, domains, node-domains,
/// nonce snapshots, guardian extras, commit-wait helpers.
/// </summary>
public class QuidnugClientParityTests
{
    /// <summary>Stub HttpMessageHandler returning queued responses, capturing requests.</summary>
    private sealed class StubHandler : HttpMessageHandler
    {
        public readonly List<HttpRequestMessage> Requests = new();
        public readonly List<string> Bodies = new();
        public readonly List<byte[]> RawBodies = new();
        public readonly Queue<Func<HttpResponseMessage>> Responses = new();

        public void EnqueueJson(HttpStatusCode status, string body)
        {
            Responses.Enqueue(() => new HttpResponseMessage(status)
            {
                Content = new StringContent(body, Encoding.UTF8, "application/json"),
            });
        }

        public void EnqueueBytes(HttpStatusCode status, byte[] body, string contentType)
        {
            Responses.Enqueue(() =>
            {
                var c = new ByteArrayContent(body);
                c.Headers.ContentType = new System.Net.Http.Headers.MediaTypeHeaderValue(contentType);
                return new HttpResponseMessage(status) { Content = c };
            });
        }

        protected override async Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken ct)
        {
            Requests.Add(request);
            if (request.Content is null)
            {
                Bodies.Add("");
                RawBodies.Add(Array.Empty<byte>());
            }
            else
            {
                byte[] raw = await request.Content.ReadAsByteArrayAsync(ct);
                RawBodies.Add(raw);
                Bodies.Add(Encoding.UTF8.GetString(raw));
            }
            if (Responses.Count == 0)
                throw new InvalidOperationException(
                    $"no response queued for {request.Method} {request.RequestUri}");
            return Responses.Dequeue()();
        }
    }

    private static (QuidnugClient, StubHandler) MakeClient(int maxRetries = 0)
    {
        var handler = new StubHandler();
        var http = new HttpClient(handler);
        var client = new QuidnugClient("http://node.local", http: http, maxRetries: maxRetries);
        return (client, handler);
    }

    // --- Registry queries ---------------------------------------------------

    [Fact]
    public async Task QueryIdentityRegistryAppendsParams()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""rows"":[]}}");
        await client.QueryIdentityRegistryAsync(limit: 10, offset: 5, domain: "homes.au");
        var uri = h.Requests[0].RequestUri!;
        Assert.EndsWith("/api/registry/identity", uri.AbsolutePath);
        var query = uri.Query;
        Assert.Contains("limit=10", query);
        Assert.Contains("offset=5", query);
        Assert.Contains("domain=homes.au", query);
    }

    [Fact]
    public async Task QueryTrustRegistryOmitsNullParams()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""rows"":[]}}");
        await client.QueryTrustRegistryAsync(truster: "alice");
        var uri = h.Requests[0].RequestUri!;
        Assert.Equal("/api/registry/trust", uri.AbsolutePath);
        Assert.Equal("?truster=alice", uri.Query);
    }

    [Fact]
    public async Task QueryTitleRegistryUsesSnakeCaseKeys()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""rows"":[]}}");
        await client.QueryTitleRegistryAsync(assetId: "house-1", ownerId: "alice");
        var query = h.Requests[0].RequestUri!.Query;
        Assert.Contains("asset_id=house-1", query);
        Assert.Contains("owner_id=alice", query);
    }

    [Fact]
    public async Task QueryRelationalTrustPostsBodyAndDecodes()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""observer"":""a"",""target"":""b"",""trustLevel"":0.75,""trustPath"":[""a"",""x"",""b""],""pathDepth"":2,""domain"":""d""}}");
        var tr = await client.QueryRelationalTrustAsync("a", "b", "d", maxDepth: 4);
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/trust/query", h.Requests[0].RequestUri!.AbsolutePath);
        var body = JsonNode.Parse(h.Bodies[0])!;
        Assert.Equal("a", body["observer"]!.GetValue<string>());
        Assert.Equal("b", body["target"]!.GetValue<string>());
        Assert.Equal("d", body["domain"]!.GetValue<string>());
        Assert.Equal(4, body["maxDepth"]!.GetValue<int>());
        Assert.Equal(0.75, tr.TrustLevel);
        Assert.Equal(3, tr.PathOrEmpty.Count);
    }

    [Fact]
    public async Task QueryDomainBuildsUrl()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{}}");
        await client.QueryDomainAsync("homes.au", "title", "house-7");
        var uri = h.Requests[0].RequestUri!;
        Assert.EndsWith("/api/domains/homes.au/query", uri.AbsolutePath);
        Assert.Contains("type=title", uri.Query);
        Assert.Contains("param=house-7", uri.Query);
    }

    // --- Pending / tentative / pagination ----------------------------------

    [Fact]
    public async Task GetPendingTransactionsPaginates()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""rows"":[]}}");
        await client.GetPendingTransactionsAsync(limit: 25, offset: 100);
        var uri = h.Requests[0].RequestUri!;
        Assert.Equal("/api/transactions", uri.AbsolutePath);
        Assert.Contains("limit=25", uri.Query);
        Assert.Contains("offset=100", uri.Query);
    }

    [Fact]
    public async Task GetTentativeBlocksRoute()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""blocks"":[]}}");
        await client.GetTentativeBlocksAsync("homes.au");
        Assert.EndsWith("/api/blocks/tentative/homes.au",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetTentativeBlocksRequiresDomain()
    {
        var (client, _) = MakeClient();
        await Assert.ThrowsAsync<QuidnugValidationException>(
            () => client.GetTentativeBlocksAsync(""));
    }

    // --- Guardian extras ----------------------------------------------------

    [Fact]
    public async Task SubmitGuardianResignationPostsJson()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}");
        await client.SubmitGuardianResignationAsync(new { subjectQuid = "alice" });
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/guardian/resign",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetGuardianResignationsParsesArray()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""resignations"":[{""guardianQuid"":""g1""},{""guardianQuid"":""g2""}]}}");
        var rs = await client.GetGuardianResignationsAsync("alice");
        Assert.Equal(2, rs.Count);
        Assert.EndsWith("/api/guardian/resignations/alice",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetPendingRecoveryReturnsNullOn404()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""none""}}");
        var pr = await client.GetPendingRecoveryAsync("alice");
        Assert.Null(pr);
    }

    [Fact]
    public async Task GetPendingRecoveryUnwrapsData()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""state"":""waiting""}}");
        var pr = await client.GetPendingRecoveryAsync("alice");
        Assert.NotNull(pr);
        Assert.Equal("waiting", pr!["state"]!.GetValue<string>());
    }

    // --- Push gossip --------------------------------------------------------

    [Fact]
    public async Task PushAnchorRoute()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""accepted"":true}}");
        await client.PushAnchorAsync(new { messageId = "m1" });
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/gossip/push-anchor",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task PushFingerprintRoute()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""accepted"":true}}");
        await client.PushFingerprintAsync(new { domain = "homes.au" });
        Assert.EndsWith("/api/gossip/push-fingerprint",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    // --- Bootstrap / nonce snapshots ---------------------------------------

    [Fact]
    public async Task SubmitNonceSnapshotRoute()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""accepted"":true}}");
        await client.SubmitNonceSnapshotAsync(new { blockHeight = 42 });
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/nonce-snapshots",
            h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task GetLatestNonceSnapshotDecodes()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""blockHeight"":7,""blockHash"":""abc"",""timestamp"":1,""trustDomain"":""d"",""entries"":[{""quid"":""q1"",""epoch"":1,""maxNonce"":12}],""producerQuid"":""p""}}");
        var snap = await client.GetLatestNonceSnapshotAsync("d");
        Assert.NotNull(snap);
        Assert.Equal(7, snap!.BlockHeight);
        Assert.Single(snap.Entries!);
        Assert.Equal(12, snap.Entries![0].MaxNonce);
    }

    [Fact]
    public async Task GetLatestNonceSnapshotReturnsNullOn404()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""none""}}");
        var snap = await client.GetLatestNonceSnapshotAsync("d");
        Assert.Null(snap);
    }

    // --- Domains -----------------------------------------------------------

    [Fact]
    public async Task ListDomainsRoute()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""domains"":[]}}");
        await client.ListDomainsAsync();
        Assert.EndsWith("/api/domains", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task RegisterDomainPostsName()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}");
        await client.RegisterDomainAsync("homes.au");
        var body = JsonNode.Parse(h.Bodies[0])!;
        Assert.Equal("homes.au", body["name"]!.GetValue<string>());
    }

    [Fact]
    public async Task EnsureDomainSwallowsAlreadyExists()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.BadRequest,
            @"{""success"":false,""error"":{""code"":""ALREADY_EXISTS"",""message"":""trust domain already exists""}}");
        var node = await client.EnsureDomainAsync("homes.au");
        Assert.NotNull(node);
        Assert.Equal("homes.au", node!["domain"]!.GetValue<string>());
    }

    [Fact]
    public async Task EnsureDomainSwallows409Conflict()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.Conflict,
            @"{""success"":false,""error"":{""code"":""ALREADY_EXISTS"",""message"":""domain already exists""}}");
        var node = await client.EnsureDomainAsync("homes.au");
        Assert.NotNull(node);
        Assert.Equal("success", node!["status"]!.GetValue<string>());
    }

    [Fact]
    public async Task EnsureDomainRethrowsOtherErrors()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.BadRequest,
            @"{""success"":false,""error"":{""code"":""INVALID"",""message"":""bad name""}}");
        await Assert.ThrowsAsync<QuidnugValidationException>(
            () => client.EnsureDomainAsync("oops"));
    }

    [Fact]
    public async Task GetNodeDomainsRoute()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""managedDomains"":[""a"",""b""]}}");
        var node = await client.GetNodeDomainsAsync();
        Assert.EndsWith("/api/node/domains", h.Requests[0].RequestUri!.AbsolutePath);
        Assert.Equal(2, (node!["managedDomains"] as JsonArray)!.Count);
    }

    [Fact]
    public async Task UpdateNodeDomainsSendsList()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK, @"{""success"":true,""data"":{""ok"":true}}");
        await client.UpdateNodeDomainsAsync(new[] { "a.home", "b.home" });
        Assert.Equal(HttpMethod.Post, h.Requests[0].Method);
        Assert.EndsWith("/api/node/domains", h.Requests[0].RequestUri!.AbsolutePath);
        var body = JsonNode.Parse(h.Bodies[0])!;
        var arr = body["managedDomains"] as JsonArray;
        Assert.NotNull(arr);
        Assert.Equal(2, arr!.Count);
        Assert.Equal("a.home", arr[0]!.GetValue<string>());
    }

    // --- IPFS --------------------------------------------------------------

    [Fact]
    public async Task IpfsPinSendsBytesAndReturnsCid()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""cid"":""bafy123""}}");
        var content = new byte[] { 1, 2, 3, 4 };
        string cid = await client.IpfsPinAsync(content);
        Assert.Equal("bafy123", cid);
        Assert.EndsWith("/api/ipfs/pin", h.Requests[0].RequestUri!.AbsolutePath);
        Assert.Equal("application/octet-stream",
            h.Requests[0].Content!.Headers.ContentType!.MediaType);
        Assert.Equal(content, h.RawBodies[0]);
    }

    [Fact]
    public async Task IpfsPinUsesValueAlias()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""value"":""bafyXYZ""}}");
        string cid = await client.IpfsPinAsync(new byte[] { 1 });
        Assert.Equal("bafyXYZ", cid);
    }

    [Fact]
    public async Task IpfsGetReturnsBytes()
    {
        var (client, h) = MakeClient();
        var payload = Encoding.UTF8.GetBytes("hello quidnug");
        h.EnqueueBytes(HttpStatusCode.OK, payload, "application/octet-stream");
        var got = await client.IpfsGetAsync("bafy123");
        Assert.Equal(payload, got);
        Assert.EndsWith("/api/ipfs/bafy123", h.Requests[0].RequestUri!.AbsolutePath);
    }

    [Fact]
    public async Task IpfsGetThrowsOn404()
    {
        var (client, h) = MakeClient();
        h.EnqueueBytes(HttpStatusCode.NotFound, Array.Empty<byte>(), "text/plain");
        await Assert.ThrowsAsync<QuidnugNodeException>(
            () => client.IpfsGetAsync("bafy-missing"));
    }

    // --- Commit-wait helpers -----------------------------------------------

    [Fact]
    public async Task WaitForIdentityReturnsOnFirstHit()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""quidId"":""q1"",""name"":""Q"",""homeDomain"":null,""publicKey"":""abc"",""updateNonce"":1}}");
        var rec = await client.WaitForIdentityAsync(
            "q1", null, TimeSpan.FromSeconds(5), TimeSpan.FromMilliseconds(10));
        Assert.Equal("q1", rec.QuidId);
    }

    [Fact]
    public async Task WaitForIdentityPollsUntilCommitted()
    {
        var (client, h) = MakeClient();
        // First two attempts: not found. Third: present.
        h.EnqueueJson(HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""nope""}}");
        h.EnqueueJson(HttpStatusCode.NotFound,
            @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""nope""}}");
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""quidId"":""q2"",""name"":null,""homeDomain"":null,""publicKey"":null,""updateNonce"":1}}");
        var rec = await client.WaitForIdentityAsync(
            "q2", null, TimeSpan.FromSeconds(2), TimeSpan.FromMilliseconds(5));
        Assert.Equal("q2", rec.QuidId);
        Assert.Equal(3, h.Requests.Count);
    }

    [Fact]
    public async Task WaitForIdentityTimesOut()
    {
        var (client, h) = MakeClient();
        for (int i = 0; i < 50; i++)
        {
            h.EnqueueJson(HttpStatusCode.NotFound,
                @"{""success"":false,""error"":{""code"":""NOT_FOUND"",""message"":""nope""}}");
        }
        await Assert.ThrowsAsync<TimeoutException>(
            () => client.WaitForIdentityAsync(
                "q3", null,
                TimeSpan.FromMilliseconds(50), TimeSpan.FromMilliseconds(10)));
    }

    [Fact]
    public async Task WaitForTitleReturnsOnFirstHit()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""assetId"":""asset-1"",""domain"":""d"",""titleType"":""LAND"",""ownershipMap"":[]}}");
        var t = await client.WaitForTitleAsync(
            "asset-1", "d", TimeSpan.FromSeconds(5), TimeSpan.FromMilliseconds(10));
        Assert.Equal("asset-1", t.AssetId);
    }

    [Fact]
    public async Task WaitForIdentitiesIteratesAllIds()
    {
        var (client, h) = MakeClient();
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""quidId"":""q1"",""name"":null,""homeDomain"":null,""publicKey"":null,""updateNonce"":1}}");
        h.EnqueueJson(HttpStatusCode.OK,
            @"{""success"":true,""data"":{""quidId"":""q2"",""name"":null,""homeDomain"":null,""publicKey"":null,""updateNonce"":1}}");
        await client.WaitForIdentitiesAsync(
            new[] { "q1", "q2" }, null,
            TimeSpan.FromSeconds(5), TimeSpan.FromMilliseconds(10));
        Assert.Equal(2, h.Requests.Count);
    }
}
