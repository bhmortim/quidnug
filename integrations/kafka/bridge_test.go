package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

type fakeProducer struct {
	mu       sync.Mutex
	messages []struct {
		Topic   string
		Key     string
		Value   string
		Headers map[string]string
	}
	failN int // fail the first N publishes
}

func (f *fakeProducer) Publish(ctx context.Context, topic string, key []byte, value []byte, headers map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failN > 0 {
		f.failN--
		return errors.New("simulated kafka failure")
	}
	f.messages = append(f.messages, struct {
		Topic   string
		Key     string
		Value   string
		Headers map[string]string
	}{topic, string(key), string(value), headers})
	return nil
}

func (f *fakeProducer) Close() error { return nil }

func mockNode(t *testing.T, events []client.Event) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"data": events,
			},
		})
	}))
	return srv
}

func TestBridgePublishesEventsOnce(t *testing.T) {
	events := []client.Event{
		{SubjectID: "s1", SubjectType: "QUID", EventType: "LOGIN", Sequence: 1, Timestamp: 1000},
		{SubjectID: "s1", SubjectType: "QUID", EventType: "LOGIN", Sequence: 2, Timestamp: 1001},
		{SubjectID: "s1", SubjectType: "QUID", EventType: "LOGIN", Sequence: 3, Timestamp: 1002},
	}
	srv := mockNode(t, events)
	defer srv.Close()

	c, _ := client.New(srv.URL, client.WithMaxRetries(0))
	prod := &fakeProducer{}

	b, err := New(Options{
		Client:   c,
		Producer: prod,
		Subjects: []Subject{
			{SubjectID: "s1", Domain: "demo", Topic: "quidnug.events", StartAt: 0},
		},
		PollInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = b.Run(ctx)

	prod.mu.Lock()
	defer prod.mu.Unlock()

	if len(prod.messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(prod.messages))
	}
	for i, m := range prod.messages {
		if m.Topic != "quidnug.events" {
			t.Errorf("msg %d: topic %q", i, m.Topic)
		}
		if m.Key != "s1" {
			t.Errorf("msg %d: key %q", i, m.Key)
		}
		if m.Headers["quidnug-event-type"] != "LOGIN" {
			t.Errorf("msg %d: event-type header %q", i, m.Headers["quidnug-event-type"])
		}
	}
	if b.HighWaterMark("s1") != 3 {
		t.Errorf("hwm: got %d, want 3", b.HighWaterMark("s1"))
	}
}

func TestBridgeSkipsAlreadyPublished(t *testing.T) {
	events := []client.Event{
		{SubjectID: "s2", SubjectType: "QUID", EventType: "x", Sequence: 5},
		{SubjectID: "s2", SubjectType: "QUID", EventType: "y", Sequence: 6},
	}
	srv := mockNode(t, events)
	defer srv.Close()

	c, _ := client.New(srv.URL, client.WithMaxRetries(0))
	prod := &fakeProducer{}

	b, _ := New(Options{
		Client:   c,
		Producer: prod,
		Subjects: []Subject{
			{SubjectID: "s2", Domain: "demo", Topic: "t", StartAt: 5},
		},
		PollInterval: 50 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = b.Run(ctx)

	prod.mu.Lock()
	defer prod.mu.Unlock()
	if len(prod.messages) != 1 {
		t.Fatalf("expected 1 message (sequence 6 only), got %d: %v", len(prod.messages), prod.messages)
	}
}

func TestBridgeRetriesOnKafkaFailure(t *testing.T) {
	events := []client.Event{
		{SubjectID: "s3", SubjectType: "QUID", EventType: "x", Sequence: 1},
	}
	srv := mockNode(t, events)
	defer srv.Close()

	c, _ := client.New(srv.URL, client.WithMaxRetries(0))
	prod := &fakeProducer{failN: 2}

	b, _ := New(Options{
		Client:         c,
		Producer:       prod,
		Subjects:       []Subject{{SubjectID: "s3", Domain: "d", Topic: "t"}},
		MaxRetries:     5,
		RetryBaseDelay: 10 * time.Millisecond,
		PollInterval:   20 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() { _ = b.Run(ctx) }()

	// Wait briefly for retry / publish.
	time.Sleep(500 * time.Millisecond)
	cancel()

	prod.mu.Lock()
	defer prod.mu.Unlock()
	if len(prod.messages) != 1 {
		t.Fatalf("expected 1 successful message after retries, got %d", len(prod.messages))
	}
}

func TestValidation(t *testing.T) {
	c, _ := client.New("http://x")
	_, err := New(Options{Client: c, Producer: &fakeProducer{}})
	if err == nil {
		t.Fatal("expected error when Subjects empty")
	}
	_, err = New(Options{Producer: &fakeProducer{}, Subjects: []Subject{{SubjectID: "s"}}})
	if err == nil {
		t.Fatal("expected error when Client nil")
	}
}

func TestLoggerFormatting(t *testing.T) {
	var seen []string
	logger := func(format string, args ...any) {
		seen = append(seen, fmt.Sprintf(format, args...))
	}
	prod := &fakeProducer{failN: 100}
	srv := mockNode(t, []client.Event{
		{SubjectID: "s", SubjectType: "QUID", EventType: "x", Sequence: 1},
	})
	defer srv.Close()
	c, _ := client.New(srv.URL, client.WithMaxRetries(0))
	b, _ := New(Options{
		Client:         c,
		Producer:       prod,
		Subjects:       []Subject{{SubjectID: "s", Domain: "d", Topic: "t"}},
		MaxRetries:     2,
		RetryBaseDelay: 10 * time.Millisecond,
		Logger:         logger,
		PollInterval:   20 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = b.Run(ctx)

	if len(seen) == 0 {
		t.Fatal("expected logger to receive at least one retry warning")
	}
}
