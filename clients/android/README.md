# Quidnug Android SDK

Kotlin-first Android wrapper over the [Java SDK](../java/), adding:

- **`AndroidKeystoreSigner`** — P-256 keys held in the Android
  Keystore, StrongBox-backed on Pixel 3+/Titan M-class hardware.
  Private keys never enter the JVM heap.
- **`QuidnugAndroidClient`** — suspending (coroutines-first)
  wrappers over the Java `QuidnugClient`, running I/O on
  `Dispatchers.IO`.
- **`QuidVault`** — dev-grade SharedPreferences-backed quid store
  for tests and demos.

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

### `QuidnugAndroidClient`

```kotlin
val client = QuidnugAndroidClient.create(
    nodeUrl = "https://api.example.com",
    authToken = BuildConfig.QUIDNUG_TOKEN
)

// All methods are suspending (coroutines-first).
lifecycleScope.launch {
    client.registerIdentity(quid, name = "Alice", homeDomain = "myapp.users")
    client.grantTrust(quid, trustee = "bob-id", level = 0.9, domain = "myapp.users")
    val tr = client.getTrust(quid.id(), "bob-id", "myapp.users")
    Log.i("Quidnug", "trust = ${tr.trustLevel}")
}
```

Full protocol surface is available via `client.client` (the
underlying Java `QuidnugClient`) — every Java method works
unchanged.

### `QuidVault`

Dev-grade SharedPreferences-backed key store. Use for demos and
tests only — production should prefer `AndroidKeystoreSigner`.

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

## Protocol version compatibility

| SDK | Node | QDPs |
| --- | --- | --- |
| 2.x | 2.x | 0001–0010 |

## License

Apache-2.0.
