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
