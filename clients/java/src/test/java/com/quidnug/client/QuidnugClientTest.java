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
 * Uses the built-in jdk.httpserver to stub responses — no dependency on
 * mockwebserver / WireMock so the test module stays light.
 */
class QuidnugClientTest {

    private HttpServer server;
    private String baseUrl;
    private final List<String> hitPaths = new ArrayList<>();
    private final List<String> hitMethods = new ArrayList<>();
    private final List<String> hitBodies = new ArrayList<>();
    private final AtomicInteger responseIndex = new AtomicInteger(0);
    private final List<int[]> responseStatuses = new ArrayList<>();
    private final List<String> responseBodies = new ArrayList<>();

    @BeforeEach
    void setup() throws IOException {
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/", exchange -> {
            hitMethods.add(exchange.getRequestMethod());
            hitPaths.add(exchange.getRequestURI().toString());
            hitBodies.add(new String(
                    exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
            int i = Math.min(responseIndex.getAndIncrement(), responseStatuses.size() - 1);
            int[] status = responseStatuses.get(i);
            String body = responseBodies.get(i);
            byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(status[0], bytes.length);
            try (OutputStream os = exchange.getResponseBody()) { os.write(bytes); }
        });
        server.start();
        baseUrl = "http://127.0.0.1:" + server.getAddress().getPort();
    }

    @AfterEach
    void teardown() {
        server.stop(0);
    }

    private void queue(int status, String body) {
        responseStatuses.add(new int[]{status});
        responseBodies.add(body);
    }

    @Test
    void grantTrustPostsCorrectEnvelope() throws Exception {
        queue(200, "{\"success\":true,\"data\":{\"txId\":\"abc\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        Quid alice = Quid.generate();
        JsonNode data = c.grantTrust(alice, QuidnugClient.TrustParams.of("bob", 0.9, "demo.home"));
        assertEquals("abc", data.get("txId").asText());
        assertEquals("POST", hitMethods.get(0));
        assertTrue(hitPaths.get(0).endsWith("/api/transactions/trust"));
        assertTrue(hitBodies.get(0).contains("\"type\":\"TRUST\""));
        assertTrue(hitBodies.get(0).contains("\"trustee\":\"bob\""));
        assertTrue(hitBodies.get(0).contains("\"signature\":"));
    }

    @Test
    void grantTrustValidatesLevelRange() {
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        Quid alice = Quid.generate();
        assertThrows(QuidnugException.ValidationException.class,
                () -> c.grantTrust(alice, QuidnugClient.TrustParams.of("bob", 1.5, "x")));
    }

    @Test
    void conflictEnvelopeRaisesConflict() {
        queue(409, "{\"success\":false,\"error\":{\"code\":\"NONCE_REPLAY\",\"message\":\"replay\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        Quid alice = Quid.generate();
        QuidnugException.ConflictException ex = assertThrows(
                QuidnugException.ConflictException.class,
                () -> c.grantTrust(alice, QuidnugClient.TrustParams.of("b", 0.5, "x")));
        assertTrue(ex.getMessage().toLowerCase().contains("replay"));
    }

    @Test
    void serviceUnavailableRaisesUnavailable() {
        queue(503, "{\"success\":false,\"error\":{\"code\":\"BOOTSTRAPPING\",\"message\":\"warm\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        assertThrows(QuidnugException.UnavailableException.class, c::health);
    }

    @Test
    void retriesTransient5xx() {
        queue(500, "{\"success\":false,\"error\":{\"code\":\"INTERNAL\"}}");
        queue(500, "{\"success\":false,\"error\":{\"code\":\"INTERNAL\"}}");
        queue(200, "{\"success\":true,\"data\":{\"ok\":true}}");
        QuidnugClient c = QuidnugClient.builder()
                .baseUrl(baseUrl)
                .maxRetries(3)
                .retryBaseDelay(Duration.ofMillis(5))
                .build();
        JsonNode data = c.health();
        assertTrue(data.get("ok").asBoolean());
        assertEquals(3, hitMethods.size());
    }

    @Test
    void postsAreNotRetriedByDefault() {
        queue(500, "{\"success\":false,\"error\":{\"code\":\"INTERNAL\"}}");
        QuidnugClient c = QuidnugClient.builder()
                .baseUrl(baseUrl)
                .maxRetries(3)
                .retryBaseDelay(Duration.ofMillis(5))
                .build();
        Quid q = Quid.generate();
        assertThrows(QuidnugException.NodeException.class,
                () -> c.grantTrust(q, QuidnugClient.TrustParams.of("bob", 0.5, "x")));
        assertEquals(1, hitMethods.size());
    }

    @Test
    void getIdentityReturnsNullOnNotFound() {
        queue(404, "{\"success\":false,\"error\":{\"code\":\"NOT_FOUND\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        assertNull(c.getIdentity("missing"));
    }

    // --- Coverage for the v1 + v2 surface added in the audit sweep --------

    @Test
    void peersHitsPeersEndpoint() {
        queue(200, "{\"success\":true,\"data\":{\"peers\":[],\"count\":0}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.peers();
        assertTrue(hitPaths.get(0).endsWith("/api/peers"));
    }

    @Test
    void getPeerReturnsNullOnPeerNotFound() {
        queue(404, "{\"success\":false,\"error\":{\"code\":\"PEER_NOT_FOUND\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        assertNull(c.getPeer("nodex"));
    }

    @Test
    void topDomainsHitsExpectedPath() {
        queue(200, "{\"success\":true,\"data\":[]}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.topDomains();
        assertTrue(hitPaths.get(0).endsWith("/api/domains/top"));
    }

    @Test
    void queryDomainBuildsTypeAndParamQueryString() {
        queue(200, "{\"success\":true,\"data\":{}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.queryDomain("foo.com", "trust", "a:b");
        String path = hitPaths.get(0);
        assertTrue(path.contains("/api/domains/foo.com/query"));
        assertTrue(path.contains("type=trust"));
        assertTrue(path.contains("param=a%3Ab"));
    }

    @Test
    void queryDomainRejectsBadType() {
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        assertThrows(QuidnugException.ValidationException.class,
                () -> c.queryDomain("x", "bogus", "y"));
    }

    @Test
    void registerDomainPostsName() {
        queue(200, "{\"success\":true,\"data\":{\"domain\":\"x.example\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.registerDomain("x.example", null);
        assertEquals("POST", hitMethods.get(0));
        assertTrue(hitBodies.get(0).contains("\"name\":\"x.example\""));
    }

    @Test
    void createModerationActionPostsBody() {
        queue(200, "{\"success\":true,\"data\":{\"id\":\"tx\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        java.util.LinkedHashMap<String, Object> action = new java.util.LinkedHashMap<>();
        action.put("moderatorQuid", "mod");
        action.put("targetType", "QUID");
        action.put("targetId", "t");
        action.put("scope", "hide");
        action.put("reasonCode", "spam");
        action.put("nonce", 1);
        c.createModerationAction(action);
        assertTrue(hitPaths.get(0).endsWith("/api/moderation/actions"));
        assertTrue(hitBodies.get(0).contains("\"moderatorQuid\":\"mod\""));
    }

    @Test
    void getModerationActionsBuildsTargetPath() {
        queue(200, "{\"success\":true,\"data\":{\"actions\":[]}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.getModerationActions("QUID", "abc");
        assertTrue(hitPaths.get(0).endsWith("/api/moderation/actions/QUID/abc"));
    }

    @Test
    void auditHeadAndEntriesAndEntry() {
        queue(200, "{\"success\":true,\"data\":{\"height\":0}}");
        queue(200, "{\"success\":true,\"data\":{\"entries\":[]}}");
        queue(404, "{\"success\":false,\"error\":{\"code\":\"NOT_FOUND\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.auditHead();
        c.auditEntries(10L, 50);
        assertNull(c.auditEntry(999L));
        assertTrue(hitPaths.get(0).endsWith("/api/audit/head"));
        assertTrue(hitPaths.get(1).contains("/api/audit/entries"));
        assertTrue(hitPaths.get(1).contains("since=10"));
        assertTrue(hitPaths.get(1).contains("limit=50"));
        assertTrue(hitPaths.get(2).endsWith("/api/audit/entry/999"));
    }

    @Test
    void createDSRAndGetStatusPaths() {
        queue(200, "{\"success\":true,\"data\":{\"id\":\"tx\"}}");
        queue(200, "{\"success\":true,\"data\":{\"request\":{}}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        java.util.LinkedHashMap<String, Object> req = new java.util.LinkedHashMap<>();
        req.put("subjectQuid", "s");
        req.put("requestType", "ERASURE");
        req.put("nonce", 1);
        c.createDSR(req);
        c.getDSRStatus("tx");
        assertTrue(hitPaths.get(0).endsWith("/api/privacy/dsr"));
        assertTrue(hitPaths.get(1).endsWith("/api/privacy/dsr/tx"));
    }

    @Test
    void consentGrantWithdrawHistory() {
        queue(200, "{\"success\":true,\"data\":{}}");
        queue(200, "{\"success\":true,\"data\":{}}");
        queue(200, "{\"success\":true,\"data\":{\"entries\":[]}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        java.util.LinkedHashMap<String, Object> g = new java.util.LinkedHashMap<>();
        g.put("subjectQuid", "s"); g.put("controllerQuid", "c");
        g.put("scope", java.util.List.of("MARKETING")); g.put("nonce", 1);
        c.createConsentGrant(g);
        java.util.LinkedHashMap<String, Object> w = new java.util.LinkedHashMap<>();
        w.put("subjectQuid", "s"); w.put("withdrawsGrantTxId", "g"); w.put("nonce", 2);
        c.createConsentWithdraw(w);
        c.getConsentHistory("subq");
        assertTrue(hitPaths.get(0).endsWith("/api/privacy/consent/grants"));
        assertTrue(hitPaths.get(1).endsWith("/api/privacy/consent/withdraws"));
        assertTrue(hitPaths.get(2).contains("/api/privacy/consent/history?subject=subq"));
    }

    @Test
    void processingRestrictionAndRestrictionsForSubject() {
        queue(200, "{\"success\":true,\"data\":{}}");
        queue(200, "{\"success\":true,\"data\":{\"restrictedUses\":[]}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        java.util.LinkedHashMap<String, Object> r = new java.util.LinkedHashMap<>();
        r.put("subjectQuid", "s");
        r.put("restrictedUses", java.util.List.of("MARKETING"));
        r.put("nonce", 1);
        c.createProcessingRestriction(r);
        c.getRestrictionsForSubject("s");
        assertTrue(hitPaths.get(0).endsWith("/api/privacy/restrictions"));
        assertTrue(hitPaths.get(1).endsWith("/api/privacy/restrictions/s"));
    }

    @Test
    void discoveryEndpointsHitV2Prefix() {
        for (int i = 0; i < 5; i++) queue(200, "{\"success\":true,\"data\":{}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.discoverDomain("foo");
        c.discoverNode("n");
        c.discoverOperator("op");
        c.discoverQuids();
        c.discoverTrustedQuids();
        assertTrue(hitPaths.get(0).endsWith("/api/v2/discovery/domain/foo"));
        assertTrue(hitPaths.get(1).endsWith("/api/v2/discovery/node/n"));
        assertTrue(hitPaths.get(2).endsWith("/api/v2/discovery/operator/op"));
        assertTrue(hitPaths.get(3).endsWith("/api/v2/discovery/quids"));
        assertTrue(hitPaths.get(4).endsWith("/api/v2/discovery/trusted-quids"));
    }

    @Test
    void dnsSurfaceCoversAllElevenEndpoints() {
        for (int i = 0; i < 10; i++) queue(200, "{\"success\":true,\"data\":{\"id\":\"tx\"}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        java.util.Map<String, Object> stub = new java.util.LinkedHashMap<>();
        stub.put("domain", "example.com");
        stub.put("claimRef", "ref");
        stub.put("attestationRef", "att");
        stub.put("delegateRef", "del");
        stub.put("ownerQuid", "own");
        stub.put("rootQuid", "root");
        stub.put("delegateQuid", "d");
        stub.put("domainScope", "x");
        stub.put("nonce", 1);
        stub.put("reason", "t");
        c.submitDNSClaim(stub);
        c.submitDNSChallenge(stub);
        c.submitDNSAttestation(stub);
        c.submitDNSRenewal(stub);
        c.submitDNSRevocation(stub);
        c.submitAuthorityDelegate(stub);
        c.submitAuthorityDelegateRevocation(stub);
        c.getDNSAttestations("example.com");
        c.getDNSAttestationsWeighted("example.com");
        c.resolveDNSRecord("example.com", "A");
        assertTrue(hitPaths.get(0).endsWith("/api/v2/dns/claim"));
        assertTrue(hitPaths.get(1).endsWith("/api/v2/dns/challenge"));
        assertTrue(hitPaths.get(2).endsWith("/api/v2/dns/attestation"));
        assertTrue(hitPaths.get(3).endsWith("/api/v2/dns/renewal"));
        assertTrue(hitPaths.get(4).endsWith("/api/v2/dns/revocation"));
        assertTrue(hitPaths.get(5).endsWith("/api/v2/dns/delegate"));
        assertTrue(hitPaths.get(6).endsWith("/api/v2/dns/delegate-revocation"));
        assertTrue(hitPaths.get(7).endsWith("/api/v2/dns/attestations/example.com"));
        assertTrue(hitPaths.get(8).endsWith("/api/v2/dns/attestations/example.com/weighted"));
        assertTrue(hitPaths.get(9).endsWith("/api/v2/dns/resolve/example.com/A"));
    }

    @Test
    void nodeDomainsRoundTrip() {
        queue(200, "{\"success\":true,\"data\":{\"managedDomains\":[]}}");
        queue(200, "{\"success\":true,\"data\":{}}");
        QuidnugClient c = QuidnugClient.builder().baseUrl(baseUrl).maxRetries(0).build();
        c.getNodeDomains();
        c.updateNodeDomains(java.util.List.of("a", "b"));
        assertTrue(hitPaths.get(0).endsWith("/api/node/domains"));
        assertEquals("POST", hitMethods.get(1));
        assertTrue(hitBodies.get(1).contains("\"managedDomains\":[\"a\",\"b\"]"));
    }
}
