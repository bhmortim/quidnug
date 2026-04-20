"""
Python consumer for the v1.0 cross-SDK test vectors at
``docs/test-vectors/v1.0/``.

Asserts the five conformance properties specified in
``docs/test-vectors/v1.0/README.md`` for every case in every
vector file:

1. canonical_signable_bytes_utf8 hashes to the declared SHA-256.
2. Hex and UTF-8 forms of canonical bytes are equivalent.
3. The reference signature verifies against the canonical bytes
   and the test key's public key.
4. A tampered signature rejects.
5. An independent sign-then-verify round-trip via the test
   signer succeeds and produces a 64-byte IEEE-1363 signature.

The current ``clients/python/quidnug/crypto.py`` still uses
DER-encoded signatures and alphabetical canonical bytes (same
historical divergence as pkg/client pre-convergence). This
test file therefore imports its own IEEE-1363 + struct-decl
serialization helpers rather than depending on ``quidnug.crypto``.

When ``clients/python/quidnug/crypto.py`` is migrated to
match the v1.0 canonical form, this file should be updated to
exercise the SDK's public API directly (analogous to
``pkg/client/vectors_test.go`` post-convergence).
"""

from __future__ import annotations

import hashlib
import json
import pathlib
from typing import Any

import pytest

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import decode_dss_signature
from cryptography.exceptions import InvalidSignature


# ---------------------------------------------------------------
# Vector loading
# ---------------------------------------------------------------

# Repo-root-relative path to the vectors directory.
VECTORS_ROOT = (
    pathlib.Path(__file__).resolve().parents[3]
    / "docs" / "test-vectors" / "v1.0"
)


def load_vector_file(filename: str) -> dict[str, Any]:
    with open(VECTORS_ROOT / filename) as f:
        return json.load(f)


def load_keys() -> dict[str, dict[str, Any]]:
    out: dict[str, dict[str, Any]] = {}
    for path in (VECTORS_ROOT / "test-keys").glob("*.json"):
        with open(path) as f:
            k = json.load(f)
        out[k["name"]] = k
    return out


# ---------------------------------------------------------------
# IEEE-1363 crypto helpers (temporary local copies)
# ---------------------------------------------------------------


def sign_ieee1363(priv: ec.EllipticCurvePrivateKey, data: bytes) -> bytes:
    """Sign ``data`` with ECDSA-P256 + SHA-256 and return the
    64-byte IEEE-1363 raw (r||s) encoding."""
    der = priv.sign(data, ec.ECDSA(hashes.SHA256()))
    r, s = decode_dss_signature(der)
    return r.to_bytes(32, "big") + s.to_bytes(32, "big")


def verify_ieee1363(public_key_hex: str, data: bytes, sig_bytes: bytes) -> bool:
    """Verify an IEEE-1363 64-byte signature against a SEC1
    uncompressed-point hex pubkey."""
    if len(sig_bytes) != 64:
        return False
    pub_bytes = bytes.fromhex(public_key_hex)
    try:
        pub = ec.EllipticCurvePublicKey.from_encoded_point(
            ec.SECP256R1(), pub_bytes,
        )
    except ValueError:
        return False
    r = int.from_bytes(sig_bytes[:32], "big")
    s = int.from_bytes(sig_bytes[32:], "big")
    from cryptography.hazmat.primitives.asymmetric.utils import (
        encode_dss_signature,
    )
    der = encode_dss_signature(r, s)
    try:
        pub.verify(der, data, ec.ECDSA(hashes.SHA256()))
        return True
    except InvalidSignature:
        return False
    except Exception:
        return False


def private_key_from_scalar_hex(scalar_hex: str) -> ec.EllipticCurvePrivateKey:
    """Reconstruct an ECDSA P-256 private key from its raw scalar
    (deterministic test keys use this format, matching the Go
    generator's seed-derived keys)."""
    d = int(scalar_hex, 16)
    return ec.derive_private_key(d, ec.SECP256R1())


# ---------------------------------------------------------------
# Parametrization: every case in every vector file
# ---------------------------------------------------------------


VECTOR_FILES = [
    "trust-tx.json",
    "identity-tx.json",
    "event-tx.json",
    "title-tx.json",
    "node-advertisement-tx.json",
    "moderation-action-tx.json",
    "dsr-tx.json",
]


def _collect_cases() -> list[tuple[str, dict[str, Any]]]:
    out: list[tuple[str, dict[str, Any]]] = []
    for filename in VECTOR_FILES:
        try:
            vf = load_vector_file(filename)
        except FileNotFoundError:
            continue  # skip missing files (generator may be older)
        for c in vf["cases"]:
            out.append((f"{filename}::{c['name']}", c))
    return out


@pytest.fixture(scope="module")
def keys() -> dict[str, dict[str, Any]]:
    return load_keys()


@pytest.mark.parametrize("name,case", _collect_cases(), ids=lambda p: p if isinstance(p, str) else "")
def test_vector_conformance(name: str, case: dict[str, Any], keys: dict[str, dict[str, Any]]) -> None:
    """Assert all five conformance properties for a single case."""
    del name  # used for parametrize id only

    signer_ref = case["signer_key_ref"]
    key = keys[signer_ref]
    expected = case["expected"]

    utf8_str: str = expected["canonical_signable_bytes_utf8"]
    hex_str: str = expected["canonical_signable_bytes_hex"]
    sha256_hex: str = expected["sha256_of_canonical_hex"]
    sig_hex: str = expected["reference_signature_hex"]
    expected_id: str = expected["expected_id"]
    expected_sig_len: int = expected["signature_length_bytes"]

    signable_bytes = utf8_str.encode("utf-8")

    # Property 1: SHA-256 of canonical UTF-8 matches declared hash.
    got_sha = hashlib.sha256(signable_bytes).hexdigest()
    assert got_sha == sha256_hex, (
        f"SHA-256 mismatch\n want: {sha256_hex}\n  got: {got_sha}"
    )

    # Property 2: hex and utf8 canonical forms equivalent.
    from_hex = bytes.fromhex(hex_str).decode("utf-8")
    assert from_hex == utf8_str, "hex and utf8 canonical forms diverge"

    # Property 3: reference signature verifies.
    sig_bytes = bytes.fromhex(sig_hex)
    assert len(sig_bytes) == expected_sig_len, (
        f"signature_length_bytes mismatch: "
        f"expected {expected_sig_len}, got {len(sig_bytes)}"
    )
    assert len(sig_bytes) == 64, (
        f"v1.0 requires 64-byte IEEE-1363 signatures; got {len(sig_bytes)}"
    )
    assert verify_ieee1363(key["public_key_sec1_hex"], signable_bytes, sig_bytes), (
        "reference signature did not verify"
    )

    # Property 4: tampered signature rejects.
    tampered = bytearray(sig_bytes)
    tampered[5] ^= 0x01
    assert not verify_ieee1363(
        key["public_key_sec1_hex"], signable_bytes, bytes(tampered),
    ), "tampered signature unexpectedly verified"

    # Property 5: independent sign-then-verify round-trip.
    priv = private_key_from_scalar_hex(key["private_scalar_hex"])
    new_sig = sign_ieee1363(priv, signable_bytes)
    assert len(new_sig) == 64, (
        f"Python SDK produced {len(new_sig)}-byte signature; v1.0 mandates 64"
    )
    assert verify_ieee1363(key["public_key_sec1_hex"], signable_bytes, new_sig), (
        "Python sign-then-verify round-trip failed"
    )

    # Property 6 (bonus, derived): quid_id derivation matches.
    expected_quid = key["quid_id"]
    pub_bytes = bytes.fromhex(key["public_key_sec1_hex"])
    computed_quid = hashlib.sha256(pub_bytes).hexdigest()[:16]
    assert computed_quid == expected_quid, (
        f"quid_id derivation mismatch: "
        f"expected {expected_quid}, got {computed_quid}"
    )


# ---------------------------------------------------------------
# Divergence probes against clients/python/quidnug
# ---------------------------------------------------------------


def test_clients_python_sdk_sign_diverges_from_authoritative() -> None:
    """Document the current state of clients/python/quidnug.crypto.sign_bytes
    vs the authoritative v1.0 form.

    Passes as a divergence-detector today; when the SDK is
    migrated to IEEE-1363, the assertion inverts and this
    test logs "converged!" without failing.
    """
    try:
        from quidnug.crypto import Quid as SDKQuid  # type: ignore
    except Exception as exc:
        pytest.skip(f"quidnug SDK not importable in this env: {exc}")

    q = SDKQuid.generate()
    data = b"quidnug-test-data-for-signing-comparison"
    sig_hex = q.sign(data)
    sig_bytes = bytes.fromhex(sig_hex)

    if len(sig_bytes) == 64:
        # Converged: SDK now produces IEEE-1363. Remove this
        # probe test on the next PR.
        print(
            "\n[divergence probe] clients/python/quidnug.crypto.sign now "
            "produces 64-byte IEEE-1363 signatures. Divergence resolved. "
            "This test can be removed."
        )
        return

    print(
        f"\n[divergence probe] clients/python/quidnug.crypto.sign "
        f"produces {len(sig_bytes)}-byte DER signatures. v1.0 mandates "
        f"64-byte IEEE-1363. Launch-blocker tracked in "
        f"docs/test-vectors/v1.0/README.md § Known divergences."
    )


def test_clients_python_canonical_bytes_diverges_from_authoritative() -> None:
    """Document clients/python/quidnug.crypto.canonical_bytes vs the
    authoritative v1.0 form (struct-declaration order, not
    alphabetical).
    """
    try:
        from quidnug.crypto import canonical_bytes as sdk_canonical_bytes  # type: ignore
    except Exception as exc:
        pytest.skip(f"quidnug SDK not importable: {exc}")

    vf = load_vector_file("trust-tx.json")
    case = vf["cases"][0]
    tx_dict = case["input"]

    sdk_bytes = sdk_canonical_bytes(tx_dict, exclude_fields=("signature",))
    authoritative_utf8 = case["expected"]["canonical_signable_bytes_utf8"]

    if sdk_bytes.decode("utf-8") == authoritative_utf8:
        print(
            "\n[divergence probe] clients/python/quidnug.crypto."
            "canonical_bytes has converged with authoritative form. "
            "Probe can be removed."
        )
        return

    print(
        "\n[divergence probe] clients/python/quidnug.crypto.canonical_bytes "
        "produces alphabetical-order output; authoritative v1.0 form is "
        "struct-declaration order. Launch-blocker tracked in "
        "docs/test-vectors/v1.0/README.md § Known divergences."
    )
