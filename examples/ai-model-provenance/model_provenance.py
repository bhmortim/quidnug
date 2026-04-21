"""AI model provenance evaluation (standalone, no SDK dep).

An enterprise considering deploying an AI model wants to answer:

  - What datasets was this model trained on? Are their licenses
    compatible with my use?
  - Has any trusted safety-auditor evaluated it? What was the
    rating?
  - Is it a derivative of an authorized base model?
  - What independent benchmarks has it passed?
  - Do I trust the producer?

This module takes:
  - ModelV1: the model's on-chain title metadata
  - The model's event stream (training, safety, benchmark events)
  - The set of referenced dataset metadata
  - A trust_path function
  - A ModelPolicy describing the consumer's acceptance rules

And returns a ProvenanceVerdict (accept | warn | reject) with
a rich breakdown for audit.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, List, Optional


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

@dataclass(frozen=True)
class DatasetV1:
    dataset_id: str
    owner_quid: str
    license: str                    # "CC0" | "CC-BY" | "proprietary-licensed" | ...
    content_type: str = ""          # "web-text" | "code" | "images" | ...
    size_bytes: int = 0
    hash_hex: str = ""


@dataclass(frozen=True)
class ModelV1:
    model_id: str
    producer_quid: str
    architecture: str
    parameters: int
    weights_hash: str
    license: str = ""
    base_model_id: str = ""         # empty for foundation models
    training_dataset_ids: List[str] = field(default_factory=list)
    released_at_unix: int = 0

    def is_derivative(self) -> bool:
        return bool(self.base_model_id)


@dataclass(frozen=True)
class SafetyEval:
    evaluator_quid: str
    overall_rating: str             # "acceptable" | "concerning" | "failing"
    eval_hash: str = ""
    evaluated_at_unix: int = 0


@dataclass(frozen=True)
class BenchmarkResult:
    benchmark_org: str
    benchmark_id: str
    score: float
    verified: bool = False
    reported_at_unix: int = 0


@dataclass
class ProvenanceBreakdown:
    producer_trust: float = 0.0
    base_model_trust: float = 1.0   # default 1.0 for non-derivatives
    safety_evals_acceptable: int = 0
    safety_evals_failing: int = 0
    dataset_license_violations: List[str] = field(default_factory=list)
    benchmarks_trusted: int = 0
    overall_ok: bool = False


@dataclass
class ProvenanceVerdict:
    verdict: str                    # "accept" | "warn" | "reject"
    model_id: str
    breakdown: ProvenanceBreakdown = field(default_factory=ProvenanceBreakdown)
    reasons: List[str] = field(default_factory=list)

    def short(self) -> str:
        return (
            f"{self.verdict.upper():7s} model={self.model_id} "
            f"producer_trust={self.breakdown.producer_trust:.3f} "
            f"safety_ok={self.breakdown.safety_evals_acceptable}"
        )


TrustFn = Callable[[str, str], float]


@dataclass
class ModelPolicy:
    """Consumer's acceptance rules."""

    min_producer_trust: float = 0.6
    min_base_model_trust: float = 0.6
    require_safety_eval: bool = True
    min_trusted_safety_eval_trust: float = 0.7
    require_any_benchmark: bool = False
    min_trusted_benchmark_trust: float = 0.5
    # Licenses the consumer will not accept in the training set.
    prohibited_dataset_licenses: List[str] = field(
        default_factory=lambda: [
            "unknown", "scraped-no-license",
            "shadow-library", "pirated",
        ]
    )
    # If True, any "failing" safety evaluation is a hard reject,
    # even if an "acceptable" one also exists.
    strict_safety: bool = True


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def extract_safety_evals(events: List[dict]) -> List[SafetyEval]:
    out: List[SafetyEval] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "safety.evaluated":
            continue
        p = ev.get("payload") or {}
        out.append(SafetyEval(
            evaluator_quid=p.get("evaluatorQuid") or p.get("evaluatorOrg", ""),
            overall_rating=p.get("overallRating", "unknown"),
            eval_hash=p.get("evaluationHash", ""),
            evaluated_at_unix=int(p.get("evaluatedAt") or ev.get("timestamp") or 0),
        ))
    return out


def extract_benchmarks(events: List[dict]) -> List[BenchmarkResult]:
    out: List[BenchmarkResult] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "benchmark.reported":
            continue
        p = ev.get("payload") or {}
        out.append(BenchmarkResult(
            benchmark_org=p.get("benchmarkOrg", ""),
            benchmark_id=p.get("benchmarkId", ""),
            score=float(p.get("score") or 0.0),
            verified=bool(p.get("verified", False)),
            reported_at_unix=int(p.get("reportedAt") or ev.get("timestamp") or 0),
        ))
    return out


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_model_provenance(
    consumer: str,
    model: ModelV1,
    datasets: List[DatasetV1],
    events: List[dict],
    trust_fn: TrustFn,
    policy: Optional[ModelPolicy] = None,
) -> ProvenanceVerdict:
    """Pure decision function."""
    p = policy or ModelPolicy()
    reasons: List[str] = []
    bd = ProvenanceBreakdown()

    # Step 1: producer trust.
    bd.producer_trust = _checked_trust(trust_fn, consumer, model.producer_quid)
    reasons.append(f"producer trust = {bd.producer_trust:.3f}")
    if bd.producer_trust < p.min_producer_trust:
        return ProvenanceVerdict(
            verdict="reject",
            model_id=model.model_id,
            breakdown=bd,
            reasons=reasons + [
                f"producer trust below threshold {p.min_producer_trust}"
            ],
        )

    # Step 2: base model trust (if this is a derivative).
    if model.is_derivative():
        # For the POC we treat the base model's producer as the
        # target of the trust query. In a production deployment
        # we'd resolve the base model's own title and chase its
        # producer_quid.
        bd.base_model_trust = _checked_trust(
            trust_fn, consumer, model.base_model_id,
        )
        reasons.append(
            f"derivative of {model.base_model_id[:16]}, "
            f"base trust = {bd.base_model_trust:.3f}"
        )
        if bd.base_model_trust < p.min_base_model_trust:
            return ProvenanceVerdict(
                verdict="reject",
                model_id=model.model_id,
                breakdown=bd,
                reasons=reasons + [
                    f"base model trust below threshold {p.min_base_model_trust}"
                ],
            )

    # Step 3: dataset licensing.
    ds_by_id = {d.dataset_id: d for d in datasets}
    violations: List[str] = []
    for ref in model.training_dataset_ids:
        d = ds_by_id.get(ref)
        if d is None:
            violations.append(f"{ref}: dataset metadata not available")
            continue
        if d.license.lower() in [l.lower() for l in p.prohibited_dataset_licenses]:
            violations.append(f"{ref}: license '{d.license}' is prohibited")
    bd.dataset_license_violations = violations
    if violations:
        return ProvenanceVerdict(
            verdict="reject",
            model_id=model.model_id,
            breakdown=bd,
            reasons=reasons + [f"dataset license violations: {violations}"],
        )
    reasons.append(f"{len(model.training_dataset_ids)} datasets, licenses OK")

    # Step 4: safety evaluations.
    safety = extract_safety_evals(events)
    trusted_safety = []
    for se in safety:
        et = _checked_trust(trust_fn, consumer, se.evaluator_quid)
        if et >= p.min_trusted_safety_eval_trust:
            trusted_safety.append(se)
    bd.safety_evals_acceptable = sum(
        1 for se in trusted_safety if se.overall_rating == "acceptable"
    )
    bd.safety_evals_failing = sum(
        1 for se in trusted_safety if se.overall_rating == "failing"
    )
    reasons.append(
        f"safety: {bd.safety_evals_acceptable} acceptable, "
        f"{bd.safety_evals_failing} failing (from trusted evaluators)"
    )
    if p.strict_safety and bd.safety_evals_failing > 0:
        return ProvenanceVerdict(
            verdict="reject",
            model_id=model.model_id,
            breakdown=bd,
            reasons=reasons + ["strict safety: trusted evaluator returned failing"],
        )
    if p.require_safety_eval and bd.safety_evals_acceptable == 0:
        return ProvenanceVerdict(
            verdict="warn",
            model_id=model.model_id,
            breakdown=bd,
            reasons=reasons + [
                "no acceptable safety evaluation from a trusted evaluator"
            ],
        )

    # Step 5: benchmarks.
    benchmarks = extract_benchmarks(events)
    trusted_bench = [
        b for b in benchmarks
        if _checked_trust(trust_fn, consumer, b.benchmark_org)
        >= p.min_trusted_benchmark_trust
    ]
    bd.benchmarks_trusted = len(trusted_bench)
    reasons.append(f"benchmarks: {bd.benchmarks_trusted} from trusted orgs")
    if p.require_any_benchmark and bd.benchmarks_trusted == 0:
        return ProvenanceVerdict(
            verdict="warn",
            model_id=model.model_id,
            breakdown=bd,
            reasons=reasons + ["no benchmarks from trusted orgs"],
        )

    bd.overall_ok = True
    return ProvenanceVerdict(
        verdict="accept",
        model_id=model.model_id,
        breakdown=bd,
        reasons=reasons + ["all provenance gates passed"],
    )


def _checked_trust(trust_fn: TrustFn, observer: str, target: str) -> float:
    t = trust_fn(observer, target)
    if t < 0.0 or t > 1.0:
        raise ValueError(f"trust out of range for {target}: {t}")
    return t
