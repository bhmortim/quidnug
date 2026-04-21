"""Tests for split_horizon.py. No Quidnug node required."""

import pytest

from split_horizon import (
    ERecord,
    QueryResult,
    VisibilityPolicy,
    extract_group_memberships,
    extract_records,
    query_record,
    query_zone,
)


def _rec(
    visibility: str = "public",
    name: str = "api.bigcorp.com", rtype: str = "A",
    value: str = "203.0.113.42",
) -> ERecord:
    return ERecord(
        name=name, record_type=rtype, value=value,
        visibility=visibility,
        signer_quid="bigcorp-governor",
    )


def _trust(mapping):
    return lambda obs, target: mapping.get((obs, target), 0.0)


def _groups(memberships):
    def fn(obs, group):
        return (obs, group) in memberships
    return fn


# ---------------------------------------------------------------------------
# Public
# ---------------------------------------------------------------------------

def test_public_record_visible_to_anyone():
    for observer in ("employee", "partner", "random"):
        q = query_record(
            observer, _rec("public"),
            trust_fn=_trust({}), group_membership_fn=_groups(set()),
        )
        assert q.verdict == "ok"


# ---------------------------------------------------------------------------
# Trust-gated
# ---------------------------------------------------------------------------

def test_trust_gated_visible_to_trusted_observer():
    q = query_record(
        "partner",
        _rec("trust-gated:bigcorp-partners"),
        trust_fn=_trust({("partner", "bigcorp-partners"): 0.9}),
        group_membership_fn=_groups(set()),
    )
    assert q.verdict == "ok"


def test_trust_gated_nxdomain_for_untrusted():
    q = query_record(
        "random",
        _rec("trust-gated:bigcorp-partners"),
        trust_fn=_trust({}),
        group_membership_fn=_groups(set()),
    )
    assert q.verdict == "nxdomain"


def test_trust_gated_custom_threshold():
    gated = _rec("trust-gated:bigcorp-partners")
    borderline_trust = _trust({("partner", "bigcorp-partners"): 0.4})
    strict = VisibilityPolicy(min_trust_for_gated=0.6)
    lenient = VisibilityPolicy(min_trust_for_gated=0.3)
    assert query_record("partner", gated, borderline_trust,
                         _groups(set()), strict).verdict == "nxdomain"
    assert query_record("partner", gated, borderline_trust,
                         _groups(set()), lenient).verdict == "ok"


def test_trust_gated_malformed_visibility_is_invalid():
    q = query_record(
        "observer", _rec("trust-gated:"),
        _trust({}), _groups(set()),
    )
    assert q.verdict == "invalid"


# ---------------------------------------------------------------------------
# Private
# ---------------------------------------------------------------------------

def test_private_record_visible_to_group_member():
    q = query_record(
        "employee",
        _rec("private:bigcorp-employees"),
        trust_fn=_trust({}),
        group_membership_fn=_groups({("employee", "bigcorp-employees")}),
    )
    assert q.verdict == "ok"


def test_private_record_nxdomain_for_non_member():
    q = query_record(
        "random",
        _rec("private:bigcorp-employees"),
        trust_fn=_trust({}),
        group_membership_fn=_groups({("other-quid", "bigcorp-employees")}),
    )
    assert q.verdict == "nxdomain"


def test_private_malformed_visibility_is_invalid():
    q = query_record(
        "observer", _rec("private:"),
        _trust({}), _groups(set()),
    )
    assert q.verdict == "invalid"


# ---------------------------------------------------------------------------
# Unknown scheme
# ---------------------------------------------------------------------------

def test_unknown_visibility_scheme_invalid():
    q = query_record(
        "observer", _rec("classified:top-secret"),
        _trust({}), _groups(set()),
    )
    assert q.verdict == "invalid"


# ---------------------------------------------------------------------------
# Out-of-range trust
# ---------------------------------------------------------------------------

def test_trust_out_of_range_raises():
    with pytest.raises(ValueError):
        query_record(
            "observer", _rec("trust-gated:g"),
            _trust({("observer", "g"): 1.5}),
            _groups(set()),
        )


# ---------------------------------------------------------------------------
# Zone query
# ---------------------------------------------------------------------------

def test_zone_query_filters_invisible():
    records = [
        _rec("public",
              name="bigcorp.com", rtype="A", value="203.0.113.1"),
        _rec("trust-gated:bigcorp-partners",
              name="api.bigcorp.com", rtype="A", value="203.0.113.10"),
        _rec("private:bigcorp-employees",
              name="internal.bigcorp.com", rtype="A", value="10.0.0.5"),
    ]
    outsider_results = query_zone(
        "random", records,
        _trust({}), _groups(set()),
    )
    # Only the public record survives for an outsider.
    assert [r.record.name for r in outsider_results] == ["bigcorp.com"]

    partner_results = query_zone(
        "partner", records,
        _trust({("partner", "bigcorp-partners"): 0.9}),
        _groups(set()),
    )
    # Partner sees public + trust-gated.
    partner_names = {r.record.name for r in partner_results}
    assert "bigcorp.com" in partner_names
    assert "api.bigcorp.com" in partner_names

    emp_results = query_zone(
        "employee", records,
        _trust({("employee", "bigcorp-partners"): 0.0}),
        _groups({("employee", "bigcorp-employees")}),
    )
    emp_names = {r.record.name for r in emp_results}
    # Employee is in the employees group, not the partners
    # trust set, so they see public + private.
    assert "bigcorp.com" in emp_names
    assert "internal.bigcorp.com" in emp_names
    assert "api.bigcorp.com" not in emp_names


# ---------------------------------------------------------------------------
# Stream extraction
# ---------------------------------------------------------------------------

def test_extract_records():
    events = [
        {
            "eventType": "edns.record-published",
            "sequence": 1,
            "payload": {
                "name": "bigcorp.com", "recordType": "A",
                "value": "203.0.113.1", "visibility": "public",
                "signerQuid": "bigcorp-governor",
            },
        },
        {
            "eventType": "edns.record-published",
            "sequence": 2,
            "payload": {
                "name": "api.bigcorp.com", "recordType": "A",
                "value": "203.0.113.10",
                "visibility": "trust-gated:bigcorp-partners",
                "signerQuid": "bigcorp-governor",
            },
        },
        {"eventType": "other", "payload": {}},
    ]
    records = extract_records(events)
    assert len(records) == 2


def test_extract_group_memberships():
    events = [
        {"eventType": "group.member-added", "sequence": 1,
         "payload": {"memberQuid": "alice"}},
        {"eventType": "group.member-added", "sequence": 2,
         "payload": {"memberQuid": "bob"}},
        {"eventType": "group.member-removed", "sequence": 3,
         "payload": {"memberQuid": "alice"}},
    ]
    m = extract_group_memberships(events)
    assert "bob" in m
    assert "alice" not in m
