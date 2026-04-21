"""AI content authenticity evaluation (standalone, no SDK dep).

A consumer of news or social media wants to answer: is this
photo / video / clip authentic and traceable to a trustworthy
source? Given a signed title plus a stream of provenance events
on that title, the answer is a blended score across:

  - The capture trust: how well-trusted the camera manufacturer
    and photographer are. (For AI-generated content, there is
    no camera; the generator model replaces the camera.)
  - The edit trust: the MINIMUM trust across any editor in the
    chain. One weak editor taints the asset.
  - The publisher trust: how well-trusted the final publisher is.
  - An optional fact-check bonus when a trusted fact-checker has
    explicitly endorsed.

Hash continuity: each edit event claims an input_hash and
output_hash. The input_hash must match the prior event's
output_hash, or the chain is broken (tampered-in-transit).

This module is pure policy. It does not fetch anything; the
caller provides the signed title, the list of provenance events,
and a trust-path function.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, List, Optional


# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

ORIGIN_CAPTURED = "captured"
ORIGIN_GENERATED = "generated"


@dataclass(frozen=True)
class MediaAssetV1:
    """The on-chain title metadata for a media asset."""

    asset_id: str
    asset_type: str                 # "photo" | "video" | "audio"
    origin: str                     # ORIGIN_CAPTURED or ORIGIN_GENERATED
    creator_quid: str               # camera or generator-model quid
    original_hash: str              # sha256 of the original capture/generation
    created_at_unix: int = 0

    def is_ai_generated(self) -> bool:
        return self.origin == ORIGIN_GENERATED


@dataclass(frozen=True)
class ProvenanceEvent:
    """One step in the editing chain."""

    step_type: str                  # "media.captured" | "media.cropped" | ...
    actor_quid: str
    input_hash: str                 # hash of prior state (or empty on first)
    output_hash: str
    software: str = ""
    timestamp_unix: int = 0

    @property
    def is_capture_or_generation(self) -> bool:
        return self.step_type in ("media.captured", "media.generated")

    @property
    def is_publication(self) -> bool:
        return self.step_type == "media.published"

    @property
    def is_fact_check(self) -> bool:
        return self.step_type == "media.fact-checked"

    @property
    def is_edit(self) -> bool:
        return not (
            self.is_capture_or_generation
            or self.is_publication
            or self.is_fact_check
        )


@dataclass
class TrustBreakdown:
    capture_trust: float = 0.0
    edit_trust_min: float = 1.0     # no edits -> no degradation
    publisher_trust: float = 0.0
    fact_check_bonus: float = 0.0
    overall_score: float = 0.0
    edit_count: int = 0
    publisher_present: bool = False
    fact_checker_present: bool = False


@dataclass
class AuthenticityVerdict:
    verdict: str                    # "accept" | "warn" | "reject"
    asset_id: str
    trust: TrustBreakdown = field(default_factory=TrustBreakdown)
    reasons: List[str] = field(default_factory=list)
    ai_generated: bool = False

    def short(self) -> str:
        return (
            f"{self.verdict.upper():7s} asset={self.asset_id} "
            f"overall={self.trust.overall_score:.3f} "
            f"{'(AI)' if self.ai_generated else ''}"
        )


TrustFn = Callable[[str, str], float]


@dataclass
class AuthenticityPolicy:
    """Consumer knobs."""

    min_overall_score: float = 0.6
    ai_generated_verdict: str = "warn"   # "accept" | "warn" | "reject"
    require_publisher: bool = True
    fact_check_bonus_trust_min: float = 0.8
    fact_check_bonus_amount: float = 0.1


# ---------------------------------------------------------------------------
# Hash-chain check
# ---------------------------------------------------------------------------

def _hash_chain_broken(
    asset: MediaAssetV1, events: List[ProvenanceEvent],
) -> Optional[str]:
    """Return a human description if the chain is broken, else None."""
    expected = asset.original_hash
    for ev in events:
        # Capture/generation event's output_hash should match the asset's
        # original_hash; we accept either the asset's original_hash or the
        # event's own output_hash as the starting state.
        if ev.is_capture_or_generation:
            if ev.output_hash and ev.output_hash != expected:
                return (
                    f"capture event {ev.step_type} output hash "
                    f"{ev.output_hash[:12]} != asset original_hash "
                    f"{expected[:12]}"
                )
            continue
        # Publication and fact-check don't transform the content.
        if ev.is_publication or ev.is_fact_check:
            continue
        # Edit event: must chain.
        if ev.input_hash and ev.input_hash != expected:
            return (
                f"edit {ev.step_type} input_hash {ev.input_hash[:12]} "
                f"!= expected {expected[:12]}"
            )
        if ev.output_hash:
            expected = ev.output_hash
    return None


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_authenticity(
    observer: str,
    asset: MediaAssetV1,
    events: List[ProvenanceEvent],
    trust_fn: TrustFn,
    policy: Optional[AuthenticityPolicy] = None,
) -> AuthenticityVerdict:
    """Pure decision function."""
    p = policy or AuthenticityPolicy()
    reasons: List[str] = []

    # Step 1: hash chain integrity.
    broken = _hash_chain_broken(asset, events)
    if broken:
        return AuthenticityVerdict(
            verdict="reject",
            asset_id=asset.asset_id,
            reasons=[f"hash chain broken: {broken}"],
            ai_generated=asset.is_ai_generated(),
        )
    reasons.append("hash chain intact")

    tb = TrustBreakdown()

    # Step 2: capture/generation trust.
    tb.capture_trust = _checked_trust(trust_fn, observer, asset.creator_quid)
    reasons.append(
        f"{'generator' if asset.is_ai_generated() else 'capture'} "
        f"trust = {tb.capture_trust:.3f}"
    )

    # Step 3: edit trust (minimum across all editors).
    edit_events = [ev for ev in events if ev.is_edit]
    tb.edit_count = len(edit_events)
    if edit_events:
        for ev in edit_events:
            t = _checked_trust(trust_fn, observer, ev.actor_quid)
            if t < tb.edit_trust_min:
                tb.edit_trust_min = t
        reasons.append(
            f"edit_trust_min across {len(edit_events)} editor(s) = "
            f"{tb.edit_trust_min:.3f}"
        )
    else:
        reasons.append("no edit events")

    # Step 4: publisher trust.
    pub_events = [ev for ev in events if ev.is_publication]
    if pub_events:
        pub = pub_events[-1]
        tb.publisher_trust = _checked_trust(trust_fn, observer, pub.actor_quid)
        tb.publisher_present = True
        reasons.append(f"publisher trust = {tb.publisher_trust:.3f}")
    elif p.require_publisher:
        return AuthenticityVerdict(
            verdict="reject",
            asset_id=asset.asset_id,
            trust=tb,
            reasons=reasons + ["no publisher event (required by policy)"],
            ai_generated=asset.is_ai_generated(),
        )

    # Step 5: fact-check bonus.
    for ev in events:
        if ev.is_fact_check:
            ft = _checked_trust(trust_fn, observer, ev.actor_quid)
            tb.fact_checker_present = True
            if ft >= p.fact_check_bonus_trust_min:
                tb.fact_check_bonus = p.fact_check_bonus_amount
                reasons.append(
                    f"fact-checker {ev.actor_quid[:12]} trust "
                    f"{ft:.3f} (+{tb.fact_check_bonus:.2f} bonus)"
                )
            else:
                reasons.append(
                    f"fact-checker {ev.actor_quid[:12]} trust {ft:.3f} "
                    f"below bonus floor {p.fact_check_bonus_trust_min}"
                )
            break

    # Step 6: overall = min of inputs + fact-check bonus.
    components = [tb.capture_trust]
    if tb.edit_count > 0:
        components.append(tb.edit_trust_min)
    if tb.publisher_present:
        components.append(tb.publisher_trust)
    base = min(components)
    tb.overall_score = min(1.0, base + tb.fact_check_bonus)
    reasons.append(
        f"overall = min({[f'{c:.3f}' for c in components]}) "
        f"+ bonus {tb.fact_check_bonus:.3f} = {tb.overall_score:.3f}"
    )

    # Step 7: AI-generation policy.
    if asset.is_ai_generated():
        if p.ai_generated_verdict == "reject":
            return AuthenticityVerdict(
                verdict="reject",
                asset_id=asset.asset_id,
                trust=tb,
                reasons=reasons + ["policy: AI-generated content rejected"],
                ai_generated=True,
            )
        if p.ai_generated_verdict == "warn":
            return AuthenticityVerdict(
                verdict="warn",
                asset_id=asset.asset_id,
                trust=tb,
                reasons=reasons + [
                    "policy: AI-generated -> warn (even if score meets threshold)"
                ],
                ai_generated=True,
            )

    # Step 8: threshold check.
    if tb.overall_score < p.min_overall_score:
        return AuthenticityVerdict(
            verdict="reject",
            asset_id=asset.asset_id,
            trust=tb,
            reasons=reasons + [
                f"overall {tb.overall_score:.3f} below threshold "
                f"{p.min_overall_score}"
            ],
            ai_generated=asset.is_ai_generated(),
        )

    return AuthenticityVerdict(
        verdict="accept",
        asset_id=asset.asset_id,
        trust=tb,
        reasons=reasons + [
            f"accept: overall {tb.overall_score:.3f} >= threshold "
            f"{p.min_overall_score}"
        ],
        ai_generated=asset.is_ai_generated(),
    )


def _checked_trust(trust_fn: TrustFn, observer: str, target: str) -> float:
    t = trust_fn(observer, target)
    if t < 0.0 or t > 1.0:
        raise ValueError(f"trust out of range for {target}: {t}")
    return t


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def extract_provenance(events: List[dict]) -> List[ProvenanceEvent]:
    out: List[ProvenanceEvent] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if not et.startswith("media."):
            continue
        p = ev.get("payload") or {}
        actor = (
            p.get("editor")
            or p.get("publisher")
            or p.get("checker")
            or p.get("captureDevice")
            or p.get("generator")
            or p.get("actor")
            or p.get("signerQuid")
            or ""
        )
        out.append(ProvenanceEvent(
            step_type=et,
            actor_quid=actor,
            input_hash=p.get("inputHash", ""),
            output_hash=p.get("outputHash", ""),
            software=p.get("software", ""),
            timestamp_unix=int(ev.get("timestamp") or 0),
        ))
    return out
