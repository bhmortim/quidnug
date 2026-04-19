// Package kafka bridges Quidnug event streams to Apache Kafka.
//
// Each Quidnug EVENT published on a configured subject's stream
// becomes one Kafka message on a configured topic. This lets
// existing Kafka consumers (Spark, Flink, Kafka Streams, downstream
// microservices) subscribe to Quidnug activity without speaking
// Quidnug's HTTP API.
//
// # Direction
//
// This package is Quidnug → Kafka. For the reverse — accepting
// signed events off a Kafka topic and forwarding them into
// Quidnug's stream endpoints — write a thin consumer using
// pkg/client's EmitEvent method; no special bridge package is
// needed.
//
// # Message format
//
// Each Kafka message has:
//
//   - Key: the subject quid/title ID (deterministic per-subject
//     partitioning, so a consumer sees ordered events per subject).
//   - Value: the JSON-serialized Quidnug Event wire form.
//   - Headers:
//       "quidnug-domain"      = trust domain
//       "quidnug-event-type"  = event_type
//       "quidnug-sequence"    = sequence number
//       "quidnug-subject-type" = "QUID" | "TITLE"
//
// Consumers can filter by header without parsing the value.
//
// # At-least-once delivery
//
// The bridge uses manual acks: a Kafka publish must succeed before
// the bridge advances its "high water mark" for the subject. On
// restart, it rewinds to the last confirmed sequence and replays
// any gap — the consumer may see a duplicate. This matches standard
// Kafka semantics; downstream consumers should be idempotent.
//
// # Backpressure
//
// When Kafka acks slow down, the bridge applies backpressure on
// its stream subscription — it stops polling Quidnug until Kafka
// catches up. The oldest un-acked subject is retried with
// exponential backoff; after MaxRetries consecutive failures, the
// bridge emits an alert and moves on.
package kafka
