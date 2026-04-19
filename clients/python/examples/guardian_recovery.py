"""Install a guardian set (QDP-0002) and walk through a recovery flow.

For brevity this example uses a 2-of-3 guardian set with a short
recovery delay. Production deployments typically use 3-of-5 or 5-of-7
with multi-day delays.

Prereqs: the node must have QDP-0002 enabled (default in current
build). Uses in-memory quids — real integrations should source
guardian keys from HSMs, WebAuthn authenticators, or PKCS#11 devices.
"""

from __future__ import annotations

import time

from quidnug import Quid, QuidnugClient
from quidnug.types import (
    GuardianRef,
    GuardianSet,
    GuardianSetUpdate,
    GuardianSignature,
    PrimarySignature,
)
from quidnug.crypto import canonical_bytes


def main() -> None:
    client = QuidnugClient("http://localhost:8080")

    owner = Quid.generate()
    g1, g2, g3 = Quid.generate(), Quid.generate(), Quid.generate()

    client.register_identity(owner, name="Owner")
    for i, g in enumerate((g1, g2, g3), 1):
        client.register_identity(g, name=f"Guardian-{i}")

    new_set = GuardianSet(
        subject_quid=owner.id,
        guardians=[
            GuardianRef(quid=g1.id, weight=1, epoch=0),
            GuardianRef(quid=g2.id, weight=1, epoch=0),
            GuardianRef(quid=g3.id, weight=1, epoch=0),
        ],
        threshold=2,
        recovery_delay_seconds=60,  # 60 seconds — demo only
    )

    update = GuardianSetUpdate(
        subject_quid=owner.id,
        new_set=new_set,
        anchor_nonce=1,
        valid_from=int(time.time()),
    )

    # Owner signs the primary consent.
    signable = canonical_bytes(
        update,
        exclude_fields=(
            "primary_signature",
            "new_guardian_consents",
            "current_guardian_sigs",
        ),
    )
    update.primary_signature = PrimarySignature(key_epoch=0, signature=owner.sign(signable))

    # Each new guardian consents.
    update.new_guardian_consents = [
        GuardianSignature(guardian_quid=g1.id, key_epoch=0, signature=g1.sign(signable)),
        GuardianSignature(guardian_quid=g2.id, key_epoch=0, signature=g2.sign(signable)),
        GuardianSignature(guardian_quid=g3.id, key_epoch=0, signature=g3.sign(signable)),
    ]

    client.submit_guardian_set_update(update)
    print(f"Guardian set installed for {owner.id}: 2-of-3 with 60s delay.")

    current = client.get_guardian_set(owner.id)
    assert current is not None
    print(f"Readback: threshold={current.threshold}, total_weight={current.total_weight}")


if __name__ == "__main__":
    main()
