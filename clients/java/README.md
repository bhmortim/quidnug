# Quidnug Java SDK

Java 17+ client SDK for [Quidnug](https://github.com/bhmortim/quidnug),
a decentralized protocol for relational, per-observer trust.

Covers the **full v2 protocol surface** (QDPs 0001–0010): identity,
trust, titles, event streams, anchors, guardian sets + recovery,
cross-domain gossip, K-of-K bootstrap, fork-block activation, and
compact Merkle inclusion proofs.

## Install

**Maven:**

```xml
<dependency>
    <groupId>com.quidnug</groupId>
    <artifactId>quidnug-client</artifactId>
    <version>2.0.0</version>
</dependency>
```

**Gradle (Kotlin DSL):**

```kotlin
dependencies {
    implementation("com.quidnug:quidnug-client:2.0.0")
}
```

Requires **Java 17+**. Works on Kotlin, Scala, Clojure, and any JVM
language via the same Maven coordinate.

## Thirty-second example

```java
import com.quidnug.client.Quid;
import com.quidnug.client.QuidnugClient;
import com.quidnug.client.QuidnugClient.IdentityParams;
import com.quidnug.client.QuidnugClient.TrustParams;
import com.quidnug.client.Types.TrustResult;

QuidnugClient client = QuidnugClient.builder()
        .baseUrl("http://localhost:8080")
        .build();

Quid alice = Quid.generate();
Quid bob = Quid.generate();

client.registerIdentity(alice, IdentityParams.name("Alice").homeDomain("contractors.home"));
client.registerIdentity(bob,   IdentityParams.name("Bob").homeDomain("contractors.home"));
client.grantTrust(alice, TrustParams.of(bob.id(), 0.9, "contractors.home"));

TrustResult tr = client.getTrust(alice.id(), bob.id(), "contractors.home", 5);
System.out.printf("%.3f via %s%n", tr.trustLevel, String.join(" -> ", tr.path));
```

Runnable examples under [`examples/`](examples/):

| File | What it shows |
| --- | --- |
| `Quickstart.java` | Two-party trust + relational trust query. |
| `EnterpriseOnboarding.java` | Vendor KYC → threshold-gated PO release. |
| `AuditStream.java` | Emit events onto a service quid; read them back. |

## What ships in the SDK

### `Quid` — ECDSA P-256 identity

```java
Quid alice = Quid.generate();                         // fresh keypair
Quid bob   = Quid.fromPrivateHex(storedHex);          // reconstruct
Quid carol = Quid.fromPublicHex(networkPubHex);       // read-only

String sig = alice.sign(data);
boolean ok = alice.verify(data, sig);
```

P-256 + SHA-256, DER-encoded hex signatures. The quid ID is
`sha256(publicKey)[0..8]` — byte-for-byte compatible with Go, Python,
JavaScript, and Rust SDKs. A signature produced by one SDK verifies
against any other.

### `QuidnugClient` — HTTP surface

Thread-safe, builder-constructed. Every endpoint has a typed method.

| Area | Methods |
| --- | --- |
| Health | `health`, `info`, `nodes`, `blocks`, `pendingTransactions`, `listDomains` |
| Identity | `registerIdentity`, `getIdentity` |
| Trust | `grantTrust`, `getTrust`, `getTrustEdges` |
| Title | `registerTitle`, `getTitle` |
| Events | `emitEvent`, `getEventStream`, `getStreamEvents` |
| Guardians (QDP-0002) | `submitGuardianSetUpdate`, `submitRecoveryInit/Veto/Commit`, `submitGuardianResignation`, `getGuardianSet`, `getPendingRecovery` |
| Gossip (QDP-0003/5) | `submitDomainFingerprint`, `getLatestDomainFingerprint`, `submitAnchorGossip`, `pushAnchor`, `pushFingerprint` |
| Bootstrap (QDP-0008) | `submitNonceSnapshot`, `getLatestNonceSnapshot`, `bootstrapStatus` |
| Fork-block (QDP-0009) | `submitForkBlock`, `forkBlockStatus` |

### `CanonicalBytes` — signable-bytes encoder

```java
byte[] signable = CanonicalBytes.of(tx, "signature", "txId");
String sig = signer.sign(signable);
```

Matches the Go / Python / JS / Rust canonicalization byte-for-byte
(sorted keys, nested recursively, named top-level fields excluded).
See [`schemas/types/canonicalization.md`](../../schemas/types/canonicalization.md).

### `Merkle.verifyInclusionProof` — QDP-0010

```java
List<Merkle.Frame> frames = List.of(
    new Merkle.Frame("aabb..", "right"),
    new Merkle.Frame("ccdd..", "left")
);
boolean ok = Merkle.verifyInclusionProof(canonicalTxBytes, frames, rootHex);
```

## Error handling

```java
try {
    client.grantTrust(alice, TrustParams.of(bob.id(), 0.9, "contractors.home"));
} catch (QuidnugException.ConflictException e) {
    // Nonce replay, quorum not met, guardian-set-hash mismatch, etc.
} catch (QuidnugException.UnavailableException e) {
    // 503 / feature not yet active / bootstrapping
} catch (QuidnugException.NodeException e) {
    // Transport / unexpected 5xx
    System.err.println("HTTP " + e.statusCode());
} catch (QuidnugException.ValidationException e) {
    // Local precondition failed
}
```

All exceptions inherit from `QuidnugException`. Catch that for a
blanket handler.

## Retry policy

- **GETs** retry up to `maxRetries` times (default 3) on 5xx and 429.
  Exponential backoff + ±100 ms jitter, capped at 60s. Respects
  `Retry-After`.
- **POSTs** are **not** retried. Reconcile with a follow-up GET
  before replaying a write — Quidnug transactions are not all
  idempotent.

```java
QuidnugClient client = QuidnugClient.builder()
        .baseUrl("https://node.example.com")
        .timeout(Duration.ofSeconds(60))
        .maxRetries(5)
        .retryBaseDelay(Duration.ofMillis(500))
        .authToken(System.getenv("QUIDNUG_TOKEN"))
        .userAgent("my-app/1.0")
        .build();
```

## Build

```bash
cd clients/java

# Maven
mvn test
mvn package

# Gradle (alternative)
./gradlew test
./gradlew build
```

## Verifying tests

The SDK ships 20 unit tests covering keypair generation, signing,
canonicalization, Merkle proof verification, HTTP envelope parsing,
error taxonomy, and retry behavior:

```
Tests run: 20, Failures: 0, Errors: 0, Skipped: 0
  CanonicalBytesTest: 3/3
  MerkleTest:         5/5
  QuidnugClientTest:  7/7
  QuidTest:           5/5
```

The client tests use the built-in `jdk.httpserver` to stub responses
— no WireMock or MockWebServer dependency.

## Java-specific patterns

### Spring Boot wiring

```java
@Configuration
public class QuidnugConfig {
    @Bean
    public QuidnugClient quidnugClient(@Value("${quidnug.node}") String nodeUrl,
                                        @Value("${quidnug.token:}") String token) {
        QuidnugClient.Builder b = QuidnugClient.builder().baseUrl(nodeUrl);
        if (!token.isEmpty()) b.authToken(token);
        return b.build();
    }
}
```

### Kotlin callers

```kotlin
val client = QuidnugClient.builder().baseUrl("http://localhost:8080").build()
val alice = Quid.generate()
val bob = Quid.generate()
client.grantTrust(alice, QuidnugClient.TrustParams.of(bob.id(), 0.9, "demo.home"))
```

### Using an HSM for signing

Because the `Quid` API takes a `PrivateKey`, you can substitute a
JCA-backed HSM-held key (e.g. via `PKCS11` provider or AWS CloudHSM
provider) anywhere a quid would be used. For a ready-made abstraction,
see the Go `pkg/signer/hsm/` package — the JVM equivalent lives on the
roadmap.

## Protocol version compatibility

| SDK | Node | QDPs |
| --- | --- | --- |
| 2.x | 2.x | 0001–0010 |

## Contributing

PRs welcome. Run `mvn test` before submitting. See the Python SDK
(`clients/python/`) for the canonical reference implementation.

## License

Apache-2.0.
