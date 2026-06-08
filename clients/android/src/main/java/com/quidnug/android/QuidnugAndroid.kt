package com.quidnug.android

import android.content.Context
import android.content.SharedPreferences
import com.fasterxml.jackson.databind.JsonNode
import com.quidnug.client.Quid
import com.quidnug.client.QuidnugClient
import com.quidnug.client.Types
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * High-level Android convenience wrapper over [QuidnugClient].
 *
 * Adds:
 *  - [QuidVault]: wraps SharedPreferences for dev persistence and
 *    [AndroidKeystoreSigner] for production Secure-Enclave-backed keys.
 *  - Suspending bridge methods that run the blocking Java client on
 *    Dispatchers.IO so Kotlin coroutines-first callers don't block the
 *    main thread.
 *
 * Every HTTP-touching public method on the underlying [QuidnugClient]
 * has a matching `suspend fun` here, so Android callers never need to
 * drop down to the Java client just to dispatch I/O off the main
 * thread. The Java client remains reachable via [client] for direct
 * access (param builders, byte-for-byte parity with JVM tutorials).
 */
class QuidnugAndroidClient(
    val client: QuidnugClient
) {

    // =====================================================================
    // Health / info / peers
    // =====================================================================

    suspend fun health(): JsonNode = withContext(Dispatchers.IO) { client.health() }

    suspend fun info(): JsonNode = withContext(Dispatchers.IO) { client.info() }

    suspend fun nodes(): JsonNode = withContext(Dispatchers.IO) { client.nodes() }

    suspend fun blocks(): JsonNode = withContext(Dispatchers.IO) { client.blocks() }

    suspend fun pendingTransactions(): JsonNode = withContext(Dispatchers.IO) {
        client.pendingTransactions()
    }

    suspend fun listDomains(): JsonNode = withContext(Dispatchers.IO) { client.listDomains() }

    suspend fun getTentativeBlocks(domain: String): JsonNode = withContext(Dispatchers.IO) {
        client.getTentativeBlocks(domain)
    }

    // =====================================================================
    // Quids (server-side keypair generation)
    // =====================================================================

    suspend fun createQuid(metadata: Map<String, Any>? = null): JsonNode =
        withContext(Dispatchers.IO) { client.createQuid(metadata) }

    // =====================================================================
    // Identity
    // =====================================================================

    suspend fun registerIdentity(
        signer: Quid,
        name: String? = null,
        homeDomain: String? = null,
        domain: String = "default",
        description: String? = null,
        subjectQuid: String? = null,
        attributes: Map<String, Any>? = null,
        updateNonce: Long = 1,
    ): JsonNode = withContext(Dispatchers.IO) {
        val p = QuidnugClient.IdentityParams().apply {
            this.name = name
            this.homeDomain = homeDomain
            this.domain = domain
            this.description = description
            this.subjectQuid = subjectQuid
            this.attributes = attributes
            this.updateNonce = updateNonce
        }
        client.registerIdentity(signer, p)
    }

    suspend fun getIdentity(quidId: String, domain: String? = null): Types.IdentityRecord? =
        withContext(Dispatchers.IO) { client.getIdentity(quidId, domain) }

    suspend fun queryIdentityRegistry(
        quidId: String? = null,
        limit: Int? = null,
        offset: Int? = null,
    ): JsonNode = withContext(Dispatchers.IO) {
        client.queryIdentityRegistry(quidId, limit, offset)
    }

    // =====================================================================
    // Trust
    // =====================================================================

    suspend fun grantTrust(
        signer: Quid,
        trustee: String,
        level: Double,
        domain: String = "default",
        nonce: Long = 1,
        validUntil: Long = 0,
        description: String? = null,
    ): JsonNode = withContext(Dispatchers.IO) {
        val p = QuidnugClient.TrustParams.of(trustee, level, domain).apply {
            this.nonce = nonce
            this.validUntil = validUntil
            this.description = description
        }
        client.grantTrust(signer, p)
    }

    suspend fun getTrust(
        observer: String,
        target: String,
        domain: String,
        maxDepth: Int = 5,
    ): Types.TrustResult = withContext(Dispatchers.IO) {
        client.getTrust(observer, target, domain, maxDepth)
    }

    suspend fun getTrustEdges(quidId: String): List<Types.TrustEdge> =
        withContext(Dispatchers.IO) { client.getTrustEdges(quidId) }

    suspend fun queryRelationalTrust(
        observer: String,
        target: String,
        domain: String = "default",
        maxDepth: Int = 5,
        includeUnverified: Boolean? = null,
    ): Types.TrustResult = withContext(Dispatchers.IO) {
        client.queryRelationalTrust(observer, target, domain, maxDepth, includeUnverified)
    }

    suspend fun queryTrustRegistry(
        truster: String? = null,
        trustee: String? = null,
        limit: Int? = null,
        offset: Int? = null,
    ): JsonNode = withContext(Dispatchers.IO) {
        client.queryTrustRegistry(truster, trustee, limit, offset)
    }

    suspend fun queryTrustRegistryRelational(
        observer: String,
        target: String,
        maxDepth: Int? = null,
    ): JsonNode = withContext(Dispatchers.IO) {
        client.queryTrustRegistryRelational(observer, target, maxDepth)
    }

    // =====================================================================
    // Title
    // =====================================================================

    suspend fun registerTitle(
        signer: Quid,
        assetId: String,
        owners: List<Types.OwnershipStake>,
        domain: String = "default",
        titleType: String? = null,
        prevTitleTxId: String? = null,
    ): JsonNode = withContext(Dispatchers.IO) {
        val p = QuidnugClient.TitleParams.of(assetId, owners).apply {
            this.domain = domain
            this.titleType = titleType
            this.prevTitleTxId = prevTitleTxId
        }
        client.registerTitle(signer, p)
    }

    suspend fun getTitle(assetId: String, domain: String? = null): Types.Title? =
        withContext(Dispatchers.IO) { client.getTitle(assetId, domain) }

    suspend fun queryTitleRegistry(
        assetId: String? = null,
        ownerId: String? = null,
        limit: Int? = null,
        offset: Int? = null,
    ): JsonNode = withContext(Dispatchers.IO) {
        client.queryTitleRegistry(assetId, ownerId, limit, offset)
    }

    // =====================================================================
    // Events + streams
    // =====================================================================

    suspend fun emitEvent(
        signer: Quid,
        subjectId: String,
        subjectType: String,
        eventType: String,
        domain: String = "default",
        payload: Map<String, Any?>? = null,
        payloadCid: String? = null,
        sequence: Long = 0,
    ): JsonNode = withContext(Dispatchers.IO) {
        val p = QuidnugClient.EventParams.of(subjectId, subjectType, eventType).apply {
            this.domain = domain
            this.payload = payload
            this.payloadCid = payloadCid
            this.sequence = sequence
        }
        client.emitEvent(signer, p)
    }

    suspend fun getEventStream(subjectId: String, domain: String? = null): JsonNode? =
        withContext(Dispatchers.IO) { client.getEventStream(subjectId, domain) }

    suspend fun streamEvents(
        subjectId: String,
        domain: String? = null,
        limit: Int = 50,
        offset: Int = 0,
    ): List<Types.Event> = withContext(Dispatchers.IO) {
        client.getStreamEvents(subjectId, domain, limit, offset)
    }

    // Underlying Java name kept as an alias for callers reading Java docs.
    suspend fun getStreamEvents(
        subjectId: String,
        domain: String? = null,
        limit: Int = 50,
        offset: Int = 0,
    ): List<Types.Event> = streamEvents(subjectId, domain, limit, offset)

    // =====================================================================
    // Guardians (QDP-0002)
    // =====================================================================

    suspend fun submitGuardianSetUpdate(update: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitGuardianSetUpdate(update) }

    suspend fun submitRecoveryInit(init: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitRecoveryInit(init) }

    suspend fun submitRecoveryVeto(veto: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitRecoveryVeto(veto) }

    suspend fun submitRecoveryCommit(commit: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitRecoveryCommit(commit) }

    suspend fun submitGuardianResignation(resig: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitGuardianResignation(resig) }

    suspend fun getGuardianSet(quidId: String): Types.GuardianSet? =
        withContext(Dispatchers.IO) { client.getGuardianSet(quidId) }

    suspend fun getPendingRecovery(quidId: String): JsonNode? =
        withContext(Dispatchers.IO) { client.getPendingRecovery(quidId) }

    // =====================================================================
    // Gossip + bootstrap + fork-block
    // =====================================================================

    suspend fun submitDomainFingerprint(fp: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitDomainFingerprint(fp) }

    suspend fun getLatestDomainFingerprint(domain: String): Types.DomainFingerprint? =
        withContext(Dispatchers.IO) { client.getLatestDomainFingerprint(domain) }

    suspend fun submitAnchorGossip(msg: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitAnchorGossip(msg) }

    suspend fun pushAnchor(msg: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.pushAnchor(msg) }

    suspend fun pushFingerprint(fp: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.pushFingerprint(fp) }

    suspend fun submitNonceSnapshot(snapshot: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitNonceSnapshot(snapshot) }

    suspend fun getLatestNonceSnapshot(domain: String): JsonNode? =
        withContext(Dispatchers.IO) { client.getLatestNonceSnapshot(domain) }

    suspend fun bootstrapStatus(): JsonNode = withContext(Dispatchers.IO) {
        client.bootstrapStatus()
    }

    suspend fun submitForkBlock(fb: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.submitForkBlock(fb) }

    suspend fun forkBlockStatus(): JsonNode = withContext(Dispatchers.IO) {
        client.forkBlockStatus()
    }

    // =====================================================================
    // Domains
    // =====================================================================

    suspend fun registerDomain(
        name: String,
        trustThreshold: Double,
        validatorNodes: List<String>? = null,
        validators: Map<String, Double>? = null,
        validatorPublicKeys: Map<String, String>? = null,
    ): JsonNode = withContext(Dispatchers.IO) {
        val p = QuidnugClient.DomainParams.of(name, trustThreshold).apply {
            this.validatorNodes = validatorNodes
            this.validators = validators
            this.validatorPublicKeys = validatorPublicKeys
        }
        client.registerDomain(p)
    }

    // Escape-hatch overload mirroring the Java Map<String,Object> variant —
    // useful when callers already hold a body assembled elsewhere.
    suspend fun registerDomain(body: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.registerDomain(body) }

    suspend fun queryDomain(name: String, type: String, param: String): JsonNode =
        withContext(Dispatchers.IO) { client.queryDomain(name, type, param) }

    // =====================================================================
    // Node-managed domains + node-to-node gossip
    // =====================================================================

    suspend fun getNodeDomains(): JsonNode = withContext(Dispatchers.IO) {
        client.getNodeDomains()
    }

    suspend fun updateNodeDomains(domains: List<String>): JsonNode =
        withContext(Dispatchers.IO) { client.updateNodeDomains(domains) }

    suspend fun receiveDomainGossip(gossipMessage: Map<String, Any>): JsonNode =
        withContext(Dispatchers.IO) { client.receiveDomainGossip(gossipMessage) }

    suspend fun receiveDomainGossip(gossipMessage: JsonNode): JsonNode =
        withContext(Dispatchers.IO) { client.receiveDomainGossip(gossipMessage) }

    // =====================================================================
    // IPFS / large-payload storage
    // =====================================================================

    suspend fun pinToIPFS(content: ByteArray): String =
        withContext(Dispatchers.IO) { client.pinToIPFS(content) }

    suspend fun getFromIPFS(cid: String): ByteArray =
        withContext(Dispatchers.IO) { client.getFromIPFS(cid) }

    // =====================================================================
    // Metrics
    // =====================================================================

    suspend fun getMetrics(): String = withContext(Dispatchers.IO) { client.getMetrics() }

    companion object {
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

    fun load(alias: String): Quid? {
        val hex = prefs.getString("priv:$alias", null) ?: return null
        return Quid.fromPrivateHex(hex)
    }

    fun generate(alias: String): Quid {
        val quid = Quid.generate()
        prefs.edit().putString("priv:$alias", quid.privateKeyHex()).apply()
        return quid
    }

    fun remove(alias: String) {
        prefs.edit().remove("priv:$alias").apply()
    }

    fun aliases(): List<String> =
        prefs.all.keys.filter { it.startsWith("priv:") }.map { it.removePrefix("priv:") }
}
