import com.quidnug.client.Quid;
import com.quidnug.client.QuidnugClient;
import com.quidnug.client.QuidnugClient.EventParams;
import com.quidnug.client.QuidnugClient.IdentityParams;
import com.quidnug.client.Types.Event;

import java.util.List;
import java.util.Map;

/**
 * Audit-stream example — emit events onto a quid's stream and read
 * them back. Typical Java use cases:
 *
 *   - Spring Boot / Micronaut app emits an event per HTTP request as
 *     an audit trail.
 *   - Kafka consumer records a tamper-evident event for each message
 *     it processes.
 *   - JVM scheduled job checkpoints its progress as events so a
 *     downstream monitor can detect stalls.
 */
public class AuditStream {
    public static void main(String[] args) {
        QuidnugClient client = QuidnugClient.builder()
                .baseUrl("http://localhost:8080")
                .build();

        Quid service = Quid.generate();
        client.registerIdentity(service, IdentityParams.name("payment-service"));

        // Emit a few LOGIN events
        for (int i = 0; i < 3; i++) {
            client.emitEvent(service, EventParams
                    .of(service.id(), "QUID", "LOGIN")
                    .payload(Map.of(
                            "user", "u-" + i,
                            "ip", "10.0.0." + i)));
        }

        // Read the stream back
        List<Event> events = client.getStreamEvents(service.id(), null, 10, 0);
        System.out.printf("%d events on stream%n", events.size());
        for (Event e : events) {
            System.out.printf("  #%d %s @ %d: %s%n",
                    e.sequence, e.eventType, e.timestamp, e.payload);
        }
    }
}
