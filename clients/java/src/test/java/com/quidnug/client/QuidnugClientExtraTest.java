package com.quidnug.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Coverage for the methods added to reach Python parity: registry
 * queries, IPFS, domain registration, commit-wait helpers, node-domain
 * configuration, tentative-block / guardian-resignation reads.
 */
class QuidnugClientExtraTest {

    private HttpServer server;
    private String baseUrl;
    private final List<String> hitPaths = new ArrayList<>();
    private final List<String> hitMethods = new ArrayList<>();
    private final List<String> hitBodies = new ArrayList<>();
    private final List<byte[]> hitRawBodies = new ArrayList<>();
    private final List<String> hitContentTypes = new ArrayList<>();
    private final AtomicInteger responseIndex = new AtomicInteger(0);
    private final List<int[]> responseStatuses = new ArrayList<>();
    private final List<byte[]> responseBodies = new ArrayList<>();
    private final List<String> responseContentTypes = new ArrayList<>();

    @BeforeEach
    void setup() throws IOException {
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/", exchange -> {
            hitMethods.add(exchange.getRequestMethod());
            hitPaths.add(exchange.getRequestURI().toString());
            byte[] raw = exchange.getRequestBody().readAllBytes();
            hitRawBodies.add(raw);
            hitBodies.add(new String(raw, StandardCharsets.UTF_8));
            hitContentTypes.add(exchange.getRequestHeaders().getFirst("Content-Type"));
            int i = Math.min(responseIndex.getAndIncrement(), responseStatuses.size() - 1);
            int[] status = responseStatuses.get(i);
            byte[] bytes = responseBodies.get(i);
            String ct = responseContentTypes.get(i);
            exchange.getResponseHeaders().set("Content-Type", ct);
            exchange.sendResponseHeaders(status[0], bytes.length);
            try (OutputStream os = exchange.getResponseBody()) { os.write(bytes); }
        });
        server.start();
        baseUrl = "http://127.0.0.1:" + server.getAddress().getPort();
    }

    @AfterEach
    void teardown() { server.stop(0); }

    private void queueJson(int status, String body) {
        responseStatuses.add(new int[]{status});
        responseBodies.add(body.getBytes(StandardCharsets.UTF_8));
        responseContentTypes.add("application/json");
    }

    private void queueRaw(int status, byte[] body, String contentType) {
        responseStatuses.add(new int[]{status});
        responseBodies.add(body);
        responseContentTypes.add(contentType);
    }

    private QuidnugClient client() {
        return QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
    }

    // --- Registry queries -------------------------------------------------

    @Test
    void queryIdentityRegistryBuildsPaginatedPath() {
        queueJson(200, "{\"success\":true,\"data\":{\"items\":[]}}");
        JsonNode out = client().queryIdentityRegistry(50, 100, "demo.home");
        assertTrue(out.has("items"));
        assertEquals("GET", hitMethods.get(0));
        assertTrue(hitPaths.get(0).contains("/api/registry/identity?"));
        assertTrue(hitPaths.get(0).contains("limit=50"));
        assertTrue(hitPaths.get(0).contains("offset=100"));
        assertTrue(hitPaths.get(0).contains("domain=demo.home"));
    }

    @Test
    void queryIdentityRegistryOmitsZeroParams() {
        queueJson(200, "{\"success\":true,\"data\":{}}");
        client().queryIdentityRegistry(0, 0, null);
        assertEquals("/api/registry/identity", hitPaths.get(0));
    }

    @Test
    void queryTrustRegistryBuildsPath() {
        queueJson(200, "{\"success\":true,\"data\":{}}");
        client().queryTrustRegistry(10, 0, null);
        assertTrue(hitPaths.get(0).startsWith("/api/registry/trust"));
        assertTrue(hitPaths.get(0).contains("limit=10"));
    }

    @Test
    void queryTitleRegistryBuildsPath() {
        queueJson(200, "{\"success\":true,\"data\":{}}");
        client().queryTitleRegistry(0, 5, "x.home");
        assertTrue(hitPaths.get(0).startsWith("/api/registry/title"));
        assertTrue(hitPaths.get(0).contains("offset=5"));
        assertTrue(hitPaths.get(0).contains("domain=x.home"));
    }

    @Test
    void queryRelationalTrustPostsBodyAndDecodes() {
        queueJson(200,
                "{\"success\":true,\"data\":{\"observer\":\"a\",\"target\":\"b\","
                + "\"trustLevel\":0.42,\"trustPath\":[\"a\",\"b\"],\"pathDepth\":1,\"domain\":\"d\"}}");
        Types.TrustResult tr = client().queryRelationalTrust("a", "b", "d", 5);
        assertEquals("a", tr.observer);
        assertEquals("b", tr.target);
        assertEquals(0.42, tr.trustLevel, 1e-9);
        assertEquals(1, tr.pathDepth);
        assertEquals("POST", hitMethods.get(0));
        assertTrue(hitPaths.get(0).endsWith("/api/trust/query"));
        assertTrue(hitBodies.get(0).contains("\"maxDepth\":5"));
    }

    @Test
    void queryDomainBuildsPath() {
        queueJson(200, "{\"success\":true,\"data\":{\"hits\":3}}");
        JsonNode out = client().queryDomain("demo.home", "identity", "alice");
        assertEquals(3, out.get("hits").asInt());
        assertTrue(hitPaths.get(0).startsWith("/api/domains/demo.home/query"));
        assertTrue(hitPaths.get(0).contains("type=identity"));
        assertTrue(hitPaths.get(0).contains("param=alice"));
    }

    // --- Reads ------------------------------------------------------------

    @Test
    void getGuardianResignationsParsesArray() {
        queueJson(200,
                "{\"success\":true,\"data\":{\"data\":["
                + "{\"guardianQuid\":\"g1\"},{\"guardianQuid\":\"g2\"}]}}");
        List<JsonNode> rs = client().getGuardianResignations("alice");
        assertEquals(2, rs.size());
        assertEquals("g1", rs.get(0).get("guardianQuid").asText());
    }

    @Test
    void getGuardianResignationsEmptyOnNotFound() {
        queueJson(404, "{\"success\":false,\"error\":{\"code\":\"NOT_FOUND\"}}");
        assertEquals(0, client().getGuardianResignations("alice").size());
    }

    @Test
    void getTentativeBlocksHitsPath() {
        queueJson(200, "{\"success\":true,\"data\":{\"blocks\":[]}}");
        JsonNode out = client().getTentativeBlocks("demo.home");
        assertTrue(out.has("blocks"));
        assertEquals("/api/blocks/tentative/demo.home", hitPaths.get(0));
    }

    @Test
    void nodeDomainsRoundTrip() {
        queueJson(200, "{\"success\":true,\"data\":{\"managedDomains\":[\"a\",\"b\"]}}");
        queueJson(200, "{\"success\":true,\"data\":{\"updated\":true}}");
        JsonNode got = client().getNodeDomains();
        assertEquals("a", got.get("managedDomains").get(0).asText());
        JsonNode updated = client().updateNodeDomains(List.of("c", "d"));
        assertTrue(updated.get("updated").asBoolean());
        assertEquals("GET", hitMethods.get(0));
        assertEquals("POST", hitMethods.get(1));
        assertTrue(hitBodies.get(1).contains("\"managedDomains\":[\"c\",\"d\"]"));
    }

    // --- IPFS -------------------------------------------------------------

    @Test
    void ipfsPinSendsBinaryAndReturnsCid() {
        queueJson(200, "{\"success\":true,\"data\":{\"cid\":\"bafybeigdyrz\"}}");
        byte[] payload = "hello-ipfs".getBytes(StandardCharsets.UTF_8);
        String cid = client().ipfsPin(payload);
        assertEquals("bafybeigdyrz", cid);
        assertEquals("POST", hitMethods.get(0));
        assertEquals("application/octet-stream", hitContentTypes.get(0));
        assertArrayEquals(payload, hitRawBodies.get(0));
    }

    @Test
    void ipfsPinFallsBackToValueField() {
        queueJson(200, "{\"success\":true,\"data\":{\"value\":\"bafy-fallback\"}}");
        assertEquals("bafy-fallback", client().ipfsPin(new byte[]{1, 2, 3}));
    }

    @Test
    void ipfsGetReturnsRawBytes() {
        byte[] payload = new byte[]{0, 1, 2, 3, (byte) 0xff};
        queueRaw(200, payload, "application/octet-stream");
        byte[] got = client().ipfsGet("bafyabc");
        assertArrayEquals(payload, got);
        assertTrue(hitPaths.get(0).endsWith("/api/ipfs/bafyabc"));
    }

    @Test
    void ipfsGetRaisesOnHttpError() {
        queueRaw(404, "not found".getBytes(StandardCharsets.UTF_8), "text/plain");
        assertThrows(QuidnugException.NodeException.class, () -> client().ipfsGet("bafy-missing"));
    }

    // --- Domain registration ----------------------------------------------

    @Test
    void registerDomainPostsName() {
        queueJson(200, "{\"success\":true,\"data\":{\"name\":\"new.home\"}}");
        JsonNode out = client().registerDomain("new.home");
        assertEquals("new.home", out.get("name").asText());
        assertTrue(hitBodies.get(0).contains("\"name\":\"new.home\""));
    }

    @Test
    void ensureDomainSwallowsAlreadyExists() {
        queueJson(400,
                "{\"success\":false,\"error\":{\"code\":\"ALREADY_EXISTS\","
                + "\"message\":\"trust domain already exists\"}}");
        JsonNode out = client().ensureDomain("already.home");
        assertEquals("success", out.get("status").asText());
        assertEquals("already.home", out.get("domain").asText());
    }

    @Test
    void ensureDomainPropagatesOtherErrors() {
        queueJson(400,
                "{\"success\":false,\"error\":{\"code\":\"INVALID\",\"message\":\"bad name\"}}");
        assertThrows(QuidnugException.ValidationException.class,
                () -> client().ensureDomain("..."));
    }

    // --- Commit-wait helpers ---------------------------------------------

    @Test
    void waitForIdentityReturnsOnFirstSuccess() {
        queueJson(200,
                "{\"success\":true,\"data\":{\"quidId\":\"alice\",\"name\":\"Alice\","
                + "\"updateNonce\":1}}");
        Types.IdentityRecord rec = client().waitForIdentity(
                "alice", null, Duration.ofSeconds(2), Duration.ofMillis(20));
        assertEquals("alice", rec.quidId);
        assertEquals(1, hitMethods.size());
    }

    @Test
    void waitForIdentityPollsThenSucceeds() {
        queueJson(404, "{\"success\":false,\"error\":{\"code\":\"NOT_FOUND\"}}");
        queueJson(404, "{\"success\":false,\"error\":{\"code\":\"NOT_FOUND\"}}");
        queueJson(200,
                "{\"success\":true,\"data\":{\"quidId\":\"a\",\"updateNonce\":2}}");
        Types.IdentityRecord rec = client().waitForIdentity(
                "a", null, Duration.ofSeconds(5), Duration.ofMillis(5));
        assertEquals("a", rec.quidId);
        assertEquals(3, hitMethods.size());
    }

    @Test
    void waitForIdentityTimesOut() {
        for (int i = 0; i < 50; i++) {
            queueJson(404, "{\"success\":false,\"error\":{\"code\":\"NOT_FOUND\"}}");
        }
        assertThrows(QuidnugException.NodeException.class,
                () -> client().waitForIdentity(
                        "ghost", null, Duration.ofMillis(80), Duration.ofMillis(10)));
    }

    @Test
    void waitForIdentitiesWaitsForAll() {
        queueJson(200, "{\"success\":true,\"data\":{\"quidId\":\"a\",\"updateNonce\":1}}");
        queueJson(200, "{\"success\":true,\"data\":{\"quidId\":\"b\",\"updateNonce\":1}}");
        client().waitForIdentities(
                List.of("a", "b"), null, Duration.ofSeconds(2), Duration.ofMillis(10));
        assertEquals(2, hitMethods.size());
    }

    @Test
    void waitForTitleReturnsOnFirstSuccess() {
        queueJson(200,
                "{\"success\":true,\"data\":{\"assetId\":\"asset-1\",\"domain\":\"d\","
                + "\"titleType\":\"deed\",\"ownershipMap\":[]}}");
        Types.Title t = client().waitForTitle(
                "asset-1", "d", Duration.ofSeconds(2), Duration.ofMillis(10));
        assertEquals("asset-1", t.assetId);
    }
}
