package com.quidnug.client;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.charset.StandardCharsets;
import java.util.*;

/**
 * Canonical signable-bytes encoder — byte-for-byte compatible with the
 * Go, Python, Rust, and JavaScript Quidnug SDKs.
 *
 * <p>The rule (see {@code schemas/types/canonicalization.md}):
 * <ol>
 *   <li>Serialize the object to JSON.</li>
 *   <li>Parse it back into a generic JSON tree.</li>
 *   <li>Serialize again with <b>alphabetized keys</b> (matches Go's
 *       {@code encoding/json} output for {@code map[string]interface{}}).</li>
 *   <li>Exclude named top-level fields (typically {@code signature} and
 *       {@code txId}).</li>
 * </ol>
 *
 * <p>This matches every other SDK exactly, so a signature produced in
 * Java verifies against a signature produced anywhere else.
 */
public final class CanonicalBytes {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private CanonicalBytes() {}

    /** Return the canonical signable bytes for an object map. */
    public static byte[] of(Map<String, Object> obj, String... excludeFields) {
        try {
            JsonNode tree = MAPPER.valueToTree(obj);
            if (!tree.isObject()) {
                throw new IllegalArgumentException("expected a JSON object at the root");
            }
            ObjectNode shallow = (ObjectNode) tree;
            for (String f : excludeFields) {
                shallow.remove(f);
            }
            JsonNode sorted = sortKeysDeep(shallow);
            return MAPPER.writeValueAsString(sorted).getBytes(StandardCharsets.UTF_8);
        } catch (JsonProcessingException e) {
            throw new RuntimeException("canonicalize: " + e.getMessage(), e);
        }
    }

    static JsonNode sortKeysDeep(JsonNode n) {
        if (n.isObject()) {
            ObjectNode src = (ObjectNode) n;
            List<String> keys = new ArrayList<>();
            src.fieldNames().forEachRemaining(keys::add);
            Collections.sort(keys);
            ObjectNode out = JsonNodeFactory.instance.objectNode();
            for (String k : keys) {
                out.set(k, sortKeysDeep(src.get(k)));
            }
            return out;
        }
        if (n.isArray()) {
            ArrayNode out = JsonNodeFactory.instance.arrayNode(n.size());
            for (JsonNode item : n) {
                out.add(sortKeysDeep(item));
            }
            return out;
        }
        return n;
    }
}
