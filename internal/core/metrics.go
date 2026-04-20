package core

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

	// QDP-0001 observability (see design doc §14.3).
	// Counted regardless of whether enforcement is enabled so operators
	// can validate correctness in shadow mode before flipping the flag.
	nonceReplayRejections = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_nonce_replay_rejections_total",
		Help: "Transactions rejected by the nonce ledger, labelled by reason and whether enforcement was active.",
	}, []string{"reason", "enforced"})

	nonceLedgerEntries = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "quidnug_nonce_ledger_entries",
		Help: "Number of (signer, domain, epoch) keys tracked in the nonce ledger, by tier.",
	}, []string{"tier"})

	// QDP-0006 guardian resignation (H6). Counter per subject so
	// operator dashboards can spot when a specific high-value
	// quid's set is being dismantled.
	guardianResignationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_guardian_resignations_total",
		Help: "Accepted guardian resignations, by subject quid.",
	}, []string{"subject"})

	guardianResignationsRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_guardian_resignations_rejected_total",
		Help: "Rejected guardian resignations, by validation reason.",
	}, []string{"reason"})

	guardianSetWeakened = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_guardian_set_weakened_total",
		Help: "Guardian sets whose effective weight dropped below threshold after a resignation.",
	}, []string{"subject"})

	// QDP-0007 lazy epoch propagation (H4).
	quarantineSizeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "quidnug_quarantine_size",
		Help: "Current number of transactions held in the stale-epoch quarantine.",
	})
	quarantineEnqueuedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_quarantine_enqueued_total",
		Help: "Transactions enqueued to quarantine, by reason.",
	}, []string{"reason"})
	quarantineReleasedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_quarantine_released_total",
		Help: "Transactions released from quarantine, by trigger.",
	}, []string{"trigger"})
	quarantineRejectedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_quarantine_rejected_total",
		Help: "Transactions dropped from quarantine without admission, by reason.",
	}, []string{"reason"})
	probeAttemptsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "quidnug_probe_attempts_total",
		Help: "Epoch-refresh probe attempts.",
	})
	probeSuccessTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "quidnug_probe_success_total",
		Help: "Successful epoch-refresh probes.",
	})
	probeFailureTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_probe_failure_total",
		Help: "Failed epoch-refresh probes, by reason.",
	}, []string{"reason"})

	// QDP-0009 fork-block migration trigger (H5).
	forkBlockAcceptedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_fork_block_accepted_total",
		Help: "Fork-block transactions accepted and queued for activation.",
	}, []string{"domain", "feature"})
	forkBlockRejectedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_fork_block_rejected_total",
		Help: "Fork-block transactions rejected, by validation reason.",
	}, []string{"reason"})
	forkBlockActivatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_fork_block_activated_total",
		Help: "Fork-block activations at ForkHeight.",
	}, []string{"domain", "feature"})

	// QDP-0010 / H2 compact Merkle proofs.
	merkleProofUsedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "quidnug_merkle_proof_used_total",
		Help: "Gossip messages verified via Merkle proof rather than full-block walk.",
	})
	merkleProofFallbackTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quidnug_merkle_proof_fallback_total",
		Help: "Gossip messages falling back to full-block verification, by reason.",
	}, []string{"reason"})
	blockMissingTxRootRejectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "quidnug_block_missing_tx_root_rejected_total",
		Help: "Blocks rejected post-fork for empty TransactionsRoot.",
	})
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

// rateLimitDenials is the QDP-0016 Phase 1 counter for writes
// rejected at the mempool-admission layer. Labelled by which
// layer raised the rejection so operators can see which part
// of the multi-layer stack is carrying load.
var rateLimitDenials = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "quidnug_ratelimit_denials_total",
	Help: "Writes rejected at mempool admission by the QDP-0016 multi-layer rate limiter, by layer.",
}, []string{"layer"})

// RecordRateLimitDenial increments the per-layer denial counter.
func RecordRateLimitDenial(layer string) {
	rateLimitDenials.WithLabelValues(layer).Inc()
}

// UpdateConnectedNodesGauge updates the connected nodes gauge
func UpdateConnectedNodesGauge(count int) {
	connectedNodesGauge.Set(float64(count))
}
