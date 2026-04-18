// Package core — gossip_push_metrics.go
//
// Prometheus metrics for push-based gossip (QDP-0005 §7 / §12).
// Separate from the generic metrics file because these are
// specific to the H1 feature and make it easier to ship / omit
// a Grafana dashboard per subphase.
package core

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// gossipPushReceivedTotal counts every push message that
	// reached the receive handler, by kind and disposition.
	// Disposition "applied" means the message caused local
	// state change. "dup" means dedup fired. "dropped" means
	// the message was rejected for any other reason.
	gossipPushReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_gossip_push_received_total",
		Help: "Push gossip messages received, by kind (anchor|fingerprint) and disposition (applied|dup|dropped).",
	}, []string{"kind", "disposition"})

	// gossipPushForwardDropped counts forward-stage drops by
	// reason. Reasons are the DropReason* constants.
	gossipPushForwardDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_gossip_push_forward_dropped_total",
		Help: "Push gossip messages that were not forwarded, by kind and reason.",
	}, []string{"kind", "reason"})

	// gossipPushRateLimited counts rate-limit events, labelled
	// by producer QUID. Used to spot a single producer flooding.
	// Producer-labelled metrics can cardinality-explode; we
	// accept that because the set of gossip producers is
	// bounded by the set of validators in the network.
	gossipPushRateLimited = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_gossip_push_rate_limited_total",
		Help: "Producer rate-limit events, by producer quid.",
	}, []string{"producer"})

	// gossipPushPropagationLatency captures the observed
	// producer→receiver latency. Bucketed for the expected
	// latency range (sub-second to tens of minutes).
	gossipPushPropagationLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "quidnug_gossip_push_propagation_latency_seconds",
		Help:    "Latency between producer timestamp and local apply for push gossip.",
		Buckets: []float64{0.1, 0.5, 1, 5, 15, 60, 300, 900, 1800},
	}, []string{"kind"})
)

// recordGossipApplied increments the applied counter and should
// be called exactly once per successfully applied message.
func recordGossipApplied(kind string) {
	gossipPushReceivedTotal.WithLabelValues(kind, "applied").Inc()
}

// recordGossipDrop increments the drop counter. A single message
// should produce exactly one disposition increment — either
// "applied", "dup", or "dropped" — so the three counters sum to
// total receives. For dropped messages we additionally increment
// the forward-dropped breakdown.
func recordGossipDrop(kind, reason string) {
	if reason == DropReasonDuplicate {
		gossipPushReceivedTotal.WithLabelValues(kind, "dup").Inc()
		return
	}
	gossipPushReceivedTotal.WithLabelValues(kind, "dropped").Inc()
	gossipPushForwardDropped.WithLabelValues(kind, reason).Inc()
}

// recordGossipRateLimited fires when a producer has exhausted
// their token bucket for the current window.
func recordGossipRateLimited(producer string) {
	gossipPushRateLimited.WithLabelValues(producer).Inc()
}

// observeGossipLatency records producer-to-receiver latency based
// on the message's producer-set Timestamp field. Clock skew can
// make this negative; we clamp at 0 to avoid confusing the
// histogram.
func observeGossipLatency(kind string, producerTs int64) {
	if producerTs <= 0 {
		return
	}
	delta := time.Now().Unix() - producerTs
	if delta < 0 {
		delta = 0
	}
	gossipPushPropagationLatency.WithLabelValues(kind).Observe(float64(delta))
}
