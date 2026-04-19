# MQTT bridge (scaffold)

Status: **SCAFFOLD.**

Bridges MQTT broker messages onto Quidnug event streams, giving IoT
devices a per-observer-trust-aware audit log.

## Planned flow

```
Device ──MQTT publish──► Broker ──webhook──► mqtt-bridge
                                                │
                                                ├─ parse payload
                                                ├─ validate device signature
                                                └─► EmitEvent on device's
                                                    Quidnug Title
```

## Topics ↔ Quidnug events

| MQTT topic | Quidnug SubjectID | event_type |
| --- | --- | --- |
| `devices/{deviceId}/telemetry` | `<deviceId>` (Title) | `MQTT.telemetry` |
| `devices/{deviceId}/commands` | `<deviceId>` (Title) | `MQTT.command` |
| `devices/{deviceId}/status` | `<deviceId>` (Title) | `MQTT.status` |
| `fleet/{fleetId}/events` | `<fleetId>` (Title) | `MQTT.fleet` |

## Trust model

Each device is represented as a Quidnug Title. Its issuer is the
fleet operator's quid. Relational trust lets dashboards ask "which
devices do I trust at ≥ 0.7 based on the fleet operator's quid?" —
useful for filtering noise from compromised or spoofed devices.

## Roadmap

1. Implement as a Go service consuming an MQTT broker (EMQX, HiveMQ,
   or AWS IoT Core) via an HTTP webhook or shared broker connection.
2. Device-side SDKs under `clients/embedded/` (future).

## License

Apache-2.0.
