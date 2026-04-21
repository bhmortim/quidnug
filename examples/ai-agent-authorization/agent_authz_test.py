"""Tests for agent_authz.py. No Quidnug node required."""

import pytest

from agent_authz import (
    AgentAction,
    AuthzDecision,
    CapabilityGrant,
    Cosignature,
    GuardianSet,
    GuardianWeight,
    VERDICT_AUTHORIZED,
    VERDICT_DENIED,
    VERDICT_PENDING,
    Veto,
    evaluate_authorization,
    extract_cosignatures,
    extract_vetoes,
)


NOW = 1_700_000_000
FAR_FUTURE = NOW + 86400 * 30
LONG_PAST = NOW - 86400 * 30


def _guardian_set(threshold: int = 2) -> GuardianSet:
    return GuardianSet(
        agent_quid="agent-1",
        members=[
            GuardianWeight("principal",        1, "principal"),
            GuardianWeight("safety-committee", 2, "safety-committee"),
            GuardianWeight("audit-bot",        1, "audit-bot"),
        ],
        threshold=threshold,
    )


def _grant(
    max_cents: int = 20_000_00,
    domain: str = "money.acme",
    valid_until: int = FAR_FUTURE,
) -> CapabilityGrant:
    return CapabilityGrant(
        truster_quid="principal",
        agent_quid="agent-1",
        domain=domain,
        max_amount_cents=max_cents,
        valid_until_unix=valid_until,
        description=f"up to ${max_cents/100} in {domain}",
    )


def _action(
    action_id: str = "act-1",
    amount_cents: int = 5_000_00,
    risk_class: str = "medium",
    domain: str = "money.acme",
) -> AgentAction:
    return AgentAction(
        action_id=action_id,
        agent_quid="agent-1",
        domain=domain,
        action_type="wire.send",
        amount_cents=amount_cents,
        risk_class=risk_class,
        proposed_at_unix=NOW,
    )


# ---------------------------------------------------------------------------
# Risk-class routing
# ---------------------------------------------------------------------------

def test_trivial_self_authorizes():
    """Trivial-class actions need no cosigners."""
    d = evaluate_authorization(
        _action(risk_class="trivial", amount_cents=100),
        [_grant()], _guardian_set(),
        cosignatures=[], vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_AUTHORIZED
    assert d.required_weight == 0


def test_low_routine_one_cosigner_suffices():
    """low-routine: one weight unit is enough. The audit bot
    auto-cosigns and that's sufficient."""
    d = evaluate_authorization(
        _action(risk_class="low-routine"),
        [_grant()], _guardian_set(),
        cosignatures=[Cosignature("audit-bot", "act-1", NOW)],
        vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_AUTHORIZED
    assert d.collected_weight == 1


def test_medium_meets_threshold():
    """medium: need total weight >= threshold (2)."""
    d = evaluate_authorization(
        _action(risk_class="medium"),
        [_grant()], _guardian_set(threshold=2),
        cosignatures=[
            Cosignature("principal", "act-1", NOW),
            Cosignature("audit-bot", "act-1", NOW),
        ],
        vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_AUTHORIZED
    assert d.collected_weight == 2


def test_medium_insufficient_is_pending():
    """medium with only 1 weight cosigning returns pending, not denied."""
    d = evaluate_authorization(
        _action(risk_class="medium"),
        [_grant()], _guardian_set(threshold=2),
        cosignatures=[Cosignature("principal", "act-1", NOW)],
        vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_PENDING
    assert d.collected_weight == 1
    assert d.required_weight == 2


def test_high_needs_threshold():
    """high class needs threshold weight, same as medium in
    this POC's simple policy."""
    d = evaluate_authorization(
        _action(risk_class="high", amount_cents=15_000_00),
        [_grant(max_cents=20_000_00)], _guardian_set(threshold=2),
        cosignatures=[
            Cosignature("safety-committee", "act-1", NOW),
        ],
        vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_AUTHORIZED
    # safety committee alone weighs 2, meets threshold.
    assert d.collected_weight == 2


def test_emergency_safety_committee_alone():
    """Emergency class: safety committee can alone authorize."""
    d = evaluate_authorization(
        _action(risk_class="emergency"),
        [_grant()], _guardian_set(threshold=3),
        cosignatures=[Cosignature("safety-committee", "act-1", NOW)],
        vetoes=[], now_unix=NOW,
    )
    # safety-committee has weight 2 which equals sum of safety
    # committee weights in the set, which is what emergency
    # resolves to.
    assert d.verdict == VERDICT_AUTHORIZED


# ---------------------------------------------------------------------------
# Vetoes
# ---------------------------------------------------------------------------

def test_veto_overrides_cosignatures():
    """Even if the cosigner weight meets threshold, a veto
    from a guardian denies the action."""
    d = evaluate_authorization(
        _action(risk_class="medium"),
        [_grant()], _guardian_set(threshold=2),
        cosignatures=[
            Cosignature("principal", "act-1", NOW),
            Cosignature("safety-committee", "act-1", NOW),
        ],
        vetoes=[Veto("audit-bot", "act-1", "anomalous-pattern", NOW)],
        now_unix=NOW,
    )
    assert d.verdict == VERDICT_DENIED
    assert "vetoed" in d.reasons[0].lower()


def test_veto_from_non_guardian_ignored():
    """A 'veto' from someone not in the guardian set is ignored."""
    d = evaluate_authorization(
        _action(risk_class="low-routine"),
        [_grant()], _guardian_set(),
        cosignatures=[Cosignature("audit-bot", "act-1", NOW)],
        vetoes=[Veto("random-quid", "act-1", "nah", NOW)],
        now_unix=NOW,
    )
    assert d.verdict == VERDICT_AUTHORIZED


# ---------------------------------------------------------------------------
# Capability grants
# ---------------------------------------------------------------------------

def test_expired_grant_denies():
    """An expired capability grant is ignored."""
    d = evaluate_authorization(
        _action(risk_class="trivial"),
        [_grant(valid_until=LONG_PAST)], _guardian_set(),
        cosignatures=[], vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_DENIED
    assert "no valid capability grant" in d.reasons[0]


def test_grant_domain_mismatch_denies():
    """An action in a different domain than any grant is denied."""
    d = evaluate_authorization(
        _action(risk_class="trivial", domain="code.acme-backend"),
        [_grant(domain="money.acme")], _guardian_set(),
        cosignatures=[], vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_DENIED


def test_grant_amount_exceeded_denies():
    """Action amount above the grant's cap is denied."""
    d = evaluate_authorization(
        _action(risk_class="trivial", amount_cents=30_000_00),
        [_grant(max_cents=20_000_00)], _guardian_set(),
        cosignatures=[], vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_DENIED


def test_multi_grant_picks_one_that_covers():
    """When multiple grants exist, any covering grant suffices."""
    d = evaluate_authorization(
        _action(risk_class="trivial", amount_cents=5_000_00,
                domain="money.acme"),
        [
            _grant(domain="code.acme-backend", max_cents=10_00),
            _grant(domain="money.acme", max_cents=10_000_00),
        ],
        _guardian_set(),
        cosignatures=[], vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_AUTHORIZED


# ---------------------------------------------------------------------------
# Duplicate and unknown cosigners
# ---------------------------------------------------------------------------

def test_duplicate_cosignatures_count_once():
    """Same guardian cosigning twice doesn't double-count."""
    d = evaluate_authorization(
        _action(risk_class="medium"),
        [_grant()], _guardian_set(threshold=2),
        cosignatures=[
            Cosignature("principal", "act-1", NOW),
            Cosignature("principal", "act-1", NOW + 1),
        ],
        vetoes=[], now_unix=NOW,
    )
    # principal alone = weight 1, below threshold 2 -> pending.
    assert d.verdict == VERDICT_PENDING
    assert d.collected_weight == 1


def test_non_guardian_cosignatures_ignored():
    """Cosignatures from quids not in the guardian set don't count."""
    d = evaluate_authorization(
        _action(risk_class="medium"),
        [_grant()], _guardian_set(threshold=2),
        cosignatures=[
            Cosignature("principal", "act-1", NOW),
            Cosignature("some-other-random", "act-1", NOW),
        ],
        vetoes=[], now_unix=NOW,
    )
    assert d.verdict == VERDICT_PENDING
    assert d.collected_weight == 1


def test_invalid_risk_class_raises():
    """An unknown risk class is a programming error, not a runtime decision."""
    with pytest.raises(ValueError):
        evaluate_authorization(
            _action(risk_class="galaxy-brain"),
            [_grant()], _guardian_set(),
            cosignatures=[], vetoes=[], now_unix=NOW,
        )


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def test_extract_cosignatures_from_event_stream():
    events = [
        {
            "eventType": "agent.action.proposed",
            "payload": {"actionId": "act-1"},
            "timestamp": NOW,
        },
        {
            "eventType": "agent.action.cosigned",
            "payload": {"signerQuid": "principal", "cosigns": "act-1"},
            "timestamp": NOW + 1,
        },
        {
            "eventType": "agent.action.cosigned",
            "payload": {"signerQuid": "audit-bot", "cosigns": "act-1"},
            "timestamp": NOW + 2,
        },
    ]
    out = extract_cosignatures(events)
    assert len(out) == 2
    assert {c.guardian_quid for c in out} == {"principal", "audit-bot"}


def test_extract_vetoes_from_event_stream():
    events = [
        {
            "eventType": "agent.action.vetoed",
            "payload": {"signerQuid": "safety-committee",
                        "vetoes": "act-99", "reason": "suspicious"},
            "timestamp": NOW,
        },
    ]
    out = extract_vetoes(events)
    assert len(out) == 1
    assert out[0].action_id == "act-99"
    assert out[0].reason == "suspicious"
