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

    /** Suspending wrapper around `getIdentity`. */
    suspend fun getIdentity(quidId: String, domain: String? = null) =
        withContext(Dispatchers.IO) {
            if (domain != null) client.getIdentity(quidId, domain) else client.getIdentity(quidId)
        }

    /** Suspending wrapper around `getTitle`. */
    suspend fun getTitle(assetId: String, domain: String = "default") =
        withContext(Dispatchers.IO) { client.getTitle(assetId, domain) }

    /** Suspending wrapper around `getTrustEdges`. */
    suspend fun getTrustEdges(quidId: String) =
        withContext(Dispatchers.IO) { client.getTrustEdges(quidId) }

    /** Suspending wrapper around `getGuardianSet`. */
    suspend fun getGuardianSet(quidId: String) =
        withContext(Dispatchers.IO) { client.getGuardianSet(quidId) }

    /** Suspending wrapper around `submitGuardianSetUpdate`. */
    suspend fun submitGuardianSetUpdate(update: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitGuardianSetUpdate(update) }

    /** Suspending wrapper around `submitRecoveryInit`. */
    suspend fun submitRecoveryInit(init: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitRecoveryInit(init) }

    /** Suspending wrapper around `submitRecoveryVeto`. */
    suspend fun submitRecoveryVeto(veto: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitRecoveryVeto(veto) }

    /** Suspending wrapper around `submitRecoveryCommit`. */
    suspend fun submitRecoveryCommit(commit: Map<String, Any>) =
        withContext(Dispatchers.IO) { client.submitRecoveryCommit(commit) }

    /** Suspending wrapper around `health` — useful for bootstrap UI. */
    suspend fun health() = withContext(Dispatchers.IO) { client.health() }

    /** Suspending wrapper around `info` — node features for capability gating. */
    suspend fun info() = withContext(Dispatchers.IO) { client.info() }

    /**
     * Full Java [QuidnugClient] surface escape hatch.
     *
     * Every method on the underlying client is reachable via [client]; the
     * suspending bridges above only cover the most common mobile flows.
     * Wrap rarely-used calls in `withContext(Dispatchers.IO) { client.foo() }`
     * to keep them off the main thread.
     */
    @Suppress("unused")
    fun raw(): QuidnugClient = client

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
