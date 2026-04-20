# QDP-0019: Reputation Decay & Time-Weighted Trust

| Field      | Value                                                            |
|------------|------------------------------------------------------------------|
| Status     | Draft — design only                                              |
| Track      | Protocol                                                         |
| Author     | The Quidnug Authors                                              |
| Created    | 2026-04-20                                                       |
| Requires   | QDP-0001 (nonce ledger), QDP-0016 (abuse prevention)             |
| Implements | Time-weighted trust decay + stale-edge handling + reputation freshness |

## 1. Summary

The current trust graph treats every edge equally regardless of
age. A `TRUST` transaction published five years ago contributes
identically to one published yesterday. That's cheap and simple
but creates slow-growing problems:

- **Stale trust.** A reviewer who stopped reviewing three years
  ago still has their trust graph position, which drags on
  rating quality when observers walk paths through them.
- **Dead-account resurrection.** An attacker who compromises a
  quid that hasn't been used in years gets the full historical
  reputation with no indication it's been dormant.
- **Reputation unfalsifiability.** Once trust is earned, it
  never fades — reviewers who become disengaged stay
  "authoritative" indefinitely.
- **First-mover lock-in.** Early reviewers accumulate edges;
  late-arrivals can never catch up even if they become more
  active and trusted.

The QRP-0001 rating algorithm already has a recency factor `R`
in its per-review computation — a 2-year half-life with a
floor at 0.3. That works at the review aggregation layer but
doesn't affect the underlying trust graph itself.

QDP-0019 adds a protocol-layer decay model that's applied during
relational trust computation. Edges have an **effective weight**
that decreases over time since their last activity, not just
their last publication. Observers can tune the decay curve per
domain to match the expected churn rate in that community.

## 2. Goals and non-goals

**Goals:**

- Old trust edges gradually become less influential in trust
  computations without being deleted.
- Quids that remain active (issue new transactions, attract new
  trust edges) retain full influence.
- Quids that go dormant slowly fade from prominence.
- Decay is observer-configurable — different observers can
  apply different curves to the same shared chain state.
- No new transaction types. The decay is purely a computation-
  time parameter applied to existing edges.

**Non-goals:**

- Automatic deletion of old edges. The chain remains append-only.
- Global consensus on decay parameters. Each observer picks
  their own curve.
- Retroactive invalidation of past ratings. The historical
  aggregations computed under older curves stay as-is; future
  computations use the new curve.
- Replacement of QRP-0001's review-level recency factor. The
  two layers compose: review-level recency decays individual
  events, graph-level decay fades connecting edges.

## 3. The two-layer decay model

### 3.1 Edge-level decay

Every trust edge has an **effective weight** computed as a
function of its nominal weight and the time since the edge was
last "refreshed":

```
effectiveWeight(edge, now) =
    nominalWeight * decayFunction(now - lastRefreshed)
```

Where `lastRefreshed` is the most recent of:

- The edge's `Timestamp` (when the TRUST transaction was published)
- The timestamp of any re-issue of the same edge (by nonce bump)
- The timestamp of any transaction by the trustee that the
  truster could have countermanded but didn't (passive
  re-endorsement — see §3.4)

### 3.2 Default decay function

A two-parameter exponential decay with a floor:

```
decayFunction(ageNanos) =
    max(floor, exp(-ageNanos / halfLife * ln(2)))
```

Default parameters per domain:

```yaml
trust_decay:
    half_life_years: 2      # weight halves every 2 years
    floor: 0.2              # minimum effective weight (never fully fades)
    refresh_on_trustee_activity: true
```

Intuition:

- Year 0: weight = nominal (e.g., 0.9)
- Year 2: weight = 0.5 * nominal (0.45)
- Year 4: weight = 0.25 * nominal (0.225)
- Year 6: weight = 0.125 * nominal — clipped to floor (0.18)
- Forever: weight ≥ floor * nominal (0.18)

The floor prevents old edges from fully disappearing (which
would be a silent deletion) while substantially reducing their
influence.

### 3.3 Passive re-endorsement

The trickiest part. An edge "Alice trusts Bob" that hasn't been
republished is still meaningful if Alice keeps endorsing Bob's
actions implicitly — upvoting his reviews, using him as a
reference, etc.

QDP-0019 defines passive re-endorsement via observable signals:

- Alice publishes any new transaction where Bob is a direct
  beneficiary (helpful vote on Bob's content, trust edge
  toward someone in Bob's trust graph, etc.) → the Alice→Bob
  edge's `lastRefreshed` advances to Alice's new tx timestamp.
- Alice takes NO action for the signal window (default 90
  days) → decay begins at the original edge timestamp.

This captures the intuition "silence is the absence of
endorsement, not the absence of opinion." If Alice's been
actively using the network and hasn't objected to Bob, that's
tacit continued endorsement.

### 3.4 Observer-local decay curves

Different observers may want different decay policies:

- A security-conscious observer might use a short half-life
  (6 months) — they want the trust graph to reflect current
  reality, not history.
- A research or archival observer might use a long half-life
  (10 years) — they want historical edges to count for
  scholarly purposes.
- A low-traffic domain might disable decay entirely — the
  recency signal is noisy below some activity threshold.

Config:

```yaml
trust_decay:
    default:
        half_life_years: 2
        floor: 0.2

    # Per-domain overrides
    domains:
        "reviews.public.restaurants":
            half_life_years: 1     # cuisine scenes change fast
            floor: 0.1

        "operators.network.*":
            half_life_years: 5     # infrastructure is stable
            floor: 0.3

        "academic.citations.*":
            half_life_years: 20    # scholarly trust endures
            floor: 0.5
```

### 3.5 Path-level aggregation

When computing relational trust across a path, decay compounds:

```
pathWeight = product of effectiveWeight across edges
           * inheritanceDecay ^ (pathLength - 1)
```

The existing inheritance-decay (0.8 per hop) is unchanged.
Edge-level decay adds a second temporal dimension.

Example: Alice→Bob (edge weight 0.9, age 4 years, half-life 2
years) → Bob→Carol (edge weight 0.8, age 6 months):

- Alice→Bob effective: 0.9 * 0.25 = 0.225 (hit floor; actually
  0.9 * max(0.2, 0.25) = 0.225)
- Bob→Carol effective: 0.8 * 0.854 = 0.683
- Combined path: 0.225 * 0.683 * 0.8 = 0.123

vs the same path computed without decay:

- 0.9 * 0.8 * 0.8 = 0.576

The decayed path is ~20% of the fresh path. That's by design —
an old reference gets discounted heavily unless refreshed.

## 4. Quid-level dormancy state

### 4.1 The quid activity signal

Each quid has an implicit activity level based on their
transaction history. Computing it is cheap (`LastSeen` from the
QDP-0014 quid index):

```
activity(quid, domain, now) =
    max(0, 1 - (now - lastSeen(quid, domain)) / dormancyWindow)
```

Default `dormancyWindow`: 1 year. After a year of no activity,
a quid is "fully dormant" — activity = 0. Partial dormancy is
the 0..1 range.

### 4.2 Dormant-quid trust dampening

When computing `effectiveWeight(A→B)`, multiply by
`sqrt(activity(A) * activity(B))`:

- Both parties active → no dampening (multiplier = 1.0)
- Truster active, trustee dormant → moderate dampening
  (multiplier ~0.7 if trustee half-dormant)
- Both parties dormant → heavy dampening (multiplier ~0.0-0.3)

This fights the "dead-account resurrection" attack. An attacker
who compromises a long-dormant quid gets their historical
reputation but only at the dampened multiplier — they'd have to
rebuild activity before their stolen trust graph becomes fully
effective.

### 4.3 Configurable windows

```yaml
trust_decay:
    dormancy:
        window_days: 365           # how long before fully dormant
        dampen_dormant_edges: true
        min_multiplier: 0.1        # never fully zero out
```

## 5. Interaction with review aggregation

QRP-0001's rating algorithm multiplies per-review weight by
four factors (T, H, A, R). QDP-0019's decay affects the **T**
(topical trust) factor — the trust-graph weight from observer
to reviewer.

The two models compose naturally:

| Layer | Decay mechanism | Default half-life |
|---|---|---|
| Edge (QDP-0019) | `lastRefreshed`-based exponential | 2 years |
| Quid dormancy (QDP-0019) | Cliff at 1 year of inactivity | 1 year |
| Review recency (QRP-0001 R factor) | Per-review timestamp decay | 2 years |
| Helpful-vote age (QRP-0001 H factor) | Per-vote timestamp decay | 90 days |

Combined effect for a five-year-old review by a three-year-dormant
reviewer: heavy discount at every layer. The trust path fades
faster than linear because each layer compounds.

## 6. Attack vectors and mitigations

### 6.1 Churn farming

**Attack:** Adversary publishes low-value "refresh" transactions
to keep their edges perpetually fresh without doing meaningful
work.

**Mitigation:**
- Passive re-endorsement (§3.3) specifies that the refresh
  must be substantive. A "renew my own trust edge" tx with no
  other meaningful content doesn't count as substantive.
- Rate limits (QDP-0016) constrain high-volume low-value
  transactions.
- Community flagging (QRP-0001 FLAG event) de-weights
  low-quality content.

The combination means churn farming is expensive to sustain
while being indistinguishable from regular activity.

### 6.2 Dead-account resurrection

**Attack:** Attacker takes over a long-dormant quid and
exploits its accumulated trust graph.

**Mitigation:** Quid-level dampening (§4.2) multiplies edge
weights by the quid's activity level, capped at sqrt-combined.
A dormant quid's edges effectively have ~0.1-0.3x their nominal
weight. To fully reactivate, the attacker would need to
re-establish activity over months — during which community
signals (helpful votes / flags) would reveal the break with
the original owner's style.

Layer this with QDP-0002 guardian recovery (which should've
happened anyway for long-lived keys) and the compromise surface
shrinks further.

### 6.3 Historical-edge manipulation via long-running sybils

**Attack:** Attacker runs a sybil army for years, slowly
accumulating "aged" trust edges to make them look legitimate.

**Mitigation:** Aging edges still face:
- Decay (they compound with regular sybil's lack of activity)
- Reputation graduation caps (QDP-0016) — a sybil's
  write-throughput stays low even if they've been around.
- Trust-graph shape anomalies that flagging catches.

Time alone doesn't manufacture trust if the activity isn't
genuine.

### 6.4 Parameter drift across observers

**Attack:** Observers disagree wildly on decay curves, creating
"forking views" of the same underlying graph.

**Mitigation:** This is by design — observers are autonomous,
and different communities reasonably want different decay
behavior. The protocol doesn't force agreement; the UI should
surface "this observer is using a non-standard decay curve" as
context when presenting trust queries.

### 6.5 Edge-decay cliff gaming

**Attack:** Adversary times their action to exploit a specific
decay floor. E.g., wait for a target edge to hit the floor,
then issue an action that would've been blocked by the
not-decayed edge.

**Mitigation:** Floors are a default; operators facing
cliff-gaming risks set lower floors or disable them entirely.
The decay curve is smooth exponential, not stepped, so most
attacks fall back to the general "old trust is cheap to
discount" finding.

## 7. Implementation plan

### Phase 1: Observer-side decay computation

- `ComputeRelationalTrustEnhanced` already exists; extend its
  signature to accept a decay config.
- Implement the edge-decay formula + path aggregation.
- No changes to the wire format; decay is computation-time.

Effort: ~3-5 days.

### Phase 2: Configuration surface

- Add `trust_decay` section to the node config schema.
- CLI flags on `trust get` / `discover quids --observer`
  to override decay parameters for one-off queries.
- YAML hot-reload for operator tuning without restart.

Effort: ~2-3 days.

### Phase 3: Passive re-endorsement detection

- Add a `lastTrusteeActivity` tracking to the trust registry
  indexed by (truster, trustee).
- Update on every transaction where the truster's observable
  activity could be an implicit endorsement.
- Feed into `effectiveWeight`.

Effort: ~1 person-week.

### Phase 4: Quid dormancy computation

- Reuse QDP-0014 quid-index's `lastSeen` for dormancy
  computation.
- Integrate quid-activity dampening into the trust formula.

Effort: ~3-5 days.

### Phase 5: Metrics + visualization

- Emit Prometheus metrics showing the decay distribution
  (`quidnug_trust_edge_decay_ratio_histogram`).
- Update `@quidnug/web-components` rating primitives to
  visualize decay: edge age colorization in the constellation
  view, time-stamped recency indicators on aurora.

Effort: ~1 person-week.

## 8. Open questions

1. **Is 2-year half-life the right default?** Review-site data
   suggests 1-3 years depending on domain. Per-domain overrides
   let operators tune; the default is educated-guess. Revisit
   after real-world usage data.

2. **Should decay interact with guardian recovery?** If a quid
   rotates keys via guardian recovery, should all incoming
   trust edges auto-refresh (since they're endorsing a newly-
   validated key)? Lean toward yes; the recovery event is a
   substantive signal.

3. **Per-reviewer-vs-per-domain decay.** Currently decay is
   per-domain (operator config). Should individual observers
   be able to override per-quid (e.g., "trust this reviewer's
   years-old reviews forever because I know they're authoritative")?
   Probably yes, defer as a future enhancement.

4. **Interaction with the no-node lightweight participation**
   (QDP-0014 §14). Apps using the public api directly can't
   change the decay curve server-side. They'd need the server
   to expose it as a query parameter. Easy extension: add
   `?halfLifeYears=X&floor=Y` params to relational-trust
   endpoints.

5. **Archival / audit observer mode.** Researchers auditing
   historical activity need undecayed trust. Should there be a
   `?decay=off` query parameter? Yes; easy to add and
   useful for legitimate use.

## 9. Review status

Draft. Needs:

- Simulation against real-ish review data to validate the
  2-year / 0.2-floor defaults don't cause surprising behavior.
- UX review of how decayed vs fresh edges are surfaced to
  users in the visualization primitives.
- Operator input on whether per-domain configuration is the
  right granularity.

Implementation is relatively straightforward; the hardest work
is the passive re-endorsement (§3.3) detection logic, which
requires careful definition of what counts as a substantive
implicit endorsement.

## 10. References

- [QDP-0001 (Nonce ledger)](0001-global-nonce-ledger.md) —
  `lastRefreshed` tracking fits alongside the nonce state
- [QDP-0014 (Node Discovery)](0014-node-discovery-and-sharding.md) —
  quid-index's `lastSeen` timestamp powers dormancy
- [QDP-0016 (Abuse Prevention)](0016-abuse-prevention.md) —
  rate limits deter churn farming
- [QRP-0001 Review rating algorithm](../../examples/reviews-and-comments/algorithm.md)
  — the complementary per-review recency factor
- [Exponential decay in reputation systems](https://arxiv.org/abs/1401.4626) —
  background on decay-based reputation in distributed systems
