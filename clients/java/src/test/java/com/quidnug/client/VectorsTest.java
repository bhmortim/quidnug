package com.quidnug.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Java SDK consumer for the v1.0 cross-SDK test vectors at
 * {@code docs/test-vectors/v1.0/}.
 *
 * Asserts the five conformance properties against the Java SDK's
 * public API ({@link Quid#sign}, {@link Quid#verify},
 * {@link Quid#fromPublicHex}, {@link Quid#fromPrivateScalarHex}).
 *
 * Run with: {@code mvn test -Dtest=VectorsTest}.
 */
class VectorsTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static Path vectorsRoot() {
        // Maven test runs from clients/java; vectors live at
        // <repo>/docs/test-vectors/v1.0.
        Path start = Paths.get(".").toAbsolutePath().normalize();
        Path p = start.resolve("../../docs/test-vectors/v1.0").normalize();
        if (!Files.isDirectory(p)) {
            throw new RuntimeException("vectors root not found at " + p);
        }
        return p;
    }

    private static Map<String, JsonNode> loadKeys() throws IOException {
        Map<String, JsonNode> out = new HashMap<>();
        File[] files = vectorsRoot().resolve("test-keys").toFile().listFiles(
                (d, n) -> n.endsWith(".json"));
        if (files == null) throw new RuntimeException("no keys dir");
        for (File f : files) {
            JsonNode k = MAPPER.readTree(f);
            out.put(k.get("name").asText(), k);
        }
        return out;
    }

    private static JsonNode loadVectorFile(String name) throws IOException {
        return MAPPER.readTree(vectorsRoot().resolve(name).toFile());
    }

    /**
     * Execute all conformance properties for a single case. The
     * caller supplies the canonical signable bytes + derived ID
     * built via whatever tx-type-specific builder applies.
     */
    private static void runCase(
            JsonNode caseNode, JsonNode key,
            byte[] signable, String derivedId) throws Exception {

        JsonNode expected = caseNode.get("expected");
        String name = caseNode.get("name").asText();

        // Property 1: sha256 of canonical bytes matches.
        MessageDigest md = MessageDigest.getInstance("SHA-256");
        String gotSha = Hex.encode(md.digest(signable));
        assertEquals(
                expected.get("sha256_of_canonical_hex").asText(),
                gotSha,
                name + ": SHA-256 mismatch");

        // Property 2: canonical hex + utf8 match the vector.
        assertEquals(
                expected.get("canonical_signable_bytes_hex").asText(),
                Hex.encode(signable),
                name + ": canonical hex diverges");
        assertEquals(
                expected.get("canonical_signable_bytes_utf8").asText(),
                new String(signable, StandardCharsets.UTF_8),
                name + ": canonical utf8 diverges");

        // Property 3: derived ID matches.
        assertEquals(
                expected.get("expected_id").asText(),
                derivedId,
                name + ": ID derivation mismatch");

        // Property 4: reference signature verifies via Quid.verify.
        Quid qRO = Quid.fromPublicHex(key.get("public_key_sec1_hex").asText());
        assertTrue(
                qRO.verify(signable, expected.get("reference_signature_hex").asText()),
                name + ": SDK Verify rejected reference signature");
        assertEquals(
                64,
                expected.get("signature_length_bytes").asInt(),
                name + ": signature_length_bytes != 64");

        // Property 5: tampered signature rejects.
        byte[] tampered = Hex.decode(expected.get("reference_signature_hex").asText());
        tampered[5] ^= 0x01;
        assertFalse(
                qRO.verify(signable, Hex.encode(tampered)),
                name + ": tampered signature accepted");

        // Property 6: independent SDK sign-verify round-trip.
        Quid qSign = Quid.fromPrivateScalarHex(key.get("private_scalar_hex").asText());
        String sdkSig = qSign.sign(signable);
        byte[] sdkSigBytes = Hex.decode(sdkSig);
        assertEquals(64, sdkSigBytes.length, name + ": SDK signature not 64 bytes");
        assertTrue(qSign.verify(signable, sdkSig),
                name + ": SDK sign-verify round-trip failed");

        // Property 7: quid_id derivation matches.
        byte[] pubBytes = Hex.decode(key.get("public_key_sec1_hex").asText());
        String derivedQuid = Hex.encode(java.util.Arrays.copyOf(
                md.digest(pubBytes), 8));
        assertEquals(key.get("quid_id").asText(), derivedQuid,
                name + ": quid_id derivation mismatch");
    }

    // -----------------------------------------------------------
    // Per-type signable-bytes builders (Go-struct-order JSON)
    // -----------------------------------------------------------
    //
    // Jackson's ObjectNode preserves insertion order as of
    // jackson-databind 2.0+, so the same pattern as pkg/client
    // (explicit field-by-field construction) produces the canonical
    // bytes we want.

    private static byte[] trustSignable(JsonNode inp) {
        var node = MAPPER.createObjectNode();
        node.put("id", inp.get("id").asText());
        node.put("type", "TRUST");
        node.put("trustDomain", inp.get("trustDomain").asText());
        node.put("timestamp", inp.get("timestamp").asLong());
        node.put("signature", "");
        node.put("publicKey", inp.get("publicKey").asText());
        node.put("truster", inp.get("truster").asText());
        node.put("trustee", inp.get("trustee").asText());
        // Float: use Go's convention via serializeGoCompatFloat.
        putGoCompatFloat(node, "trustLevel", inp.get("trustLevel").asDouble());
        node.put("nonce", inp.get("nonce").asLong());
        if (inp.hasNonNull("description") && !inp.get("description").asText().isEmpty()) {
            node.put("description", inp.get("description").asText());
        }
        if (inp.hasNonNull("validUntil") && inp.get("validUntil").asLong() != 0) {
            node.put("validUntil", inp.get("validUntil").asLong());
        }
        return nodeToBytes(node);
    }

    private static String trustId(JsonNode inp) {
        try {
            var seed = MAPPER.createObjectNode();
            seed.put("Truster", inp.get("truster").asText());
            seed.put("Trustee", inp.get("trustee").asText());
            putGoCompatFloat(seed, "TrustLevel", inp.get("trustLevel").asDouble());
            seed.put("TrustDomain", inp.get("trustDomain").asText());
            seed.put("Timestamp", inp.get("timestamp").asLong());
            byte[] bytes = MAPPER.writeValueAsBytes(seed);
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            return Hex.encode(md.digest(bytes));
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    private static byte[] identitySignable(JsonNode inp) {
        var node = MAPPER.createObjectNode();
        node.put("id", inp.get("id").asText());
        node.put("type", "IDENTITY");
        node.put("trustDomain", inp.get("trustDomain").asText());
        node.put("timestamp", inp.get("timestamp").asLong());
        node.put("signature", "");
        node.put("publicKey", inp.get("publicKey").asText());
        node.put("quidId", inp.get("quidId").asText());
        node.put("name", inp.get("name").asText());
        if (inp.hasNonNull("description") && !inp.get("description").asText().isEmpty()) {
            node.put("description", inp.get("description").asText());
        }
        if (inp.hasNonNull("attributes") && inp.get("attributes").size() > 0) {
            node.set("attributes", inp.get("attributes"));
        }
        node.put("creator", inp.get("creator").asText());
        node.put("updateNonce", inp.get("updateNonce").asLong());
        if (inp.hasNonNull("homeDomain") && !inp.get("homeDomain").asText().isEmpty()) {
            node.put("homeDomain", inp.get("homeDomain").asText());
        }
        return nodeToBytes(node);
    }

    private static String identityId(JsonNode inp) {
        try {
            var seed = MAPPER.createObjectNode();
            seed.put("QuidID", inp.get("quidId").asText());
            seed.put("Name", inp.get("name").asText());
            seed.put("Creator", inp.get("creator").asText());
            seed.put("TrustDomain", inp.get("trustDomain").asText());
            seed.put("UpdateNonce", inp.get("updateNonce").asLong());
            seed.put("Timestamp", inp.get("timestamp").asLong());
            byte[] bytes = MAPPER.writeValueAsBytes(seed);
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            return Hex.encode(md.digest(bytes));
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    private static byte[] eventSignable(JsonNode inp) {
        var node = MAPPER.createObjectNode();
        node.put("id", inp.get("id").asText());
        node.put("type", "EVENT");
        node.put("trustDomain", inp.get("trustDomain").asText());
        node.put("timestamp", inp.get("timestamp").asLong());
        node.put("signature", "");
        node.put("publicKey", inp.get("publicKey").asText());
        node.put("subjectId", inp.get("subjectId").asText());
        node.put("subjectType", inp.get("subjectType").asText());
        node.put("sequence", inp.get("sequence").asLong());
        node.put("eventType", inp.get("eventType").asText());
        if (inp.hasNonNull("payload") && inp.get("payload").size() > 0) {
            node.set("payload", inp.get("payload"));
        }
        if (inp.hasNonNull("payloadCid") && !inp.get("payloadCid").asText().isEmpty()) {
            node.put("payloadCid", inp.get("payloadCid").asText());
        }
        if (inp.hasNonNull("previousEventId") && !inp.get("previousEventId").asText().isEmpty()) {
            node.put("previousEventId", inp.get("previousEventId").asText());
        }
        return nodeToBytes(node);
    }

    private static String eventId(JsonNode inp) {
        try {
            var seed = MAPPER.createObjectNode();
            seed.put("SubjectID", inp.get("subjectId").asText());
            seed.put("EventType", inp.get("eventType").asText());
            seed.put("Sequence", inp.get("sequence").asLong());
            seed.put("TrustDomain", inp.get("trustDomain").asText());
            seed.put("Timestamp", inp.get("timestamp").asLong());
            byte[] bytes = MAPPER.writeValueAsBytes(seed);
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            return Hex.encode(md.digest(bytes));
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    private static byte[] nodeToBytes(JsonNode node) {
        try {
            return MAPPER.writeValueAsBytes(node);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * Put a float on an ObjectNode in Go-compatible form: integer-
     * valued floats serialize as integers, matching Go's
     * encoding/json behavior of eliding ".0" on whole-number
     * floats. Non-integer floats serialize as floats.
     */
    private static void putGoCompatFloat(
            com.fasterxml.jackson.databind.node.ObjectNode node,
            String key, double value) {
        if (Double.isFinite(value) && value == Math.floor(value) && Math.abs(value) < 1e15) {
            node.put(key, (long) value);
        } else {
            node.put(key, value);
        }
    }

    // -----------------------------------------------------------
    // Dynamic tests
    // -----------------------------------------------------------

    @TestFactory
    Stream<DynamicTest> trustVectors() throws Exception {
        JsonNode vf = loadVectorFile("trust-tx.json");
        assertEquals("TRUST", vf.get("tx_type").asText());
        Map<String, JsonNode> keys = loadKeys();
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : vf.get("cases")) {
            String name = c.get("name").asText();
            tests.add(DynamicTest.dynamicTest(name, () -> {
                JsonNode key = keys.get(c.get("signer_key_ref").asText());
                JsonNode inp = c.get("input");
                byte[] signable = trustSignable(inp);
                String id = trustId(inp);
                runCase(c, key, signable, id);
            }));
        }
        return tests.stream();
    }

    @TestFactory
    Stream<DynamicTest> identityVectors() throws Exception {
        JsonNode vf = loadVectorFile("identity-tx.json");
        assertEquals("IDENTITY", vf.get("tx_type").asText());
        Map<String, JsonNode> keys = loadKeys();
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : vf.get("cases")) {
            String name = c.get("name").asText();
            tests.add(DynamicTest.dynamicTest(name, () -> {
                JsonNode key = keys.get(c.get("signer_key_ref").asText());
                JsonNode inp = c.get("input");
                byte[] signable = identitySignable(inp);
                String id = identityId(inp);
                runCase(c, key, signable, id);
            }));
        }
        return tests.stream();
    }

    @TestFactory
    Stream<DynamicTest> eventVectors() throws Exception {
        JsonNode vf = loadVectorFile("event-tx.json");
        assertEquals("EVENT", vf.get("tx_type").asText());
        Map<String, JsonNode> keys = loadKeys();
        List<DynamicTest> tests = new ArrayList<>();
        for (JsonNode c : vf.get("cases")) {
            String name = c.get("name").asText();
            tests.add(DynamicTest.dynamicTest(name, () -> {
                JsonNode key = keys.get(c.get("signer_key_ref").asText());
                JsonNode inp = c.get("input");
                byte[] signable = eventSignable(inp);
                String id = eventId(inp);
                runCase(c, key, signable, id);
            }));
        }
        return tests.stream();
    }

    @Test
    void sdkSignProducesIEEE1363() throws Exception {
        Quid q = Quid.generate();
        byte[] data = "test-data".getBytes(StandardCharsets.UTF_8);
        String sigHex = q.sign(data);
        assertEquals(128, sigHex.length(),
                "v1.0 mandates 64-byte IEEE-1363 (128 hex chars)");
        assertTrue(q.verify(data, sigHex), "SDK Sign/Verify round-trip failed");
    }
}
