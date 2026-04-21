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
 * <p>This class ships two helpers:
 *
 * <dl>
 *   <dt>{@link #v1Of(Map, String...)} — v1.0-spec-conformant</dt>
 *   <dd>Top-level keys preserve caller insertion order (must match
 *       the Go struct declaration order for the tx type). Nested
 *       {@code Map<String,Object>} payloads and attributes are
 *       recursively sorted alphabetically (matches Go's
 *       {@code encoding/json} default for
 *       {@code map[string]interface{}}).
 *       <br><b>Use this for any transaction bound for a v1.0 node.</b></dd>
 *
 *   <dt>{@link #of(Map, String...)} — legacy fully-sorted mode</dt>
 *   <dd>Sorts <em>every</em> level of the tree alphabetically,
 *       including the top-level keys. This was the pre-v1.0 encoding.
 *       It does <b>not</b> match the v1.0 canonical form and
 *       signatures produced this way will not verify against a
 *       v1.0 node. Kept only for backward compatibility with the
 *       pre-v1.0 {@code QuidnugClient} code path.</dd>
 * </dl>
 *
 * <p>Until {@code QuidnugClient} is fully converted to v1.0 wire
 * shapes (tracked as a separate task), direct users of the Java SDK
 * that need to sign v1.0 transactions should build their own
 * top-level field lists (mirroring
 * {@code docs/test-vectors/v1.0/README.md}) and pass them through
 * {@link #v1Of(Map, String...)}. The {@code VectorsTest} class in
 * {@code src/test/java} has worked examples.
 */
public final class CanonicalBytes {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private CanonicalBytes() {}

    /**
     * v1.0-conformant canonical bytes: preserve top-level caller
     * insertion order, sort nested object keys alphabetically.
     *
     * <p>Pass transaction fields into the {@code obj} map in Go
     * struct declaration order for the tx type. Any nested
     * {@code Map<String,Object>} fields (payload / attributes) will
     * have their keys sorted before serialization.
     */
    public static byte[] v1Of(Map<String, Object> obj, String... excludeFields) {
        try {
            JsonNode tree = MAPPER.valueToTree(obj);
            if (!tree.isObject()) {
                throw new IllegalArgumentException("expected a JSON object at the root");
            }
            ObjectNode shallow = (ObjectNode) tree;
            for (String f : excludeFields) {
                shallow.remove(f);
            }
            JsonNode normalized = sortNestedKeysOnly(shallow);
            return MAPPER.writeValueAsString(normalized).getBytes(StandardCharsets.UTF_8);
        } catch (JsonProcessingException e) {
            throw new RuntimeException("canonicalize: " + e.getMessage(), e);
        }
    }

    /**
     * Legacy fully-sorted canonical bytes. Sorts every level.
     *
     * <p>Does <b>not</b> match the v1.0 canonical form. Kept only
     * for backward compatibility with pre-v1.0 signing paths.
     *
     * @deprecated since 2.0; use {@link #v1Of(Map, String...)} for
     *             signatures bound for a v1.0 node.
     */
    @Deprecated
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

    /**
     * Recursively sort the keys of every object in the tree — including
     * the root. Used only by the legacy {@link #of(Map, String...)}
     * helper; new code should prefer {@link #sortNestedKeysOnly(JsonNode)}.
     */
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

    /**
     * Preserve the root object's key order, but recursively sort
     * any nested object's keys alphabetically. Arrays preserve
     * index order; their elements are recursively normalized.
     */
    static JsonNode sortNestedKeysOnly(JsonNode n) {
        if (n.isObject()) {
            ObjectNode src = (ObjectNode) n;
            ObjectNode out = JsonNodeFactory.instance.objectNode();
            src.fieldNames().forEachRemaining(k -> out.set(k, sortKeysDeep(src.get(k))));
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
