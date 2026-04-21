"""Consumer-side oracle aggregation logic (standalone, no SDK dep).

A DeFi protocol consumes signed price reports from multiple
reporter quids and needs to compute an effective price. The
industry's typical approach (median or mean of all reporters)
treats every reporter equally. This POC instead weighs each
reporter by the consumer's own trust in them -- so the $500M
lending protocol and the $5k micro-app can reach different
effective prices from the same set of signed reports.

Inputs:
  - A list of PriceReport dataclasses extracted from the feed's
    event stream.
  - A trust function returning the consumer's weight for each
    reporter.
  - An AggregationPolicy: freshness window, minimum reporter
    count, outlier threshold, trust floor.

Output:
  - PriceAggregate: effective_price, included/excluded count,
    trust-weighted stddev, reasoning.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, List, Optional


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class PriceReport:
    reporter_quid: str
    symbol: str
    price: float
    timestamp_unix: int
    confidence: float = 1.0
    source: str = ""
    round_id: int = 0


@dataclass
class PriceAggregate:
    verdict: str                   # "ok" | "no-consensus" | "insufficient"
    symbol: str
    effective_price: Optional[float] = None
    trust_weighted_stddev: float = 0.0
    included_reporter_count: int = 0
    excluded: List[str] = field(default_factory=list)
    reasons: List[str] = field(default_factory=list)

    def short(self) -> str:
        if self.effective_price is None:
            return f"{self.verdict.upper()} {self.symbol}"
        return (
            f"{self.verdict.upper()} {self.symbol} "
            f"price={self.effective_price:.4f} "
            f"n={self.included_reporter_count} "
            f"stddev={self.trust_weighted_stddev:.4f}"
        )


TrustFn = Callable[[str, str], float]


@dataclass
class AggregationPolicy:
    """Knobs the consumer tunes per-feed."""

    window_seconds: int = 60
    min_reporters: int = 3
    min_reporter_trust: float = 0.3
    # Exclude reports more than this many weighted-stddev from the
    # weighted median. 0 disables outlier rejection.
    outlier_stddev_threshold: float = 3.0


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _weighted_median(values: List[float], weights: List[float]) -> float:
    """Return the weighted median of a list. values and weights
    must be same length and weights non-negative."""
    if not values:
        raise ValueError("weighted_median of empty list")
    total = sum(weights)
    if total <= 0:
        raise ValueError("weighted_median: total weight is zero")
    pairs = sorted(zip(values, weights), key=lambda p: p[0])
    cum = 0.0
    half = total / 2.0
    for v, w in pairs:
        cum += w
        if cum >= half:
            return v
    return pairs[-1][0]


def _weighted_mean_stddev(values: List[float], weights: List[float]) -> float:
    total = sum(weights)
    if total <= 0:
        return 0.0
    mean = sum(v * w for v, w in zip(values, weights)) / total
    var = sum(w * (v - mean) ** 2 for v, w in zip(values, weights)) / total
    return var ** 0.5


def _weighted_mad(
    values: List[float], weights: List[float], center: float,
) -> float:
    """Median absolute deviation (robust scale estimator).
    Weighted median of |v - center| for each value. Multiplied
    by 1.4826 to normalize to a stddev equivalent under Gaussian
    data."""
    if not values:
        return 0.0
    devs = [abs(v - center) for v in values]
    m = _weighted_median(devs, weights)
    return 1.4826 * m


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def aggregate_price(
    consumer: str,
    symbol: str,
    reports: List[PriceReport],
    trust_fn: TrustFn,
    *,
    now_unix: int,
    policy: Optional[AggregationPolicy] = None,
) -> PriceAggregate:
    """Pure aggregation function.

    Steps:
      1. Filter for reports matching the symbol and within the
         freshness window.
      2. Deduplicate by reporter (keep the most recent).
      3. Filter reports whose reporter's trust is below the floor.
      4. If fewer than min_reporters remain, return "insufficient".
      5. Compute trust-weighted median. Compute trust-weighted
         stddev around the median.
      6. If outlier threshold is set, drop reports more than
         outlier_stddev_threshold stddev from the median. Recompute.
      7. Return the effective price (weighted median of the
         final set).
    """
    p = policy or AggregationPolicy()
    reasons: List[str] = []
    excluded: List[str] = []

    # Step 1: filter by symbol and freshness.
    fresh = [
        r for r in reports
        if r.symbol == symbol
        and (now_unix - r.timestamp_unix) <= p.window_seconds
    ]
    reasons.append(
        f"{len(fresh)} reports in {p.window_seconds}s window for {symbol}"
    )

    # Step 2: dedup by reporter (latest wins).
    latest_by_reporter: dict = {}
    for r in fresh:
        prev = latest_by_reporter.get(r.reporter_quid)
        if prev is None or r.timestamp_unix > prev.timestamp_unix:
            latest_by_reporter[r.reporter_quid] = r
    deduped = list(latest_by_reporter.values())
    reasons.append(f"{len(deduped)} unique reporters")

    # Step 3: trust floor.
    kept: List[PriceReport] = []
    trust_by_reporter: dict = {}
    for r in deduped:
        t = trust_fn(consumer, r.reporter_quid)
        if t < 0.0 or t > 1.0:
            raise ValueError(f"trust out of range for {r.reporter_quid}: {t}")
        trust_by_reporter[r.reporter_quid] = t
        if t < p.min_reporter_trust:
            excluded.append(f"{r.reporter_quid[:12]} (trust {t:.3f} < floor)")
            continue
        kept.append(r)

    # Step 4: minimum count.
    if len(kept) < p.min_reporters:
        return PriceAggregate(
            verdict="insufficient",
            symbol=symbol,
            reasons=reasons + [
                f"only {len(kept)} reporters above trust floor "
                f"{p.min_reporter_trust} (need {p.min_reporters})"
            ],
            excluded=excluded,
        )

    # Step 5: initial weighted median + robust scale (MAD).
    values = [r.price for r in kept]
    weights = [trust_by_reporter[r.reporter_quid] for r in kept]
    med = _weighted_median(values, weights)
    mad = _weighted_mad(values, weights, center=med)
    stddev = _weighted_mean_stddev(values, weights)
    reasons.append(
        f"initial weighted median={med:.4f} mad={mad:.4f} stddev={stddev:.4f}"
    )

    # Step 6: outlier rejection using MAD (robust).
    if p.outlier_stddev_threshold > 0 and mad > 0:
        kept_post: List[PriceReport] = []
        for r, w in zip(kept, weights):
            dist = abs(r.price - med)
            if dist > p.outlier_stddev_threshold * mad:
                excluded.append(
                    f"{r.reporter_quid[:12]} (price {r.price:.4f} "
                    f"more than {p.outlier_stddev_threshold} MAD from median)"
                )
            else:
                kept_post.append(r)
        if len(kept_post) >= p.min_reporters:
            kept = kept_post
            values = [r.price for r in kept]
            weights = [trust_by_reporter[r.reporter_quid] for r in kept]
            med = _weighted_median(values, weights)
            stddev = _weighted_mean_stddev(values, weights)
            reasons.append(
                f"after outlier prune: median={med:.4f} stddev={stddev:.4f}"
            )

    return PriceAggregate(
        verdict="ok",
        symbol=symbol,
        effective_price=med,
        trust_weighted_stddev=stddev,
        included_reporter_count=len(kept),
        excluded=excluded,
        reasons=reasons,
    )


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def extract_reports(events: List[dict]) -> List[PriceReport]:
    out: List[PriceReport] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "oracle.price-report":
            continue
        p = ev.get("payload") or {}
        try:
            price = float(p.get("price", 0.0))
        except (TypeError, ValueError):
            continue
        out.append(PriceReport(
            reporter_quid=p.get("reporter", ""),
            symbol=p.get("symbol", ""),
            price=price,
            timestamp_unix=int(p.get("timestamp") or ev.get("timestamp") or 0),
            confidence=float(p.get("confidence", 1.0)),
            source=p.get("source", ""),
            round_id=int(p.get("roundId") or 0),
        ))
    return out
