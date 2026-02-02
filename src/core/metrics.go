package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Block metrics
	blocksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_blocks_total",
		Help: "Total number of blocks processed",
	}, []string{"domain", "status"})

	// Transaction metrics
	transactionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_transactions_total",
		Help: "Total number of transactions processed",
	}, []string{"type", "status"})

	// Trust computation metrics
	trustComputationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "quidnug_trust_computation_duration_seconds",
		Help:    "Duration of trust computation operations",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
	})

	// Gauge metrics
	pendingTransactionsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "quidnug_pending_transactions",
		Help: "Current number of pending transactions",
	})

	connectedNodesGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "quidnug_connected_nodes",
		Help: "Current number of connected nodes",
	})

	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "quidnug_http_request_duration_seconds",
		Help:    "Duration of HTTP requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// RecordBlockGenerated records a block generation event
func RecordBlockGenerated(domain string) {
	blocksTotal.WithLabelValues(domain, "generated").Inc()
}

// RecordBlockReceived records a block reception event with its acceptance status
func RecordBlockReceived(domain string, acceptance BlockAcceptance) {
	var status string
	switch acceptance {
	case BlockTrusted:
		status = "received_trusted"
	case BlockTentative:
		status = "received_tentative"
	case BlockUntrusted:
		status = "received_untrusted"
	case BlockInvalid:
		status = "rejected"
	}
	blocksTotal.WithLabelValues(domain, status).Inc()
}

// RecordTransactionProcessed records a transaction processing event
func RecordTransactionProcessed(txType string, accepted bool) {
	status := "accepted"
	if !accepted {
		status = "rejected"
	}
	transactionsTotal.WithLabelValues(txType, status).Inc()
}

// UpdatePendingTransactionsGauge updates the pending transactions gauge
func UpdatePendingTransactionsGauge(count int) {
	pendingTransactionsGauge.Set(float64(count))
}

// UpdateConnectedNodesGauge updates the connected nodes gauge
func UpdateConnectedNodesGauge(count int) {
	connectedNodesGauge.Set(float64(count))
}
