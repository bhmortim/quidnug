package core

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	iop "github.com/prometheus/client_model/go"
)

// counterVecValue returns the current count for a labelled counter, or -1
// if the label combination has no observations yet.
func counterVecValue(t *testing.T, cv *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	c, err := cv.GetMetricWithLabelValues(labels...)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues: %v", err)
	}
	m := &iop.Metric{}
	if err := c.Write(m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Counter == nil {
		return -1
	}
	return m.Counter.GetValue()
}

func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	m := &iop.Metric{}
	if err := g.Write(m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Gauge == nil {
		return -1
	}
	return m.Gauge.GetValue()
}

func TestRecordBlockGenerated_Increments(t *testing.T) {
	before := counterVecValue(t, blocksTotal, "test-gen", "generated")
	RecordBlockGenerated("test-gen")
	RecordBlockGenerated("test-gen")
	after := counterVecValue(t, blocksTotal, "test-gen", "generated")
	if after-before != 2 {
		t.Fatalf("expected counter to advance by 2, got %v -> %v", before, after)
	}
}

func TestRecordBlockReceived_AllAcceptanceTiers(t *testing.T) {
	cases := []struct {
		name   string
		accept BlockAcceptance
		status string
	}{
		{"trusted", BlockTrusted, "received_trusted"},
		{"tentative", BlockTentative, "received_tentative"},
		{"untrusted", BlockUntrusted, "received_untrusted"},
		{"invalid", BlockInvalid, "rejected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			domain := "test-recv-" + tc.name
			before := counterVecValue(t, blocksTotal, domain, tc.status)
			RecordBlockReceived(domain, tc.accept)
			after := counterVecValue(t, blocksTotal, domain, tc.status)
			if after-before != 1 {
				t.Fatalf("expected +1 on %s tier, got %v -> %v", tc.name, before, after)
			}
		})
	}
}

func TestRecordTransactionProcessed_AcceptedAndRejected(t *testing.T) {
	beforeOK := counterVecValue(t, transactionsTotal, "TRUST", "accepted")
	beforeBad := counterVecValue(t, transactionsTotal, "TRUST", "rejected")
	RecordTransactionProcessed("TRUST", true)
	RecordTransactionProcessed("TRUST", false)
	afterOK := counterVecValue(t, transactionsTotal, "TRUST", "accepted")
	afterBad := counterVecValue(t, transactionsTotal, "TRUST", "rejected")
	if afterOK-beforeOK != 1 || afterBad-beforeBad != 1 {
		t.Fatalf("expected +1 in each bucket, got ok:%v->%v bad:%v->%v",
			beforeOK, afterOK, beforeBad, afterBad)
	}
}

func TestGauges_SetAndReadBack(t *testing.T) {
	UpdatePendingTransactionsGauge(42)
	if got := gaugeValue(t, pendingTransactionsGauge); got != 42 {
		t.Fatalf("pendingTransactionsGauge: want 42, got %v", got)
	}
	UpdateConnectedNodesGauge(7)
	if got := gaugeValue(t, connectedNodesGauge); got != 7 {
		t.Fatalf("connectedNodesGauge: want 7, got %v", got)
	}
}
