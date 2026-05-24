package com.quidnug.android

import com.fasterxml.jackson.databind.JsonNode
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * Kotlin-first suspending wrappers for the rest of the
 * [com.quidnug.client.QuidnugClient] surface.
 *
 * The five hand-written wrappers in [QuidnugAndroidClient] (identity,
 * trust, events, stream-events) cover the most-common flows;
 * everything else is reached via `client.client.*` on the JVM type.
 * These extension functions provide ergonomic Kotlin signatures + the
 * `Dispatchers.IO` switch so callers don't have to remember to bridge
 * blocking JVM calls themselves.
 *
 * Each function delegates 1:1 to the wrapped JVM method — see the
 * Java SDK Javadoc for parameter semantics.
 */

// ----- Health / info / peers --------------------------------------------

suspend fun QuidnugAndroidClient.health(): JsonNode =
    withContext(Dispatchers.IO) { client.health() }

suspend fun QuidnugAndroidClient.info(): JsonNode =
    withContext(Dispatchers.IO) { client.info() }

suspend fun QuidnugAndroidClient.nodes(): JsonNode =
    withContext(Dispatchers.IO) { client.nodes() }

suspend fun QuidnugAndroidClient.peers(): JsonNode =
    withContext(Dispatchers.IO) { client.peers() }

suspend fun QuidnugAndroidClient.getPeer(nodeQuid: String): JsonNode? =
    withContext(Dispatchers.IO) { client.getPeer(nodeQuid) }

// ----- Blocks / domains -------------------------------------------------

suspend fun QuidnugAndroidClient.blocks(): JsonNode =
    withContext(Dispatchers.IO) { client.blocks() }

suspend fun QuidnugAndroidClient.getTentativeBlocks(domain: String): JsonNode =
    withContext(Dispatchers.IO) { client.getTentativeBlocks(domain) }

suspend fun QuidnugAndroidClient.listDomains(): JsonNode =
    withContext(Dispatchers.IO) { client.listDomains() }

suspend fun QuidnugAndroidClient.registerDomain(
    domain: String, attrs: Map<String, Any?>? = null
): JsonNode = withContext(Dispatchers.IO) { client.registerDomain(domain, attrs) }

suspend fun QuidnugAndroidClient.topDomains(): JsonNode =
    withContext(Dispatchers.IO) { client.topDomains() }

suspend fun QuidnugAndroidClient.queryDomain(
    domain: String, queryType: String, param: String
): JsonNode = withContext(Dispatchers.IO) { client.queryDomain(domain, queryType, param) }

suspend fun QuidnugAndroidClient.getNodeDomains(): JsonNode =
    withContext(Dispatchers.IO) { client.getNodeDomains() }

suspend fun QuidnugAndroidClient.updateNodeDomains(domains: List<String>): JsonNode =
    withContext(Dispatchers.IO) { client.updateNodeDomains(domains) }

suspend fun QuidnugAndroidClient.sendDomainGossip(gossip: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.sendDomainGossip(gossip) }

suspend fun QuidnugAndroidClient.createNodeAdvertisement(advertisement: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.createNodeAdvertisement(advertisement) }

// ----- Quids ------------------------------------------------------------

suspend fun QuidnugAndroidClient.generateQuidOnNode(metadata: Map<String, Any?>? = null): JsonNode =
    withContext(Dispatchers.IO) { client.generateQuid(metadata) }

// ----- IPFS -------------------------------------------------------------

suspend fun QuidnugAndroidClient.ipfsPin(content: ByteArray): String =
    withContext(Dispatchers.IO) { client.ipfsPin(content) }

suspend fun QuidnugAndroidClient.ipfsGet(cid: String): ByteArray =
    withContext(Dispatchers.IO) { client.ipfsGet(cid) }

// ----- Registry queries -------------------------------------------------

suspend fun QuidnugAndroidClient.queryTrustRegistry(): JsonNode =
    withContext(Dispatchers.IO) { client.queryTrustRegistry() }

suspend fun QuidnugAndroidClient.queryIdentityRegistry(): JsonNode =
    withContext(Dispatchers.IO) { client.queryIdentityRegistry() }

suspend fun QuidnugAndroidClient.queryTitleRegistry(): JsonNode =
    withContext(Dispatchers.IO) { client.queryTitleRegistry() }

suspend fun QuidnugAndroidClient.queryRelationalTrust(query: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.queryRelationalTrust(query) }

// ----- Guardians (QDP-0002 / 0006) --------------------------------------

suspend fun QuidnugAndroidClient.submitGuardianSetUpdate(update: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitGuardianSetUpdate(update) }

suspend fun QuidnugAndroidClient.submitRecoveryInit(init: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitRecoveryInit(init) }

suspend fun QuidnugAndroidClient.submitRecoveryVeto(veto: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitRecoveryVeto(veto) }

suspend fun QuidnugAndroidClient.submitRecoveryCommit(commit: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitRecoveryCommit(commit) }

suspend fun QuidnugAndroidClient.submitGuardianResignation(r: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitGuardianResignation(r) }

suspend fun QuidnugAndroidClient.getGuardianSet(quidId: String) =
    withContext(Dispatchers.IO) { client.getGuardianSet(quidId) }

suspend fun QuidnugAndroidClient.getPendingRecovery(quidId: String): JsonNode? =
    withContext(Dispatchers.IO) { client.getPendingRecovery(quidId) }

suspend fun QuidnugAndroidClient.getGuardianResignations(quidId: String): JsonNode =
    withContext(Dispatchers.IO) { client.getGuardianResignations(quidId) }

// ----- Gossip / bootstrap / fork-block ----------------------------------

suspend fun QuidnugAndroidClient.submitDomainFingerprint(fp: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitDomainFingerprint(fp) }

suspend fun QuidnugAndroidClient.getLatestDomainFingerprint(domain: String) =
    withContext(Dispatchers.IO) { client.getLatestDomainFingerprint(domain) }

suspend fun QuidnugAndroidClient.submitAnchorGossip(msg: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitAnchorGossip(msg) }

suspend fun QuidnugAndroidClient.pushAnchor(msg: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.pushAnchor(msg) }

suspend fun QuidnugAndroidClient.pushFingerprint(fp: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.pushFingerprint(fp) }

suspend fun QuidnugAndroidClient.submitNonceSnapshot(snap: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitNonceSnapshot(snap) }

suspend fun QuidnugAndroidClient.getLatestNonceSnapshot(domain: String): JsonNode? =
    withContext(Dispatchers.IO) { client.getLatestNonceSnapshot(domain) }

suspend fun QuidnugAndroidClient.bootstrapStatus(): JsonNode =
    withContext(Dispatchers.IO) { client.bootstrapStatus() }

suspend fun QuidnugAndroidClient.submitForkBlock(fb: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitForkBlock(fb) }

suspend fun QuidnugAndroidClient.forkBlockStatus(): JsonNode =
    withContext(Dispatchers.IO) { client.forkBlockStatus() }

// ----- Moderation (QDP-0015) --------------------------------------------

suspend fun QuidnugAndroidClient.createModerationAction(action: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.createModerationAction(action) }

suspend fun QuidnugAndroidClient.getModerationActions(
    targetType: String, targetId: String
): JsonNode = withContext(Dispatchers.IO) { client.getModerationActions(targetType, targetId) }

// ----- Privacy (QDP-0017) -----------------------------------------------

suspend fun QuidnugAndroidClient.createDSR(request: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.createDSR(request) }

suspend fun QuidnugAndroidClient.getDSRStatus(requestTxId: String): JsonNode? =
    withContext(Dispatchers.IO) { client.getDSRStatus(requestTxId) }

suspend fun QuidnugAndroidClient.createConsentGrant(grant: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.createConsentGrant(grant) }

suspend fun QuidnugAndroidClient.createConsentWithdraw(withdraw: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.createConsentWithdraw(withdraw) }

suspend fun QuidnugAndroidClient.getConsentHistory(subjectQuid: String): JsonNode =
    withContext(Dispatchers.IO) { client.getConsentHistory(subjectQuid) }

suspend fun QuidnugAndroidClient.createProcessingRestriction(restriction: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.createProcessingRestriction(restriction) }

suspend fun QuidnugAndroidClient.getRestrictionsForSubject(subjectQuid: String): JsonNode =
    withContext(Dispatchers.IO) { client.getRestrictionsForSubject(subjectQuid) }

suspend fun QuidnugAndroidClient.createDSRCompliance(compliance: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.createDSRCompliance(compliance) }

// ----- Audit (QDP-0018) -------------------------------------------------

suspend fun QuidnugAndroidClient.auditHead(): JsonNode =
    withContext(Dispatchers.IO) { client.auditHead() }

suspend fun QuidnugAndroidClient.auditEntries(
    since: Long? = null, limit: Int? = null
): JsonNode = withContext(Dispatchers.IO) { client.auditEntries(since, limit) }

suspend fun QuidnugAndroidClient.auditEntry(sequence: Long): JsonNode? =
    withContext(Dispatchers.IO) { client.auditEntry(sequence) }

// ----- Discovery (QDP-0014) ---------------------------------------------

suspend fun QuidnugAndroidClient.discoverDomain(name: String): JsonNode =
    withContext(Dispatchers.IO) { client.discoverDomain(name) }

suspend fun QuidnugAndroidClient.discoverNode(quid: String): JsonNode =
    withContext(Dispatchers.IO) { client.discoverNode(quid) }

suspend fun QuidnugAndroidClient.discoverOperator(quid: String): JsonNode =
    withContext(Dispatchers.IO) { client.discoverOperator(quid) }

suspend fun QuidnugAndroidClient.discoverQuids(): JsonNode =
    withContext(Dispatchers.IO) { client.discoverQuids() }

suspend fun QuidnugAndroidClient.discoverTrustedQuids(): JsonNode =
    withContext(Dispatchers.IO) { client.discoverTrustedQuids() }

// ----- DNS attestation (QDP-0023) ---------------------------------------

suspend fun QuidnugAndroidClient.submitDNSClaim(claim: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitDNSClaim(claim) }

suspend fun QuidnugAndroidClient.submitDNSChallenge(challenge: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitDNSChallenge(challenge) }

suspend fun QuidnugAndroidClient.submitDNSAttestation(attestation: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitDNSAttestation(attestation) }

suspend fun QuidnugAndroidClient.submitDNSRenewal(renewal: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitDNSRenewal(renewal) }

suspend fun QuidnugAndroidClient.submitDNSRevocation(revocation: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitDNSRevocation(revocation) }

suspend fun QuidnugAndroidClient.submitAuthorityDelegate(delegate: Map<String, Any?>): JsonNode =
    withContext(Dispatchers.IO) { client.submitAuthorityDelegate(delegate) }

suspend fun QuidnugAndroidClient.submitAuthorityDelegateRevocation(
    revocation: Map<String, Any?>
): JsonNode = withContext(Dispatchers.IO) { client.submitAuthorityDelegateRevocation(revocation) }

suspend fun QuidnugAndroidClient.getDNSAttestations(domain: String): JsonNode =
    withContext(Dispatchers.IO) { client.getDNSAttestations(domain) }

suspend fun QuidnugAndroidClient.getDNSAttestationsWeighted(domain: String): JsonNode =
    withContext(Dispatchers.IO) { client.getDNSAttestationsWeighted(domain) }

suspend fun QuidnugAndroidClient.resolveDNSRecord(domain: String, recordType: String): JsonNode =
    withContext(Dispatchers.IO) { client.resolveDNSRecord(domain, recordType) }
