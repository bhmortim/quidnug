// QDP-0018 Phase 1 audit-log integration with QuidnugNode.
//
// The audit log itself lives in internal/audit (no cross-
// dependency). This file is the thin glue that:
//
//   - Offers emitAudit(category, payload, note) — a best-effort
//     append helper used by every mempool-admission path that
//     should leave an auditable trace (moderation, DSR, consent,
//     governance).
//   - Wires the three Phase 1 HTTP query endpoints:
//       GET /api/v2/audit/head
//       GET /api/v2/audit/entries?since=&limit=
//       GET /api/v2/audit/entry/{seq}
//
// Phase 3 will add AUDIT_ANCHOR emission (hourly goroutine +
// chain-backed commitment); Phase 4 adds Merkle-proof endpoints
// and the verifier CLI.
package core

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/quidnug/quidnug/internal/audit"
)

// emitAudit appends one audit entry. Best-effort: a logging
// failure does not abort the caller's operation, since audit
// gaps are less damaging than unavailability. Nil AuditLog is
// tolerated so tests can disable audit without rewriting
// helper usage.
//
// Timestamp / sequence / prev-hash / hash are all assigned by
// the log; callers supply the category and payload only.
func (node *QuidnugNode) emitAudit(category string, payload map[string]interface{}, note string) {
	if node.AuditLog == nil {
		return
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	entry := audit.Entry{
		Category: category,
		Payload:  payload,
		Note:     note,
	}
	if _, err := node.AuditLog.Append(entry); err != nil {
		logger.Warn("Audit log append failed",
			"category", category, "err", err)
	}
}

// AuditHeadHandler returns the current head of the operator's
// audit log: sequence, hash, and the quid the log is stamped
// against.
func (node *QuidnugNode) AuditHeadHandler(w http.ResponseWriter, r *http.Request) {
	if node.AuditLog == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"audit log is not enabled on this node")
		return
	}
	head, ok := node.AuditLog.Head()
	if !ok {
		// Empty log — return a structured zero-entry response
		// so clients can distinguish "no log" from "log present
		// but empty."
		WriteSuccess(w, map[string]interface{}{
			"operatorQuid": node.AuditLog.OperatorQuid(),
			"height":       0,
			"headHash":     audit.ZeroPrevHash,
		})
		return
	}
	WriteSuccess(w, map[string]interface{}{
		"operatorQuid": node.AuditLog.OperatorQuid(),
		"height":       node.AuditLog.Height(),
		"headHash":     head.Hash,
		"headSequence": head.Sequence,
		"headTimestamp": head.Timestamp,
	})
}

// AuditEntriesHandler returns entries after a cursor, bounded
// by `limit`. Query params:
//
//   since — sequence of the last entry the caller has already
//           ingested. Default -1 so the first call returns
//           from entry 0.
//   limit — max entries in this page. Capped at
//           MaxPaginationLimit.
func (node *QuidnugNode) AuditEntriesHandler(w http.ResponseWriter, r *http.Request) {
	if node.AuditLog == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"audit log is not enabled on this node")
		return
	}

	since := int64(-1)
	if raw := r.URL.Query().Get("since"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			since = v
		}
	}
	limit := int64(DefaultPaginationLimit)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > 0 {
			if v > int64(MaxPaginationLimit) {
				v = int64(MaxPaginationLimit)
			}
			limit = v
		}
	}

	entries := node.AuditLog.EntriesSince(since, limit)
	WriteSuccess(w, map[string]interface{}{
		"operatorQuid": node.AuditLog.OperatorQuid(),
		"entries":      entries,
		"height":       node.AuditLog.Height(),
	})
}

// AuditEntryHandler returns a specific entry by sequence. 404
// if the sequence is out of range.
func (node *QuidnugNode) AuditEntryHandler(w http.ResponseWriter, r *http.Request) {
	if node.AuditLog == nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"audit log is not enabled on this node")
		return
	}
	vars := mux.Vars(r)
	raw := vars["sequence"]
	seq, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST",
			"sequence must be a non-negative integer")
		return
	}
	entry, ok := node.AuditLog.Get(seq)
	if !ok {
		WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"audit entry not found")
		return
	}
	WriteSuccess(w, entry)
}
