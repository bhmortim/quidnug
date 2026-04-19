"""Trust-weighted review rating — reference Python implementation.

Implements QRP-0001 §Algorithm with the four factors described
in algorithm.md: topical transitive trust (T), helpfulness
reputation (H), activity (A), recency (R).

The implementation is intentionally dependency-light — only the
Quidnug Python SDK is imported. Any web framework, async
runtime, or cache layer can wrap this.

Usage:

    from quidnug import QuidnugClient
    from quidnug_reviews.algorithm import TrustWeightedRater

    client = QuidnugClient("http://localhost:8080")
    rater = TrustWeightedRater(client)

    result = rater.effective_rating(
        observer="alice1234abcd5678",
        product="laptop-xps15-asin-b0c1234",
        topic="reviews.public.technology.laptops",
    )

    print(f"Rating: {result.rating:.2f} ± {result.confidence_range:.2f}")
    print(f"Based on {result.contributing_reviews} weighted reviews")
"""

from __future__ import annotations

import math
import time
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Tuple

from quidnug import QuidnugClient
from quidnug.types import Event


# --- Tunable parameters ----------------------------------------------------


@dataclass
class RaterConfig:
    """Tuning knobs (see algorithm.md §Tunable parameters)."""

    recency_halflife_days: float = 730.0
    recency_floor: float = 0.3
    activity_saturation: int = 50
    topic_inheritance_decay: float = 0.8
    min_weight_threshold: float = 0.01
    max_depth_t: int = 5
    max_depth_h: int = 3
    helpfulness_neutral: float = 0.5
    # Floor for helpfulness: even a reviewer with 100% unhelpful votes
    # contributes at this multiplier (not zero). Prevents a small band
    # of hostile voters from erasing a reviewer entirely.
    helpfulness_floor: float = 0.1
    # Maximum reviews to score per product (caps worst-case cost).
    max_reviews_per_product: int = 500
    # Maximum votes to load per reviewer (caps worst-case cost).
    max_votes_per_reviewer: int = 1000


# --- Result shapes ---------------------------------------------------------


@dataclass
class ReviewContribution:
    """A single review's contribution to the weighted rating."""

    review_tx_id: str
    reviewer_quid: str
    rating: float
    weight: float
    t_component: float
    h_component: float
    a_component: float
    r_component: float
    age_days: float


@dataclass
class WeightedRatingResult:
    """The full per-observer rating computation."""

    product: str
    topic: str
    observer: str
    rating: Optional[float]
    total_weight: float
    contributing_reviews: int
    total_reviews_considered: int
    contributions: List[ReviewContribution] = field(default_factory=list)
    confidence_range: float = 0.0
    computed_at: float = 0.0


# --- Algorithm -------------------------------------------------------------


class TrustWeightedRater:
    """Compute per-observer effective ratings for a product title."""

    def __init__(self, client: QuidnugClient, config: Optional[RaterConfig] = None):
        self.client = client
        self.config = config or RaterConfig()
        # Simple in-process caches keyed by (observer, target, topic).
        # Real deployments substitute Redis / Memcached.
        self._trust_cache: Dict[Tuple[str, str, str, int], Optional[float]] = {}

    # =====================================================================
    # Public API
    # =====================================================================

    def effective_rating(
        self,
        observer: str,
        product: str,
        topic: str,
    ) -> WeightedRatingResult:
        """Compute the observer's effective rating for the product."""
        result = WeightedRatingResult(
            product=product,
            topic=topic,
            observer=observer,
            rating=None,
            total_weight=0.0,
            contributing_reviews=0,
            total_reviews_considered=0,
            computed_at=time.time(),
        )

        reviews = self._fetch_reviews(product, topic)
        result.total_reviews_considered = len(reviews)

        contributions: List[ReviewContribution] = []
        weighted_sum = 0.0
        total_weight = 0.0

        for ev in reviews:
            reviewer = ev.creator
            if not reviewer or reviewer == observer:
                # Never include the observer's own review.
                continue
            payload = ev.payload or {}
            raw_rating = payload.get("rating")
            max_rating = payload.get("maxRating", 5.0)
            if raw_rating is None or max_rating in (None, 0):
                continue
            # Normalize to a 0-1 scale internally so mixing scales works.
            normalized = float(raw_rating) / float(max_rating)

            t = self._topical_trust(observer, reviewer, topic, self.config.max_depth_t)
            if t <= 0:
                continue  # no basis to include

            h = self._helpfulness(observer, reviewer, topic)
            a = self._activity(reviewer, topic)
            r, age_days = self._recency(ev.timestamp)

            w = t * h * a * r
            if w < self.config.min_weight_threshold:
                continue

            contributions.append(
                ReviewContribution(
                    review_tx_id=payload.get("txId") or _fallback_id(ev),
                    reviewer_quid=reviewer,
                    rating=normalized * max_rating,  # back to display scale
                    weight=w,
                    t_component=t,
                    h_component=h,
                    a_component=a,
                    r_component=r,
                    age_days=age_days,
                )
            )
            weighted_sum += normalized * w
            total_weight += w

        result.contributions = contributions
        result.contributing_reviews = len(contributions)
        result.total_weight = total_weight

        if total_weight > 0:
            # Scale back to the display scale of the dominant contributor;
            # if max_rating varies, we return normalized-to-5 by default.
            normalized_avg = weighted_sum / total_weight
            result.rating = normalized_avg * 5.0
            # Confidence range: wider with fewer trusted reviews. Heuristic:
            # half the stddev across contributions, clamped.
            result.confidence_range = self._confidence_range(contributions)
        else:
            result.rating = None

        return result

    # =====================================================================
    # Factor T — topical transitive trust, with topic-inheritance fallback
    # =====================================================================

    def _topical_trust(self, observer: str, target: str, topic: str, max_depth: int) -> float:
        """Transitive trust at the most-specific topic available."""
        segments = topic.split(".")
        best = 0.0
        decay = 1.0
        # Walk from most specific to ancestor with progressive decay.
        for depth in range(len(segments), 0, -1):
            candidate_topic = ".".join(segments[:depth])
            score = self._trust_direct(observer, target, candidate_topic, max_depth)
            if score > 0:
                best = max(best, score * decay)
            decay *= self.config.topic_inheritance_decay
            if depth == 2 and best > 0:
                # Once we've found a hit above, further ancestors can't exceed.
                break
        return min(best, 1.0)

    def _trust_direct(self, observer: str, target: str, topic: str, max_depth: int) -> float:
        key = (observer, target, topic, max_depth)
        if key in self._trust_cache:
            cached = self._trust_cache[key]
            return cached if cached is not None else 0.0
        try:
            tr = self.client.get_trust(observer, target, domain=topic, max_depth=max_depth)
            level = float(tr.trust_level)
            self._trust_cache[key] = level
            return level
        except Exception:
            self._trust_cache[key] = None
            return 0.0

    # =====================================================================
    # Factor H — helpfulness reputation
    # =====================================================================

    def _helpfulness(self, observer: str, reviewer: str, topic: str) -> float:
        """Helpfulness score: weighted votes, observer-biased."""
        helpful_weight = 0.0
        unhelpful_weight = 0.0
        # Fetch vote events from the REVIEWER's stream (per QRP-0001 §5.2).
        try:
            events, _ = self.client.get_stream_events(
                reviewer,
                limit=self.config.max_votes_per_reviewer,
            )
        except Exception:
            return self.config.helpfulness_neutral

        for ev in events:
            if ev.event_type not in ("HELPFUL_VOTE", "UNHELPFUL_VOTE"):
                continue
            voter = ev.creator
            if not voter:
                continue
            # Score the voter's authority from the observer's viewpoint,
            # scoped to the same topic domain as the review.
            voter_trust = self._topical_trust(
                observer, voter, topic, self.config.max_depth_h
            )
            if voter_trust <= 0:
                continue
            if ev.event_type == "HELPFUL_VOTE":
                helpful_weight += voter_trust
            else:
                unhelpful_weight += voter_trust

        total = helpful_weight + unhelpful_weight
        if total == 0:
            return self.config.helpfulness_neutral
        raw = helpful_weight / total
        return max(self.config.helpfulness_floor, raw)

    # =====================================================================
    # Factor A — activity
    # =====================================================================

    def _activity(self, reviewer: str, topic: str) -> float:
        """Log-scaled count of this reviewer's reviews in this topic."""
        try:
            events, _ = self.client.get_stream_events(
                reviewer,
                limit=self.config.max_votes_per_reviewer,
            )
        except Exception:
            return 0.0

        # We count REVIEW events the reviewer authored. Since events on
        # the reviewer's own stream are typically votes (per QRP-0001),
        # the reviewer's own reviews actually live on product streams.
        # Production implementations track this via a per-reviewer index.
        # For now: approximate by counting events that look like reviews.
        count = sum(
            1
            for ev in events
            if ev.event_type == "REVIEW"
        )
        if count <= 0:
            return 0.0
        saturation = max(2, self.config.activity_saturation)
        return min(1.0, math.log(count + 1) / math.log(saturation))

    # =====================================================================
    # Factor R — recency
    # =====================================================================

    def _recency(self, timestamp: Optional[int]) -> Tuple[float, float]:
        """Exponential decay with a floor."""
        if not timestamp:
            return (self.config.recency_floor, 9999.0)
        age_seconds = max(0.0, time.time() - float(timestamp))
        age_days = age_seconds / 86400.0
        hl = max(1.0, self.config.recency_halflife_days)
        decayed = math.exp(-age_days / hl)
        return (max(self.config.recency_floor, decayed), age_days)

    # =====================================================================
    # Data loaders
    # =====================================================================

    def _fetch_reviews(self, product: str, topic: str) -> List[Event]:
        """All REVIEW events on the product's stream in the topic."""
        try:
            events, _ = self.client.get_stream_events(
                product,
                domain=topic,
                limit=self.config.max_reviews_per_product,
            )
        except Exception:
            return []
        return [
            ev
            for ev in events
            if ev.event_type == "REVIEW"
        ]

    # =====================================================================
    # Confidence
    # =====================================================================

    def _confidence_range(self, contributions: List[ReviewContribution]) -> float:
        """±-style confidence based on contribution spread + sample size."""
        if not contributions:
            return 1.0  # maximum uncertainty
        total_w = sum(c.weight for c in contributions)
        if total_w == 0:
            return 1.0
        # Weighted mean
        mean = sum(c.rating * c.weight for c in contributions) / total_w
        # Weighted variance
        var = sum(c.weight * (c.rating - mean) ** 2 for c in contributions) / total_w
        stddev = math.sqrt(max(0.0, var))
        # Shrink toward zero with more effective sample size.
        effective_n = max(1.0, total_w * 2)
        interval = stddev / math.sqrt(effective_n)
        return min(2.5, interval)


# --- Helpers ---------------------------------------------------------------


def _fallback_id(ev: Event) -> str:
    """Best-effort stable identifier when payload lacks txId."""
    return f"{ev.subject_id}:{ev.sequence}"


__all__ = [
    "RaterConfig",
    "ReviewContribution",
    "TrustWeightedRater",
    "WeightedRatingResult",
]
