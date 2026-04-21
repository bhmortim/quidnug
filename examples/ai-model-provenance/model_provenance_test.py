"""Tests for model_provenance.py. No Quidnug node required."""

import pytest

from model_provenance import (
    BenchmarkResult,
    DatasetV1,
    ModelPolicy,
    ModelV1,
    ProvenanceVerdict,
    SafetyEval,
    evaluate_model_provenance,
    extract_benchmarks,
    extract_safety_evals,
)


NOW = 1_700_000_000


def _model(
    model_id: str = "model-acme-7b-v2",
    producer: str = "acme-ai",
    base_model: str = "",
    datasets: list = None,
) -> ModelV1:
    return ModelV1(
        model_id=model_id,
        producer_quid=producer,
        architecture="decoder-transformer",
        parameters=7_000_000_000,
        weights_hash="hash-weights-1234",
        license="Apache-2.0",
        base_model_id=base_model,
        training_dataset_ids=datasets or ["ds-commoncrawl", "ds-licensed"],
        released_at_unix=NOW,
    )


def _dataset(
    dataset_id: str, license: str = "CC0", owner: str = "curator",
) -> DatasetV1:
    return DatasetV1(
        dataset_id=dataset_id, owner_quid=owner, license=license,
        content_type="web-text", size_bytes=10_000_000_000,
        hash_hex="hash-ds",
    )


def _trust(mapping):
    return lambda obs, target: mapping.get((obs, target), 0.0)


def _full_datasets():
    return [_dataset("ds-commoncrawl", "CC0"),
            _dataset("ds-licensed", "proprietary-licensed")]


def _stream(safety_rating: str = "acceptable", benchmark: bool = True,
             safety_evaluator: str = "eval-mlcommons") -> list:
    out = []
    out.append({
        "eventType": "training.started",
        "payload": {"trainingDataRefs": ["ds-commoncrawl", "ds-licensed"]},
    })
    out.append({
        "eventType": "training.completed",
        "payload": {"totalFLOPs": 1.2e23},
    })
    out.append({
        "eventType": "safety.evaluated",
        "payload": {
            "evaluatorQuid": safety_evaluator,
            "overallRating": safety_rating,
            "evaluationHash": "hash-eval",
            "evaluatedAt": NOW + 3600,
        },
    })
    if benchmark:
        out.append({
            "eventType": "benchmark.reported",
            "payload": {
                "benchmarkOrg": "helm",
                "benchmarkId": "MMLU",
                "score": 0.78,
                "verified": True,
                "reportedAt": NOW + 7200,
            },
        })
    return out


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

def test_foundation_model_accepts():
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.9,
        ("enterprise", "helm"):            0.8,
    })
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), _stream(), trust,
    )
    assert v.verdict == "accept"
    assert v.breakdown.safety_evals_acceptable == 1


def test_derivative_model_with_trusted_base_accepts():
    trust = _trust({
        ("enterprise", "acme-finetune-ai"):  0.9,
        ("enterprise", "model-acme-7b-v2"):  0.85,
        ("enterprise", "eval-mlcommons"):    0.9,
        ("enterprise", "helm"):               0.8,
    })
    model = _model(
        model_id="finetune-acme-7b-v2-medical",
        producer="acme-finetune-ai",
        base_model="model-acme-7b-v2",
    )
    v = evaluate_model_provenance(
        "enterprise", model, _full_datasets(), _stream(), trust,
    )
    assert v.verdict == "accept"
    assert v.breakdown.base_model_trust == 0.85


# ---------------------------------------------------------------------------
# Producer trust
# ---------------------------------------------------------------------------

def test_low_producer_trust_rejects():
    trust = _trust({
        ("enterprise", "acme-ai"):        0.3,
        ("enterprise", "eval-mlcommons"): 0.9,
    })
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), _stream(), trust,
    )
    assert v.verdict == "reject"
    assert "producer trust below" in v.reasons[-1]


# ---------------------------------------------------------------------------
# Derivative base trust
# ---------------------------------------------------------------------------

def test_derivative_of_weak_base_rejects():
    trust = _trust({
        ("enterprise", "acme-finetune-ai"):  0.95,
        ("enterprise", "model-acme-7b-v2"):  0.3,   # weak base
        ("enterprise", "eval-mlcommons"):    0.9,
    })
    model = _model(
        producer="acme-finetune-ai",
        base_model="model-acme-7b-v2",
    )
    v = evaluate_model_provenance(
        "enterprise", model, _full_datasets(), _stream(), trust,
    )
    assert v.verdict == "reject"


# ---------------------------------------------------------------------------
# Dataset licensing
# ---------------------------------------------------------------------------

def test_prohibited_dataset_license_rejects():
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.9,
    })
    datasets = [_dataset("ds-commoncrawl", "CC0"),
                _dataset("ds-suspicious", "shadow-library")]
    model = _model(datasets=["ds-commoncrawl", "ds-suspicious"])
    v = evaluate_model_provenance(
        "enterprise", model, datasets, _stream(), trust,
    )
    assert v.verdict == "reject"
    assert len(v.breakdown.dataset_license_violations) == 1


def test_missing_dataset_metadata_rejects():
    """Model references a dataset we don't have metadata for.
    Policy says reject."""
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.9,
    })
    v = evaluate_model_provenance(
        "enterprise",
        _model(datasets=["ds-commoncrawl", "ds-mystery"]),
        [_dataset("ds-commoncrawl", "CC0")],  # ds-mystery missing
        _stream(), trust,
    )
    assert v.verdict == "reject"


# ---------------------------------------------------------------------------
# Safety
# ---------------------------------------------------------------------------

def test_no_safety_eval_warns():
    trust = _trust({
        ("enterprise", "acme-ai"): 0.9,
    })
    stream = [
        {"eventType": "training.completed", "payload": {}},
    ]
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), stream, trust,
    )
    assert v.verdict == "warn"


def test_untrusted_safety_evaluator_warns():
    """Safety eval exists but evaluator isn't trusted enough."""
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.2,   # below trust floor
    })
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), _stream(), trust,
    )
    # Since no trusted eval exists, warn.
    assert v.verdict == "warn"


def test_failing_safety_eval_rejects_strict():
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.9,
    })
    stream = _stream(safety_rating="failing")
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), stream, trust,
    )
    assert v.verdict == "reject"


def test_failing_safety_eval_non_strict_continues():
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.9,
    })
    stream = _stream(safety_rating="failing")
    policy = ModelPolicy(strict_safety=False, require_safety_eval=False)
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), stream, trust, policy,
    )
    assert v.verdict == "accept"


# ---------------------------------------------------------------------------
# Benchmarks
# ---------------------------------------------------------------------------

def test_no_benchmarks_warns_if_required():
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.9,
    })
    stream = _stream(benchmark=False)
    policy = ModelPolicy(require_any_benchmark=True)
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), stream, trust, policy,
    )
    assert v.verdict == "warn"


def test_benchmark_not_required_by_default():
    trust = _trust({
        ("enterprise", "acme-ai"):        0.9,
        ("enterprise", "eval-mlcommons"): 0.9,
    })
    stream = _stream(benchmark=False)
    v = evaluate_model_provenance(
        "enterprise", _model(), _full_datasets(), stream, trust,
    )
    assert v.verdict == "accept"


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def test_extract_safety_evals():
    evals = extract_safety_evals(_stream())
    assert len(evals) == 1
    assert evals[0].evaluator_quid == "eval-mlcommons"
    assert evals[0].overall_rating == "acceptable"


def test_extract_benchmarks():
    benchmarks = extract_benchmarks(_stream())
    assert len(benchmarks) == 1
    assert benchmarks[0].benchmark_org == "helm"
    assert benchmarks[0].verified is True
