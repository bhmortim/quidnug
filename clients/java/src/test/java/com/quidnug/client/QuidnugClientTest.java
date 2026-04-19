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
}
