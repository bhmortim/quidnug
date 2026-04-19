package com.quidnug.client;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.time.Duration;
import java.time.Instant;
import java.util.*;

import static com.quidnug.client.QuidnugException.*;

/**
 * Strongly-typed HTTP client for a Quidnug node.
 *
 * <p>Covers the full v2 protocol surface (QDPs 0001–0010): identity,
 * trust, titles, event streams, anchors, guardian sets + recovery,
 * cross-domain gossip, K-of-K bootstrap, fork-block activation, and
 * compact Merkle inclusion proofs.
 *
 * <p>Thread-safe: one instance may be shared across request-handling
 * threads. The underlying {@link HttpClient} is reused.
 *
 * <p><b>Retry policy.</b> GETs retry up to {@code maxRetries} times on
 * HTTP 5xx or 429 with exponential backoff + ±100ms jitter. POSTs are
 * <i>not</i> retried — reconcile via a follow-up GET before replaying
 * a write.
 *
 * <h3>Quickstart</h3>
 * <pre>{@code
 * QuidnugClient client = QuidnugClient.builder()
 *     .baseUrl("http://localhost:8080")
 *     .build();
 *
 * Quid alice = Quid.generate();
 * Quid bob   = Quid.generate();
 *
 * client.registerIdentity(alice, IdentityParams.name("Alice").homeDomain("contractors.home"));
 * client.registerIdentity(bob,   IdentityParams.name("Bob").homeDomain("contractors.home"));
 * client.grantTrust(alice, TrustParams.of(bob.id(), 0.9, "contractors.home"));
 *
 * TrustResult tr = client.getTrust(alice.id(), bob.id(), "contractors.home", 5);
 * System.out.printf("%.3f via %s%n", tr.trustLevel, String.join(" -> ", tr.path));
 * }</pre>
 */
public final class QuidnugClient {

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final Set<String> CONFLICT_CODES = Set.of(
            "NONCE_REPLAY", "GUARDIAN_SET_MISMATCH", "QUORUM_NOT_MET", "VETOED",
            "INVALID_SIGNATURE", "FORK_ALREADY_ACTIVE", "DUPLICATE",
            "ALREADY_EXISTS", "INVALID_STATE_TRANSITION");
    private static final Set<String> UNAVAILABLE_CODES = Set.of(
            "FEATURE_NOT_ACTIVE", "NOT_READY", "BOOTSTRAPPING");

    private final String apiBase;
    private final HttpClient http;
    private final Duration timeout;
    private final int maxRetries;
    private final Duration retryBaseDelay;
    private final String authToken;
    private final String userAgent;

    private QuidnugClient(Builder b) {
        this.apiBase        = b.baseUrl.replaceAll("/+$", "") + "/api";
        this.http           = b.http != null ? b.http : HttpClient.newBuilder()
                                   .connectTimeout(b.timeout)
                                   .build();
        this.timeout        = b.timeout;
        this.maxRetries     = b.maxRetries;
        this.retryBaseDelay = b.retryBaseDelay;
        this.authToken      = b.authToken;
        this.userAgent      = b.userAgent;
    }

    public static Builder builder() { return new Builder(); }

    // =====================================================================
    // Health / info / peers
    // =====================================================================

    public JsonNode health()       { return doGet("health"); }
    public JsonNode info()         { return doGet("info"); }
    public JsonNode nodes()        { return doGet("nodes"); }
    public JsonNode blocks()       { return doGet("blocks"); }
    public JsonNode pendingTransactions() { return doGet("transactions"); }
    public JsonNode listDomains()  { return doGet("domains"); }

    // =====================================================================
    // Identity
    // =====================================================================

    public JsonNode registerIdentity(Quid signer, IdentityParams p) {
        requireSigner(signer);
        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("type", "IDENTITY");
        tx.put("timestamp", Instant.now().getEpochSecond());
        tx.put("trustDomain", p.domain == null ? "default" : p.domain);
        tx.put("signerQuid",   signer.id());
        tx.put("definerQuid",  signer.id());
        tx.put("subjectQuid",  p.subjectQuid != null ? p.subjectQuid : signer.id());
        tx.put("updateNonce",  p.updateNonce);
        tx.put("schemaVersion","1.0");
        tx.put("attributes",   p.attributes == null ? Collections.emptyMap() : p.attributes);
        if (p.name != null)         tx.put("name", p.name);
        if (p.description != null)  tx.put("description", p.description);
        if (p.homeDomain != null)   tx.put("homeDomain", p.homeDomain);
        signIntoTx(signer, tx);
        return doPost("transactions/identity", tx);
    }

    public Types.IdentityRecord getIdentity(String quidId) {
        return getIdentity(quidId, null);
    }

    public Types.IdentityRecord getIdentity(String quidId, String domain) {
        try {
            String path = "identity/" + urlencode(quidId);
            if (domain != null) path += "?domain=" + urlencode(domain);
            JsonNode j = doGet(path);
            return MAPPER.treeToValue(j, Types.IdentityRecord.class);
        } catch (ValidationException e) {
            if ("NOT_FOUND".equals(e.details().get("code"))) return null;
            throw e;
        } catch (Exception e) {
            throw new NodeException("decode identity: " + e.getMessage(), e);
        }
    }

    // =====================================================================
    // Trust
    // =====================================================================

    public JsonNode grantTrust(Quid signer, TrustParams p) {
        requireSigner(signer);
        if (p.trustee == null || p.trustee.isEmpty())
            throw new ValidationException("trustee is required");
        if (p.level < 0.0 || p.level > 1.0)
            throw new ValidationException("level must be in [0, 1]");
        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("type", "TRUST");
        tx.put("timestamp", Instant.now().getEpochSecond());
        tx.put("trustDomain", p.domain);
        tx.put("signerQuid", signer.id());
        tx.put("truster",    signer.id());
        tx.put("trustee",    p.trustee);
        tx.put("trustLevel", p.level);
        tx.put("nonce",      p.nonce);
        if (p.validUntil > 0)       tx.put("validUntil", p.validUntil);
        if (p.description != null)  tx.put("description", p.description);
        signIntoTx(signer, tx);
        return doPost("transactions/trust", tx);
    }

    public Types.TrustResult getTrust(String observer, String target, String domain, int maxDepth) {
        String path = "trust/" + urlencode(observer) + "/" + urlencode(target)
                    + "?domain=" + urlencode(domain) + "&maxDepth=" + maxDepth;
        try {
            return MAPPER.treeToValue(doGet(path), Types.TrustResult.class);
        } catch (Exception e) {
            throw new NodeException("decode trust result: " + e.getMessage(), e);
        }
    }

    @SuppressWarnings("unchecked")
    public List<Types.TrustEdge> getTrustEdges(String quidId) {
        try {
            JsonNode j = doGet("trust/edges/" + urlencode(quidId));
            JsonNode arr = j.has("edges") ? j.get("edges")
                           : j.has("data") ? j.get("data") : null;
            if (arr == null || !arr.isArray()) return Collections.emptyList();
            List<Types.TrustEdge> out = new ArrayList<>(arr.size());
            for (JsonNode n : arr) out.add(MAPPER.treeToValue(n, Types.TrustEdge.class));
            return out;
        } catch (Exception e) {
            throw new NodeException("decode trust edges: " + e.getMessage(), e);
        }
    }

    // =====================================================================
    // Title
    // =====================================================================

    public JsonNode registerTitle(Quid signer, TitleParams p) {
        requireSigner(signer);
        if (p.assetId == null || p.assetId.isEmpty())
            throw new ValidationException("assetId is required");
        if (p.owners == null || p.owners.isEmpty())
            throw new ValidationException("owners is required");
        double total = 0.0;
        for (Types.OwnershipStake s : p.owners) total += s.percentage;
        if (Math.abs(total - 100.0) > 0.001)
            throw new ValidationException("owner percentages must sum to 100 (got " + total + ")");

        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("type", "TITLE");
        tx.put("timestamp", Instant.now().getEpochSecond());
        tx.put("trustDomain", p.domain);
        tx.put("signerQuid", signer.id());
        tx.put("issuerQuid", signer.id());
        tx.put("assetQuid",  p.assetId);
        tx.put("ownershipMap", p.owners);
        tx.put("transferSigs", Collections.emptyMap());
        if (p.titleType != null)      tx.put("titleType", p.titleType);
        if (p.prevTitleTxId != null)  tx.put("prevTitleTxID", p.prevTitleTxId);
        signIntoTx(signer, tx);
        return doPost("transactions/title", tx);
    }

    public Types.Title getTitle(String assetId, String domain) {
        try {
            String path = "title/" + urlencode(assetId);
            if (domain != null) path += "?domain=" + urlencode(domain);
            return MAPPER.treeToValue(doGet(path), Types.Title.class);
        } catch (ValidationException e) {
            if ("NOT_FOUND".equals(e.details().get("code"))) return null;
            throw e;
        } catch (Exception e) {
            throw new NodeException("decode title: " + e.getMessage(), e);
        }
    }

    // =====================================================================
    // Events + streams
    // =====================================================================

    public JsonNode emitEvent(Quid signer, EventParams p) {
        requireSigner(signer);
        if (!"QUID".equals(p.subjectType) && !"TITLE".equals(p.subjectType))
            throw new ValidationException("subjectType must be 'QUID' or 'TITLE'");
        if (p.eventType == null || p.eventType.isEmpty())
            throw new ValidationException("eventType is required");
        if ((p.payload == null) == (p.payloadCid == null))
            throw new ValidationException("exactly one of payload or payloadCid is required");

        long seq = p.sequence;
        if (seq == 0) {
            try {
                JsonNode stream = doGet("streams/" + urlencode(p.subjectId)
                        + (p.domain != null ? "?domain=" + urlencode(p.domain) : ""));
                seq = stream.has("latestSequence") ? stream.get("latestSequence").asLong() + 1 : 1;
            } catch (QuidnugException ignore) {
                seq = 1;
            }
        }

        Map<String, Object> tx = new LinkedHashMap<>();
        tx.put("type", "EVENT");
        tx.put("timestamp", Instant.now().getEpochSecond());
        tx.put("trustDomain", p.domain);
        tx.put("subjectId",   p.subjectId);
        tx.put("subjectType", p.subjectType);
        tx.put("eventType",   p.eventType);
        tx.put("sequence",    seq);
        if (p.payload != null)    tx.put("payload", p.payload);
        if (p.payloadCid != null) tx.put("payloadCid", p.payloadCid);

        byte[] signable = CanonicalBytes.of(tx, "signature", "txId", "publicKey");
        try {
            tx.put("signature", signer.sign(signable));
        } catch (GeneralSecurityException e) {
            throw new CryptoException("sign: " + e.getMessage());
        }
        tx.put("publicKey", signer.publicKeyHex());
        return doPost("events", tx);
    }

    public JsonNode getEventStream(String subjectId, String domain) {
        try {
            String path = "streams/" + urlencode(subjectId);
            if (domain != null) path += "?domain=" + urlencode(domain);
            return doGet(path);
        } catch (ValidationException e) {
            if ("NOT_FOUND".equals(e.details().get("code"))) return null;
            throw e;
        }
    }

    public List<Types.Event> getStreamEvents(String subjectId, String domain, int limit, int offset) {
        StringBuilder path = new StringBuilder("streams/").append(urlencode(subjectId)).append("/events?");
        if (domain != null) path.append("domain=").append(urlencode(domain)).append("&");
        if (limit > 0) path.append("limit=").append(limit).append("&");
        if (offset > 0) path.append("offset=").append(offset);
        JsonNode j = doGet(path.toString());
        JsonNode arr = j.has("data") ? j.get("data") : j.get("events");
        if (arr == null || !arr.isArray()) return Collections.emptyList();
        List<Types.Event> out = new ArrayList<>(arr.size());
        try {
            for (JsonNode n : arr) out.add(MAPPER.treeToValue(n, Types.Event.class));
        } catch (Exception e) {
            throw new NodeException("decode events: " + e.getMessage(), e);
        }
        return out;
    }

    // =====================================================================
    // Guardians (QDP-0002)
    // =====================================================================

    public JsonNode submitGuardianSetUpdate(Map<String, Object> update) {
        return doPost("guardian/set-update", update);
    }

    public JsonNode submitRecoveryInit(Map<String, Object> init) {
        return doPost("guardian/recovery/init", init);
    }

    public JsonNode submitRecoveryVeto(Map<String, Object> veto) {
        return doPost("guardian/recovery/veto", veto);
    }

    public JsonNode submitRecoveryCommit(Map<String, Object> commit) {
        return doPost("guardian/recovery/commit", commit);
    }

    public JsonNode submitGuardianResignation(Map<String, Object> resig) {
        return doPost("guardian/resign", resig);
    }

    public Types.GuardianSet getGuardianSet(String quidId) {
        try {
            return MAPPER.treeToValue(
                    doGet("guardian/set/" + urlencode(quidId)),
                    Types.GuardianSet.class);
        } catch (ValidationException e) {
            if ("NOT_FOUND".equals(e.details().get("code"))) return null;
            throw e;
        } catch (Exception e) {
            throw new NodeException("decode guardian set: " + e.getMessage(), e);
        }
    }

    public JsonNode getPendingRecovery(String quidId) {
        try {
            return doGet("guardian/pending-recovery/" + urlencode(quidId));
        } catch (ValidationException e) {
            if ("NOT_FOUND".equals(e.details().get("code"))) return null;
            throw e;
        }
    }

    // =====================================================================
    // Gossip + bootstrap + fork-block
    // =====================================================================

    public JsonNode submitDomainFingerprint(Map<String, Object> fp) {
        return doPost("domain-fingerprints", fp);
    }

    public Types.DomainFingerprint getLatestDomainFingerprint(String domain) {
        try {
            return MAPPER.treeToValue(
                    doGet("domain-fingerprints/" + urlencode(domain) + "/latest"),
                    Types.DomainFingerprint.class);
        } catch (ValidationException e) {
            if ("NOT_FOUND".equals(e.details().get("code"))) return null;
            throw e;
        } catch (Exception e) {
            throw new NodeException("decode fingerprint: " + e.getMessage(), e);
        }
    }

    public JsonNode submitAnchorGossip(Map<String, Object> msg) {
        return doPost("anchor-gossip", msg);
    }

    public JsonNode pushAnchor(Map<String, Object> msg) {
        return doPost("gossip/push-anchor", msg);
    }

    public JsonNode pushFingerprint(Map<String, Object> fp) {
        return doPost("gossip/push-fingerprint", fp);
    }

    public JsonNode submitNonceSnapshot(Map<String, Object> snapshot) {
        return doPost("nonce-snapshots", snapshot);
    }

    public JsonNode getLatestNonceSnapshot(String domain) {
        try {
            return doGet("nonce-snapshots/" + urlencode(domain) + "/latest");
        } catch (ValidationException e) {
            if ("NOT_FOUND".equals(e.details().get("code"))) return null;
            throw e;
        }
    }

    public JsonNode bootstrapStatus() { return doGet("bootstrap/status"); }

    public JsonNode submitForkBlock(Map<String, Object> fb) {
        return doPost("fork-block", fb);
    }

    public JsonNode forkBlockStatus() { return doGet("fork-block/status"); }

    // =====================================================================
    // Param objects (fluent builders)
    // =====================================================================

    /** Identity registration params. */
    public static final class IdentityParams {
        public String subjectQuid;
        public String domain = "default";
        public String name;
        public String description;
        public String homeDomain;
        public Map<String, Object> attributes;
        public long updateNonce = 1;

        public static IdentityParams name(String n) { IdentityParams p = new IdentityParams(); p.name = n; return p; }
        public IdentityParams domain(String d) { this.domain = d; return this; }
        public IdentityParams homeDomain(String d) { this.homeDomain = d; return this; }
        public IdentityParams description(String d) { this.description = d; return this; }
        public IdentityParams attributes(Map<String, Object> a) { this.attributes = a; return this; }
        public IdentityParams subjectQuid(String q) { this.subjectQuid = q; return this; }
        public IdentityParams updateNonce(long n) { this.updateNonce = n; return this; }
    }

    /** Trust-grant params. */
    public static final class TrustParams {
        public String trustee;
        public double level;
        public String domain = "default";
        public long nonce = 1;
        public long validUntil;
        public String description;

        public static TrustParams of(String trustee, double level, String domain) {
            TrustParams p = new TrustParams();
            p.trustee = trustee; p.level = level; p.domain = domain;
            return p;
        }
        public TrustParams nonce(long n) { this.nonce = n; return this; }
        public TrustParams validUntil(long v) { this.validUntil = v; return this; }
        public TrustParams description(String d) { this.description = d; return this; }
    }

    /** Title-register params. */
    public static final class TitleParams {
        public String assetId;
        public String domain = "default";
        public List<Types.OwnershipStake> owners;
        public String titleType;
        public String prevTitleTxId;

        public static TitleParams of(String assetId, List<Types.OwnershipStake> owners) {
            TitleParams p = new TitleParams();
            p.assetId = assetId;
            p.owners = owners;
            return p;
        }
        public TitleParams domain(String d) { this.domain = d; return this; }
        public TitleParams titleType(String t) { this.titleType = t; return this; }
    }

    /** Event-emit params. */
    public static final class EventParams {
        public String subjectId;
        public String subjectType;
        public String eventType;
        public String domain = "default";
        public Map<String, Object> payload;
        public String payloadCid;
        public long sequence; // 0 = auto

        public static EventParams of(String subjectId, String subjectType, String eventType) {
            EventParams p = new EventParams();
            p.subjectId = subjectId;
            p.subjectType = subjectType;
            p.eventType = eventType;
            return p;
        }
        public EventParams payload(Map<String, Object> pl) { this.payload = pl; return this; }
        public EventParams payloadCid(String cid) { this.payloadCid = cid; return this; }
        public EventParams domain(String d) { this.domain = d; return this; }
        public EventParams sequence(long s) { this.sequence = s; return this; }
    }

    // =====================================================================
    // Builder
    // =====================================================================

    public static final class Builder {
        private String baseUrl;
        private Duration timeout = Duration.ofSeconds(30);
        private int maxRetries = 3;
        private Duration retryBaseDelay = Duration.ofSeconds(1);
        private String authToken;
        private String userAgent = "quidnug-java-sdk/2.0.0";
        private HttpClient http;

        public Builder baseUrl(String url) { this.baseUrl = url; return this; }
        public Builder timeout(Duration d) { this.timeout = d; return this; }
        public Builder maxRetries(int n) { this.maxRetries = n; return this; }
        public Builder retryBaseDelay(Duration d) { this.retryBaseDelay = d; return this; }
        public Builder authToken(String t) { this.authToken = t; return this; }
        public Builder userAgent(String ua) { this.userAgent = ua; return this; }
        public Builder httpClient(HttpClient c) { this.http = c; return this; }

        public QuidnugClient build() {
            if (baseUrl == null || baseUrl.isEmpty())
                throw new ValidationException("baseUrl is required");
            return new QuidnugClient(this);
        }
    }

    // =====================================================================
    // HTTP plumbing
    // =====================================================================

    private JsonNode doGet(String path) {
        return request("GET", path, null);
    }

    private JsonNode doPost(String path, Object body) {
        return request("POST", path, body);
    }

    private JsonNode request(String method, String path, Object body) {
        int attempts = method.equals("GET") ? (maxRetries + 1) : 1;
        QuidnugException last = null;
        for (int attempt = 0; attempt < attempts; attempt++) {
            HttpRequest.Builder rb = HttpRequest.newBuilder()
                    .uri(URI.create(apiBase + "/" + path.replaceFirst("^/+", "")))
                    .timeout(timeout)
                    .header("Accept", "application/json")
                    .header("User-Agent", userAgent);
            if (authToken != null) rb.header("Authorization", "Bearer " + authToken);
            if (body != null) {
                rb.header("Content-Type", "application/json");
                try {
                    rb.method(method, HttpRequest.BodyPublishers.ofString(
                            MAPPER.writeValueAsString(body), StandardCharsets.UTF_8));
                } catch (Exception e) {
                    throw new ValidationException("marshal body: " + e.getMessage());
                }
            } else {
                rb.method(method, HttpRequest.BodyPublishers.noBody());
            }

            HttpResponse<String> resp;
            try {
                resp = http.send(rb.build(), HttpResponse.BodyHandlers.ofString());
            } catch (Exception e) {
                last = new NodeException(method + " " + path + ": " + e.getMessage(), e);
                if (attempt < attempts - 1) { sleepBackoff(attempt, null); continue; }
                throw last;
            }

            int sc = resp.statusCode();
            if ((sc >= 500 || sc == 429) && attempt < attempts - 1) {
                sleepBackoff(attempt, resp.headers().firstValue("Retry-After").orElse(null));
                continue;
            }
            return parseEnvelope(resp);
        }
        throw last != null ? last : new NodeException(method + " " + path + ": retries exhausted", 0, "");
    }

    private JsonNode parseEnvelope(HttpResponse<String> resp) {
        int sc = resp.statusCode();
        JsonNode env;
        try {
            env = MAPPER.readTree(resp.body());
        } catch (Exception e) {
            throw new NodeException("non-JSON response (HTTP " + sc + ")", sc, resp.body());
        }
        if (env.path("success").asBoolean(false)) {
            JsonNode data = env.get("data");
            return data != null ? data : MAPPER.nullNode();
        }
        JsonNode err = env.path("error");
        String code = err.path("code").asText("UNKNOWN_ERROR");
        String message = err.path("message").asText("HTTP " + sc);
        Map<String, Object> details = new HashMap<>();
        details.put("code", code);

        if (sc == 503 || UNAVAILABLE_CODES.contains(code))
            throw new UnavailableException(message, details);
        if (sc == 409 || CONFLICT_CODES.contains(code))
            throw new ConflictException(message, details);
        if (sc >= 400 && sc < 500)
            throw new ValidationException(message, details);
        throw new NodeException(message, sc, resp.body());
    }

    private void sleepBackoff(int attempt, String retryAfter) {
        long delay = -1;
        if (retryAfter != null) {
            try { delay = Long.parseLong(retryAfter.trim()) * 1000L; } catch (NumberFormatException ignore) {}
        }
        if (delay < 0) {
            delay = (long) (retryBaseDelay.toMillis() * Math.pow(2, attempt) + Math.random() * 100);
        }
        delay = Math.min(delay, 60_000);
        try { Thread.sleep(delay); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
    }

    private static void signIntoTx(Quid signer, Map<String, Object> tx) {
        byte[] signable = CanonicalBytes.of(tx, "signature", "txId");
        try {
            tx.put("signature", signer.sign(signable));
        } catch (GeneralSecurityException e) {
            throw new CryptoException("sign: " + e.getMessage());
        }
    }

    private static void requireSigner(Quid signer) {
        if (signer == null || !signer.hasPrivateKey())
            throw new ValidationException("signer must have a private key");
    }

    private static String urlencode(String s) {
        return URLEncoder.encode(s, StandardCharsets.UTF_8).replace("+", "%20");
    }
}
