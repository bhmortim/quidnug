package com.quidnug.client;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.Collections;
import java.util.List;
import java.util.Map;

/** Wire types. Fields are nullable where the server may omit them. */
public final class Types {
    private Types() {}

    /** Relational trust query result. */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class TrustResult {
        @JsonProperty("observer")   public final String observer;
        @JsonProperty("target")     public final String target;
        @JsonProperty("trustLevel") public final double trustLevel;
        @JsonProperty("trustPath")  public final List<String> path;
        @JsonProperty("pathDepth")  public final int pathDepth;
        @JsonProperty("domain")     public final String domain;

        @JsonCreator
        public TrustResult(
                @JsonProperty("observer") String observer,
                @JsonProperty("target") String target,
                @JsonProperty("trustLevel") double trustLevel,
                @JsonProperty("trustPath") List<String> path,
                @JsonProperty("pathDepth") int pathDepth,
                @JsonProperty("domain") String domain) {
            this.observer   = observer;
            this.target     = target;
            this.trustLevel = trustLevel;
            this.path       = path == null ? Collections.emptyList() : path;
            this.pathDepth  = pathDepth;
            this.domain     = domain;
        }
    }

    /** Direct outbound trust edge. */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class TrustEdge {
        public final String truster;
        public final String trustee;
        public final double trustLevel;
        public final String domain;
        public final long nonce;

        @JsonCreator
        public TrustEdge(
                @JsonProperty("truster") String truster,
                @JsonProperty("trustee") String trustee,
                @JsonProperty("trustLevel") double trustLevel,
                @JsonProperty("domain") String domain,
                @JsonProperty("nonce") long nonce) {
            this.truster    = truster;
            this.trustee    = trustee;
            this.trustLevel = trustLevel;
            this.domain     = domain;
            this.nonce      = nonce;
        }
    }

    /** Identity record snapshot. */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class IdentityRecord {
        public final String quidId;
        public final String name;
        public final String homeDomain;
        public final String publicKey;
        public final long updateNonce;
        public final Map<String, Object> attributes;

        @JsonCreator
        public IdentityRecord(
                @JsonProperty("quidId") String quidId,
                @JsonProperty("name") String name,
                @JsonProperty("homeDomain") String homeDomain,
                @JsonProperty("publicKey") String publicKey,
                @JsonProperty("updateNonce") long updateNonce,
                @JsonProperty("attributes") Map<String, Object> attributes) {
            this.quidId      = quidId;
            this.name        = name;
            this.homeDomain  = homeDomain;
            this.publicKey   = publicKey;
            this.updateNonce = updateNonce;
            this.attributes  = attributes == null ? Collections.emptyMap() : attributes;
        }
    }

    /** Ownership stake. */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class OwnershipStake {
        public final String ownerId;
        public final double percentage;
        public final String stakeType;

        public OwnershipStake(String ownerId, double percentage) {
            this(ownerId, percentage, null);
        }

        @JsonCreator
        public OwnershipStake(
                @JsonProperty("ownerId") String ownerId,
                @JsonProperty("percentage") double percentage,
                @JsonProperty("stakeType") String stakeType) {
            this.ownerId    = ownerId;
            this.percentage = percentage;
            this.stakeType  = stakeType;
        }
    }

    /** Title record. */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class Title {
        public final String assetId;
        public final String domain;
        public final String titleType;
        public final List<OwnershipStake> owners;

        @JsonCreator
        public Title(
                @JsonProperty("assetId") String assetId,
                @JsonProperty("domain") String domain,
                @JsonProperty("titleType") String titleType,
                @JsonProperty("ownershipMap") List<OwnershipStake> owners) {
            this.assetId   = assetId;
            this.domain    = domain;
            this.titleType = titleType;
            this.owners    = owners == null ? Collections.emptyList() : owners;
        }
    }

    /** One event in a subject's stream. */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class Event {
        public final String subjectId;
        public final String subjectType;
        public final String eventType;
        public final Map<String, Object> payload;
        public final String payloadCid;
        public final long timestamp;
        public final long sequence;

        @JsonCreator
        public Event(
                @JsonProperty("subjectId") String subjectId,
                @JsonProperty("subjectType") String subjectType,
                @JsonProperty("eventType") String eventType,
                @JsonProperty("payload") Map<String, Object> payload,
                @JsonProperty("payloadCid") String payloadCid,
                @JsonProperty("timestamp") long timestamp,
                @JsonProperty("sequence") long sequence) {
            this.subjectId   = subjectId;
            this.subjectType = subjectType;
            this.eventType   = eventType;
            this.payload     = payload == null ? Collections.emptyMap() : payload;
            this.payloadCid  = payloadCid;
            this.timestamp   = timestamp;
            this.sequence    = sequence;
        }
    }

    /** Guardian set (QDP-0002). */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class GuardianSet {
        public final String subjectQuid;
        public final List<GuardianRef> guardians;
        public final int threshold;
        public final long recoveryDelaySeconds;

        @JsonCreator
        public GuardianSet(
                @JsonProperty("subjectQuid") String subjectQuid,
                @JsonProperty("guardians") List<GuardianRef> guardians,
                @JsonProperty("threshold") int threshold,
                @JsonProperty("recoveryDelaySeconds") long recoveryDelaySeconds) {
            this.subjectQuid          = subjectQuid;
            this.guardians            = guardians == null ? Collections.emptyList() : guardians;
            this.threshold            = threshold;
            this.recoveryDelaySeconds = recoveryDelaySeconds;
        }
    }

    /** One guardian entry. */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class GuardianRef {
        public final String quid;
        public final int weight;
        public final int epoch;

        @JsonCreator
        public GuardianRef(
                @JsonProperty("quid") String quid,
                @JsonProperty("weight") int weight,
                @JsonProperty("epoch") int epoch) {
            this.quid   = quid;
            this.weight = weight <= 0 ? 1 : weight;
            this.epoch  = epoch;
        }
    }

    /** Domain fingerprint (QDP-0003). */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static final class DomainFingerprint {
        public final String domain;
        public final long blockHeight;
        public final String blockHash;
        public final String producerQuid;
        public final long timestamp;

        @JsonCreator
        public DomainFingerprint(
                @JsonProperty("domain") String domain,
                @JsonProperty("blockHeight") long blockHeight,
                @JsonProperty("blockHash") String blockHash,
                @JsonProperty("producerQuid") String producerQuid,
                @JsonProperty("timestamp") long timestamp) {
            this.domain       = domain;
            this.blockHeight  = blockHeight;
            this.blockHash    = blockHash;
            this.producerQuid = producerQuid;
            this.timestamp    = timestamp;
        }
    }
}
