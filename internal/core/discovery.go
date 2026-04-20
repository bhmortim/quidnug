// Package core — QDP-0014 discovery surface.
//
// This file implements the per-domain quid index and the
// discovery HTTP handlers. The index is an incrementally-
// maintained view over block state; it's rebuilt from scratch
// on node restart by replaying registry contents, and updated
// in-place as new blocks commit.
//
// Companion to node_advertisement.go, which owns the
// advertisement-level registry. Together they serve the
// /api/v2/discovery/* endpoints.
package core

import (
	"sort"
	"strings"
	"sync"
)

// QuidDomainStats is the per-(domain, quid) activity profile
// that populates the discovery /quids endpoint.
type QuidDomainStats struct {
	QuidID          string           `json:"quidId"`
	FirstSeen       int64            `json:"firstSeen"`       // UnixNano of first observed tx in this domain
	LastSeen        int64            `json:"lastSeen"`        // UnixNano of most recent
	TxCount         int64            `json:"txCount"`         // total transactions signed by this quid in this domain
	EventTypeCounts map[string]int64 `json:"eventTypeCounts"` // by EventTransaction.EventType; nil for quids that only signed non-event txs
	TrustEdgesOut   int64            `json:"trustEdgesOut"`   // TRUST txs this quid issued (truster = quid)
	TrustEdgesIn    int64            `json:"trustEdgesIn"`    // TRUST txs targeting this quid (trustee = quid)
}

// QuidDomainIndex tracks per-(domain, quid) activity for fast
// discovery queries. Domain strings are exact; observers that
// want glob-style filtering walk the returned stats themselves.
type QuidDomainIndex struct {
	mu    sync.RWMutex
	stats map[string]map[string]*QuidDomainStats // domain → quid → stats
}

// NewQuidDomainIndex constructs an empty index.
func NewQuidDomainIndex() *QuidDomainIndex {
	return &QuidDomainIndex{
		stats: make(map[string]map[string]*QuidDomainStats),
	}
}

// observe records a single transaction's footprint in the
// index. Called incrementally from processBlockTransactions.
// `txTimestampSec` is seconds since epoch (the tx.Timestamp
// convention); promoted to UnixNano internally for sub-second
// precision vs future tx types.
func (x *QuidDomainIndex) observe(domain, quid string, txTimestampSec int64) *QuidDomainStats {
	if domain == "" || quid == "" {
		return nil
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	bucket, ok := x.stats[domain]
	if !ok {
		bucket = make(map[string]*QuidDomainStats)
		x.stats[domain] = bucket
	}
	entry, ok := bucket[quid]
	if !ok {
		entry = &QuidDomainStats{
			QuidID:    quid,
			FirstSeen: txTimestampSec * 1e9,
		}
		bucket[quid] = entry
	}
	entry.LastSeen = txTimestampSec * 1e9
	entry.TxCount++
	return entry
}

// observeEvent extends observe() with event-type bookkeeping.
func (x *QuidDomainIndex) observeEvent(domain, quid string, eventType string, txTimestampSec int64) {
	entry := x.observe(domain, quid, txTimestampSec)
	if entry == nil || eventType == "" {
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if entry.EventTypeCounts == nil {
		entry.EventTypeCounts = make(map[string]int64)
	}
	entry.EventTypeCounts[eventType]++
}

// observeTrustEdge credits both ends of a TRUST transaction.
func (x *QuidDomainIndex) observeTrustEdge(domain, truster, trustee string, txTimestampSec int64) {
	if truster != "" {
		entry := x.observe(domain, truster, txTimestampSec)
		if entry != nil {
			x.mu.Lock()
			entry.TrustEdgesOut++
			x.mu.Unlock()
		}
	}
	if trustee != "" {
		x.mu.Lock()
		bucket, ok := x.stats[domain]
		if !ok {
			bucket = make(map[string]*QuidDomainStats)
			x.stats[domain] = bucket
		}
		entry, ok := bucket[trustee]
		if !ok {
			entry = &QuidDomainStats{
				QuidID:    trustee,
				FirstSeen: txTimestampSec * 1e9,
			}
			bucket[trustee] = entry
		}
		if entry.FirstSeen == 0 {
			entry.FirstSeen = txTimestampSec * 1e9
		}
		entry.LastSeen = txTimestampSec * 1e9
		entry.TrustEdgesIn++
		x.mu.Unlock()
	}
}

// ListByDomain returns every quid with activity in a domain.
// Caller may filter by `since` (nanoseconds), sort in-place,
// and paginate. Returns deep copies so the caller's mutations
// don't race with index updates.
func (x *QuidDomainIndex) ListByDomain(domain string, sinceNanos int64) []QuidDomainStats {
	x.mu.RLock()
	defer x.mu.RUnlock()
	bucket, ok := x.stats[domain]
	if !ok {
		return nil
	}
	out := make([]QuidDomainStats, 0, len(bucket))
	for _, entry := range bucket {
		if sinceNanos > 0 && entry.LastSeen < sinceNanos {
			continue
		}
		// Deep-copy event-type map so the caller can't race.
		copied := *entry
		if entry.EventTypeCounts != nil {
			copied.EventTypeCounts = make(map[string]int64, len(entry.EventTypeCounts))
			for k, v := range entry.EventTypeCounts {
				copied.EventTypeCounts[k] = v
			}
		}
		out = append(out, copied)
	}
	return out
}

// Quid-index sort modes, matching the discovery API's sort parameter.
const (
	QuidSortActivity    = "activity"
	QuidSortLastSeen    = "last-seen"
	QuidSortFirstSeen   = "first-seen"
	QuidSortTrustWeight = "trust-weight"
)

// SortQuidStats sorts the slice in place by the named mode. If
// mode is unrecognized, sorts by last-seen descending. For
// trust-weight, the caller must populate trustWeights before
// calling; the map is keyed by quid id.
func SortQuidStats(list []QuidDomainStats, mode string, trustWeights map[string]float64) {
	switch mode {
	case QuidSortActivity:
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].TxCount != list[j].TxCount {
				return list[i].TxCount > list[j].TxCount
			}
			return list[i].QuidID < list[j].QuidID
		})
	case QuidSortFirstSeen:
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].FirstSeen != list[j].FirstSeen {
				return list[i].FirstSeen < list[j].FirstSeen
			}
			return list[i].QuidID < list[j].QuidID
		})
	case QuidSortTrustWeight:
		sort.SliceStable(list, func(i, j int) bool {
			wi := trustWeights[list[i].QuidID]
			wj := trustWeights[list[j].QuidID]
			if wi != wj {
				return wi > wj
			}
			return list[i].QuidID < list[j].QuidID
		})
	default: // "last-seen" and any unrecognised mode
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].LastSeen != list[j].LastSeen {
				return list[i].LastSeen > list[j].LastSeen
			}
			return list[i].QuidID < list[j].QuidID
		})
	}
}

// Supporting helper — used by the trusted-quids handler to
// enumerate which trustees the consortium members have trusted.
func (node *QuidnugNode) consortiumTrustedQuids(
	domain string, minTrust float64,
) map[string]float64 {
	node.TrustDomainsMutex.RLock()
	d, ok := node.TrustDomains[domain]
	node.TrustDomainsMutex.RUnlock()
	if !ok {
		return nil
	}
	out := make(map[string]float64)
	node.TrustRegistryMutex.RLock()
	defer node.TrustRegistryMutex.RUnlock()
	for validatorID := range d.Validators {
		if trustees, exists := node.TrustRegistry[validatorID]; exists {
			for quid, level := range trustees {
				if level >= minTrust {
					// Keep the max across validators.
					if cur, already := out[quid]; !already || level > cur {
						out[quid] = level
					}
				}
			}
		}
	}
	return out
}

// filterByEventType trims the slice to quids whose
// EventTypeCounts include the named type with count > 0.
// Used by the discovery /quids?eventType=... filter.
func filterByEventType(list []QuidDomainStats, eventType string) []QuidDomainStats {
	if eventType == "" {
		return list
	}
	out := make([]QuidDomainStats, 0, len(list))
	for _, s := range list {
		if s.EventTypeCounts != nil && s.EventTypeCounts[eventType] > 0 {
			out = append(out, s)
		}
	}
	return out
}

// filterByMinTrust trims the slice to quids whose trust-weight
// meets a threshold; used in concert with observer-scoped
// trust queries.
func filterByMinTrust(list []QuidDomainStats, trustWeights map[string]float64, min float64) []QuidDomainStats {
	if min <= 0 {
		return list
	}
	out := make([]QuidDomainStats, 0, len(list))
	for _, s := range list {
		if trustWeights[s.QuidID] >= min {
			out = append(out, s)
		}
	}
	return out
}

// excludeQuids applies the discovery API's ?excludeQuid= param.
func excludeQuids(list []QuidDomainStats, excluded []string) []QuidDomainStats {
	if len(excluded) == 0 {
		return list
	}
	blocked := make(map[string]struct{}, len(excluded))
	for _, q := range excluded {
		blocked[strings.TrimSpace(q)] = struct{}{}
	}
	out := make([]QuidDomainStats, 0, len(list))
	for _, s := range list {
		if _, skip := blocked[s.QuidID]; !skip {
			out = append(out, s)
		}
	}
	return out
}
