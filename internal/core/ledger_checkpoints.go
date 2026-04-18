package core

import "sort"

// computeNonceCheckpoints builds the per-block nonce-checkpoint slice
// defined by QDP-0001 §6.1.3. For every TrustTransaction in `txs`,
// group by (Truster, Domain, 0) and emit a single NonceCheckpoint with
// the maximum nonce in that group. Results are sorted by (Quid, Epoch)
// for deterministic block hashing.
//
// Identity, Title, and Event transactions are intentionally excluded
// here. In the foundation phase they do not yet carry a uniform
// per-signer Nonce field; that lands as part of the QDP-0001 hard fork
// migration (§10.2). Once those types expose SignerQuid+Nonce, extend
// this function to cover them.
func computeNonceCheckpoints(txs []interface{}, domain string) []NonceCheckpoint {
	if len(txs) == 0 {
		return nil
	}

	type groupKey struct {
		quid  string
		epoch uint32
	}
	max := make(map[groupKey]int64)

	for _, raw := range txs {
		tx, ok := raw.(TrustTransaction)
		if !ok {
			if p, ok := raw.(*TrustTransaction); ok {
				tx = *p
			} else {
				continue
			}
		}
		if tx.Truster == "" || tx.Nonce <= 0 {
			continue
		}
		k := groupKey{quid: tx.Truster, epoch: 0}
		if cur, seen := max[k]; !seen || tx.Nonce > cur {
			max[k] = tx.Nonce
		}
	}

	if len(max) == 0 {
		return nil
	}

	out := make([]NonceCheckpoint, 0, len(max))
	for k, v := range max {
		out = append(out, NonceCheckpoint{
			Quid:     k.quid,
			Domain:   domain,
			Epoch:    k.epoch,
			MaxNonce: v,
		})
	}

	// Deterministic order so two honest producers compute identical
	// block bytes. Primary key: Quid; secondary: Epoch. (Domain is
	// constant for a block so it's not a sort dimension here.)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Quid != out[j].Quid {
			return out[i].Quid < out[j].Quid
		}
		return out[i].Epoch < out[j].Epoch
	})

	return out
}
