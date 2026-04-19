package com.quidnug.android

import android.os.Build
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import com.quidnug.client.Hex
import com.quidnug.client.QuidnugException
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.MessageDigest
import java.security.PrivateKey
import java.security.Signature
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec

/**
 * Signs Quidnug protocol bytes with a private key held in the Android
 * Keystore, optionally behind StrongBox (the Titan-M-class secure
 * element on Pixel 3+ and most flagship Androids).
 *
 * Unlike [com.quidnug.client.Quid] — which keeps the private key in
 * process memory — this signer never exposes the raw key. The JVM
 * receives only a `PrivateKey` handle; signing happens in the Keystore
 * daemon.
 *
 * Typical flow:
 *
 * ```kotlin
 * // First launch — generate and persist
 * val signer = AndroidKeystoreSigner.generate(
 *     alias = "user-quid",
 *     strongBoxIfAvailable = true,
 *     requireUserAuth = true,
 * )
 *
 * // Later sessions — load by alias
 * val signer = AndroidKeystoreSigner.load("user-quid")
 *
 * // Sign a canonical Quidnug transaction
 * val signable = CanonicalBytes.of(tx, "signature", "txId")
 * val sig = signer.sign(signable)  // hex DER
 * ```
 */
class AndroidKeystoreSigner private constructor(
    private val alias: String,
    private val privateKey: PrivateKey,
    private val publicKeyBytes: ByteArray
) {

    /** Quid ID = sha256(publicKey)[0..8] in hex. Matches every other Quidnug SDK. */
    val quidId: String by lazy {
        val digest = MessageDigest.getInstance("SHA-256").digest(publicKeyBytes)
        Hex.encode(digest.copyOf(8))
    }

    /** SEC1 uncompressed hex public key. */
    val publicKeyHex: String by lazy { Hex.encode(publicKeyBytes) }

    /**
     * Sign canonical bytes. Returns hex-encoded DER signature — the
     * same format used everywhere else in the SDK.
     *
     * @throws QuidnugException.CryptoException if the keystore refuses
     *   to sign (e.g. biometric auth timeout on a require-auth key).
     */
    fun sign(data: ByteArray): String {
        try {
            val signature = Signature.getInstance("SHA256withECDSA")
            signature.initSign(privateKey)
            signature.update(data)
            return Hex.encode(signature.sign())
        } catch (e: Exception) {
            throw QuidnugException.CryptoException("keystore sign failed: ${e.message}")
        }
    }

    companion object {
        private const val KEYSTORE = "AndroidKeyStore"

        /**
         * Generate a fresh P-256 keypair under [alias] in the Android
         * Keystore. Overwrites if the alias already exists.
         *
         * @param alias unique-per-app identifier for the keypair
         * @param strongBoxIfAvailable attempt StrongBox placement (best
         *     effort; falls back to TEE on devices without StrongBox)
         * @param requireUserAuth require biometric/pin before each
         *     signing operation
         * @param validityDurationSeconds how long after auth the key
         *     stays usable; ignored when requireUserAuth is false
         */
        fun generate(
            alias: String,
            strongBoxIfAvailable: Boolean = true,
            requireUserAuth: Boolean = false,
            validityDurationSeconds: Int = 60
        ): AndroidKeystoreSigner {
            val kpg = KeyPairGenerator.getInstance(
                KeyProperties.KEY_ALGORITHM_EC, KEYSTORE)

            val builder = KeyGenParameterSpec.Builder(
                alias, KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY)
                .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                .setDigests(KeyProperties.DIGEST_SHA256)

            if (requireUserAuth) {
                builder.setUserAuthenticationRequired(true)
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                    builder.setUserAuthenticationParameters(
                        validityDurationSeconds,
                        KeyProperties.AUTH_BIOMETRIC_STRONG or KeyProperties.AUTH_DEVICE_CREDENTIAL
                    )
                } else {
                    @Suppress("DEPRECATION")
                    builder.setUserAuthenticationValidityDurationSeconds(validityDurationSeconds)
                }
            }

            if (strongBoxIfAvailable && Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                try {
                    builder.setIsStrongBoxBacked(true)
                    kpg.initialize(builder.build())
                    return fromAlias(alias, kpg.generateKeyPair().private, keystorePublicKeyBytes(alias))
                } catch (_: Exception) {
                    // fallthrough to TEE
                }
            }

            val tee = KeyGenParameterSpec.Builder(
                alias, KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY)
                .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                .setDigests(KeyProperties.DIGEST_SHA256).apply {
                    if (requireUserAuth) {
                        setUserAuthenticationRequired(true)
                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                            setUserAuthenticationParameters(
                                validityDurationSeconds,
                                KeyProperties.AUTH_BIOMETRIC_STRONG or KeyProperties.AUTH_DEVICE_CREDENTIAL
                            )
                        } else {
                            @Suppress("DEPRECATION")
                            setUserAuthenticationValidityDurationSeconds(validityDurationSeconds)
                        }
                    }
                }.build()
            kpg.initialize(tee)
            kpg.generateKeyPair()
            return load(alias)
        }

        /** Load an existing keystore-held quid by alias. */
        fun load(alias: String): AndroidKeystoreSigner {
            val ks = KeyStore.getInstance(KEYSTORE).apply { load(null) }
            val entry = ks.getEntry(alias, null) as? KeyStore.PrivateKeyEntry
                ?: throw QuidnugException.CryptoException("alias not found: $alias")
            return fromAlias(alias, entry.privateKey, keystorePublicKeyBytes(alias))
        }

        private fun fromAlias(
            alias: String, priv: PrivateKey, pubBytes: ByteArray
        ): AndroidKeystoreSigner = AndroidKeystoreSigner(alias, priv, pubBytes)

        private fun keystorePublicKeyBytes(alias: String): ByteArray {
            val ks = KeyStore.getInstance(KEYSTORE).apply { load(null) }
            val cert = ks.getCertificate(alias)
                ?: throw QuidnugException.CryptoException("no certificate for $alias")
            val pub = cert.publicKey as ECPublicKey
            return sec1Encode(pub)
        }

        private fun sec1Encode(pk: ECPublicKey): ByteArray {
            val x = bigIntToFixed(pk.w.affineX, 32)
            val y = bigIntToFixed(pk.w.affineY, 32)
            val out = ByteArray(65)
            out[0] = 0x04
            System.arraycopy(x, 0, out, 1, 32)
            System.arraycopy(y, 0, out, 33, 32)
            return out
        }

        private fun bigIntToFixed(v: java.math.BigInteger, len: Int): ByteArray {
            val b = v.toByteArray()
            if (b.size == len) return b
            if (b.size == len + 1 && b[0] == 0.toByte()) return b.copyOfRange(1, b.size)
            val out = ByteArray(len)
            val srcOff = maxOf(0, b.size - len)
            val dstOff = maxOf(0, len - b.size)
            System.arraycopy(b, srcOff, out, dstOff, minOf(b.size, len))
            return out
        }
    }
}
