"""Unit tests for the trust-weighted rating algorithm.

Uses a lightweight fake QuidnugClient so tests run without a
live node. Covers the four factors (T, H, A, R) and their
interactions across realistic scenarios.

    cd examples/reviews-and-comments
    pytest algorithm_test.py -v
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple
from unittest.mock import MagicMock

import pytest

from algorithm import RaterConfig, TrustWeightedRater
from quidnug.types import Event, TrustResult


# --- Fake client ---------------------------------------------------------


@dataclass
class FakeReview:
    reviewer: str
    rating: float
    max_rating: float = 5.0
    age_days: float = 30.0

    def to_event(self, sequence: int) -> Event:
        return Event(
            subject_id="PRODUCT-1",
            subject_type="TITLE",
            event_type="REVIEW",
            payload={
                "qrpVersion": 1,
                "rating": self.rating,
                "maxRating": self.max_rating,
                "title": f"review by {self.reviewer}",
                "bodyMarkdown": "…",
            },
            payload_cid=None,
            timestamp=int(time.time() - self.age_days * 86400),
            sequence=sequence,
            creator=self.reviewer,
            signature="sig",
        )


@dataclass
class FakeVote:
    voter: str
    kind: str  # "HELPFUL_VOTE" or "UNHELPFUL_VOTE"

    def to_event(self, reviewer: str, sequence: int) -> Event:
        return Event(
            subject_id=reviewer,
            subject_type="QUID",
            event_type=self.kind,
            payload={"qrpVersion": 1, "reviewTxId": "fake"},
            payload_cid=None,
            timestamp=int(time.time() - 5 * 86400),
            sequence=sequence,
            creator=self.voter,
            signature="sig",
        )


class FakeClient:
    """Minimal in-memory fake of QuidnugClient for the algorithm."""

    def __init__(self) -> None:
        self.reviews_by_product: Dict[str, List[FakeReview]] = {}
        # votes_on_reviewer[reviewer] = list of FakeVote
        self.votes_by_reviewer: Dict[str, List[FakeVote]] = {}
        # trust_edges[(observer, target, domain)] = level
        self.trust_edges: Dict[Tuple[str, str, str], float] = {}
        # extra reviews counted as the reviewer's activity
        self.reviewer_activity: Dict[str, int] = {}

    def add_review(self, product: str, review: FakeReview):
        self.reviews_by_product.setdefault(product, []).append(review)

    def add_vote(self, reviewer: str, voter: str, kind: str):
        self.votes_by_reviewer.setdefault(reviewer, []).append(FakeVote(voter, kind))

    def add_trust(self, observer: str, target: str, domain: str, level: float):
        self.trust_edges[(observer, target, domain)] = level

    def set_activity(self, reviewer: str, review_count: int):
        self.reviewer_activity[reviewer] = review_count

    # --- the surface the rater uses ---

    def get_stream_events(self, subject_id, domain=None, limit=50, offset=0):
        # Product streams return REVIEWs.
        if subject_id in self.reviews_by_product:
            events = [
                r.to_event(i + 1)
                for i, r in enumerate(self.reviews_by_product[subject_id])
            ]
            return events, {}

        # Reviewer streams return their REVIEWs + HELPFUL/UNHELPFUL votes.
        if subject_id in self.votes_by_reviewer or subject_id in self.reviewer_activity:
            events: List[Event] = []
            for i, v in enumerate(self.votes_by_reviewer.get(subject_id, [])):
                events.append(v.to_event(subject_id, i + 1))
            # Synthesize REVIEW events to register activity.
            for j in range(self.reviewer_activity.get(subject_id, 0)):
                events.append(
                    Event(
                        subject_id=subject_id,
                        subject_type="QUID",
                        event_type="REVIEW",
                        payload={},
                        payload_cid=None,
                        timestamp=int(time.time()),
                        sequence=1000 + j,
                        creator=subject_id,
                        signature="sig",
                    )
                )
            return events, {}

        return [], {}

    def get_trust(self, observer, target, domain, max_depth=5):
        level = self.trust_edges.get((observer, target, domain), 0.0)
        return TrustResult(
            observer=observer,
            target=target,
            trust_level=level,
            path=[observer, target] if level > 0 else [],
            path_depth=1 if level > 0 else 0,
            domain=domain,
        )


# --- Test fixtures -----------------------------------------------------


@pytest.fixture
def client() -> FakeClient:
    return FakeClient()


@pytest.fixture
def rater(client):
    return TrustWeightedRater(client, RaterConfig(
        recency_halflife_days=365,  # 1-year half-life for faster test signal
        activity_saturation=10,     # quicker saturation for test ease
    ))


# --- Tests ---------------------------------------------------------------


def test_no_reviews_returns_none(client, rater):
    result = rater.effective_rating("alice", "PRODUCT-X", "reviews.public.technology")
    assert result.rating is None
    assert result.contributing_reviews == 0


def test_single_trusted_reviewer(client, rater):
    client.add_review("PRODUCT-1", FakeReview("bob", 4.0, age_days=30))
    client.set_activity("bob", 20)
    client.add_trust("alice", "bob", "reviews.public.technology", 1.0)

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    assert result.rating is not None
    # Rating should be close to 4.0 but slightly adjusted by recency
    assert 3.5 <= result.rating <= 4.1
    assert result.contributing_reviews == 1


def test_untrusted_reviewer_gets_zero_weight(client, rater):
    client.add_review("PRODUCT-1", FakeReview("unknown-person", 5.0))
    # No trust edge from alice to unknown-person

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    # No contributor should count — rating should be None
    assert result.contributing_reviews == 0
    assert result.rating is None


def test_per_observer_divergence(client, rater):
    """Same reviews, different observers → different ratings."""
    client.add_review("PRODUCT-1", FakeReview("bob", 5.0, age_days=10))
    client.add_review("PRODUCT-1", FakeReview("carol", 2.0, age_days=10))
    client.set_activity("bob", 30)
    client.set_activity("carol", 30)

    # Alice trusts bob, doesn't know carol
    client.add_trust("alice", "bob", "reviews.public.technology", 1.0)
    # Dave trusts carol, doesn't know bob
    client.add_trust("dave", "carol", "reviews.public.technology", 1.0)

    alice_result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    dave_result = rater.effective_rating("dave", "PRODUCT-1", "reviews.public.technology")

    assert alice_result.rating is not None
    assert dave_result.rating is not None
    # Alice sees bob's 5.0 only → high rating
    assert alice_result.rating >= 4.0
    # Dave sees carol's 2.0 only → low rating
    assert dave_result.rating <= 3.0
    # Divergence is substantive
    assert alice_result.rating - dave_result.rating >= 1.5


def test_helpful_votes_boost_trusted_reviewer(client, rater):
    """Helpful votes from trusted voters increase review weight."""
    client.add_review("PRODUCT-1", FakeReview("reviewer-a", 4.0, age_days=30))
    client.add_review("PRODUCT-1", FakeReview("reviewer-b", 4.0, age_days=30))
    client.set_activity("reviewer-a", 20)
    client.set_activity("reviewer-b", 20)

    # Alice directly trusts both reviewers
    client.add_trust("alice", "reviewer-a", "reviews.public.technology", 0.8)
    client.add_trust("alice", "reviewer-b", "reviews.public.technology", 0.8)

    # Alice trusts several voters
    for voter in ["v1", "v2", "v3", "v4", "v5"]:
        client.add_trust("alice", voter, "reviews.public.technology", 0.9)

    # Voters cast 10 helpful on reviewer-a, 10 unhelpful on reviewer-b.
    for voter in ["v1", "v2", "v3", "v4", "v5"]:
        client.add_vote("reviewer-a", voter, "HELPFUL_VOTE")
        client.add_vote("reviewer-b", voter, "UNHELPFUL_VOTE")

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")

    # Both reviewers gave 4.0, but helpfulness tilts the weighted
    # contribution: reviewer-a should dominate.
    assert result.contributing_reviews == 2
    a_contrib = next(c for c in result.contributions if c.reviewer_quid == "reviewer-a")
    b_contrib = next(c for c in result.contributions if c.reviewer_quid == "reviewer-b")
    assert a_contrib.weight > b_contrib.weight * 2


def test_recency_decays_old_reviews(client, rater):
    client.add_review("PRODUCT-1", FakeReview("bob", 5.0, age_days=10))   # recent
    client.add_review("PRODUCT-1", FakeReview("carol", 3.0, age_days=400))  # old
    client.set_activity("bob", 20)
    client.set_activity("carol", 20)
    client.add_trust("alice", "bob", "reviews.public.technology", 1.0)
    client.add_trust("alice", "carol", "reviews.public.technology", 1.0)

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    bob_contrib = next(c for c in result.contributions if c.reviewer_quid == "bob")
    carol_contrib = next(c for c in result.contributions if c.reviewer_quid == "carol")
    # Bob's review is much more recent so his recency weight is higher
    assert bob_contrib.r_component > carol_contrib.r_component
    # But carol still contributes (above the floor)
    assert carol_contrib.weight > 0


def test_activity_rewards_established_reviewers(client, rater):
    # Both reviewers have direct trust from Alice, same rating, same age.
    # Only difference: activity.
    client.add_review("PRODUCT-1", FakeReview("veteran", 4.0, age_days=10))
    client.add_review("PRODUCT-1", FakeReview("newbie", 4.0, age_days=10))

    client.set_activity("veteran", 50)
    client.set_activity("newbie", 1)

    client.add_trust("alice", "veteran", "reviews.public.technology", 0.9)
    client.add_trust("alice", "newbie", "reviews.public.technology", 0.9)

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    vet = next(c for c in result.contributions if c.reviewer_quid == "veteran")
    newbie = next(c for c in result.contributions if c.reviewer_quid == "newbie")
    assert vet.a_component > newbie.a_component


def test_topic_inheritance(client, rater):
    """Trust in a parent topic should apply (with decay) to children."""
    client.add_review("PRODUCT-1", FakeReview("bob", 4.0, age_days=30))
    client.set_activity("bob", 20)
    # Alice trusts bob in the parent topic, not the child
    client.add_trust("alice", "bob", "reviews.public.technology", 0.9)

    result = rater.effective_rating(
        "alice", "PRODUCT-1", "reviews.public.technology.laptops"
    )
    assert result.contributing_reviews == 1
    bob_contrib = result.contributions[0]
    # Should get approximately 0.9 × 0.8 = 0.72 decay for one inheritance step
    assert 0.65 <= bob_contrib.t_component <= 0.75


def test_observer_excludes_own_reviews(client, rater):
    client.add_review("PRODUCT-1", FakeReview("alice", 5.0, age_days=5))
    client.set_activity("alice", 10)
    # Even if alice trusts herself, she shouldn't weight her own review.

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    assert result.contributing_reviews == 0
    assert result.rating is None


def test_confidence_range_wider_with_fewer_reviews(client, rater):
    # Scenario 1: one trusted review → should have wider confidence
    client.add_review("PRODUCT-1", FakeReview("bob", 4.0, age_days=10))
    client.set_activity("bob", 20)
    client.add_trust("alice", "bob", "reviews.public.technology", 1.0)
    result_1 = rater.effective_rating(
        "alice", "PRODUCT-1", "reviews.public.technology"
    )

    # Scenario 2: same + many more trusted reviews → tighter confidence
    for i in range(10):
        name = f"reviewer-{i}"
        client.add_review("PRODUCT-2", FakeReview(name, 4.0, age_days=10))
        client.set_activity(name, 20)
        client.add_trust("alice", name, "reviews.public.technology", 1.0)
    result_2 = rater.effective_rating(
        "alice", "PRODUCT-2", "reviews.public.technology"
    )

    assert result_2.total_weight > result_1.total_weight


def test_mixed_rating_scales(client, rater):
    """Reviews on different scales (5-star vs 10-point) unify cleanly."""
    client.add_review("PRODUCT-1", FakeReview("bob", 4.0, max_rating=5.0, age_days=30))
    client.add_review("PRODUCT-1", FakeReview("carol", 8.0, max_rating=10.0, age_days=30))
    client.set_activity("bob", 30)
    client.set_activity("carol", 30)
    client.add_trust("alice", "bob", "reviews.public.technology", 1.0)
    client.add_trust("alice", "carol", "reviews.public.technology", 1.0)

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    # Both translate to 80% → aggregate rating ~4.0 on 5-star display
    assert result.rating is not None
    assert 3.7 <= result.rating <= 4.3


def test_min_weight_threshold_drops_noise(client, rater):
    """Very low-weight contributors (e.g., barely-trusted stale reviewers)
    should be dropped to avoid noise."""
    client.add_review("PRODUCT-1", FakeReview("bob", 5.0, age_days=10))
    client.add_review("PRODUCT-1", FakeReview("stale", 1.0, age_days=2000))
    client.set_activity("bob", 30)
    client.set_activity("stale", 1)  # low activity

    client.add_trust("alice", "bob", "reviews.public.technology", 1.0)
    # stale has very low trust + very low activity + very old review
    client.add_trust("alice", "stale", "reviews.public.technology", 0.01)

    result = rater.effective_rating("alice", "PRODUCT-1", "reviews.public.technology")
    # stale should be filtered out
    assert result.contributing_reviews == 1
    assert result.contributions[0].reviewer_quid == "bob"
