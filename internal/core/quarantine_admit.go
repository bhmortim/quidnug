// Package core — quarantine_admit.go
//
// Admission wrapper + probe orchestration for QDP-0007 / H4.
// Ties the quarantine state (quarantine.go), the probe client
// (epoch_probe.go), and the existing transaction admission
// pipeline together.
//
// The public entry point is QuarantineOrAdmit. It decides at
// admission time whether a transaction needs to be quarantined
// and fires the probe asynchronously if so. The quarantine
// orchestrator is also responsible for periodic age-out and
// for re-admitting quarantined txs when probes succeed.
package core

import (
	"context"
	"time"
)

// QuarantineResult indicates what the admission wrapper decided.
type QuarantineResult int

const (
	// QuarantineAdmitted — signer is recent or quarantine
	// disabled; caller should proceed with normal admission.
	QuarantineAdmitted QuarantineResult = iota

	// QuarantineHeld — tx has been enqueued; will be released
	// on probe success or aged out. Caller should NOT proceed
	// with normal admission.
	QuarantineHeld

	// QuarantineRejected — tx was not enqueued because the
	// probe-timeout policy is "reject" and the probe failed
	// synchronously (no peers known, etc.). Caller should
	// surface a rejection error to the client.
	QuarantineRejected

	// QuarantineDropped — tx was admitted despite probe
	// failure because policy is admit_warn. Caller proceeds
	// but should log / emit a metric.
	QuarantineAdmittedWithWarn
)

// QuarantineOrAdmit is the per-transaction gate. Call it before
// the normal nonce-ledger admission check. If it returns
// QuarantineAdmitted, proceed. Other return values handle their
// own flow.
//
// The signer/homeDomain should be extracted by the caller from
// the transaction type. We pass them explicitly rather than
// inferring from the tx because different tx types carry them
// in different fields (TrustTx.From vs IdentityTx.QuidID etc.).
func (node *QuidnugNode) QuarantineOrAdmit(tx interface{}, signer string) QuarantineResult {
	if !node.LazyEpochProbeEnabled {
		return QuarantineAdmitted
	}
	window := node.EpochRecencyWindow
	if window <= 0 {
		window = DefaultEpochRecencyWindow
	}
	if node.NonceLedger.EpochRecent(signer, window, time.Now()) {
		return QuarantineAdmitted
	}

	// Look up home domain from identity registry (if known).
	homeDomain := node.homeDomainFor(signer)

	qtx := QuarantinedTx{
		Tx:         tx,
		TxHash:     quarantineTxHash(tx),
		EnqueuedAt: time.Now(),
		Signer:     signer,
		HomeDomain: homeDomain,
	}
	inserted, evicted := node.quarantine.enqueue(qtx)
	if inserted {
		quarantineEnqueuedTotal.WithLabelValues("stale_epoch").Inc()
		quarantineSizeGauge.Set(float64(node.quarantine.size()))
	}
	if evicted != nil {
		quarantineRejectedTotal.WithLabelValues("overflow").Inc()
	}

	// Fire probe asynchronously. The returned result is
	// QuarantineHeld in all cases — the probe's completion
	// happens later and is surfaced via release hooks.
	go node.runProbeForQuarantine(signer, homeDomain)

	return QuarantineHeld
}

// homeDomainFor reads the signer's HomeDomain field from the
// identity registry. Returns empty if not known.
func (node *QuidnugNode) homeDomainFor(signer string) string {
	node.IdentityRegistryMutex.RLock()
	defer node.IdentityRegistryMutex.RUnlock()
	if id, ok := node.IdentityRegistry[signer]; ok {
		return id.HomeDomain
	}
	return ""
}

// runProbeForQuarantine fires the probe and, on success,
// releases the signer's quarantined txs for re-admission.
// Goroutine-safe; called via `go`.
func (node *QuidnugNode) runProbeForQuarantine(signer, homeDomain string) {
	timeout := node.EpochProbeTimeout
	if timeout <= 0 {
		timeout = DefaultEpochProbeTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	probeAttemptsTotal.Inc()
	_, err := node.ProbeHomeDomain(ctx, signer, homeDomain)
	if err != nil {
		probeFailureTotal.WithLabelValues(probeErrorReason(err)).Inc()
		// Policy decides whether to release or hold.
		if node.ProbeTimeoutPolicy == ProbeTimeoutPolicyAdmitWarn {
			node.releaseQuarantinedWithWarn(signer)
		}
		// Default (reject / empty): leave in quarantine; age-out
		// sweep will drop eventually.
		return
	}
	probeSuccessTotal.Inc()
	node.releaseQuarantinedForSigner(signer, "probe")
}

// releaseQuarantinedForSigner pulls all quarantined txs for the
// signer and pushes them back into the pending-tx pool for
// normal admission. Called when the signer's recency is
// refreshed (probe success or push gossip arrival).
func (node *QuidnugNode) releaseQuarantinedForSigner(signer, trigger string) {
	txs := node.quarantine.releaseSigner(signer)
	if len(txs) == 0 {
		return
	}
	quarantineReleasedTotal.WithLabelValues(trigger).Add(float64(len(txs)))
	quarantineSizeGauge.Set(float64(node.quarantine.size()))

	node.PendingTxsMutex.Lock()
	for _, qt := range txs {
		node.PendingTxs = append(node.PendingTxs, qt.Tx)
	}
	node.PendingTxsMutex.Unlock()
}

// releaseQuarantinedWithWarn is the admit_warn fallback — emit
// a metric + log for each released tx.
func (node *QuidnugNode) releaseQuarantinedWithWarn(signer string) {
	txs := node.quarantine.releaseSigner(signer)
	if len(txs) == 0 {
		return
	}
	quarantineReleasedTotal.WithLabelValues("probe_timeout_warn").Add(float64(len(txs)))
	quarantineSizeGauge.Set(float64(node.quarantine.size()))
	for _, qt := range txs {
		logger.Warn("Admitting stale-signer tx under admit_warn policy",
			"signer", qt.Signer, "home", qt.HomeDomain, "ageSeconds", time.Since(qt.EnqueuedAt).Seconds())
	}
	node.PendingTxsMutex.Lock()
	for _, qt := range txs {
		node.PendingTxs = append(node.PendingTxs, qt.Tx)
	}
	node.PendingTxsMutex.Unlock()
}

// runQuarantineAging is the periodic sweep. Starts a ticker that
// drops entries older than QuarantineMaxAge. Should be spawned
// from the node's Run() loop when LazyEpochProbeEnabled is true.
func (node *QuidnugNode) runQuarantineAging(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dropped := node.quarantine.releaseAged(time.Now())
			if len(dropped) > 0 {
				for _, d := range dropped {
					quarantineRejectedTotal.WithLabelValues(d.Reason).Inc()
				}
				quarantineSizeGauge.Set(float64(node.quarantine.size()))
			}
		}
	}
}

// probeErrorReason maps common probe errors to bounded-cardinal
// reason labels.
func probeErrorReason(err error) string {
	switch err {
	case ErrProbeNoPeers:
		return "no_peers"
	case ErrProbeAllFailed:
		return "all_failed"
	default:
		return "other"
	}
}
