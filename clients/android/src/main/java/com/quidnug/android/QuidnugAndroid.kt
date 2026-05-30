package com.quidnug.android

import android.content.Context
import android.content.SharedPreferences
import com.quidnug.client.Quid
import com.quidnug.client.QuidnugClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * High-level Android convenience wrapper over [QuidnugClient].
 *
 * Adds:
 *  - [QuidVault]: wraps SharedPreferences for dev persistence and
 *    [AndroidKeystoreSigner] for production Secure-Enclave-backed keys.
 *  - [QuidnugAndroidClient.suspending*] bridge methods that run the
 *    blocking Java client on Dispatchers.IO so Kotlin coroutines-first
 *    callers don't block the main thread.
 *
 * Keeping the JVM [QuidnugClient] reachable as the underlying engine
 * means this module stays thin and every JVM tutorial / example works
 * unchanged.
 */
class QuidnugAndroidClient(
    val client: QuidnugClient
) {

    /** Suspending wrapper around `registerIdentity`. */
    suspend fun registerIdentity(
        signer: Quid,
        name: String? = null,
        homeDomain: String? = null,
        domain: String = "default",
    ) = withContext(Dispatchers.IO) {
        val p = QuidnugClient.IdentityParams().apply {
            this.name = name
            this.homeDomain = homeDomain
            this.domain = domain
        }
        client.registerIdentity(signer, p)
    }

    /** Suspending wrapper around `grantTrust`. */
    suspend fun grantTrust(
        signer: Quid,
        trustee: String,
        level: Double,
        domain: String = "default",
        nonce: Long = 1,
    ) = withContext(Dispatchers.IO) {
        val p = QuidnugClient.TrustParams.of(trustee, level, domain).apply {
            this.nonce = nonce
        }
        client.grantTrust(signer, p)
    }

    /** Suspending wrapper around `getTrust`. */
    suspend fun getTrust(
        observer: String, target: String, domain: String, maxDepth: Int = 5
    ) = withContext(Dispatchers.IO) {
        client.getTrust(observer, target, domain, maxDepth)
    }

    /** Suspending wrapper around `emitEvent`. */
    suspend fun emitEvent(
        signer: Quid,
        subjectId: String,
        subjectType: String,
        eventType: String,
        domain: String = "default",
        payload: Map<String, Any?>? = null,
        payloadCid: String? = null,
    ) = withContext(Dispatchers.IO) {
        val p = QuidnugClient.EventParams.of(subjectId, subjectType, eventType).apply {
            this.domain = domain
            this.payload = payload
            this.payloadCid = payloadCid
        }
        client.emitEvent(signer, p)
    }

    /** Stream events (suspending). */
    suspend fun streamEvents(
        subjectId: String, domain: String? = null, limit: Int = 50, offset: Int = 0
    ) = withContext(Dispatchers.IO) {
        client.getStreamEvents(subjectId, domain, limit, offset)
    }

    // =========================================================================
    // Suspending pass-throughs for the broader protocol surface.
    //
    // The wrapper exposes a Kotlin coroutine-friendly form of every
    // non-trivial method on the underlying [QuidnugClient]. Each
    // method is a thin `withContext(Dispatchers.IO) { client.<m>(...) }`
    // bridge so callers never block the main thread.
    //
    // The full Java surface remains available via the public `client`
    // field for any method not pre-wrapped here.
    // =========================================================================

    suspend fun health() = withContext(Dispatchers.IO) { client.health() }
    suspend fun info() = withContext(Dispatchers.IO) { client.info() }
    suspend fun nodes() = withContext(Dispatchers.IO) { client.nodes() }
    suspend fun blocks() = withContext(Dispatchers.IO) { client.blocks() }
    suspend fun pendingTransactions() = withContext(Dispatchers.IO) { client.pendingTransactions() }

    suspend fun getIdentity(quidId: String, domain: String? = null) =
        withContext(Dispatchers.IO) { client.getIdentity(quidId, domain) }
    suspend fun getTrustEdges(quidId: String) =
        withContext(Dispatchers.IO) { client.getTrustEdges(quidId) }
    suspend fun getTitle(assetId: String, domain: String? = null) =
        withContext(Dispatchers.IO) { client.getTitle(assetId, domain) }
    suspend fun getEventStream(subjectId: String, domain: String? = null) =
        withContext(Dispatchers.IO) { client.getEventStream(subjectId, domain) }

    suspend fun listDomains() = withContext(Dispatchers.IO) { client.listDomains() }
    suspend fun registerDomain(name: String) = withContext(Dispatchers.IO) { client.registerDomain(name) }
    suspend fun getTopDomains(limit: Int? = null) =
        withContext(Dispatchers.IO) { client.getTopDomains(limit) }
    suspend fun queryDomain(name: String) =
        withContext(Dispatchers.IO) { client.queryDomain(name) }
    suspend fun getNodeDomains() = withContext(Dispatchers.IO) { client.getNodeDomains() }
    suspend fun updateNodeDomains(domains: List<String>) =
        withContext(Dispatchers.IO) { client.updateNodeDomains(domains) }
    suspend fun createQuid() = withContext(Dispatchers.IO) { client.createQuid() }

    // Guardians (QDP-0002, 0006)
    suspend fun submitGuardianSetUpdate(update: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitGuardianSetUpdate(update) }
    suspend fun submitRecoveryInit(init: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitRecoveryInit(init) }
    suspend fun submitRecoveryVeto(veto: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitRecoveryVeto(veto) }
    suspend fun submitRecoveryCommit(commit: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitRecoveryCommit(commit) }
    suspend fun submitGuardianResignation(resignation: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitGuardianResignation(resignation) }
    suspend fun getGuardianSet(quidId: String) =
        withContext(Dispatchers.IO) { client.getGuardianSet(quidId) }
    suspend fun getPendingRecovery(quidId: String) =
        withContext(Dispatchers.IO) { client.getPendingRecovery(quidId) }
    suspend fun getGuardianResignations(quidId: String) =
        withContext(Dispatchers.IO) { client.getGuardianResignations(quidId) }

    // Gossip / bootstrap / fork-block
    suspend fun submitDomainFingerprint(fp: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDomainFingerprint(fp) }
    suspend fun getLatestDomainFingerprint(domain: String) =
        withContext(Dispatchers.IO) { client.getLatestDomainFingerprint(domain) }
    suspend fun submitAnchorGossip(msg: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitAnchorGossip(msg) }
    suspend fun pushAnchor(msg: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.pushAnchor(msg) }
    suspend fun pushFingerprint(fp: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.pushFingerprint(fp) }
    suspend fun submitNonceSnapshot(snapshot: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitNonceSnapshot(snapshot) }
    suspend fun getLatestNonceSnapshot(domain: String) =
        withContext(Dispatchers.IO) { client.getLatestNonceSnapshot(domain) }
    suspend fun bootstrapStatus() = withContext(Dispatchers.IO) { client.bootstrapStatus() }
    suspend fun submitForkBlock(fb: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitForkBlock(fb) }
    suspend fun forkBlockStatus() = withContext(Dispatchers.IO) { client.forkBlockStatus() }

    // Peers
    suspend fun getPeers(limit: Int? = null, offset: Int? = null) =
        withContext(Dispatchers.IO) { client.getPeers(limit, offset) }
    suspend fun getPeer(nodeQuid: String) =
        withContext(Dispatchers.IO) { client.getPeer(nodeQuid) }

    // Discovery (QDP-0014)
    suspend fun discoverDomain(name: String) =
        withContext(Dispatchers.IO) { client.discoverDomain(name) }
    suspend fun discoverNode(quid: String) =
        withContext(Dispatchers.IO) { client.discoverNode(quid) }
    suspend fun discoverOperator(quid: String) =
        withContext(Dispatchers.IO) { client.discoverOperator(quid) }
    suspend fun discoverQuids(
        domain: String? = null, sort: String? = null,
        limit: Int? = null, offset: Int? = null,
    ) = withContext(Dispatchers.IO) { client.discoverQuids(domain, sort, limit, offset) }
    suspend fun discoverTrustedQuids(
        domain: String? = null, limit: Int? = null, offset: Int? = null,
    ) = withContext(Dispatchers.IO) { client.discoverTrustedQuids(domain, limit, offset) }

    // Moderation (QDP-0015)
    suspend fun submitModerationAction(action: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitModerationAction(action) }
    suspend fun getModerationActions(
        targetType: String, targetId: String, limit: Int? = null, offset: Int? = null,
    ) = withContext(Dispatchers.IO) {
        client.getModerationActions(targetType, targetId, limit, offset)
    }

    // Audit (QDP-0018)
    suspend fun getAuditHead() = withContext(Dispatchers.IO) { client.getAuditHead() }
    suspend fun getAuditEntries(
        fromSequence: Long? = null, limit: Int? = null, offset: Int? = null,
    ) = withContext(Dispatchers.IO) { client.getAuditEntries(fromSequence, limit, offset) }
    suspend fun getAuditEntry(sequence: Long) =
        withContext(Dispatchers.IO) { client.getAuditEntry(sequence) }

    // Privacy / DSR (QDP-0017)
    suspend fun submitDSR(request: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDSR(request) }
    suspend fun getDSRStatus(requestTxId: String) =
        withContext(Dispatchers.IO) { client.getDSRStatus(requestTxId) }
    suspend fun submitConsentGrant(grant: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitConsentGrant(grant) }
    suspend fun submitConsentWithdraw(withdraw: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitConsentWithdraw(withdraw) }
    suspend fun getConsentHistory(
        subjectQuid: String? = null, processorQuid: String? = null,
        limit: Int? = null, offset: Int? = null,
    ) = withContext(Dispatchers.IO) {
        client.getConsentHistory(subjectQuid, processorQuid, limit, offset)
    }
    suspend fun submitProcessingRestriction(restriction: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitProcessingRestriction(restriction) }
    suspend fun getRestrictionsForSubject(subjectQuid: String) =
        withContext(Dispatchers.IO) { client.getRestrictionsForSubject(subjectQuid) }
    suspend fun submitDSRCompliance(compliance: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDSRCompliance(compliance) }

    // DNS attestation (QDP-0016)
    suspend fun submitDnsClaim(claim: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDnsClaim(claim) }
    suspend fun submitDnsChallenge(challenge: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDnsChallenge(challenge) }
    suspend fun submitDnsAttestation(attestation: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDnsAttestation(attestation) }
    suspend fun submitDnsRenewal(renewal: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDnsRenewal(renewal) }
    suspend fun submitDnsRevocation(revocation: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDnsRevocation(revocation) }
    suspend fun submitDnsDelegate(delegation: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDnsDelegate(delegation) }
    suspend fun submitDnsDelegateRevocation(revocation: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitDnsDelegateRevocation(revocation) }
    suspend fun getDnsAttestations(domain: String) =
        withContext(Dispatchers.IO) { client.getDnsAttestations(domain) }
    suspend fun getDnsWeightedAttestations(domain: String) =
        withContext(Dispatchers.IO) { client.getDnsWeightedAttestations(domain) }
    suspend fun resolveDns(domain: String, recordType: String) =
        withContext(Dispatchers.IO) { client.resolveDns(domain, recordType) }

    companion object {
        /** Convenience constructor using sane defaults. */
        fun create(nodeUrl: String, authToken: String? = null): QuidnugAndroidClient {
            val builder = QuidnugClient.builder().baseUrl(nodeUrl)
            if (authToken != null) builder.authToken(authToken)
            return QuidnugAndroidClient(builder.build())
        }
    }
}

/**
 * Dev-grade quid vault backed by SharedPreferences.
 *
 * ⚠ For production, use [AndroidKeystoreSigner] instead — this class
 * stores the PKCS8 private key hex in app-private prefs, which is
 * protected by the OS sandbox but NOT by the Secure Element. On
 * rooted devices or via backup extraction, the key is retrievable.
 *
 * Useful for test and demo apps where Keystore setup is overkill.
 */
class QuidVault(context: Context, name: String = "quidnug-vault") {

    private val prefs: SharedPreferences =
        context.getSharedPreferences(name, Context.MODE_PRIVATE)

    /** Load an existing quid by alias, or return null if absent. */
    fun load(alias: String): Quid? {
        val hex = prefs.getString("priv:$alias", null) ?: return null
        return Quid.fromPrivateHex(hex)
    }

    /** Generate and persist a new quid under [alias]. */
    fun generate(alias: String): Quid {
        val quid = Quid.generate()
        prefs.edit().putString("priv:$alias", quid.privateKeyHex()).apply()
        return quid
    }

    /** Delete a stored quid. */
    fun remove(alias: String) {
        prefs.edit().remove("priv:$alias").apply()
    }

    /** List stored aliases. */
    fun aliases(): List<String> =
        prefs.all.keys.filter { it.startsWith("priv:") }.map { it.removePrefix("priv:") }
}
