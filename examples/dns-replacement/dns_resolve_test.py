"""Tests for dns_resolve.py. No Quidnug node required."""

import pytest

from dns_resolve import (
    DNSRecord,
    ResolvePolicy,
    ResolveResult,
    extract_records,
    resolve,
)


NOW = 1_700_000_000
GOV = {"governor-primary", "governor-backup"}


def _published(
    name: str, rtype: str, value: str, *, signer: str = "governor-primary",
    ttl: int = 300, seq: int = 1, signed_age: int = 0,
) -> dict:
    return {
        "eventType": "dns.record-published",
        "sequence": seq,
        "timestamp": NOW - signed_age,
        "payload": {
            "recordType": rtype,
            "name": name,
            "value": value,
            "ttl": ttl,
            "signerQuid": signer,
            "signedAt": NOW - signed_age,
        },
    }


def _revoked(name: str, rtype: str, seq: int, target_seq: int) -> dict:
    return {
        "eventType": "dns.record-revoked",
        "sequence": seq,
        "timestamp": NOW,
        "payload": {
            "name": name, "recordType": rtype, "revokesSeq": target_seq,
            "signerQuid": "governor-primary",
        },
    }


def _trust(mapping):
    return lambda obs, target: mapping.get((obs, target), 0.0)


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------

def test_simple_a_record_resolves():
    events = [_published("example.quidnug", "A", "192.0.2.1", seq=1)]
    trust = _trust({("resolver", "governor-primary"): 0.9})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "ok"
    assert r.records[0].value == "192.0.2.1"


def test_nxdomain_when_no_records():
    events = []
    trust = _trust({})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "nxdomain"


def test_multiple_record_types_coexist():
    events = [
        _published("example.quidnug", "A",    "192.0.2.1",  seq=1),
        _published("example.quidnug", "AAAA", "2001:db8::1", seq=2),
        _published("example.quidnug", "MX",    "mail.example.quidnug", seq=3),
    ]
    trust = _trust({("resolver", "governor-primary"): 0.9})
    a    = resolve("resolver", GOV, "example.quidnug", "A",    events, trust, now_unix=NOW)
    aaaa = resolve("resolver", GOV, "example.quidnug", "AAAA", events, trust, now_unix=NOW)
    mx   = resolve("resolver", GOV, "example.quidnug", "MX",   events, trust, now_unix=NOW)
    assert a.verdict == "ok"    and a.records[0].value == "192.0.2.1"
    assert aaaa.verdict == "ok" and aaaa.records[0].value == "2001:db8::1"
    assert mx.verdict == "ok"   and mx.records[0].value == "mail.example.quidnug"


# ---------------------------------------------------------------------------
# Cache-poisoning / tampering
# ---------------------------------------------------------------------------

def test_record_from_non_governor_rejected():
    events = [
        _published("example.quidnug", "A", "192.0.2.1", seq=1),
        _published("example.quidnug", "A", "198.51.100.1",
                    signer="rogue-quid", seq=2),
    ]
    trust = _trust({
        ("resolver", "governor-primary"): 0.9,
        ("resolver", "rogue-quid"):        0.9,
    })
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "ok"
    assert len(r.records) == 1
    assert r.records[0].value == "192.0.2.1"
    assert r.records[0].signer_quid == "governor-primary"


def test_tampered_verdict_when_only_non_governor_records():
    events = [
        _published("example.quidnug", "A", "198.51.100.1",
                    signer="rogue-quid", seq=1),
    ]
    trust = _trust({("resolver", "rogue-quid"): 0.9})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "tampered"


# ---------------------------------------------------------------------------
# Revocation
# ---------------------------------------------------------------------------

def test_revoked_record_removed():
    events = [
        _published("example.quidnug", "A", "192.0.2.1", seq=1),
        _published("example.quidnug", "A", "192.0.2.2", seq=2),
        _revoked("example.quidnug", "A", seq=3, target_seq=1),
    ]
    trust = _trust({("resolver", "governor-primary"): 0.9})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "ok"
    assert len(r.records) == 1
    assert r.records[0].value == "192.0.2.2"


def test_all_revoked_is_nxdomain():
    events = [
        _published("example.quidnug", "A", "192.0.2.1", seq=1),
        _revoked("example.quidnug", "A", seq=2, target_seq=1),
    ]
    trust = _trust({("resolver", "governor-primary"): 0.9})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "nxdomain"


# ---------------------------------------------------------------------------
# Trust gating
# ---------------------------------------------------------------------------

def test_low_trust_indeterminate():
    events = [_published("example.quidnug", "A", "192.0.2.1", seq=1)]
    trust = _trust({("resolver", "governor-primary"): 0.2})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "indeterminate"
    assert r.observer_trust_to_signer == pytest.approx(0.2)


def test_policy_trust_threshold_customizable():
    events = [_published("example.quidnug", "A", "192.0.2.1", seq=1)]
    trust = _trust({("resolver", "governor-primary"): 0.35})
    strict = ResolvePolicy(min_signer_trust=0.5)
    lenient = ResolvePolicy(min_signer_trust=0.3)
    rs = resolve("resolver", GOV, "example.quidnug", "A",
                 events, trust, now_unix=NOW, policy=strict)
    rl = resolve("resolver", GOV, "example.quidnug", "A",
                 events, trust, now_unix=NOW, policy=lenient)
    assert rs.verdict == "indeterminate"
    assert rl.verdict == "ok"


# ---------------------------------------------------------------------------
# TTL
# ---------------------------------------------------------------------------

def test_ttl_enforcement_drops_stale():
    events = [_published(
        "example.quidnug", "A", "192.0.2.1",
        seq=1, ttl=60, signed_age=3600,   # 1 hour old, TTL=60s
    )]
    trust = _trust({("resolver", "governor-primary"): 0.9})
    stale = ResolvePolicy(enforce_ttl=True)
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW, policy=stale,
    )
    assert r.verdict == "nxdomain"


def test_ttl_not_enforced_by_default():
    events = [_published(
        "example.quidnug", "A", "192.0.2.1",
        seq=1, ttl=60, signed_age=3600,
    )]
    trust = _trust({("resolver", "governor-primary"): 0.9})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "ok"


# ---------------------------------------------------------------------------
# Input validation
# ---------------------------------------------------------------------------

def test_invalid_record_type_returns_tampered():
    trust = _trust({})
    r = resolve(
        "resolver", GOV, "example.quidnug", "NOTAREAL",
        events=[], trust_fn=trust, now_unix=NOW,
    )
    assert r.verdict == "tampered"


def test_trust_out_of_range_raises():
    events = [_published("example.quidnug", "A", "192.0.2.1", seq=1)]
    trust = _trust({("resolver", "governor-primary"): 1.5})
    with pytest.raises(ValueError):
        resolve(
            "resolver", GOV, "example.quidnug", "A",
            events, trust, now_unix=NOW,
        )


# ---------------------------------------------------------------------------
# Round-robin multiple records
# ---------------------------------------------------------------------------

def test_multiple_a_records_round_robin():
    events = [
        _published("example.quidnug", "A", "192.0.2.1", seq=1),
        _published("example.quidnug", "A", "192.0.2.2", seq=2),
        _published("example.quidnug", "A", "192.0.2.3", seq=3),
    ]
    trust = _trust({("resolver", "governor-primary"): 0.9})
    r = resolve(
        "resolver", GOV, "example.quidnug", "A",
        events, trust, now_unix=NOW,
    )
    assert r.verdict == "ok"
    assert {rec.value for rec in r.records} == {"192.0.2.1", "192.0.2.2", "192.0.2.3"}
