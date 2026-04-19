import com.quidnug.client.Quid;
import com.quidnug.client.QuidnugClient;
import com.quidnug.client.QuidnugClient.IdentityParams;
import com.quidnug.client.QuidnugClient.TrustParams;
import com.quidnug.client.Types.TrustResult;

/**
 * Two-party trust quickstart.
 *
 * Assumes a local node at http://localhost:8080. Compile and run with:
 *
 *   cd clients/java
 *   ./gradlew build
 *   java -cp build/libs/quidnug-2.0.0.jar:build/deps/* examples/Quickstart
 */
public class Quickstart {
    public static void main(String[] args) {
        QuidnugClient client = QuidnugClient.builder()
                .baseUrl("http://localhost:8080")
                .build();

        System.out.println("connected to node: " + client.info());

        Quid alice = Quid.generate();
        Quid bob = Quid.generate();
        System.out.printf("alice=%s bob=%s%n", alice.id(), bob.id());

        client.registerIdentity(alice, IdentityParams.name("Alice").homeDomain("demo.home"));
        client.registerIdentity(bob,   IdentityParams.name("Bob").homeDomain("demo.home"));

        client.grantTrust(alice, TrustParams.of(bob.id(), 0.9, "demo.home"));

        TrustResult tr = client.getTrust(alice.id(), bob.id(), "demo.home", 5);
        System.out.printf("trust %.3f via %s (depth %d)%n",
                tr.trustLevel,
                String.join(" -> ", tr.path),
                tr.pathDepth);
    }
}
