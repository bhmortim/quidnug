"""End-to-end working demo of trust-weighted reviews.

Talks directly to a live Quidnug node at http://localhost:8087
using the exact wire format the reference Go node expects.

Signing is delegated to a small Go helper subprocess
(``sign_helper/main.go`` built to ``/tmp/sign-helper.exe``) which
uses ``crypto/ecdsa`` + ``elliptic.Marshal`` to produce the exact
IEEE-1363 / SEC1 encoding the reference node validates against.
Python's ``cryptography`` library emits DER, which the Go node
rejects — hence the helper.

Run:
    go build -o /tmp/sign-helper.exe ./examples/reviews-and-comments/demo/sign_helper
    python examples/reviews-and-comments/demo/demo.py
"""

from __future__ import annotations

import hashlib
import json
import math
import os
import subprocess
import sys
import time
import uuid
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import requests

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "clients" / "python"))
from quidnug import Quid  # noqa: E402


NODE = os.environ.get("QUIDNUG_NODE", "http://localhost:8087")
TOPIC = "reviews.public.technology.laptops"
PRODUCT_NAME = "Dell XPS 15 (demo)"

# Helper next to this file, or overridable via env.
_DEFAULT_HELPER = Path(__file__).parent / "sign_helper" / (
    "sign-helper.exe" if os.name == "nt" else "sign-helper"
)
HELPER = os.environ.get("QUIDNUG_SIGN_HELPER", str(_DEFAULT_HELPER))


# ---------------------------------------------------------------------
# Signing subprocess
# ---------------------------------------------------------------------

class Signer:
    """Persistent subprocess that signs transactions byte-compatibly
    with the Go reference node."""

    def __init__(self, helper_path: str):
        if not Path(helper_path).exists():
            raise FileNotFoundError(
                f"sign helper not found at {helper_path}. "
                "Build with: go build -o /tmp/sign-helper.exe "
                "./examples/reviews-and-comments/demo/sign_helper"
            )
        self.proc = subprocess.Popen(
            [helper_path],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            bufsize=0,
        )

    def sign(self, priv_hex: str, kind: str, tx: Dict[str, Any]) -> Dict[str, Any]:
        req = json.dumps(
            {"priv": priv_hex, "kind": kind, "tx": tx},
            separators=(",", ":"),
        )
        assert self.proc.stdin is not None and self.proc.stdout is not None
        self.proc.stdin.write((req + "\n").encode("utf-8"))
        self.proc.stdin.flush()
        line = self.proc.stdout.readline()
        if not line:
            err = self.proc.stderr.read().decode("utf-8", "replace") if self.proc.stderr else ""
            raise RuntimeError(f"sign helper closed stdout; stderr={err!r}")
        resp = json.loads(line)
        if not resp.get("ok"):
            raise RuntimeError(f"sign failed: {resp.get('error')}")
        # `signed` is json.RawMessage on the Go side; embedded verbatim,
        # so the Python json decoder returns it pre-parsed as dict.
        signed = resp["signed"]
        return signed if isinstance(signed, dict) else json.loads(signed)

    def close(self) -> None:
        if self.proc and self.proc.stdin:
            try:
                self.proc.stdin.close()
            except Exception:
                pass
        if self.proc:
            try:
                self.proc.wait(timeout=3)
            except Exception:
                self.proc.kill()


SIGNER: Optional[Signer] = None


def signer() -> Signer:
    global SIGNER
    if SIGNER is None:
        SIGNER = Signer(HELPER)
    return SIGNER


# ---------------------------------------------------------------------
# Wire helpers
# ---------------------------------------------------------------------

def post(path: str, body: Any) -> Dict[str, Any]:
    data = json.dumps(body, separators=(",", ":"))
    r = requests.post(f"{NODE}/api/{path}", data=data,
                      headers={"Content-Type": "application/json"}, timeout=10)
    try:
        j = r.json()
    except Exception:
        raise RuntimeError(f"{r.status_code}: {r.text[:200]}")
    if not j.get("success"):
        raise RuntimeError(f"{r.status_code}: {j.get('error', j)}")
    return j.get("data") or {}


def get(path: str) -> Dict[str, Any]:
    r = requests.get(f"{NODE}/api/{path}", timeout=10)
    j = r.json()
    if not j.get("success"):
        raise RuntimeError(f"{r.status_code}: {j.get('error', j)}")
    return j.get("data") or {}


def _fresh_tx_id() -> str:
    """Fresh hex id so the Go server won't regenerate/clobber it."""
    return hashlib.sha256(uuid.uuid4().bytes + str(time.time_ns()).encode()).hexdigest()


# ---------------------------------------------------------------------
# Go struct-order builders (match internal/core/types.go)
#
# These only build the unsigned tx skeleton; sign_helper fills in
# publicKey + signature in the Go node's exact wire format.
# ---------------------------------------------------------------------

def build_identity_tx(q: Quid, name: str, domain: str) -> Dict[str, Any]:
    tx = {
        "id":          _fresh_tx_id(),
        "type":        "IDENTITY",
        "trustDomain": domain,
        "timestamp":   int(time.time()),
        "signature":   "",
        "publicKey":   "",
        "quidId":      q.id,
        "name":        name,
        "creator":     q.id,
        "updateNonce": 1,
    }
    return signer().sign(q.private_key_hex, "IDENTITY", tx)


def build_title_tx(s: Quid, asset_id: str, domain: str, title_type: str) -> Dict[str, Any]:
    tx = {
        "id":          _fresh_tx_id(),
        "type":        "TITLE",
        "trustDomain": domain,
        "timestamp":   int(time.time()),
        "signature":   "",
        "publicKey":   "",
        "assetId":     asset_id,
        "owners":      [{"ownerId": s.id, "percentage": 100.0}],
        "signatures":  {},
        "titleType":   title_type,
    }
    return signer().sign(s.private_key_hex, "TITLE", tx)


def build_trust_tx(truster: Quid, trustee: str, level: float,
                   domain: str, nonce: int = 1) -> Dict[str, Any]:
    tx = {
        "id":          _fresh_tx_id(),
        "type":        "TRUST",
        "trustDomain": domain,
        "timestamp":   int(time.time()),
        "signature":   "",
        "publicKey":   "",
        "truster":     truster.id,
        "trustee":     trustee,
        "trustLevel":  level,
        "nonce":       nonce,
    }
    return signer().sign(truster.private_key_hex, "TRUST", tx)


def build_event_tx(
    s: Quid, subject_id: str, subject_type: str,
    event_type: str, domain: str, payload: Dict[str, Any],
    sequence: Optional[int] = None,
) -> Dict[str, Any]:
    tx: Dict[str, Any] = {
        "id":          _fresh_tx_id(),
        "type":        "EVENT",
        "trustDomain": domain,
        "timestamp":   int(time.time()),
        "signature":   "",
        "publicKey":   "",
        "subjectId":   subject_id,
        "subjectType": subject_type,
        "sequence":    sequence if sequence is not None else 0,
        "eventType":   event_type,
        "payload":     payload,
    }
    return signer().sign(s.private_key_hex, "EVENT", tx)


# ---------------------------------------------------------------------
# API wrappers
# ---------------------------------------------------------------------

def register_domain(name: str) -> None:
    try:
        post("domains", {
            "name": name,
            "validatorNodes": [],
            "trustThreshold": 0.5,
            "validators": {},
            "validatorPublicKeys": {},
        })
    except RuntimeError as e:
        s = str(e).lower()
        if "already" not in s and "exists" not in s:
            raise


def register_identity(q: Quid, name: str, domain: str) -> None:
    post("transactions/identity", build_identity_tx(q, name, domain))


def register_title(issuer: Quid, asset_id: str, domain: str, title_type: str) -> None:
    post("transactions/title", build_title_tx(issuer, asset_id, domain, title_type))


def grant_trust(truster: Quid, trustee: str, level: float, domain: str, nonce: int = 1) -> None:
    post("transactions/trust", build_trust_tx(truster, trustee, level, domain, nonce))


def emit_event(
    s: Quid, subject_id: str, subject_type: str, event_type: str,
    domain: str, payload: Dict[str, Any], sequence: Optional[int] = None,
) -> Dict[str, Any]:
    return post("events", build_event_tx(s, subject_id, subject_type,
                                         event_type, domain, payload, sequence))


def get_stream_events(subject_id: str) -> List[Dict[str, Any]]:
    data = get(f"streams/{subject_id}/events")
    return data.get("data") or data.get("events") or []


def identity_exists(quid_id: str) -> bool:
    """Check whether an identity has been committed to the registry."""
    try:
        data = get(f"identity/{quid_id}")
        return bool(data)
    except RuntimeError:
        return False


def _wait_for_identity(quid_id: str, timeout: float = 30.0, poll: float = 0.5) -> None:
    """Block until the identity is visible in the registry.

    Identity transactions go to the pending pool on POST, then get
    written to the registry when the next block is sealed. Titles
    can't reference an asset until its identity is in the registry.
    """
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if identity_exists(quid_id):
            return
        time.sleep(poll)
    raise RuntimeError(f"identity {quid_id} not committed within {timeout}s")


def _wait_for_stream_seq(subject_id: str, min_seq: int,
                         timeout: float = 30.0, poll: float = 0.4) -> None:
    """Block until the stream for `subject_id` has committed at
    least one event with sequence >= min_seq.

    Events are validated against `EventStreamRegistry`, which is
    only populated at block-commit time. Rapid-fire posting to the
    same stream can race past the commit — this waits.
    """
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        events = get_stream_events(subject_id)
        last = max((e.get("sequence", 0) for e in events), default=0)
        if last >= min_seq:
            return
        time.sleep(poll)
    raise RuntimeError(
        f"stream {subject_id} did not reach seq >= {min_seq} within {timeout}s"
    )


def get_trust(observer: str, target: str, domain: str, max_depth: int = 5) -> float:
    try:
        data = get(f"trust/{observer}/{target}?domain={domain}&maxDepth={max_depth}")
        return float(data.get("trustLevel", 0.0))
    except RuntimeError:
        return 0.0


# ---------------------------------------------------------------------
# Rating algorithm (identical to reference in algorithm.py)
# ---------------------------------------------------------------------

def topical_trust(observer: str, target: str, topic: str,
                  max_depth: int = 5, inheritance_decay: float = 0.8) -> float:
    segments = topic.split(".")
    best = 0.0
    decay = 1.0
    for depth in range(len(segments), 0, -1):
        candidate = ".".join(segments[:depth])
        score = get_trust(observer, target, candidate, max_depth)
        if score > 0:
            best = max(best, score * decay)
        decay *= inheritance_decay
        if depth == 2 and best > 0:
            break
    return min(best, 1.0)


def helpfulness_score(observer: str, reviewer: str, voter_quids: List[str], topic: str,
                      floor: float = 0.1, neutral: float = 0.5) -> float:
    """Aggregate HELPFUL/UNHELPFUL votes *about* `reviewer`, cast by
    the observer's known voter population (`voter_quids`). Each
    voter's event stream is scanned for votes whose payload
    references `reviewer`. Votes are weighted by the observer's
    trust in the voter."""
    helpful = unhelpful = 0.0
    for voter in voter_quids:
        vt = topical_trust(observer, voter, topic, max_depth=3)
        if vt <= 0:
            continue
        for ev in get_stream_events(voter):
            et = ev.get("eventType")
            if et not in ("HELPFUL_VOTE", "UNHELPFUL_VOTE"):
                continue
            payload = ev.get("payload") or {}
            if payload.get("reviewerQuid") != reviewer:
                continue
            if et == "HELPFUL_VOTE":
                helpful += vt
            else:
                unhelpful += vt
    total = helpful + unhelpful
    if total == 0:
        return neutral
    return max(floor, helpful / total)


def activity_score(reviewer: str, saturation: int = 50) -> float:
    events = get_stream_events(reviewer)
    count = sum(1 for e in events if e.get("eventType") == "REVIEW")
    if count <= 0:
        return 0.0
    return min(1.0, math.log(count + 1) / math.log(max(2, saturation)))


def recency_score(timestamp: Optional[int], halflife_days: float = 730.0,
                  floor: float = 0.3) -> Tuple[float, float]:
    if not timestamp:
        return (floor, 9999.0)
    age_days = max(0.0, (time.time() - timestamp) / 86400.0)
    return (max(floor, math.exp(-age_days / max(1.0, halflife_days))), age_days)


def effective_rating(observer: str, product: str, topic: str,
                     reviewer_quids: List[str], voter_quids: List[str],
                     min_weight: float = 0.01) -> Dict[str, Any]:
    """Compute the observer's trust-weighted rating for `product`.

    The Go node enforces that each event lives on the signer's own
    stream, so reviews for a product are scattered across every
    reviewer's personal stream. We scan each known reviewer's
    stream, keep REVIEW events whose payload points at `product`,
    and weight by the four-factor model (T, H, A, R).
    """
    contributions = []
    weighted_sum = total_weight = 0.0
    reviews_considered = 0
    for reviewer in reviewer_quids:
        if reviewer == observer:
            continue
        for ev in get_stream_events(reviewer):
            if ev.get("eventType") != "REVIEW":
                continue
            payload = ev.get("payload") or {}
            if payload.get("productAssetQuid") != product:
                continue
            reviews_considered += 1
            rating = payload.get("rating")
            max_rating = payload.get("maxRating", 5.0)
            if rating is None or not max_rating:
                continue
            normalized = float(rating) / float(max_rating)
            t = topical_trust(observer, reviewer, topic)
            if t <= 0:
                continue
            h = helpfulness_score(observer, reviewer, voter_quids, topic)
            a = activity_score(reviewer)
            r, age_days = recency_score(ev.get("timestamp"))
            w = t * h * a * r
            if w < min_weight:
                continue
            contributions.append({
                "reviewer": reviewer, "rating": float(rating),
                "weight": w, "t": t, "h": h, "a": a, "r": r,
                "age_days": age_days,
            })
            weighted_sum += normalized * w
            total_weight += w
    return {
        "observer": observer, "product": product, "topic": topic,
        "rating": (weighted_sum / total_weight) * 5.0 if total_weight > 0 else None,
        "total_weight": total_weight,
        "contributing": len(contributions),
        "total_considered": reviews_considered,
        "contributions": contributions,
    }


# ---------------------------------------------------------------------
# Scenario
# ---------------------------------------------------------------------

def main() -> None:
    print(f"\n{'=' * 70}\n TRUST-WEIGHTED REVIEWS DEMO — against {NODE} \n{'=' * 70}\n")

    print("[0] registering domain tree...")
    for d in [
        "reviews.public",
        "reviews.public.technology",
        "reviews.public.technology.laptops",
        "reviews.public.restaurants",
    ]:
        register_domain(d)
    print(f"    ok — tree through {TOPIC}")

    print("\n[1] generating keypairs & registering identities...")
    actors: Dict[str, Quid] = {
        "alice":              Quid.generate(),
        "bob":                Quid.generate(),
        "carol":              Quid.generate(),
        "veteran":            Quid.generate(),
        "newbie":             Quid.generate(),
        "rev_alice_trusts":   Quid.generate(),
        "rev_bob_trusts":     Quid.generate(),
        "rev_random":         Quid.generate(),
    }
    # Two disjoint voter populations — so votes on different
    # reviewers end up on different streams, avoiding sequence
    # collisions in the block window.
    upvoters = [Quid.generate() for _ in range(5)]
    downvoters = [Quid.generate() for _ in range(3)]
    voters = upvoters + downvoters
    for i, v in enumerate(voters):
        actors[f"voter-{i}"] = v

    for name, q in actors.items():
        register_identity(q, name=name, domain=TOPIC)
    print(f"    ok — {len(actors)} identities")

    print("\n[2] registering product as a quid + title...")
    # Products in Quidnug are first-class quids — the asset is its
    # own identity. Alice (the seller) registers it and then claims
    # 100% ownership via a TITLE tx. We have to wait one block cycle
    # between the identity and the title, because title validation
    # checks that the asset already exists in the identity registry
    # (which is only written at block-commit time).
    product_quid = Quid.generate()
    product_id = product_quid.id
    register_identity(product_quid, name=PRODUCT_NAME, domain=TOPIC)
    print(f"    ok — product quid {product_id} (waiting for block)")
    _wait_for_identity(product_id)
    register_title(actors["alice"], product_id, TOPIC, title_type="REVIEWABLE_PRODUCT")
    print(f"    ok — title registered")

    print("\n[3] building trust graphs...")
    grant_trust(actors["alice"], actors["veteran"].id,          0.9,  TOPIC, nonce=1)
    grant_trust(actors["alice"], actors["rev_alice_trusts"].id, 0.85, TOPIC, nonce=2)
    for i, v in enumerate(voters, start=3):
        grant_trust(actors["alice"], v.id, 0.7, TOPIC, nonce=i)

    grant_trust(actors["bob"], actors["veteran"].id,         0.6, TOPIC, nonce=1)
    grant_trust(actors["bob"], actors["rev_bob_trusts"].id,  0.8, TOPIC, nonce=2)
    grant_trust(actors["bob"], voters[0].id, 0.6, TOPIC, nonce=3)
    grant_trust(actors["bob"], voters[1].id, 0.6, TOPIC, nonce=4)

    grant_trust(actors["carol"], actors["veteran"].id, 0.9,
                "reviews.public.restaurants", nonce=1)
    print("    ok — trust edges posted")

    # Reviews, votes, and other events live on the SIGNER's own
    # stream (the Go node enforces "only the subject owner can append
    # to a stream"). The product/review being referenced is carried
    # in the payload. Aggregation scans across all reviewers'
    # streams and filters by `productAssetQuid`.
    print("\n[4] posting 5 reviews (each to the reviewer's own stream)...")
    reviews = [
        ("veteran",           4.5, "Solid laptop with minor keyboard quirks"),
        ("newbie",            2.0, "Not what I expected"),
        ("rev_alice_trusts",  4.8, "Best laptop I've used in years"),
        ("rev_bob_trusts",    3.5, "Middle of the road"),
        ("rev_random",        5.0, "AMAZING! BUY NOW!!!"),
    ]
    review_tx_ids: Dict[str, str] = {}
    for name, rating, title in reviews:
        result = emit_event(actors[name], actors[name].id, "QUID", "REVIEW", TOPIC, {
            "qrpVersion": 1, "rating": rating, "maxRating": 5.0,
            "productAssetQuid": product_id,
            "title": title, "bodyMarkdown": f"(demo review by {name})",
            "locale": "en-US",
        }, sequence=1)
        review_tx_ids[name] = result.get("transaction_id", "")
        print(f"    {name:<20} {rating} stars — \"{title}\"")

    print("\n[5] emitting helpfulness votes (each on the voter's stream)...")
    for v in upvoters:
        emit_event(v, v.id, "QUID", "HELPFUL_VOTE", TOPIC, {
            "qrpVersion": 1, "productAssetQuid": product_id,
            "reviewerQuid": actors["veteran"].id,
            "reviewTxId": review_tx_ids.get("veteran", ""),
        }, sequence=1)
    for v in downvoters:
        emit_event(v, v.id, "QUID", "UNHELPFUL_VOTE", TOPIC, {
            "qrpVersion": 1, "productAssetQuid": product_id,
            "reviewerQuid": actors["rev_random"].id,
            "reviewTxId": review_tx_ids.get("rev_random", ""),
        }, sequence=1)
    print(f"    {len(upvoters)} HELPFUL → veteran")
    print(f"    {len(downvoters)} UNHELPFUL → suspicious 5-star")

    print("\n[6] simulating veteran activity (10 more reviews)...")
    # Wait for the veteran's first review to commit before chaining
    # any more sequences onto that stream.
    _wait_for_stream_seq(actors["veteran"].id, 1)
    n_filler = 10
    seq = 2  # veteran already used sequence 1 for the product review
    for i in range(n_filler):
        emit_event(actors["veteran"], actors["veteran"].id, "QUID", "REVIEW", TOPIC, {
            "qrpVersion": 1, "rating": 4.0, "maxRating": 5.0,
            "productAssetQuid": f"filler-product-{i}",  # different product
            "title": f"other review {i}",
            "bodyMarkdown": "(activity filler)",
        }, sequence=seq)
        # Wait for commit before the next sequence — otherwise the
        # node rejects seq+1 because the registered stream head is
        # still at seq-1.
        _wait_for_stream_seq(actors["veteran"].id, seq)
        seq += 1
    print(f"    ok — {n_filler} filler reviews (veteran activity score boost)")

    print("\n[7] waiting for final block commit before computing ratings...")
    reviewer_quids = [actors[name].id for name, _, _ in reviews]
    voter_quids = [v.id for v in voters]
    for rid in reviewer_quids:
        _wait_for_stream_seq(rid, 1)
    for vid in voter_quids:
        _wait_for_stream_seq(vid, 1)
    print("    ok — all event streams committed\n")

    unweighted = sum(r[1] for r in reviews) / len(reviews)
    alice_view = effective_rating(actors["alice"].id, product_id, TOPIC,
                                  reviewer_quids, voter_quids)
    bob_view   = effective_rating(actors["bob"].id,   product_id, TOPIC,
                                  reviewer_quids, voter_quids)
    carol_view = effective_rating(actors["carol"].id, product_id, TOPIC,
                                  reviewer_quids, voter_quids)

    print(f"  {'Observer':<35}{'Rating':<14}{'Contributing'}")
    print("  " + "-" * 58)
    print(f"  {'Classic unweighted average':<35}{unweighted:<14.2f}{len(reviews)}")
    print(f"  {'Alice (techie)':<35}{_fmt(alice_view['rating']):<14}"
          f"{alice_view['contributing']}/{alice_view['total_considered']}")
    print(f"  {'Bob (skeptic)':<35}{_fmt(bob_view['rating']):<14}"
          f"{bob_view['contributing']}/{bob_view['total_considered']}")
    print(f"  {'Carol (restaurant reviewer)':<35}{_fmt(carol_view['rating']):<14}"
          f"{carol_view['contributing']}/{carol_view['total_considered']}")

    print("\n[8] Alice's full contribution breakdown:\n")
    print(f"  {'Reviewer':<22}{'Raw':<7}{'Weight':<10}{'T':<7}{'H':<7}{'A':<7}{'R':<7}")
    print("  " + "-" * 64)
    reverse_lookup = {q.id: name for name, q in actors.items()}
    for c in alice_view["contributions"]:
        name = reverse_lookup.get(c["reviewer"], c["reviewer"][:8])
        print(f"  {name:<22}{c['rating']:<7.1f}{c['weight']:<10.3f}"
              f"{c['t']:<7.2f}{c['h']:<7.2f}{c['a']:<7.2f}{c['r']:<7.2f}")

    state = {
        "node": NODE, "topic": TOPIC, "product": product_id,
        "reviews": [
            {"reviewer": name, "reviewerId": actors[name].id,
             "rating": rating, "title": title}
            for name, rating, title in reviews
        ],
        "observers": {
            name: {"id": actors[name].id, "description": desc, "view": view}
            for name, desc, view in [
                ("alice", "Tech-savvy, direct trust to veteran + 5 voters", alice_view),
                ("bob",   "Skeptic, narrower trust, 0.6 on veteran",        bob_view),
                ("carol", "Restaurant reviewer — no tech network",          carol_view),
            ]
        },
        "reverse_lookup": reverse_lookup,
        "unweighted": unweighted,
    }
    out = Path("/tmp/demo-state.json")
    out.write_text(json.dumps(state, indent=2, default=str), encoding="utf-8")
    repo_copy = Path(__file__).parent / "demo-state.json"
    repo_copy.write_text(json.dumps(state, indent=2, default=str), encoding="utf-8")
    print(f"\n[9] wrote state → {out} and {repo_copy}")

    print("\n" + "=" * 70)
    print(" DEMO COMPLETE — open examples/reviews-and-comments/demo/index.html")
    print("=" * 70)

    signer().close()


def _fmt(v):
    return "— (no basis)" if v is None else f"{v:.2f}"


if __name__ == "__main__":
    main()
