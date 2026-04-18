// Package core — quarantine_test.go
//
// Methodology
// -----------
// Lazy epoch propagation (QDP-0007 / H4) adds a quarantine
// queue and asynchronous probe path to transaction admission.
// These tests guard:
//
//   - The quarantine structure itself: enqueue/evict/release
//     logic, overflow behavior (oldest evicted, never newest),
//     age-out sweep.
//
//   - The recency decision: a signer whose lastEpochRefresh is
//     within window is admitted directly; outside window is
//     quarantined; never-refreshed known signer is stale.
//
//   - Admission-wrapper integration: with the flag off,
//     QuarantineOrAdmit is a no-op. With the flag on it
//     quarantines stale signers and returns Held.
//
//   - The probe orchestrator: successful probe releases the
//     signer's queued txs back into PendingTxs.
//
// We deliberately do NOT test the HTTP probe client here (covered
// in integration tests against httptest servers). Unit tests
// focus on the in-process decision logic.
package core

import (
	"testing"
	"time"
)

// ----- Quarantine state structural tests -----------------------------------

// Enqueue a tx; size goes to 1; release by signer returns it;
// size goes to 0. Baseline sanity.
func TestQuarantine_EnqueueReleaseCycle(t *testing.T) {
	q := newQuarantineState()
	qt := QuarantinedTx{
		Tx:         "payload-1",
		TxHash:     "h1",
		EnqueuedAt: time.Now(),
		Signer:     "signer-A",
	}
	if inserted, _ := q.enqueue(qt); !inserted {
		t.Fatal("first enqueue should insert")
	}
	if got := q.size(); got != 1 {
		t.Fatalf("size after enqueue: want 1, got %d", got)
	}

	released := q.releaseSigner("signer-A")
	if len(released) != 1 || released[0].TxHash != "h1" {
		t.Fatalf("releaseSigner returned %+v", released)
	}
	if got := q.size(); got != 0 {
		t.Fatalf("size after release: want 0, got %d", got)
	}
}

// Duplicate enqueue (same hash) is a no-op, not a new slot.
func TestQuarantine_DedupesOnHash(t *testing.T) {
	q := newQuarantineState()
	qt := QuarantinedTx{TxHash: "h1", Signer: "A", EnqueuedAt: time.Now()}
	q.enqueue(qt)
	inserted, _ := q.enqueue(qt)
	if inserted {
		t.Fatal("duplicate enqueue should return inserted=false")
	}
	if got := q.size(); got != 1 {
		t.Fatalf("size: want 1 after dedup, got %d", got)
	}
}

// Overflow evicts the OLDEST entry, never the newest. Critical
// anti-flood property: an attacker enqueuing N+1 valid stale
// txs can't evict legitimate earlier ones while preserving
// their own.
//
// Actually that's the OPPOSITE — we want the attacker's newer
// txs to NOT evict legitimate older ones. Let me re-check the
// QDP-0007 §5.2 design note...
//
// Re-reading: "overflow drops OLDEST" because the flood's
// OWN txs become the oldest once they pile up, so the defense
// works if the legitimate txs arrive first. In practice a
// flooder starting fresh would see their own early txs
// dropped as their later ones arrive. Either way: test the
// documented behavior.
func TestQuarantine_OverflowDropsOldest(t *testing.T) {
	q := newQuarantineState()
	q.maxSize = 3

	// Enqueue 4 entries with monotonically increasing EnqueuedAt.
	base := time.Unix(1_000_000, 0)
	for i := 0; i < 4; i++ {
		q.enqueue(QuarantinedTx{
			TxHash:     string(rune('a' + i)),
			Signer:     "s",
			EnqueuedAt: base.Add(time.Duration(i) * time.Second),
		})
	}

	// Size should be at cap.
	if got := q.size(); got != 3 {
		t.Fatalf("size at cap: want 3, got %d", got)
	}
	// 'a' (oldest) should be gone; 'b','c','d' should remain.
	released := q.releaseSigner("s")
	got := map[string]bool{}
	for _, r := range released {
		got[r.TxHash] = true
	}
	if got["a"] {
		t.Fatalf("oldest entry should have been evicted; got %v", got)
	}
	if !got["b"] || !got["c"] || !got["d"] {
		t.Fatalf("newer entries should be retained; got %v", got)
	}
}

// Age-out sweep drops entries older than maxAge and leaves
// younger ones.
func TestQuarantine_AgesOutStaleEntries(t *testing.T) {
	q := newQuarantineState()
	q.maxAge = 1 * time.Hour

	now := time.Now()
	q.enqueue(QuarantinedTx{TxHash: "old", Signer: "s", EnqueuedAt: now.Add(-2 * time.Hour)})
	q.enqueue(QuarantinedTx{TxHash: "fresh", Signer: "s", EnqueuedAt: now})

	dropped := q.releaseAged(now)
	if len(dropped) != 1 || dropped[0].Tx == nil && dropped[0].Signer != "s" {
		t.Fatalf("expected one dropped entry, got %d", len(dropped))
	}
	// 'fresh' should still be there.
	if got := q.size(); got != 1 {
		t.Fatalf("size after age-out: want 1, got %d", got)
	}
}

// releaseSigner drops entries even if the signer has multiple
// queued txs.
func TestQuarantine_MultipleTxsPerSigner(t *testing.T) {
	q := newQuarantineState()
	q.enqueue(QuarantinedTx{TxHash: "a", Signer: "s", EnqueuedAt: time.Now()})
	q.enqueue(QuarantinedTx{TxHash: "b", Signer: "s", EnqueuedAt: time.Now()})
	q.enqueue(QuarantinedTx{TxHash: "c", Signer: "other", EnqueuedAt: time.Now()})

	released := q.releaseSigner("s")
	if len(released) != 2 {
		t.Fatalf("want 2 released for s, got %d", len(released))
	}
	if got := q.size(); got != 1 {
		t.Fatalf("one entry for 'other' should remain; got size %d", got)
	}
}

// ----- Recency decision ----------------------------------------------------

// A never-refreshed known signer is stale — EpochRecent returns
// false.
func TestEpochRecent_NeverRefreshedIsStale(t *testing.T) {
	l := NewNonceLedger()
	l.SetSignerKey("signer-A", 0, "pubkey-hex")

	if l.EpochRecent("signer-A", time.Hour, time.Now()) {
		t.Fatal("never-refreshed signer with known key should be stale")
	}
}

// A signer with no key in the ledger returns recent=true (no
// state to quarantine against).
func TestEpochRecent_UnknownSignerIsNotStale(t *testing.T) {
	l := NewNonceLedger()
	if !l.EpochRecent("unknown", time.Hour, time.Now()) {
		t.Fatal("unknown signer should pass recency (nothing to compare)")
	}
}

// MarkEpochRefresh then a check within window returns recent.
// Outside window returns stale.
func TestEpochRecent_MarkAndExpire(t *testing.T) {
	l := NewNonceLedger()
	l.SetSignerKey("s", 0, "k")
	l.MarkEpochRefresh("s", time.Unix(1_000_000, 0))

	// Within window.
	if !l.EpochRecent("s", time.Hour, time.Unix(1_000_000+1800, 0)) {
		t.Fatal("should be recent within window")
	}
	// Outside window.
	if l.EpochRecent("s", time.Hour, time.Unix(1_000_000+4000, 0)) {
		t.Fatal("should be stale outside window")
	}
}

// ----- QuarantineOrAdmit admission wrapper ---------------------------------

// Flag off: always admits.
func TestQuarantineOrAdmit_FlagOffAlwaysAdmits(t *testing.T) {
	node := newTestNode()
	node.LazyEpochProbeEnabled = false
	node.NonceLedger.SetSignerKey("s", 0, "k")
	// No MarkEpochRefresh → would be stale if flag were on.

	got := node.QuarantineOrAdmit("payload", "s")
	if got != QuarantineAdmitted {
		t.Fatalf("flag-off should admit; got %v", got)
	}
}

// Flag on + signer recent: admits.
func TestQuarantineOrAdmit_RecentSignerAdmits(t *testing.T) {
	node := newTestNode()
	node.LazyEpochProbeEnabled = true
	node.EpochRecencyWindow = time.Hour
	node.NonceLedger.SetSignerKey("s", 0, "k")
	node.NonceLedger.MarkEpochRefresh("s", time.Now())

	got := node.QuarantineOrAdmit("payload", "s")
	if got != QuarantineAdmitted {
		t.Fatalf("recent signer should admit; got %v", got)
	}
}

// Flag on + stale signer: held.
func TestQuarantineOrAdmit_StaleSignerHeld(t *testing.T) {
	node := newTestNode()
	node.LazyEpochProbeEnabled = true
	node.EpochRecencyWindow = time.Hour
	node.NonceLedger.SetSignerKey("s", 0, "k")
	// Never refreshed → stale.

	got := node.QuarantineOrAdmit("payload-a", "s")
	if got != QuarantineHeld {
		t.Fatalf("stale signer should be held; got %v", got)
	}
	if node.quarantine.size() != 1 {
		t.Fatalf("quarantine should have 1 entry, got %d", node.quarantine.size())
	}
}

// ----- Release integration -------------------------------------------------

// When an anchor applies for a signer, MarkEpochRefresh fires
// and any quarantined txs for that signer are released into
// PendingTxs.
func TestRelease_AnchorApplyReleasesQuarantined(t *testing.T) {
	node := newTestNode()
	node.LazyEpochProbeEnabled = true
	node.EpochRecencyWindow = time.Hour

	// Seed signer as stale.
	node.NonceLedger.SetSignerKey("s", 0, node.GetPublicKeyHex())

	// Quarantine a tx for s.
	if got := node.QuarantineOrAdmit("tx-a", "s"); got != QuarantineHeld {
		t.Fatalf("expected Held, got %v", got)
	}

	// Manually release via the refresh hook (simulates what
	// applyAnchorFromBlock would do).
	node.NonceLedger.MarkEpochRefresh("s", time.Now())
	node.releaseQuarantinedForSigner("s", "anchor_applied")

	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()
	found := false
	for _, tx := range node.PendingTxs {
		if s, ok := tx.(string); ok && s == "tx-a" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("quarantined tx should have been released into PendingTxs")
	}
	if node.quarantine.size() != 0 {
		t.Fatalf("quarantine should be empty after release; size=%d", node.quarantine.size())
	}
}

// ----- Home domain lookup --------------------------------------------------

// homeDomainFor returns the signer's HomeDomain from the
// identity registry when present, empty otherwise.
func TestHomeDomainFor(t *testing.T) {
	node := newTestNode()
	node.IdentityRegistry["s"] = IdentityTransaction{
		BaseTransaction: BaseTransaction{Type: TxTypeIdentity},
		QuidID:          "s",
		HomeDomain:      "home.example",
	}

	if got := node.homeDomainFor("s"); got != "home.example" {
		t.Fatalf("want home.example, got %q", got)
	}
	if got := node.homeDomainFor("unknown"); got != "" {
		t.Fatalf("unknown signer should return empty home, got %q", got)
	}
}

// defaultHomeDomain picks the first supported domain.
func TestDefaultHomeDomain(t *testing.T) {
	node := newTestNode()
	node.SupportedDomains = []string{"primary.example", "secondary.example"}
	if got := node.defaultHomeDomain(); got != "primary.example" {
		t.Fatalf("want primary.example, got %q", got)
	}
	node.SupportedDomains = nil
	if got := node.defaultHomeDomain(); got != "genesis" {
		t.Fatalf("want genesis fallback, got %q", got)
	}
}

// ----- Probe failure path (policy = reject) --------------------------------

// With no peers known and policy=reject, the tx stays
// quarantined after a synchronous probe attempt. Synchronous
// call avoids a goroutine race — QuarantineOrAdmit spawns the
// probe async, which creates test flakiness under load; calling
// runProbeForQuarantine directly exercises the same code path
// deterministically.
func TestProbe_NoPeersPolicyRejectStaysQuarantined(t *testing.T) {
	node := newTestNode()
	node.LazyEpochProbeEnabled = true
	node.EpochRecencyWindow = time.Hour
	node.ProbeTimeoutPolicy = ProbeTimeoutPolicyReject
	node.NonceLedger.SetSignerKey("s", 0, node.GetPublicKeyHex())

	// Enqueue directly to avoid the async probe from
	// QuarantineOrAdmit.
	qt := QuarantinedTx{
		Tx:         "tx-a",
		TxHash:     "h-a",
		EnqueuedAt: time.Now(),
		Signer:     "s",
	}
	node.quarantine.enqueue(qt)

	// Run the probe synchronously. No peers → failure.
	node.runProbeForQuarantine("s", "")

	if node.quarantine.size() != 1 {
		t.Fatalf("under reject policy, failed probe should leave tx in quarantine; size=%d", node.quarantine.size())
	}
}

// With no peers known and policy=admit_warn, the tx is
// released despite probe failure. Synchronous variant for
// deterministic test behavior.
func TestProbe_NoPeersPolicyAdmitWarnReleases(t *testing.T) {
	node := newTestNode()
	node.LazyEpochProbeEnabled = true
	node.EpochRecencyWindow = time.Hour
	node.ProbeTimeoutPolicy = ProbeTimeoutPolicyAdmitWarn
	node.NonceLedger.SetSignerKey("s", 0, node.GetPublicKeyHex())

	qt := QuarantinedTx{
		Tx:         "tx-a",
		TxHash:     "h-a",
		EnqueuedAt: time.Now(),
		Signer:     "s",
	}
	node.quarantine.enqueue(qt)

	node.runProbeForQuarantine("s", "")

	if node.quarantine.size() != 0 {
		t.Fatalf("under admit_warn policy, failed probe should release; size=%d", node.quarantine.size())
	}
	node.PendingTxsMutex.RLock()
	defer node.PendingTxsMutex.RUnlock()
	found := false
	for _, tx := range node.PendingTxs {
		if s, ok := tx.(string); ok && s == "tx-a" {
			found = true
		}
	}
	if !found {
		t.Fatal("admit_warn should release tx into PendingTxs")
	}
}
