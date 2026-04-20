"""
Smoke tests for the elections-client crypto primitives.

Run with: pytest examples/elections/clients/tests/

Key things we verify:
  1. ECDSA key gen + sign + verify round-trip.
  2. Quid ID derivation matches the standard.
  3. RSA blind-sig round-trip (blind → authority signs → unblind →
     verify) matches RFC 9474 §4 behavior.
  4. RSA blinding is multiplicatively reversible only with the
     correct blinding factor.
"""
import hashlib
import os
import secrets
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import pytest

from common import crypto


def test_ecdsa_keygen_sign_verify():
    """Generate, sign, verify should all round-trip."""
    kp = crypto.generate_ecdsa_keypair()
    data = b"hello world"
    sig = crypto.ecdsa_sign_ieee1363(kp.private_key, data)
    assert len(sig) == 64
    assert crypto.ecdsa_verify_ieee1363(kp.public_key_hex, data, sig)


def test_ecdsa_verify_rejects_tampered():
    kp = crypto.generate_ecdsa_keypair()
    data = b"hello world"
    sig = crypto.ecdsa_sign_ieee1363(kp.private_key, data)
    assert not crypto.ecdsa_verify_ieee1363(kp.public_key_hex, b"different", sig)


def test_quid_id_shape():
    kp = crypto.generate_ecdsa_keypair()
    assert len(kp.quid_id) == 16
    assert all(c in "0123456789abcdef" for c in kp.quid_id)
    # Deterministic from the pubkey.
    assert kp.quid_id == hashlib.sha256(kp.public_key_bytes).hexdigest()[:16]


def test_ballot_token_deterministic():
    election = "elections.test.2026-nov"
    bq_pub = "04" + "ab" * 64  # fake 65-byte pubkey
    nonce = b"\x00" * 32
    t1 = crypto.ballot_token(election, bq_pub, nonce)
    t2 = crypto.ballot_token(election, bq_pub, nonce)
    assert t1 == t2
    assert len(t1) == 32
    # Different nonce → different token.
    t3 = crypto.ballot_token(election, bq_pub, b"\x00" * 31 + b"\x01")
    assert t3 != t1


def test_rsa_blind_sign_roundtrip():
    """Full blind-sign flow: voter blinds, authority signs,
    voter unblinds, anyone verifies. Should round-trip."""
    kp = crypto.generate_rsa_blind_keypair(bits=2048)  # 2048 for faster tests
    token = b"\x42" * 32

    # Voter-side: blind.
    blinded, r = crypto.blind(token, kp.public_key)

    # Authority-side: sign the blinded value.
    signed_blinded = crypto.sign_blinded(blinded, kp.private_key)

    # Voter-side: unblind.
    signature = crypto.unblind(signed_blinded, r, kp.public_key)

    # Anyone-side: verify.
    assert crypto.verify_blind_signature(signature, token, kp.public_key)


def test_rsa_blind_wrong_r_produces_invalid_signature():
    """If the voter uses the wrong blinding factor, unblind
    produces a signature that does NOT verify."""
    kp = crypto.generate_rsa_blind_keypair(bits=2048)
    token = b"\x42" * 32

    blinded, r = crypto.blind(token, kp.public_key)
    signed_blinded = crypto.sign_blinded(blinded, kp.private_key)

    # Tamper with r.
    wrong_r = (r + 1) % kp.public_modulus
    signature = crypto.unblind(signed_blinded, wrong_r, kp.public_key)

    assert not crypto.verify_blind_signature(signature, token, kp.public_key)


def test_rsa_blind_different_token_fails_verify():
    """Signature valid for token T doesn't verify against token T'."""
    kp = crypto.generate_rsa_blind_keypair(bits=2048)
    token_a = b"\x42" * 32
    token_b = b"\x43" * 32

    blinded, r = crypto.blind(token_a, kp.public_key)
    signed_blinded = crypto.sign_blinded(blinded, kp.private_key)
    signature = crypto.unblind(signed_blinded, r, kp.public_key)

    assert crypto.verify_blind_signature(signature, token_a, kp.public_key)
    assert not crypto.verify_blind_signature(signature, token_b, kp.public_key)


def test_rsa_fingerprint_stable():
    """The same RSA key produces the same fingerprint across
    generations."""
    kp = crypto.generate_rsa_blind_keypair(bits=2048)
    fp1 = kp.fingerprint
    fp2 = crypto.rsa_fingerprint(kp.public_key)
    assert fp1 == fp2
    assert len(fp1) == 64  # sha256 hex
