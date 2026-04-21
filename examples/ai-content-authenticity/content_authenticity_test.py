"""Tests for content_authenticity.py. No Quidnug node required."""

import pytest

from content_authenticity import (
    AuthenticityPolicy,
    AuthenticityVerdict,
    MediaAssetV1,
    ORIGIN_CAPTURED,
    ORIGIN_GENERATED,
    ProvenanceEvent,
    evaluate_authenticity,
    extract_provenance,
)


NOW = 1_700_000_000


def _asset(
    asset_id: str = "photo-1",
    origin: str = ORIGIN_CAPTURED,
    creator: str = "canon-camera-1",
) -> MediaAssetV1:
    return MediaAssetV1(
        asset_id=asset_id, asset_type="photo", origin=origin,
        creator_quid=creator, original_hash="hash-raw",
        created_at_unix=NOW,
    )


def _chain(
    include_pub: bool = True, include_fact: bool = False,
    editor: str = "editor-reuters", publisher: str = "reuters",
    checker: str = "factcheck-maldita",
    tamper: bool = False,
) -> list:
    out = [
        ProvenanceEvent("media.captured", "canon-camera-1",
                         input_hash="", output_hash="hash-raw",
                         timestamp_unix=NOW),
        ProvenanceEvent("media.cropped", editor,
                         input_hash="hash-raw",
                         output_hash="hash-after-crop" if not tamper else "BOGUS",
                         timestamp_unix=NOW + 60),
        ProvenanceEvent("media.color-graded", editor,
                         input_hash="hash-after-crop",
                         output_hash="hash-after-grade",
                         timestamp_unix=NOW + 120),
    ]
    if include_pub:
        out.append(ProvenanceEvent(
            "media.published", publisher,
            input_hash="", output_hash="",
            timestamp_unix=NOW + 180,
        ))
    if include_fact:
        out.append(ProvenanceEvent(
            "media.fact-checked", checker,
            input_hash="", output_hash="",
            timestamp_unix=NOW + 200,
        ))
    return out


def _trust(mapping):
    return lambda obs, target: mapping.get((obs, target), 0.0)


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

def test_full_chain_with_trusted_parties_accepts():
    trust = _trust({
        ("newsroom", "canon-camera-1"):  0.95,
        ("newsroom", "editor-reuters"):  0.9,
        ("newsroom", "reuters"):          0.95,
    })
    v = evaluate_authenticity("newsroom", _asset(), _chain(), trust)
    assert v.verdict == "accept"


def test_fact_check_bonus_boosts_score():
    trust = _trust({
        ("newsroom", "canon-camera-1"):     0.7,
        ("newsroom", "editor-reuters"):     0.7,
        ("newsroom", "reuters"):             0.7,
        ("newsroom", "factcheck-maldita"): 0.9,
    })
    # Without fact-check: base 0.7 vs min 0.6 -> accept but tight.
    no_fc = evaluate_authenticity(
        "newsroom", _asset(), _chain(include_fact=False), trust,
    )
    # With fact-check: +0.1 bonus -> 0.8.
    with_fc = evaluate_authenticity(
        "newsroom", _asset(), _chain(include_fact=True), trust,
    )
    assert no_fc.verdict == "accept"
    assert with_fc.verdict == "accept"
    assert with_fc.trust.overall_score > no_fc.trust.overall_score


# ---------------------------------------------------------------------------
# Weak-link semantics
# ---------------------------------------------------------------------------

def test_single_weak_editor_taints_chain():
    trust = _trust({
        ("newsroom", "canon-camera-1"):  0.95,
        ("newsroom", "editor-reuters"):  0.3,   # the weak link
        ("newsroom", "reuters"):          0.95,
    })
    v = evaluate_authenticity("newsroom", _asset(), _chain(), trust)
    assert v.verdict == "reject"
    assert v.trust.edit_trust_min < 0.6


def test_no_edit_events_still_authenticates_on_capture():
    trust = _trust({
        ("newsroom", "canon-camera-1"):  0.95,
        ("newsroom", "reuters"):          0.9,
    })
    chain = [
        ProvenanceEvent("media.captured", "canon-camera-1",
                         input_hash="", output_hash="hash-raw",
                         timestamp_unix=NOW),
        ProvenanceEvent("media.published", "reuters",
                         input_hash="", output_hash="",
                         timestamp_unix=NOW + 60),
    ]
    v = evaluate_authenticity("newsroom", _asset(), chain, trust)
    assert v.verdict == "accept"


# ---------------------------------------------------------------------------
# Hash chain integrity
# ---------------------------------------------------------------------------

def test_broken_hash_chain_rejects():
    trust = _trust({
        ("newsroom", "canon-camera-1"):  0.95,
        ("newsroom", "editor-reuters"):  0.9,
        ("newsroom", "reuters"):          0.95,
    })
    v = evaluate_authenticity("newsroom", _asset(), _chain(tamper=True), trust)
    assert v.verdict == "reject"
    assert "hash chain" in v.reasons[0].lower()


# ---------------------------------------------------------------------------
# AI-generated content
# ---------------------------------------------------------------------------

def test_ai_generated_warns_by_default():
    asset = _asset(origin=ORIGIN_GENERATED, creator="image-model-v2")
    # Chain starts with media.generated instead of captured.
    chain = [
        ProvenanceEvent("media.generated", "image-model-v2",
                         input_hash="", output_hash="hash-raw",
                         timestamp_unix=NOW),
        ProvenanceEvent("media.published", "news-aggregator",
                         input_hash="", output_hash="",
                         timestamp_unix=NOW + 10),
    ]
    trust = _trust({
        ("newsroom", "image-model-v2"):   0.9,
        ("newsroom", "news-aggregator"):  0.9,
    })
    v = evaluate_authenticity("newsroom", asset, chain, trust)
    assert v.verdict == "warn"
    assert v.ai_generated is True


def test_ai_generated_can_be_rejected_by_policy():
    asset = _asset(origin=ORIGIN_GENERATED, creator="image-model-v2")
    chain = [
        ProvenanceEvent("media.generated", "image-model-v2",
                         input_hash="", output_hash="hash-raw",
                         timestamp_unix=NOW),
        ProvenanceEvent("media.published", "news-aggregator",
                         input_hash="", output_hash="",
                         timestamp_unix=NOW + 10),
    ]
    trust = _trust({
        ("newsroom", "image-model-v2"):   0.95,
        ("newsroom", "news-aggregator"):  0.95,
    })
    policy = AuthenticityPolicy(ai_generated_verdict="reject")
    v = evaluate_authenticity("newsroom", asset, chain, trust, policy)
    assert v.verdict == "reject"


def test_ai_generated_can_be_accepted_by_policy():
    asset = _asset(origin=ORIGIN_GENERATED, creator="image-model-v2")
    chain = [
        ProvenanceEvent("media.generated", "image-model-v2",
                         input_hash="", output_hash="hash-raw",
                         timestamp_unix=NOW),
        ProvenanceEvent("media.published", "news-aggregator",
                         input_hash="", output_hash="",
                         timestamp_unix=NOW + 10),
    ]
    trust = _trust({
        ("newsroom", "image-model-v2"):   0.95,
        ("newsroom", "news-aggregator"):  0.95,
    })
    policy = AuthenticityPolicy(ai_generated_verdict="accept")
    v = evaluate_authenticity("newsroom", asset, chain, trust, policy)
    assert v.verdict == "accept"


# ---------------------------------------------------------------------------
# Publisher requirement
# ---------------------------------------------------------------------------

def test_missing_publisher_rejects_by_default():
    trust = _trust({
        ("newsroom", "canon-camera-1"): 0.95,
        ("newsroom", "editor-reuters"): 0.9,
    })
    v = evaluate_authenticity(
        "newsroom", _asset(), _chain(include_pub=False), trust,
    )
    assert v.verdict == "reject"
    assert "no publisher" in v.reasons[-1].lower()


def test_publisher_not_required_policy_allows_no_publisher():
    trust = _trust({
        ("newsroom", "canon-camera-1"): 0.95,
        ("newsroom", "editor-reuters"): 0.9,
    })
    policy = AuthenticityPolicy(require_publisher=False)
    v = evaluate_authenticity(
        "newsroom", _asset(), _chain(include_pub=False), trust, policy,
    )
    assert v.verdict == "accept"


# ---------------------------------------------------------------------------
# Trust out of range
# ---------------------------------------------------------------------------

def test_trust_out_of_range_raises():
    trust = _trust({("newsroom", "canon-camera-1"): 1.5})
    with pytest.raises(ValueError):
        evaluate_authenticity("newsroom", _asset(), _chain(), trust)


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def test_extract_provenance_from_stream():
    stream = [
        {"eventType": "media.captured",
         "payload": {"captureDevice": "canon-1", "outputHash": "hash-raw"},
         "timestamp": NOW},
        {"eventType": "media.cropped",
         "payload": {"editor": "photog-jane",
                      "inputHash": "hash-raw",
                      "outputHash": "hash-crop",
                      "software": "Lightroom 13"},
         "timestamp": NOW + 60},
        {"eventType": "media.published",
         "payload": {"publisher": "reuters"},
         "timestamp": NOW + 120},
        {"eventType": "media.fact-checked",
         "payload": {"checker": "factcheck-x"},
         "timestamp": NOW + 180},
        # Event with unrelated prefix -- should be ignored.
        {"eventType": "wire.cosigned", "payload": {}, "timestamp": NOW + 200},
    ]
    out = extract_provenance(stream)
    assert len(out) == 4
    assert out[0].step_type == "media.captured"
    assert out[0].actor_quid == "canon-1"
    assert out[1].actor_quid == "photog-jane"
    assert out[1].software == "Lightroom 13"
    assert out[2].actor_quid == "reuters"
    assert out[3].actor_quid == "factcheck-x"
