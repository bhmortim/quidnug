"""Agent capability authorization logic (standalone, no SDK dep).

The core question: given a proposed action by an AI agent, should
it proceed? The decision has four inputs:

  1. The agent's *capability grants* (domain-scoped, time-bounded
     trust edges from a principal or org).
  2. The *risk class* the agent's own classifier assigned to the
     action.
  3. The *cosignatures* collected from the agent's guardian set.
  4. Any *vetoes* published by a guardian.

Outputs:
  - AuthzDecision: authorized | pending | denied, with reasons.

Semantics match the use case doc's risk-class routing:

    trivial     agent self-signs
    low-routine agent + 1 cosigner (audit bot auto-cosigns)
    medium      cosigners must meet threshold weight
    high        cosigners must meet threshold; any veto blocks
    emergency   safety committee alone (weight 2+)

Everything is pure data + functions, exercisable without a node.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Dict, List, Optional

# ---------------------------------------------------------------------------
# Domain model
# ---------------------------------------------------------------------------

VALID_RISK_CLASSES = frozenset(
    {"trivial", "low-routine", "medium", "high", "emergency"}
)


@dataclass(frozen=True)
class CapabilityGrant:
    """A time-bounded, domain-scoped authorization a principal
    has extended to the agent.

    Equivalent on the node to a TRUST transaction with a
    ``validUntil`` bound and a ``maxAmount`` attribute.
    """

    truster_quid: str
    agent_quid: str
    domain: str
    max_amount_cents: int
    valid_until_unix: int
    description: str = ""

    def covers(self, action: "AgentAction", now_unix: int) -> bool:
        if self.valid_until_unix < now_unix:
            return False
        if action.domain != self.domain:
            return False
        if action.amount_cents > self.max_amount_cents:
            return False
        return True


@dataclass(frozen=True)
class AgentAction:
    """A proposed action the agent wants to execute."""

    action_id: str
    agent_quid: str
    domain: str
    action_type: str                  # "wire.send", "contract.sign", etc.
    amount_cents: int
    risk_class: str                   # one of VALID_RISK_CLASSES
    proposed_at_unix: int
    target: str = ""
    reason: str = ""


@dataclass(frozen=True)
class GuardianWeight:
    """One entry in the agent's guardian set with its voting weight."""

    guardian_quid: str
    weight: int
    role: str = ""  # "principal" | "safety-committee" | "audit-bot" | "other"


@dataclass(frozen=True)
class GuardianSet:
    """The agent's guardian set: cosigning committee for elevated actions."""

    agent_quid: str
    members: List[GuardianWeight]
    threshold: int
    # When True, safety-committee members can alone authorize
    # emergency-class actions.
    safety_committee_alone_ok: bool = True

    def weight_of(self, guardian_quid: str) -> int:
        for m in self.members:
            if m.guardian_quid == guardian_quid:
                return m.weight
        return 0

    def safety_committee_weight(self) -> int:
        return sum(
            m.weight for m in self.members if m.role == "safety-committee"
        )


@dataclass(frozen=True)
class Cosignature:
    """One guardian has cosigned the action."""

    guardian_quid: str
    action_id: str
    cosigned_at_unix: int


@dataclass(frozen=True)
class Veto:
    """A guardian has vetoed the action. Blocks regardless of
    cosignature weight."""

    guardian_quid: str
    action_id: str
    reason: str
    vetoed_at_unix: int


# ---------------------------------------------------------------------------
# Decision output
# ---------------------------------------------------------------------------

VERDICT_AUTHORIZED = "authorized"
VERDICT_PENDING = "pending"
VERDICT_DENIED = "denied"


@dataclass
class AuthzDecision:
    verdict: str                       # one of the VERDICT_* constants
    action_id: str
    reasons: List[str] = field(default_factory=list)
    collected_weight: int = 0
    required_weight: int = 0
    grant_used: Optional[CapabilityGrant] = None

    def short(self) -> str:
        return (
            f"{self.verdict.upper():11s} "
            f"action={self.action_id} "
            f"weight={self.collected_weight}/{self.required_weight}"
        )


# ---------------------------------------------------------------------------
# Risk-class routing
# ---------------------------------------------------------------------------

def _required_weight(risk_class: str, gs: GuardianSet) -> int:
    """Return the cosigner weight needed to authorize an action
    of this risk class. 0 means the agent can self-authorize.

    Semantics match use case doc's table but parameterized by
    the agent's actual guardian-set threshold so deployments
    can tune.
    """
    if risk_class == "trivial":
        return 0
    if risk_class == "low-routine":
        return 1
    if risk_class == "medium":
        return gs.threshold
    if risk_class == "high":
        return gs.threshold
    if risk_class == "emergency":
        return gs.safety_committee_weight() or gs.threshold
    raise ValueError(f"unknown risk class: {risk_class}")


# ---------------------------------------------------------------------------
# Main entry point
# ---------------------------------------------------------------------------

def evaluate_authorization(
    action: AgentAction,
    grants: List[CapabilityGrant],
    guardian_set: GuardianSet,
    cosignatures: List[Cosignature],
    vetoes: List[Veto],
    now_unix: int,
) -> AuthzDecision:
    """Pure decision function.

    Steps:
      1. Validate risk class.
      2. Vetoes always win. Any veto on this action ID denies.
      3. Find a covering grant. No covering grant denies.
      4. Compute required cosigner weight from risk class.
      5. Sum weights of cosignatures from valid guardians.
      6. authorized if collected >= required; pending otherwise.

    The caller is responsible for making sure only guardians in
    the current guardian set submitted the cosignatures (the
    node enforces signatures; this function enforces weights).
    """
    if action.risk_class not in VALID_RISK_CLASSES:
        raise ValueError(f"invalid risk class: {action.risk_class}")

    # Step 2: vetoes.
    applicable_vetoes = [
        v for v in vetoes
        if v.action_id == action.action_id
        and guardian_set.weight_of(v.guardian_quid) > 0
    ]
    if applicable_vetoes:
        v = applicable_vetoes[0]
        return AuthzDecision(
            verdict=VERDICT_DENIED,
            action_id=action.action_id,
            reasons=[f"vetoed by {v.guardian_quid}: {v.reason}"],
        )

    # Step 3: capability grant.
    covering = [g for g in grants if g.covers(action, now_unix)]
    if not covering:
        return AuthzDecision(
            verdict=VERDICT_DENIED,
            action_id=action.action_id,
            reasons=[
                f"no valid capability grant covers "
                f"domain={action.domain} amount={action.amount_cents}"
            ],
        )
    grant = covering[0]

    # Step 4: required weight from risk class.
    required = _required_weight(action.risk_class, guardian_set)

    # Step 5: collected weight.
    # Deduplicate by guardian_quid (same guardian cosigning
    # twice doesn't double-count).
    seen = set()
    collected = 0
    contributors: List[str] = []
    for cs in cosignatures:
        if cs.action_id != action.action_id:
            continue
        if cs.guardian_quid in seen:
            continue
        w = guardian_set.weight_of(cs.guardian_quid)
        if w <= 0:
            continue
        seen.add(cs.guardian_quid)
        collected += w
        contributors.append(f"{cs.guardian_quid}(w={w})")

    reasons = [
        f"grant: {grant.description or 'unnamed grant'} "
        f"(max ${grant.max_amount_cents/100:.2f}, valid until {grant.valid_until_unix})",
        f"risk class: {action.risk_class}",
    ]
    if contributors:
        reasons.append("cosigners: " + ", ".join(contributors))

    # Step 6: verdict.
    if collected >= required:
        reasons.append(
            f"authorized: weight {collected} >= required {required}"
        )
        return AuthzDecision(
            verdict=VERDICT_AUTHORIZED,
            action_id=action.action_id,
            reasons=reasons,
            collected_weight=collected,
            required_weight=required,
            grant_used=grant,
        )

    reasons.append(
        f"pending: weight {collected} below required {required}"
    )
    return AuthzDecision(
        verdict=VERDICT_PENDING,
        action_id=action.action_id,
        reasons=reasons,
        collected_weight=collected,
        required_weight=required,
        grant_used=grant,
    )


# ---------------------------------------------------------------------------
# Event-stream helpers: extract authz-relevant data from the
# agent's event stream.
# ---------------------------------------------------------------------------

def extract_cosignatures(events: List[dict]) -> List[Cosignature]:
    """Scan an event stream and return all ``agent.action.cosigned``
    events as Cosignature records."""
    out: List[Cosignature] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "agent.action.cosigned":
            continue
        p = ev.get("payload") or {}
        out.append(Cosignature(
            guardian_quid=p.get("signerQuid", ""),
            action_id=p.get("cosigns", ""),
            cosigned_at_unix=int(p.get("cosignedAt") or ev.get("timestamp") or 0),
        ))
    return out


def extract_vetoes(events: List[dict]) -> List[Veto]:
    """Scan an event stream and return all ``agent.action.vetoed``
    events as Veto records."""
    out: List[Veto] = []
    for ev in events:
        et = ev.get("eventType") or ev.get("event_type") or ""
        if et != "agent.action.vetoed":
            continue
        p = ev.get("payload") or {}
        out.append(Veto(
            guardian_quid=p.get("signerQuid", ""),
            action_id=p.get("vetoes", ""),
            reason=p.get("reason", ""),
            vetoed_at_unix=int(p.get("vetoedAt") or ev.get("timestamp") or 0),
        ))
    return out
