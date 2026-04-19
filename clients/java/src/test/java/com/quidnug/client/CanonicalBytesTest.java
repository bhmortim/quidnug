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
}
