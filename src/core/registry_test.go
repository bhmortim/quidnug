package main

import (
	"fmt"
	"testing"
)

func TestGetDirectTrustees(t *testing.T) {
	node := newTestNode()

	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
		"cccc333333333333": 0.6,
	}

	trustees := node.GetDirectTrustees("aaaa111111111111")

	if len(trustees) != 2 {
		t.Errorf("Expected 2 trustees, got %d", len(trustees))
	}

	if trustees["bbbb222222222222"] != 0.8 {
		t.Errorf("Expected trust 0.8 for bbbb222222222222, got %f", trustees["bbbb222222222222"])
	}

	if trustees["cccc333333333333"] != 0.6 {
		t.Errorf("Expected trust 0.6 for cccc333333333333, got %f", trustees["cccc333333333333"])
	}

	// Test non-existent quid returns empty map
	empty := node.GetDirectTrustees("00000000000000ff")
	if len(empty) != 0 {
		t.Errorf("Expected 0 trustees for non-existent quid, got %d", len(empty))
	}
}

func TestComputeRelationalTrust_SameEntity(t *testing.T) {
	node := newTestNode()

	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "aaaa111111111111", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if trust != 1.0 {
		t.Errorf("Expected trust 1.0 for same entity, got %f", trust)
	}

	if len(path) != 1 || path[0] != "aaaa111111111111" {
		t.Errorf("Expected path [aaaa111111111111], got %v", path)
	}
}

func TestComputeRelationalTrust_DirectTrust(t *testing.T) {
	node := newTestNode()

	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}

	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "bbbb222222222222", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if trust != 0.8 {
		t.Errorf("Expected trust 0.8, got %f", trust)
	}

	if len(path) != 2 || path[0] != "aaaa111111111111" || path[1] != "bbbb222222222222" {
		t.Errorf("Expected path [aaaa111111111111, bbbb222222222222], got %v", path)
	}
}

func TestComputeRelationalTrust_TwoHopWithDecay(t *testing.T) {
	node := newTestNode()

	// A trusts B with 0.8, B trusts C with 0.5
	// Transitive trust A->C should be 0.8 * 0.5 = 0.4
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.5,
	}

	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "cccc333333333333", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := 0.8 * 0.5
	if trust != expected {
		t.Errorf("Expected trust %f (0.8 * 0.5), got %f", expected, trust)
	}

	if len(path) != 3 {
		t.Errorf("Expected path length 3, got %d", len(path))
	}

	if path[0] != "aaaa111111111111" || path[1] != "bbbb222222222222" || path[2] != "cccc333333333333" {
		t.Errorf("Expected path [A, B, C], got %v", path)
	}
}

func TestComputeRelationalTrust_NoPath(t *testing.T) {
	node := newTestNode()

	// A has no trust relationships
	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "bbbb222222222222", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if trust != 0.0 {
		t.Errorf("Expected trust 0.0 for no path, got %f", trust)
	}

	if path != nil && len(path) != 0 {
		t.Errorf("Expected nil or empty path, got %v", path)
	}
}

func TestComputeRelationalTrust_CycleHandling(t *testing.T) {
	node := newTestNode()

	// Create a cycle: A -> B -> C -> A, and B -> D (target)
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.6,
		"dddd444444444444": 0.9,
	}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{
		"aaaa111111111111": 0.7, // cycle back to A
	}

	// Should find A -> B -> D with trust 0.8 * 0.9 = 0.72
	trust, path, err := node.ComputeRelationalTrust("aaaa111111111111", "dddd444444444444", 5)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := 0.8 * 0.9
	if !floatEquals(trust, expected, 0.0001) {
		t.Errorf("Expected trust %f, got %f", expected, trust)
	}

	if len(path) != 3 || path[2] != "dddd444444444444" {
		t.Errorf("Expected path ending in dddd444444444444, got %v", path)
	}
}

func TestComputeRelationalTrust_DepthLimit(t *testing.T) {
	node := newTestNode()

	// Create a chain: A -> B -> C -> D -> E (4 hops)
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{"bbbb222222222222": 0.9}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{"cccc333333333333": 0.9}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{"dddd444444444444": 0.9}
	node.TrustRegistry["dddd444444444444"] = map[string]float64{"eeee555555555555": 0.9}

	// With maxDepth=2, should not reach E (4 hops away)
	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "eeee555555555555", 2)

	if trust != 0.0 {
		t.Errorf("Expected trust 0.0 with maxDepth=2, got %f", trust)
	}

	if path != nil && len(path) != 0 {
		t.Errorf("Expected no path with maxDepth=2, got %v", path)
	}

	// With maxDepth=4, should reach E
	trust, path, _ = node.ComputeRelationalTrust("aaaa111111111111", "eeee555555555555", 4)

	expected := 0.9 * 0.9 * 0.9 * 0.9
	if !floatEquals(trust, expected, 0.0001) {
		t.Errorf("Expected trust %f with maxDepth=4, got %f", expected, trust)
	}

	if len(path) != 5 {
		t.Errorf("Expected path length 5, got %d", len(path))
	}
}

func TestComputeRelationalTrust_DefaultDepth(t *testing.T) {
	node := newTestNode()

	// Create a chain of 5 hops
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{"bbbb222222222222": 0.9}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{"cccc333333333333": 0.9}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{"dddd444444444444": 0.9}
	node.TrustRegistry["dddd444444444444"] = map[string]float64{"eeee555555555555": 0.9}
	node.TrustRegistry["eeee555555555555"] = map[string]float64{"ffff666666666666": 0.9}

	// With maxDepth=0 (default 5), should reach F
	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "ffff666666666666", 0)

	if trust == 0.0 {
		t.Errorf("Expected non-zero trust with default depth, got 0")
	}

	if len(path) != 6 {
		t.Errorf("Expected path length 6, got %d", len(path))
	}
}

func TestComputeRelationalTrust_BestPathSelection(t *testing.T) {
	node := newTestNode()

	// Two paths to target:
	// A -> B -> D: 0.5 * 0.5 = 0.25
	// A -> C -> D: 0.9 * 0.9 = 0.81
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.5,
		"cccc333333333333": 0.9,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"dddd444444444444": 0.5,
	}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{
		"dddd444444444444": 0.9,
	}

	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "dddd444444444444", 5)

	expected := 0.9 * 0.9
	if trust != expected {
		t.Errorf("Expected best trust %f, got %f", expected, trust)
	}

	// Path should go through C for best trust
	if len(path) != 3 || path[1] != "cccc333333333333" {
		t.Errorf("Expected path through cccc333333333333, got %v", path)
	}
}

func TestComputeRelationalTrust_Distrust(t *testing.T) {
	node := newTestNode()

	// A trusts B with 0.8, B "distrusts" C with 0.3 (< 0.5)
	// Transitive trust A->C should be 0.8 * 0.3 = 0.24 (low due to distrust)
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.8,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.3,
	}

	trust, _, _ := node.ComputeRelationalTrust("aaaa111111111111", "cccc333333333333", 5)

	expected := 0.8 * 0.3
	if trust != expected {
		t.Errorf("Expected trust %f with distrust edge, got %f", expected, trust)
	}
}

func TestUpdateEventStreamRegistry_CreatesStream(t *testing.T) {
	node := newTestNode()

	// Create and process an event transaction
	tx := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "event_001",
			Type:        TxTypeEvent,
			TrustDomain: "test.domain.com",
			Timestamp:   1000000,
		},
		SubjectID:   "aaaa111111111111",
		SubjectType: "quid",
		Sequence:    1,
		EventType:   "created",
	}

	node.updateEventStreamRegistry(tx)

	// Verify stream was created
	stream, exists := node.GetEventStream("aaaa111111111111")
	if !exists {
		t.Fatal("Expected stream to exist")
	}

	if stream.SubjectID != "aaaa111111111111" {
		t.Errorf("Expected SubjectID 'aaaa111111111111', got '%s'", stream.SubjectID)
	}

	if stream.SubjectType != "quid" {
		t.Errorf("Expected SubjectType 'quid', got '%s'", stream.SubjectType)
	}

	if stream.LatestSequence != 1 {
		t.Errorf("Expected LatestSequence 1, got %d", stream.LatestSequence)
	}

	if stream.EventCount != 1 {
		t.Errorf("Expected EventCount 1, got %d", stream.EventCount)
	}

	if stream.LatestEventID != "event_001" {
		t.Errorf("Expected LatestEventID 'event_001', got '%s'", stream.LatestEventID)
	}

	if stream.CreatedAt != 1000000 {
		t.Errorf("Expected CreatedAt 1000000, got %d", stream.CreatedAt)
	}
}

func TestUpdateEventStreamRegistry_AppendsEvents(t *testing.T) {
	node := newTestNode()
	subjectID := "bbbb222222222222"

	// Add first event
	tx1 := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "event_001",
			Type:      TxTypeEvent,
			Timestamp: 1000000,
		},
		SubjectID:   subjectID,
		SubjectType: "asset",
		Sequence:    1,
		EventType:   "created",
	}
	node.updateEventStreamRegistry(tx1)

	// Add second event
	tx2 := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "event_002",
			Type:      TxTypeEvent,
			Timestamp: 1000001,
		},
		SubjectID:   subjectID,
		SubjectType: "asset",
		Sequence:    2,
		EventType:   "updated",
	}
	node.updateEventStreamRegistry(tx2)

	// Add third event
	tx3 := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "event_003",
			Type:      TxTypeEvent,
			Timestamp: 1000002,
		},
		SubjectID:   subjectID,
		SubjectType: "asset",
		Sequence:    3,
		EventType:   "transferred",
	}
	node.updateEventStreamRegistry(tx3)

	// Verify stream metadata
	stream, exists := node.GetEventStream(subjectID)
	if !exists {
		t.Fatal("Expected stream to exist")
	}

	if stream.EventCount != 3 {
		t.Errorf("Expected EventCount 3, got %d", stream.EventCount)
	}

	if stream.LatestSequence != 3 {
		t.Errorf("Expected LatestSequence 3, got %d", stream.LatestSequence)
	}

	if stream.LatestEventID != "event_003" {
		t.Errorf("Expected LatestEventID 'event_003', got '%s'", stream.LatestEventID)
	}

	if stream.UpdatedAt != 1000002 {
		t.Errorf("Expected UpdatedAt 1000002, got %d", stream.UpdatedAt)
	}

	// CreatedAt should remain from first event
	if stream.CreatedAt != 1000000 {
		t.Errorf("Expected CreatedAt 1000000, got %d", stream.CreatedAt)
	}
}

func TestGetStreamEvents_Ordering(t *testing.T) {
	node := newTestNode()
	subjectID := "cccc333333333333"

	// Add events in sequence order
	for i := 1; i <= 5; i++ {
		tx := EventTransaction{
			BaseTransaction: BaseTransaction{
				ID:        fmt.Sprintf("event_%03d", i),
				Type:      TxTypeEvent,
				Timestamp: int64(1000000 + i),
			},
			SubjectID: subjectID,
			Sequence:  int64(i),
			EventType: "update",
		}
		node.updateEventStreamRegistry(tx)
	}

	// Get all events
	events, total := node.GetStreamEvents(subjectID, 10, 0)

	if total != 5 {
		t.Errorf("Expected total 5, got %d", total)
	}

	if len(events) != 5 {
		t.Errorf("Expected 5 events, got %d", len(events))
	}

	// Verify sequence ordering (ascending)
	for i, event := range events {
		expectedSeq := int64(i + 1)
		if event.Sequence != expectedSeq {
			t.Errorf("Event %d: expected sequence %d, got %d", i, expectedSeq, event.Sequence)
		}
	}
}

func TestGetStreamEvents_Pagination(t *testing.T) {
	node := newTestNode()
	subjectID := "dddd444444444444"

	// Add 10 events
	for i := 1; i <= 10; i++ {
		tx := EventTransaction{
			BaseTransaction: BaseTransaction{
				ID:        fmt.Sprintf("event_%03d", i),
				Type:      TxTypeEvent,
				Timestamp: int64(1000000 + i),
			},
			SubjectID: subjectID,
			Sequence:  int64(i),
			EventType: "update",
		}
		node.updateEventStreamRegistry(tx)
	}

	// Test first page (limit 3, offset 0)
	events, total := node.GetStreamEvents(subjectID, 3, 0)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}
	if events[0].Sequence != 1 || events[2].Sequence != 3 {
		t.Error("First page should contain sequences 1-3")
	}

	// Test middle page (limit 3, offset 3)
	events, total = node.GetStreamEvents(subjectID, 3, 3)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}
	if events[0].Sequence != 4 || events[2].Sequence != 6 {
		t.Error("Second page should contain sequences 4-6")
	}

	// Test last partial page (limit 3, offset 9)
	events, total = node.GetStreamEvents(subjectID, 3, 9)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
	if events[0].Sequence != 10 {
		t.Error("Last page should contain sequence 10")
	}

	// Test offset beyond range
	events, total = node.GetStreamEvents(subjectID, 3, 15)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 events for offset beyond range, got %d", len(events))
	}
}

func TestGetStreamEvents_NonExistentStream(t *testing.T) {
	node := newTestNode()

	events, total := node.GetStreamEvents("nonexistent0000", 10, 0)

	if total != 0 {
		t.Errorf("Expected total 0, got %d", total)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}

func TestGetEventStream_NonExistent(t *testing.T) {
	node := newTestNode()

	stream, exists := node.GetEventStream("nonexistent0000")

	if exists {
		t.Error("Expected exists to be false")
	}

	if stream != nil {
		t.Error("Expected stream to be nil")
	}
}

func TestGetEventByID_Found(t *testing.T) {
	node := newTestNode()

	// Add events to multiple streams
	tx1 := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "target_event_id",
			Type:      TxTypeEvent,
			Timestamp: 1000000,
		},
		SubjectID: "aaaa111111111111",
		Sequence:  1,
		EventType: "created",
	}
	node.updateEventStreamRegistry(tx1)

	tx2 := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "other_event_id",
			Type:      TxTypeEvent,
			Timestamp: 1000001,
		},
		SubjectID: "bbbb222222222222",
		Sequence:  1,
		EventType: "created",
	}
	node.updateEventStreamRegistry(tx2)

	// Find the target event
	event, found := node.GetEventByID("target_event_id")

	if !found {
		t.Fatal("Expected event to be found")
	}

	if event.ID != "target_event_id" {
		t.Errorf("Expected ID 'target_event_id', got '%s'", event.ID)
	}

	if event.SubjectID != "aaaa111111111111" {
		t.Errorf("Expected SubjectID 'aaaa111111111111', got '%s'", event.SubjectID)
	}
}

func TestGetEventByID_NotFound(t *testing.T) {
	node := newTestNode()

	// Add an event
	tx := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:        "existing_event",
			Type:      TxTypeEvent,
			Timestamp: 1000000,
		},
		SubjectID: "aaaa111111111111",
		Sequence:  1,
		EventType: "created",
	}
	node.updateEventStreamRegistry(tx)

	// Search for non-existent event
	event, found := node.GetEventByID("nonexistent_event")

	if found {
		t.Error("Expected found to be false")
	}

	if event != nil {
		t.Error("Expected event to be nil")
	}
}

func TestProcessBlockTransactions_EventType(t *testing.T) {
	node := newTestNode()

	// Create a block with an event transaction
	eventTx := EventTransaction{
		BaseTransaction: BaseTransaction{
			ID:          "block_event_001",
			Type:        TxTypeEvent,
			TrustDomain: "test.domain.com",
			Timestamp:   1000000,
		},
		SubjectID:   "eeee555555555555",
		SubjectType: "document",
		Sequence:    1,
		EventType:   "published",
	}

	block := Block{
		Index:        1,
		Timestamp:    1000000,
		Transactions: []interface{}{eventTx},
		PrevHash:     "previous_hash",
		Hash:         "current_hash",
	}

	// Process the block
	node.processBlockTransactions(block)

	// Verify the event was registered
	stream, exists := node.GetEventStream("eeee555555555555")
	if !exists {
		t.Fatal("Expected stream to be created from block processing")
	}

	if stream.EventCount != 1 {
		t.Errorf("Expected EventCount 1, got %d", stream.EventCount)
	}

	if stream.LatestEventID != "block_event_001" {
		t.Errorf("Expected LatestEventID 'block_event_001', got '%s'", stream.LatestEventID)
	}

	// Verify we can retrieve the event
	event, found := node.GetEventByID("block_event_001")
	if !found {
		t.Fatal("Expected event to be retrievable by ID")
	}

	if event.SubjectType != "document" {
		t.Errorf("Expected SubjectType 'document', got '%s'", event.SubjectType)
	}
}

func TestComputeRelationalTrust_LongerPathBetterTrust(t *testing.T) {
	node := newTestNode()

	// Short path with low trust: A -> D: 0.2
	// Longer path with higher trust: A -> B -> C -> D: 0.9 * 0.9 * 0.9 = 0.729
	node.TrustRegistry["aaaa111111111111"] = map[string]float64{
		"bbbb222222222222": 0.9,
		"dddd444444444444": 0.2,
	}
	node.TrustRegistry["bbbb222222222222"] = map[string]float64{
		"cccc333333333333": 0.9,
	}
	node.TrustRegistry["cccc333333333333"] = map[string]float64{
		"dddd444444444444": 0.9,
	}

	trust, path, _ := node.ComputeRelationalTrust("aaaa111111111111", "dddd444444444444", 5)

	expected := 0.9 * 0.9 * 0.9
	if !floatEquals(trust, expected, 0.0001) {
		t.Errorf("Expected best trust %f (longer path), got %f", expected, trust)
	}

	if len(path) != 4 {
		t.Errorf("Expected longer path of length 4, got %v", path)
	}
}
