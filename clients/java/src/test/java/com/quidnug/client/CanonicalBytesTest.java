package com.quidnug.client;

import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class CanonicalBytesTest {

    @Test
    void stableAcrossInsertionOrder() {
        Map<String, Object> a = new LinkedHashMap<>();
        a.put("b", 1);
        a.put("a", 2);
        Map<String, Object> b = new LinkedHashMap<>();
        b.put("a", 2);
        b.put("b", 1);
        assertArrayEquals(CanonicalBytes.of(a), CanonicalBytes.of(b));
    }

    @Test
    void excludesNamedFields() {
        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("type", "TRUST");
        tx.put("signature", "abc");
        tx.put("level", 0.9);
        String out = new String(CanonicalBytes.of(tx, "signature"), StandardCharsets.UTF_8);
        assertFalse(out.contains("signature"));
        assertTrue(out.contains("level"));
        assertTrue(out.contains("TRUST"));
    }

    @Test
    void sortsNestedKeys() {
        Map<String, Object> nested = new LinkedHashMap<>();
        nested.put("z", 1);
        nested.put("a", 2);
        Map<String, Object> outer = new LinkedHashMap<>();
        outer.put("outer", "x");
        outer.put("nested", nested);
        String out = new String(CanonicalBytes.of(outer), StandardCharsets.UTF_8);
        assertEquals("{\"nested\":{\"a\":2,\"z\":1},\"outer\":\"x\"}", out);
    }

    /**
     * Interop lock: the Java output for this transaction MUST equal
     * the reference string that Python / Go / Rust also produce.
     * If this diverges, a Java-signed tx will not verify on a
     * Go-reference node.
     *
     * The key gotcha: Jackson's writeValueAsString emits raw UTF-8
     * for non-ASCII by default, matching Go. If Jackson is ever
     * reconfigured to escape non-ASCII, this test will fail before
     * the bug reaches production.
     */
    @Test
    void utf8InteropLock() {
        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("message", "hello 世界 🌍");
        tx.put("a", 1);
        String actual = new String(CanonicalBytes.of(tx), StandardCharsets.UTF_8);
        String expected = "{\"a\":1,\"message\":\"hello 世界 🌍\"}";
        assertEquals(expected, actual,
            "UTF-8 interop broken: Java must emit raw UTF-8, not escaped unicode");
    }

    /**
     * v1Of preserves top-level caller insertion order (Go struct
     * declaration order for the tx type) while still sorting
     * nested-object keys alphabetically to match Go's
     * encoding/json output for map[string]interface{}.
     *
     * This is the behavior v1.0 nodes require. The legacy
     * {@link CanonicalBytes#of(Map, String...)} helper sorts
     * everything and does NOT match v1.0; signatures built via
     * that helper will not verify against a v1.0 server.
     */
    @Test
    void v1OfPreservesTopLevelOrderSortsNested() {
        // Struct-declaration-order insertion (mirrors an EVENT
        // tx: id, type, trustDomain, timestamp, ...).
        Map<String, Object> nestedPayload = new LinkedHashMap<>();
        nestedPayload.put("zebra", 1);
        nestedPayload.put("apple", 2);
        nestedPayload.put("mango", 3);
        Map<String, Object> deeper = new LinkedHashMap<>();
        deeper.put("yankee", "A");
        deeper.put("alpha", "B");
        deeper.put("mike", "C");
        nestedPayload.put("nested", deeper);

        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("id", "abc");
        tx.put("type", "EVENT");
        tx.put("trustDomain", "test");
        tx.put("timestamp", 1729468800L);
        tx.put("payload", nestedPayload);

        String out = new String(
            CanonicalBytes.v1Of(tx), StandardCharsets.UTF_8);

        // Top level: id, type, trustDomain, timestamp, payload
        // (NOT alphabetical — matches struct declaration order).
        // Payload nested: keys sorted alphabetically.
        String expected = "{\"id\":\"abc\","
            + "\"type\":\"EVENT\","
            + "\"trustDomain\":\"test\","
            + "\"timestamp\":1729468800,"
            + "\"payload\":{"
            +   "\"apple\":2,"
            +   "\"mango\":3,"
            +   "\"nested\":{\"alpha\":\"B\",\"mike\":\"C\",\"yankee\":\"A\"},"
            +   "\"zebra\":1"
            + "}}";
        assertEquals(expected, out,
            "v1Of must preserve top-level order and sort nested keys");
    }

    /**
     * Regression: legacy {@link CanonicalBytes#of(Map, String...)}
     * sorts EVERY level (including the top). This test locks in
     * that behavior so the two helpers stay visibly distinct;
     * v1.0 users must call v1Of, not of.
     */
    @Test
    void legacyOfSortsTopLevelToo() {
        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("type", "EVENT");
        tx.put("id", "abc");
        @SuppressWarnings("deprecation")
        String out = new String(CanonicalBytes.of(tx), StandardCharsets.UTF_8);
        // Alphabetical: id before type.
        assertEquals("{\"id\":\"abc\",\"type\":\"EVENT\"}", out);
    }
}
