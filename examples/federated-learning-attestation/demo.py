"""Federated learning round audit, end-to-end runnable demo.

Flow:
  1. Register actors: coordinator, five participant banks, one
     auditor.
  2. Coordinator opens a round on a shared round-quid's stream.
  3. Each bank emits participant.registered with attested data
     size, then gradient.submitted with gradient hash and norm.
  4. Coordinator emits round.aggregated.
  5. Auditor runs audit_round -> valid.
  6. Second round: one bank drops out -> insufficient.
  7. Third round: one bank submits a suspicious gradient
     (very large norm) -> valid but flagged.

Prerequisites:
  - Local Quidnug node at http://localhost:8080.
  - Python SDK installed.

Run:
    python demo.py
"""

from __future__ import annotations

import os
import sys
import time
import uuid
from dataclasses import dataclass
from typing import List

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from fl_audit import (
    RoundPolicy,
    audit_round,
)

from quidnug import Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "ai.federated-learning.fraud-detection"


@dataclass
class Actor:
    name: str
    role: str
    quid: Quid


def banner(msg: str) -> None:
    print()
    print("=" * 72)
    print(f"  {msg}")
    print("=" * 72)


def register(client: QuidnugClient, name: str, role: str) -> Actor:
    q = Quid.generate()
    try:
        client.register_identity(
            q, name=name, domain=DOMAIN, home_domain=DOMAIN,
            attributes={"role": role},
        )
    except Exception as e:
        print(f"  (register {name}: {e})")
    return Actor(name=name, role=role, quid=q)


def open_round(
    client: QuidnugClient, coordinator: Actor, round_actor: Actor,
    round_number: int, model_hash: str,
) -> None:
    client.emit_event(
        signer=coordinator.quid,
        subject_id=round_actor.quid.id,
        subject_type="QUID",
        event_type="round.opened",
        domain=DOMAIN,
        payload={
            "roundNumber": round_number,
            "modelStateHash": model_hash,
            "openedBy": coordinator.quid.id,
            "openedAt": int(time.time()),
        },
    )


def register_participant(
    client: QuidnugClient, round_actor: Actor, participant: Actor,
    data_size: int,
) -> None:
    client.emit_event(
        signer=participant.quid,
        subject_id=round_actor.quid.id,
        subject_type="QUID",
        event_type="participant.registered",
        domain=DOMAIN,
        payload={
            "participant": participant.quid.id,
            "attestedDataSize": data_size,
            "attestedDataHash": f"data-schema-{uuid.uuid4().hex[:8]}",
            "registeredAt": int(time.time()),
        },
    )


def submit_gradient(
    client: QuidnugClient, round_actor: Actor, participant: Actor,
    gradient_norm: float, training_data_size: int,
) -> None:
    client.emit_event(
        signer=participant.quid,
        subject_id=round_actor.quid.id,
        subject_type="QUID",
        event_type="gradient.submitted",
        domain=DOMAIN,
        payload={
            "participant": participant.quid.id,
            "gradientCID": f"bafy-{uuid.uuid4().hex[:8]}",
            "gradientHash": f"hash-{uuid.uuid4().hex[:8]}",
            "gradientNorm": gradient_norm,
            "trainingDataSize": training_data_size,
            "submittedAt": int(time.time()),
        },
    )


def aggregate(
    client: QuidnugClient, coordinator: Actor, round_actor: Actor,
    participant_weights: dict,
) -> None:
    client.emit_event(
        signer=coordinator.quid,
        subject_id=round_actor.quid.id,
        subject_type="QUID",
        event_type="round.aggregated",
        domain=DOMAIN,
        payload={
            "coordinator": coordinator.quid.id,
            "aggregateHash": f"agg-{uuid.uuid4().hex[:8]}",
            "participantWeights": participant_weights,
            "aggregatedAt": int(time.time()),
        },
    )


def load_events(client: QuidnugClient, round_actor: Actor) -> List[dict]:
    events, _ = client.get_stream_events(round_actor.quid.id, limit=500)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
        })
    return out


def audit_and_show(
    client: QuidnugClient, round_actor: Actor, label: str,
    policy: RoundPolicy = None,
) -> None:
    events = load_events(client, round_actor)
    v = audit_round(round_actor.name, events, policy)
    print(f"\n  [{label}]")
    print(f"    {v.short()}")
    for r in v.reasons:
        print(f"      - {r}")
    if v.breakdown.suspicious_gradients:
        print(f"    suspicious:")
        for s in v.breakdown.suspicious_gradients:
            print(f"      {s}")
    if v.breakdown.missing_participants:
        print(f"    missing:")
        for m in v.breakdown.missing_participants:
            print(f"      {m}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    # -----------------------------------------------------------------
    banner("Step 1: Register actors")
    coord    = register(client, "fl-coordinator-consortium", "coordinator")
    banks    = [register(client, f"bank-{c}", "participant") for c in "ABCDE"]
    auditor  = register(client, "fl-auditor",                 "auditor")
    for a in [coord, auditor] + banks:
        print(f"  {a.role:12s} {a.name:28s} -> {a.quid.id}")

    # -----------------------------------------------------------------
    banner("Step 2: Open round 1 and have all banks participate")
    round1 = register(client, f"fl-round-1-{uuid.uuid4().hex[:6]}", "round")
    open_round(client, coord, round1, 1, "model-hash-seed")

    for b in banks:
        register_participant(client, round1, b, data_size=1_000_000)
    print(f"  All 5 banks registered on round 1")

    time.sleep(0.5)

    for b in banks:
        submit_gradient(client, round1, b,
                        gradient_norm=1.0 + (hash(b.name) % 20) / 100,
                        training_data_size=1_000_000)
    print(f"  All 5 banks submitted gradients")

    aggregate(client, coord, round1, {b.quid.id: 0.2 for b in banks})
    print(f"  Coordinator emitted round.aggregated")

    time.sleep(0.5)
    audit_and_show(client, round1, "ROUND 1 (expect valid)")

    # -----------------------------------------------------------------
    banner("Step 3: Round 2 -- one bank drops out, below threshold")
    round2 = register(client, f"fl-round-2-{uuid.uuid4().hex[:6]}", "round")
    open_round(client, coord, round2, 2, "model-hash-r1")
    for b in banks:
        register_participant(client, round2, b, data_size=1_000_000)
    # Only 2 banks actually submit (below min=5 default).
    for b in banks[:2]:
        submit_gradient(client, round2, b,
                        gradient_norm=1.0, training_data_size=1_000_000)
    # No aggregation event since the round is incomplete.

    time.sleep(0.5)
    audit_and_show(client, round2, "ROUND 2 (expect insufficient)")

    # -----------------------------------------------------------------
    banner("Step 4: Round 3 -- one bank submits a suspicious gradient")
    round3 = register(client, f"fl-round-3-{uuid.uuid4().hex[:6]}", "round")
    open_round(client, coord, round3, 3, "model-hash-r2")
    for b in banks:
        register_participant(client, round3, b, data_size=1_000_000)
    for i, b in enumerate(banks):
        # bank-E submits a gradient with norm=50 (vs others ~1)
        norm = 50.0 if i == 4 else 1.0 + (i / 10)
        submit_gradient(client, round3, b, gradient_norm=norm,
                        training_data_size=1_000_000)
    aggregate(client, coord, round3, {b.quid.id: 0.2 for b in banks})

    time.sleep(0.5)
    audit_and_show(client, round3, "ROUND 3 (expect valid, bank-E flagged)")

    # -----------------------------------------------------------------
    banner("Step 5: Round 4 -- strict-registration integrity violation")
    round4 = register(client, f"fl-round-4-{uuid.uuid4().hex[:6]}", "round")
    open_round(client, coord, round4, 4, "model-hash-r3")
    for b in banks:
        register_participant(client, round4, b, data_size=1_000_000)
    # All but bank-E submit.
    for b in banks[:-1]:
        submit_gradient(client, round4, b, gradient_norm=1.0,
                        training_data_size=1_000_000)
    aggregate(client, coord, round4, {b.quid.id: 0.25 for b in banks[:-1]})

    time.sleep(0.5)
    strict = RoundPolicy(min_participants=3, strict_registration=True)
    audit_and_show(client, round4, "ROUND 4 strict (expect integrity-violation)",
                    policy=strict)

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - Each FL round is its own quid. The round's event stream is")
    print("   the canonical record -- register, submit, aggregate are all")
    print("   signed events.")
    print(" - The audit function is a pure replay over the stream.")
    print("   Any participant or auditor can run it independently and")
    print("   reach the same verdict.")
    print(" - Suspicious gradients are surfaced with a robust threshold")
    print("   against the round's median norm, without invalidating the")
    print("   whole round.")
    print(" - Strict-registration policy catches registered-but-absent")
    print("   participants, a step beyond what a bare coordinator log")
    print("   would show.")
    print()


if __name__ == "__main__":
    main()
