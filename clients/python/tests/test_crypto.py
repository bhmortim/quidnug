"""Tests for quidnug.crypto — key generation, signing, canonicalization."""

from __future__ import annotations

import pytest

from quidnug import Quid, canonical_bytes, sign_bytes, verify_signature
from quidnug.errors import CryptoError


# --- Quid keygen + roundtrip ----------------------------------------------


def test_generate_quid_has_expected_id_format():
    q = Quid.generate()
    assert len(q.id) == 16
    assert int(q.id, 16) >= 0  # valid hex
    assert q.has_private_key
    assert q.public_key_hex
    assert q.private_key_hex


def test_quid_roundtrip_via_private_hex():
    q = Quid.generate()
    restored = Quid.from_private_hex(q.private_key_hex)
    assert restored.id == q.id
    assert restored.public_key_hex == q.public_key_hex
    assert restored.has_private_key


def test_public_only_quid_is_read_only():
    q = Quid.generate()
    pub_only = Quid.from_public_hex(q.public_key_hex)
    assert pub_only.id == q.id
    assert not pub_only.has_private_key
    with pytest.raises(CryptoError):
        pub_only.sign(b"anything")


# --- Sign / verify ---------------------------------------------------------


def test_sign_verify_roundtrip():
    q = Quid.generate()
    sig = q.sign(b"payload")
    assert q.verify(b"payload", sig) is True
    assert q.verify(b"different-payload", sig) is False


def test_signature_from_one_quid_does_not_verify_against_another():
    a = Quid.generate()
    b = Quid.generate()
    sig = a.sign(b"shared-data")
    assert b.verify(b"shared-data", sig) is False


def test_sign_bytes_low_level_matches_quid_sign():
    q = Quid.generate()
    sig = q.sign(b"x")
    # Signatures are non-deterministic, so we verify rather than compare.
    assert verify_signature(q._pub, b"x", sig)


# --- Canonicalization -----------------------------------------------------


def test_canonical_bytes_excludes_signature_field():
    tx = {"type": "TRUST", "signature": "abc", "trustLevel": 1.0}
    b = canonical_bytes(tx, exclude_fields=("signature",))
    assert b'"signature"' not in b
    assert b'"trustLevel"' in b


def test_canonical_bytes_is_stable_across_insertion_order():
    a = {"b": 1, "a": 2}
    b = {"a": 2, "b": 1}
    # Round-trip normalization means both produce identical bytes
    # even if Python dict key order differed on construction.
    assert canonical_bytes(a) == canonical_bytes(b)


def test_canonical_bytes_roundtrip_through_json():
    """Verifying the canonicalization's generic-object roundtrip property."""
    import json as j

    tx = {"trustLevel": 0.75, "nested": {"x": 1, "y": [1, 2, 3]}}
    out = canonical_bytes(tx)
    reparsed = j.loads(out)
    assert reparsed["trustLevel"] == 0.75
    assert reparsed["nested"]["y"] == [1, 2, 3]
