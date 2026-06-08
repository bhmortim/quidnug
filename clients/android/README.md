# Quidnug Android SDK

Kotlin-first Android wrapper over the [Java SDK](../java/), adding:

- **`AndroidKeystoreSigner`** — P-256 keys held in the Android
  Keystore, StrongBox-backed on Pixel 3+/Titan M-class hardware.
  Private keys never enter the JVM heap.
- **`QuidnugAndroidClient`** — suspending (coroutines-first)
  wrappers over the Java `QuidnugClient`, running I/O on
  `Dispatchers.IO`. Every HTTP-touching method on the underlying
  Java client has a matching `suspend fun` — Android callers never
  need to drop down to the blocking Java API just to dispatch I/O.
- **`QuidVault`** — dev-grade SharedPreferences-backed quid store
  for tests and demos.

Version 2.x covers the full Quidnug v2 protocol surface (QDPs
0001–0010): identity, trust, titles, event streams, anchors,
guardian sets + recovery, cross-domain gossip, K-of-K bootstrap,
fork-block activation, IPFS storage, and Prometheus metrics.

**Status**: SDK complete. Publication to Maven Central + Android's
published repository is pending.

## Install

```kotlin
// build.gradle.kts (app)
dependencies {
    implementation("com.quidnug:quidnug-android:2.0.0")
}
```

Requires **minSdk 24** (Android 7.0+), **compileSdk 34** (Android 14),
Kotlin 1.9+, Java 17 target.

## Thirty-second example

```kotlin
import com.quidnug.android.AndroidKeystoreSigner
import com.quidnug.android.QuidnugAndroidClient

val client = QuidnugAndroidClient.create("https://api.myapp.com")

// StrongBox-backed keypair
val signer = AndroidKeystoreSigner.generate(
    alias = "user-quid",
    strongBoxIfAvailable = true,
    requireUserAuth = true  // biometric prompt before each signing
)

// The signer exposes the same quid ID + public key that every other
// SDK produces — use it wherever a JVM Quid goes.
println("quid id: ${signer.quidId}")
```

Or for simple dev flows without the Keystore:

```kotlin
val vault = QuidVault(context)
val quid = vault.load("user") ?: vault.generate("user")
client.registerIdentity(quid, name = "Alice", homeDomain = "myapp.users")
```

## What ships

### `QuidnugAndroidClient`

```kotlin
val client = QuidnugAndroidClient.create(
    nodeUrl = "https://api.example.com",
    authToken = BuildConfig.QUIDNUG_TOKEN
)

// All HTTP methods are suspending — call them straight from any
// coroutine scope. No manual Dispatchers.IO juggling required.
lifecycleScope.launch {
    client.registerIdentity(quid, name = "Alice", homeDomain = "myapp.users")
    client.grantTrust(quid, trustee = "bob-id", level = 0.9, domain = "myapp.users")
    val tr = client.getTrust(quid.id(), "bob-id", "myapp.users")
    Log.i("Quidnug", "trust = ${tr.trustLevel}")
}
```

The underlying Java `QuidnugClient` is reachable as `client.client`
for direct access (param builders, byte-for-byte parity with JVM
tutorials), but every HTTP endpoint has a `suspend fun` on the
wrapper so Kotlin callers shouldn't normally need it.

#### Method coverage

Every public HTTP method on `com.quidnug.client.QuidnugClient` has a
matching `suspend fun` on `QuidnugAndroidClient`:

| Area | `suspend fun`s |
| --- | --- |
| Health / info | `health`, `info`, `nodes`, `blocks`, `pendingTransactions`, `listDomains`, `getTentativeBlocks` |
| Quids | `createQuid` |
| Identity | `registerIdentity`, `getIdentity`, `queryIdentityRegistry` |
| Trust | `grantTrust`, `getTrust`, `getTrustEdges`, `queryRelationalTrust`, `queryTrustRegistry`, `queryTrustRegistryRelational` |
| Title | `registerTitle`, `getTitle`, `queryTitleRegistry` |
| Events | `emitEvent`, `getEventStream`, `streamEvents` / `getStreamEvents` |
| Guardians | `submitGuardianSetUpdate`, `submitRecoveryInit`, `submitRecoveryVeto`, `submitRecoveryCommit`, `submitGuardianResignation`, `getGuardianSet`, `getPendingRecovery` |
| Gossip | `submitDomainFingerprint`, `getLatestDomainFingerprint`, `submitAnchorGossip`, `pushAnchor`, `pushFingerprint`, `receiveDomainGossip` |
| Bootstrap | `submitNonceSnapshot`, `getLatestNonceSnapshot`, `bootstrapStatus` |
| Fork-block | `submitForkBlock`, `forkBlockStatus` |
| Domains | `registerDomain`, `queryDomain`, `getNodeDomains`, `updateNodeDomains` |
| Storage (IPFS) | `pinToIPFS`, `getFromIPFS` |
| Metrics | `getMetrics` |

All wrappers dispatch on `Dispatchers.IO` and return the same value
shapes as the Java SDK — Jackson `JsonNode` for raw envelopes,
typed `com.quidnug.client.Types.*` records where the Java client
already deserializes (e.g. `TrustResult`, `IdentityRecord`, `Title`,
`GuardianSet`, `DomainFingerprint`, `Event`, `TrustEdge`).

### `AndroidKeystoreSigner`

Hardware-backed ECDSA P-256 signing:

```kotlin
AndroidKeystoreSigner.generate(
    alias = "user-quid",
    strongBoxIfAvailable = true,   // falls back to TEE if no StrongBox
    requireUserAuth = true,         // biometric prompt per signing
    validityDurationSeconds = 60    // reuse auth for 60s
)
```

- Keys generated under the Android Keystore are bound to the device
  and non-extractable. A compromised app binary cannot exfiltrate
  them.
- StrongBox moves the key into a Titan-M-class Secure Element,
  isolated even from a compromised OS kernel.
- `requireUserAuth` binds each signing operation to biometric or
  device-credential authentication.

The signer implements the same contract as the JVM `Quid`, so it
plugs into every `QuidnugAndroidClient` method that takes a signer
(`registerIdentity`, `grantTrust`, `registerTitle`, `emitEvent`).

### `QuidVault`

Dev-grade SharedPreferences-backed key store. Use for demos and
tests only — production should prefer `AndroidKeystoreSigner`.

```kotlin
val vault = QuidVault(context)
val alice = vault.load("alice") ?: vault.generate("alice")
vault.aliases()   // -> ["alice"]
vault.remove("alice")
```

## Examples

| File | Shows |
| --- | --- |
| `examples/MobileAuthActivity.kt` | First-launch enrollment → LOGIN audit events → fetch history. |

## Jetpack Compose integration

```kotlin
@Composable
fun TrustBadge(observer: String, target: String, client: QuidnugAndroidClient) {
    var trust by remember { mutableStateOf<Double?>(null) }
    LaunchedEffect(observer, target) {
        val tr = client.getTrust(observer, target, "myapp.users")
        trust = tr.trustLevel
    }
    Text(if (trust == null) "..." else "%.2f".format(trust))
}
```

## Error handling

The wrapper re-throws the Java SDK's exception hierarchy unchanged
(running inside `withContext(Dispatchers.IO)` propagates them onto
the calling coroutine):

| Exception | When |
| --- | --- |
| `ValidationException` | Local precondition failed before any network call. |
| `ConflictException` | Node logically rejected (nonce replay, quorum failure, …). |
| `UnavailableException` | HTTP 503 or feature-not-active. |
| `NodeException` | Transport, 5xx after retries, unexpected response shape. |
| `CryptoException` | Signature / key derivation failed. |

All inherit from `QuidnugException`, so a catch-all is safe.

## Protocol version compatibility

| SDK | Node | QDPs |
| --- | --- | --- |
| 2.x | 2.x | 0001–0010 |

## License

Apache-2.0.
