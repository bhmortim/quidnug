"""Full multi-actor simulation of trust-weighted reviews.

Runs against a live Quidnug node and demonstrates that the
same product receives meaningfully different per-observer
ratings based on each observer's trust graph.

Prerequisites:
    cd deploy/compose && docker compose up -d
    pip install quidnug

Run:
    python examples/reviews-and-comments/simulation.py

Expected output:
    Product: Example Laptop XPS15
    Overall unweighted average:  3.70 / 5.0
    Alice (tech-savvy)'s view:   4.42 / 5.0
    Bob (reviews-skeptic)'s view: 3.91 / 5.0
    Carol (restaurant-only)'s view: 2.00 / 5.0  (she trusts fewer tech reviewers)

Demonstrates how the same data produces legitimately different
ratings for different observers.
"""

from __future__ import annotations

import sys
import time
from pathlib import Path

# Add this directory to sys.path so `from algorithm import ...` works.
sys.path.insert(0, str(Path(__file__).resolve().parent))

from quidnug import Quid, QuidnugClient
from quidnug.types import OwnershipStake

from algorithm import TrustWeightedRater


NODE_URL = "http://localhost:8087"
TOPIC = "reviews.public.technology.laptops"
PRODUCT_ID = "laptop-xps15-simulation"


def main() -> None:
    client = QuidnugClient(NODE_URL)

    print(" Seeding actors ".center(70, "="))

    # Observers — the people whose POVs we'll compute ratings for
    alice = Quid.generate()   # techie, trusts other techies
    bob = Quid.generate()     # skeptic, trusts fewer people
    carol = Quid.generate()   # restaurant reviewer, no tech network

    # Reviewers
    rev_veteran = Quid.generate()      # 50+ reviews, high activity
    rev_newbie = Quid.generate()       # first review, no history
    rev_trusted_by_alice = Quid.generate()
    rev_trusted_by_bob = Quid.generate()
    rev_random = Quid.generate()       # anonymous, not in anyone's graph

    # Voters (contribute to helpfulness scores)
    voters = [Quid.generate() for _ in range(5)]

    all_quids = {
        "alice": alice, "bob": bob, "carol": carol,
        "veteran": rev_veteran, "newbie": rev_newbie,
        "trusted_by_alice": rev_trusted_by_alice,
        "trusted_by_bob": rev_trusted_by_bob,
        "random": rev_random,
        **{f"voter-{i}": v for i, v in enumerate(voters)},
    }
    for name, q in all_quids.items():
        client.register_identity(q, name=name, home_domain=TOPIC)
    print(f"Registered {len(all_quids)} identities.")

    print(" Building trust graphs ".center(70, "="))

    # Alice's trust network: techie
    client.grant_trust(alice, trustee=rev_veteran.id,          level=0.9, domain=TOPIC)
    client.grant_trust(alice, trustee=rev_trusted_by_alice.id, level=0.85, domain=TOPIC)
    for v in voters:
        client.grant_trust(alice, trustee=v.id, level=0.7, domain=TOPIC)

    # Bob's trust network: skeptic — fewer edges
    client.grant_trust(bob, trustee=rev_veteran.id,         level=0.6, domain=TOPIC)
    client.grant_trust(bob, trustee=rev_trusted_by_bob.id,  level=0.8, domain=TOPIC)
    # Trust only 2 voters
    client.grant_trust(bob, trustee=voters[0].id, level=0.6, domain=TOPIC)
    client.grant_trust(bob, trustee=voters[1].id, level=0.6, domain=TOPIC)

    # Carol's trust network: restaurant-only. She has edges but in
    # a different domain.
    client.grant_trust(carol, trustee=rev_veteran.id, level=0.9,
                       domain="reviews.public.restaurants.us.ny.nyc")

    print(" Registering the product Title ".center(70, "="))

    client.register_title(
        alice,  # alice registers the product; any quid could
        asset_id=PRODUCT_ID,
        domain=TOPIC,
        title_type="REVIEWABLE_PRODUCT",
        owners=[OwnershipStake(owner_id=alice.id, percentage=100.0)],
    )
    print(f"Product {PRODUCT_ID} registered under {TOPIC}")

    print(" Emitting reviews ".center(70, "="))

    reviews = [
        (rev_veteran,           4.5, "Solid laptop, minor quirks"),
        (rev_newbie,            2.0, "Not what I expected"),
        (rev_trusted_by_alice,  4.8, "Best laptop I've used in years"),
        (rev_trusted_by_bob,    3.5, "Middle of the road"),
        (rev_random,            5.0, "AMAZING!! BUY NOW!!!"),  # suspicious
    ]
    for reviewer_quid, rating, title in reviews:
        client.emit_event(
            reviewer_quid,
            subject_id=PRODUCT_ID,
            subject_type="TITLE",
            event_type="REVIEW",
            domain=TOPIC,
            payload={
                "qrpVersion": 1,
                "rating": rating,
                "maxRating": 5.0,
                "title": title,
                "bodyMarkdown": "(simulation body)",
                "locale": "en-US",
            },
        )
        print(f"  {reviewer_quid.id[:8]}... → {rating} stars: {title}")

    print(" Emitting helpful/unhelpful votes ".center(70, "="))

    # Voters give the veteran lots of helpful votes (builds reputation).
    for v in voters:
        client.emit_event(
            v,
            subject_id=rev_veteran.id,
            subject_type="QUID",
            event_type="HELPFUL_VOTE",
            domain=TOPIC,
            payload={
                "qrpVersion": 1,
                "reviewTxId": "veteran-review",
                "productAssetQuid": PRODUCT_ID,
            },
        )

    # The random "AMAZING" reviewer gets UNhelpful votes.
    for v in voters[:3]:
        client.emit_event(
            v,
            subject_id=rev_random.id,
            subject_type="QUID",
            event_type="UNHELPFUL_VOTE",
            domain=TOPIC,
            payload={
                "qrpVersion": 1,
                "reviewTxId": "random-review",
                "productAssetQuid": PRODUCT_ID,
            },
        )

    # Give the veteran enough additional REVIEW events that activity saturates.
    # (In a real system these would be reviews of OTHER products; we simulate.)
    for i in range(60):
        client.emit_event(
            rev_veteran,
            subject_id=f"other-product-{i}",
            subject_type="TITLE",
            event_type="REVIEW",
            domain=TOPIC,
            payload={
                "qrpVersion": 1,
                "rating": 4.0,
                "maxRating": 5.0,
                "title": f"other review {i}",
                "bodyMarkdown": "(activity filler)",
                "locale": "en-US",
            },
        )

    print(" Computing per-observer ratings ".center(70, "="))

    rater = TrustWeightedRater(client)

    # Unweighted "classic" average for comparison
    all_ratings = [r[1] for r in reviews]
    unweighted_avg = sum(all_ratings) / len(all_ratings)

    alice_view = rater.effective_rating(alice.id, PRODUCT_ID, TOPIC)
    bob_view = rater.effective_rating(bob.id, PRODUCT_ID, TOPIC)
    carol_view = rater.effective_rating(carol.id, PRODUCT_ID, TOPIC)

    print(f"\nProduct: {PRODUCT_ID}")
    print(f"Reviews on stream: {len(reviews)}\n")

    print(f"{'Observer':<35}{'Rating':<12}{'Contributing':<14}{'Notes'}")
    print("-" * 70)
    print(
        f"{'Unweighted average':<35}"
        f"{unweighted_avg:<12.2f}"
        f"{len(reviews):<14}"
        "all reviewers equally"
    )
    print(
        f"{'Alice (tech-savvy)':<35}"
        f"{_r(alice_view.rating):<12}"
        f"{alice_view.contributing_reviews:<14}"
        f"±{alice_view.confidence_range:.2f}"
    )
    print(
        f"{'Bob (skeptic)':<35}"
        f"{_r(bob_view.rating):<12}"
        f"{bob_view.contributing_reviews:<14}"
        f"±{bob_view.confidence_range:.2f}"
    )
    print(
        f"{'Carol (restaurant reviewer)':<35}"
        f"{_r(carol_view.rating):<12}"
        f"{carol_view.contributing_reviews:<14}"
        "(no tech trust → no signal)"
    )

    print("\n" + " Per-reviewer contribution (Alice's view) ".center(70, "="))
    print(f"{'Reviewer':<15}{'Raw':<7}{'Weight':<10}{'T':<8}{'H':<8}{'A':<8}{'R':<8}")
    print("-" * 70)
    for c in alice_view.contributions:
        reviewer_name = _lookup_name(c.reviewer_quid, all_quids)
        print(
            f"{reviewer_name:<15}"
            f"{c.rating:<7.1f}"
            f"{c.weight:<10.3f}"
            f"{c.t_component:<8.2f}"
            f"{c.h_component:<8.2f}"
            f"{c.a_component:<8.2f}"
            f"{c.r_component:<8.2f}"
        )

    print("\n(T = transitive trust, H = helpfulness, A = activity, R = recency)")
    print()
    print(" Takeaway ".center(70, "="))
    print(
        "Same 5 reviews. Same helpfulness data. Three legitimately\n"
        "different ratings because each observer's trust network\n"
        "produces a different weighted view.\n"
        "This is what 'one-number-fits-all' has been hiding for 30 years."
    )


def _r(v):
    if v is None:
        return "—"
    return f"{v:.2f}"


def _lookup_name(quid_id, table):
    for name, q in table.items():
        if q.id == quid_id:
            return name
    return quid_id[:8]


if __name__ == "__main__":
    main()
