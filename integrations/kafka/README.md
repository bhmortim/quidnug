# Quidnug → Kafka bridge

Tails Quidnug event streams and republishes them to Apache Kafka
topics so existing Kafka infrastructure (Spark, Flink, Kafka
Streams, downstream microservices) can subscribe without
speaking Quidnug's HTTP API.

## Architecture

```
 ┌──────────────┐  HTTP poll  ┌──────────────┐  publish  ┌──────────────┐
 │ Quidnug node │ ◄────────── │    bridge    │ ────────► │    Kafka     │
 │  streams     │             │              │           │   broker     │
 └──────────────┘             └──────────────┘           └──────────────┘
                                     │
                                     └── per-subject high-water mark
```

Each subject configured on the bridge is mapped to a single
Kafka topic. Events are keyed by subject ID so per-subject
ordering is preserved across consumer partitions.

## Usage

```go
import (
    "context"
    "github.com/quidnug/quidnug/integrations/kafka"
    "github.com/quidnug/quidnug/pkg/client"
)

func main() {
    c, _ := client.New("http://quidnug-node:8080")

    // Adapt your preferred Kafka client to the Producer interface.
    // See examples/ for kafka-go and confluent-kafka-go adapters.
    producer := NewKafkaProducer(...)

    bridge, err := kafka.New(kafka.Options{
        Client:   c,
        Producer: producer,
        Subjects: []kafka.Subject{
            {SubjectID: "vendor-titles",  Domain: "supply", Topic: "quidnug.vendor-events"},
            {SubjectID: "audit-stream",   Domain: "corp.audit", Topic: "quidnug.audit"},
        },
        PollInterval: 2 * time.Second,
        MaxRetries:   5,
    })
    if err != nil { panic(err) }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    _ = bridge.Run(ctx)
}
```

## Message format

Each Kafka message produced:

| Field | Value |
| --- | --- |
| `key` | subject ID (for consistent partitioning) |
| `value` | JSON of the Quidnug `Event` type |
| `header["quidnug-domain"]` | trust domain |
| `header["quidnug-event-type"]` | event type (e.g. `LOGIN`, `SIGSTORE_SIGNATURE`) |
| `header["quidnug-sequence"]` | decimal string of event sequence |
| `header["quidnug-subject-type"]` | `QUID` or `TITLE` |

## Delivery semantics

- **At-least-once.** The bridge advances its high-water mark only
  after a successful Kafka publish. On restart, persist and pass
  the last-committed HWM via `Subject.StartAt` to avoid duplicate
  replay.
- **Idempotent consumers recommended.** Write your consumer so
  that replaying the same event (by `quidnug-sequence` +
  subject ID) is safe.

## Backpressure + retries

If Kafka publish fails, the bridge retries up to `MaxRetries`
times with exponential backoff (1s → 2s → 4s → ...). After
exhausting retries, the bridge logs and moves on — the failing
subject's HWM isn't advanced, so the event will be retried on
the next poll.

## Tests

```bash
cd integrations/kafka
go test -v
```

The test suite uses a `fakeProducer` (no real Kafka required) and
covers:

- Happy-path publish of all events
- Resume from a non-zero `StartAt`
- Retry-then-succeed on transient Kafka failure
- Options validation

## Adapting your Kafka client

The bridge consumes a minimal `Producer` interface:

```go
type Producer interface {
    Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error
    Close() error
}
```

Adapting `segmentio/kafka-go`:

```go
type kafkaGoProducer struct{ w *kafka.Writer }

func (p *kafkaGoProducer) Publish(ctx context.Context, topic string, key, value []byte, headers map[string]string) error {
    h := make([]kafka.Header, 0, len(headers))
    for k, v := range headers {
        h = append(h, kafka.Header{Key: k, Value: []byte(v)})
    }
    return p.w.WriteMessages(ctx, kafka.Message{
        Topic:   topic,
        Key:     key,
        Value:   value,
        Headers: h,
    })
}

func (p *kafkaGoProducer) Close() error { return p.w.Close() }
```

(~15 lines; do the same for your preferred client.)

## License

Apache-2.0.
