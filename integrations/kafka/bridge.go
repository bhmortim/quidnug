package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

// Producer is the minimal Kafka producer interface this bridge
// needs. Adapting kafka-go or confluent-kafka-go is a ~20-line
// shim (see examples/).
type Producer interface {
	// Publish sends one message. Blocks until ack or error.
	Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error
	// Close flushes and releases resources.
	Close() error
}

// Options configures the bridge.
type Options struct {
	// Client is the Quidnug HTTP client.
	Client *client.Client

	// Producer is the Kafka producer.
	Producer Producer

	// Subjects lists the Quidnug subject IDs to subscribe to. The
	// bridge tails each subject's event stream and publishes new
	// events to Kafka. At least one subject is required.
	Subjects []Subject

	// PollInterval is how often to poll each subject's stream for
	// new events. Default 5s.
	PollInterval time.Duration

	// MaxRetries is how many times to retry a failing Kafka publish
	// before emitting an alert. Default 5.
	MaxRetries int

	// RetryBaseDelay is the initial backoff; doubles each retry up
	// to 60s. Default 1s.
	RetryBaseDelay time.Duration

	// Logger is optional. Defaults to discard.
	Logger func(format string, args ...any)
}

// Subject configures one Quidnug → Kafka subscription.
type Subject struct {
	// QuidID / TitleID of the subject whose stream to tail.
	SubjectID string
	// Domain filter for events on the stream.
	Domain string
	// Topic to publish events to.
	Topic string
	// StartAt is the highest sequence number already published.
	// On bridge restart, pass the last-committed checkpoint to
	// avoid re-sending everything.
	StartAt int64
}

// Bridge tails Quidnug streams and publishes events to Kafka.
type Bridge struct {
	opts Options

	// highWaterMarks tracks the last successfully-published sequence
	// per subject.
	highWaterMarks map[string]int64
}

// New constructs a Bridge.
func New(opts Options) (*Bridge, error) {
	if opts.Client == nil {
		return nil, errors.New("Client is required")
	}
	if opts.Producer == nil {
		return nil, errors.New("Producer is required")
	}
	if len(opts.Subjects) == 0 {
		return nil, errors.New("at least one Subject is required")
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 5 * time.Second
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 5
	}
	if opts.RetryBaseDelay == 0 {
		opts.RetryBaseDelay = time.Second
	}
	if opts.Logger == nil {
		opts.Logger = func(string, ...any) {}
	}

	hwm := make(map[string]int64)
	for _, s := range opts.Subjects {
		hwm[s.SubjectID] = s.StartAt
	}

	return &Bridge{opts: opts, highWaterMarks: hwm}, nil
}

// Run starts the bridge loop. Returns when ctx is cancelled.
func (b *Bridge) Run(ctx context.Context) error {
	ticker := time.NewTicker(b.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for _, subject := range b.opts.Subjects {
				if err := b.syncSubject(ctx, subject); err != nil {
					b.opts.Logger("subject %s sync failed: %v", subject.SubjectID, err)
				}
			}
		}
	}
}

// HighWaterMark returns the last successfully-published sequence for
// the given subject (for checkpointing to persistent storage).
func (b *Bridge) HighWaterMark(subjectID string) int64 {
	return b.highWaterMarks[subjectID]
}

func (b *Bridge) syncSubject(ctx context.Context, subject Subject) error {
	hwm := b.highWaterMarks[subject.SubjectID]

	events, _, err := b.opts.Client.GetStreamEvents(
		ctx, subject.SubjectID, subject.Domain, 100, 0,
	)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}

	// Only publish events with sequence > hwm. Server returns newest-
	// first on most nodes, so sort / iterate accordingly.
	for _, ev := range events {
		if ev.Sequence <= hwm {
			continue
		}
		if err := b.publish(ctx, subject, ev); err != nil {
			return fmt.Errorf("publish seq %d: %w", ev.Sequence, err)
		}
		if ev.Sequence > b.highWaterMarks[subject.SubjectID] {
			b.highWaterMarks[subject.SubjectID] = ev.Sequence
		}
	}
	return nil
}

func (b *Bridge) publish(ctx context.Context, subject Subject, ev client.Event) error {
	value, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	headers := map[string]string{
		"quidnug-domain":       subject.Domain,
		"quidnug-event-type":   ev.EventType,
		"quidnug-sequence":     strconv.FormatInt(ev.Sequence, 10),
		"quidnug-subject-type": ev.SubjectType,
	}

	backoff := b.opts.RetryBaseDelay
	for attempt := 0; attempt < b.opts.MaxRetries; attempt++ {
		err = b.opts.Producer.Publish(
			ctx,
			subject.Topic,
			[]byte(ev.SubjectID),
			value,
			headers,
		)
		if err == nil {
			return nil
		}
		b.opts.Logger(
			"kafka publish failed (attempt %d/%d): %v",
			attempt+1, b.opts.MaxRetries, err,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}
	return fmt.Errorf("kafka publish exhausted retries: %w", err)
}

// Close releases the underlying producer.
func (b *Bridge) Close() error {
	return b.opts.Producer.Close()
}
