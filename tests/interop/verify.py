"""Verify cross-SDK vector files.

Usage:
    python tests/interop/verify.py tests/interop/vectors-*.json

Exit code 0 if every signature in every file verifies against the
included public key and canonical bytes. Non-zero on any mismatch.

This verifier only confirms that a given SDK's output is internally
consistent (public key + canonical bytes + signature triple). For
true cross-SDK interop, each language's verifier must accept vectors
from every other language — run all verifiers, not just this one.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any, Dict, List, Tuple

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT / "clients" / "python"))

from quidnug import Quid, canonical_bytes  # noqa: E402


def verify_file(path: Path) -> Tuple[int, int, List[str]]:
    data: Dict[str, Any] = json.loads(path.read_text(encoding="utf-8"))
    sdk = data.get("sdk", "unknown")
    pub_hex = data["quid"]["publicKeyHex"]
    verifier_quid = Quid.from_public_hex(pub_hex)

    ok = 0
    failed = 0
    failures: List[str] = []

    for case in data["cases"]:
        name = case["name"]
        expected_cb = case["canonicalBytesHex"]
        sig = case["signatureHex"]
        excluded = tuple(case.get("excludeFields", ()))

        # 1. Re-derive canonical bytes using THIS SDK; they must match.
        local_cb = canonical_bytes(case["tx"], exclude_fields=excluded).hex()
        if local_cb != expected_cb:
            failed += 1
            failures.append(
                f"[{sdk}:{name}] canonical-bytes divergence — "
                f"foreign: {expected_cb[:48]}…, local: {local_cb[:48]}…"
            )
            continue

        # 2. Verify the foreign-signed signature against our local
        #    canonical bytes using this SDK's verifier.
        if not verifier_quid.verify(bytes.fromhex(local_cb), sig):
            failed += 1
            failures.append(f"[{sdk}:{name}] signature verification failed")
            continue

        ok += 1

    return ok, failed, failures


def main(argv: List[str]) -> int:
    if len(argv) < 2:
        print("usage: verify.py vectors-*.json", file=sys.stderr)
        return 2

    total_ok = 0
    total_fail = 0
    for arg in argv[1:]:
        path = Path(arg)
        if not path.exists():
            print(f"skipping missing: {arg}", file=sys.stderr)
            continue
        ok, failed, failures = verify_file(path)
        total_ok += ok
        total_fail += failed
        print(f"{path.name}: {ok} ok, {failed} failed")
        for msg in failures:
            print(f"  FAIL: {msg}")

    print(f"\nTOTAL: {total_ok} ok, {total_fail} failed")
    return 0 if total_fail == 0 else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
