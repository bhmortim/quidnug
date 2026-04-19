"""Produce Python canonical-bytes + signature vectors.

Usage:
    python tests/interop/produce.py > tests/interop/vectors-python.json
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any, Dict, List

# Allow running from repo root without install
REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT / "clients" / "python"))

from quidnug import Quid, canonical_bytes  # noqa: E402


def deterministic_quid() -> Quid:
    """Generate a Quid with fixed bytes so vectors are reproducible."""
    # NOTE: in production we'd use true randomness. For the interop
    # harness, stability is what matters — we want every run to emit
    # the same public key + transactions. Since real ECDSA P-256
    # private keys must come from a secure RNG, we use a fixed
    # 32-byte seed through the SDK's normal keygen path here;
    # the SDK re-generates, not seeds, so we instead persist the
    # first generated keypair to a local .keypair file on first run
    # and reuse it thereafter.
    keypair_path = Path(__file__).parent / ".keypair"
    if keypair_path.exists():
        return Quid.from_private_hex(keypair_path.read_text().strip())
    q = Quid.generate()
    keypair_path.write_text(q.private_key_hex)
    return q


def produce_vectors() -> Dict[str, Any]:
    q = deterministic_quid()
    cases: List[Dict[str, Any]] = []

    # Case 1 — simple trust transaction
    tx_trust = {
        "type": "TRUST",
        "timestamp": 1_700_000_000,
        "trustDomain": "interop.test",
        "signerQuid": q.id,
        "truster": q.id,
        "trustee": "abc0123456789def",
        "trustLevel": 0.9,
        "nonce": 1,
    }
    cases.append(make_case("trust-basic", tx_trust, q, ["signature", "txId"]))

    # Case 2 — identity transaction with attributes
    tx_id = {
        "type": "IDENTITY",
        "timestamp": 1_700_000_000,
        "trustDomain": "interop.test",
        "signerQuid": q.id,
        "definerQuid": q.id,
        "subjectQuid": q.id,
        "updateNonce": 1,
        "schemaVersion": "1.0",
        "name": "Alice",
        "homeDomain": "interop.home",
        "attributes": {"role": "admin", "tier": 3},
    }
    cases.append(make_case("identity-with-attrs", tx_id, q, ["signature", "txId"]))

    # Case 3 — edge case: unicode + nested objects
    tx_unicode = {
        "type": "EVENT",
        "timestamp": 1_700_000_000,
        "trustDomain": "interop.test",
        "subjectId": q.id,
        "subjectType": "QUID",
        "eventType": "NOTE",
        "sequence": 1,
        "payload": {
            "message": "hello 世界 🌍",
            "nested": {"z": 1, "a": "x", "m": [1, 2, 3]},
        },
    }
    cases.append(make_case("event-unicode-nested", tx_unicode, q, ["signature", "txId", "publicKey"]))

    # Case 4 — numerical edge case: zero, negative-ish, max int range
    tx_numbers = {
        "type": "EVENT",
        "timestamp": 1_700_000_000,
        "trustDomain": "interop.test",
        "subjectId": q.id,
        "subjectType": "QUID",
        "eventType": "MEASURE",
        "sequence": 42,
        "payload": {
            "count": 0,
            "weight": 0.5,
            "n64": 9_007_199_254_740_991,  # Max safe int in JS
        },
    }
    cases.append(make_case("numerical-edge", tx_numbers, q, ["signature", "txId", "publicKey"]))

    return {
        "sdk": "python",
        "version": "2.0.0",
        "quid": {
            "id": q.id,
            "publicKeyHex": q.public_key_hex,
        },
        "cases": cases,
    }


def make_case(name: str, tx: Dict[str, Any], q: Quid, exclude: List[str]) -> Dict[str, Any]:
    cb = canonical_bytes(tx, exclude_fields=tuple(exclude))
    sig = q.sign(cb)
    return {
        "name": name,
        "tx": tx,
        "excludeFields": exclude,
        "canonicalBytesHex": cb.hex(),
        "signatureHex": sig,
    }


if __name__ == "__main__":
    vectors = produce_vectors()
    json.dump(vectors, sys.stdout, indent=2)
    sys.stdout.write("\n")
