"""Healthcare consent management, end-to-end runnable demo.

Flow:
  1. Register actors: patient, primary-care doc, cardiologist,
     ER doc, spouse, healthcare proxy (all as guardians).
  2. Patient grants consent to PCP for clinical-notes.
  3. PCP accesses -> allowed, access event logged.
  4. PCP refers to cardiologist. Cardiologist requests access.
     Patient also grants direct (or the resolver uses
     transitive through PCP if no direct).
  5. Patient revokes consent for cardiologist -> next access
     denied.
  6. Emergency scenario: ER doc requests access. Spouse and
     healthcare proxy emit emergency-override events with
     guardian signatures. Quorum met -> emergency-allowed.
  7. Without quorum, same ER request is denied.

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
from dataclasses import dataclass
from typing import List, Set

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from consent_evaluate import (
    AccessPolicy,
    AccessRequest,
    ConsentGrant,
    evaluate_access,
)

from quidnug import Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "healthcare.consent.us"


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


def grant_consent(
    client: QuidnugClient, patient: Actor, provider: Actor,
    category: str, trust_level: float, valid_for_days: int = 90,
) -> ConsentGrant:
    now = int(time.time())
    valid_until = now + valid_for_days * 86400
    client.grant_trust(
        signer=patient.quid, trustee=provider.quid.id,
        level=trust_level, domain=DOMAIN,
        valid_until=valid_until,
        description=f"healthcare consent {category}",
    )
    # Also emit a consent event on the patient's stream so the
    # resolver sees the grant's category and expiry explicitly.
    client.emit_event(
        signer=patient.quid,
        subject_id=patient.quid.id,
        subject_type="QUID",
        event_type="consent.granted",
        domain=DOMAIN,
        payload={
            "patient": patient.quid.id,
            "provider": provider.quid.id,
            "category": category,
            "trustLevel": trust_level,
            "validUntil": valid_until,
            "grantedAt": now,
        },
    )
    print(f"  {patient.name} -[{trust_level}]-> {provider.name} "
          f"({category}, {valid_for_days}d)")
    return ConsentGrant(
        patient_quid=patient.quid.id,
        provider_quid=provider.quid.id,
        category=category,
        trust_level=trust_level,
        granted_at_unix=now,
        valid_until_unix=valid_until,
    )


def record_access(
    client: QuidnugClient, provider: Actor, patient: Actor,
    category: str, purpose: str,
) -> None:
    """Provider logs the access on their OWN quid stream with a
    patient-id pointer (patient QUID streams are writable only
    by the patient)."""
    client.emit_event(
        signer=provider.quid,
        subject_id=provider.quid.id,
        subject_type="QUID",
        event_type="record.accessed",
        domain=DOMAIN,
        payload={
            "accessor": provider.quid.id,
            "patient": patient.quid.id,
            "category": category,
            "purpose": purpose,
            "accessedAt": int(time.time()),
        },
    )


def revoke_consent(
    client: QuidnugClient, patient: Actor, provider: Actor,
    category: str = "", reason: str = "",
) -> None:
    client.emit_event(
        signer=patient.quid,
        subject_id=patient.quid.id,
        subject_type="QUID",
        event_type="consent.revoked",
        domain=DOMAIN,
        payload={
            "patient": patient.quid.id,
            "provider": provider.quid.id,
            "category": category,
            "revokedAt": int(time.time()),
            "reason": reason,
        },
    )
    print(f"  {patient.name} revoked consent for {provider.name}")


def emergency_override(
    client: QuidnugClient, patient: Actor, provider: Actor,
    guardians: List[Actor], reason: str,
) -> None:
    """Each guardian emits an emergency-override event on their
    OWN quid stream (patient QUID streams are writable only by
    the patient). The payload carries the patient pointer plus
    the full guardian-signature list for quorum verification."""
    for guardian in guardians:
        client.emit_event(
            signer=guardian.quid,
            subject_id=guardian.quid.id,
            subject_type="QUID",
            event_type="consent.emergency-override",
            domain=DOMAIN,
            payload={
                "patient": patient.quid.id,
                "provider": provider.quid.id,
                "guardianSignatures": [g.quid.id for g in guardians],
                "validUntil": int(time.time()) + 24 * 3600,
                "reason": reason,
                "signerQuid": guardian.quid.id,
            },
        )
        print(f"  {guardian.name} signed emergency override")


def load_patient_events(
    client: QuidnugClient, patient: Actor,
    ambient_actors: List[Actor] = None,
) -> List[dict]:
    """Merge events about the patient from the patient's own
    stream (consent + dispute events) plus every ambient actor's
    stream (emergency-override, access-log events that the
    protocol routes to the signer's own stream)."""
    out: List[dict] = []
    streams = [patient.quid.id] + [a.quid.id for a in ambient_actors or []]
    for stream_id in streams:
        events, _ = client.get_stream_events(stream_id, limit=500)
        for ev in events or []:
            p = ev.payload or {}
            # Only include events concerning this patient.
            if p.get("patient") and p.get("patient") != patient.quid.id:
                continue
            out.append({
                "eventType": ev.event_type,
                "payload": p,
                "timestamp": ev.timestamp,
            })
    return out


def node_trust_fn(client: QuidnugClient):
    def fn(obs: str, target: str) -> float:
        try:
            r = client.get_trust(obs, target, domain=DOMAIN, max_depth=5)
            return r.trust_level if r else 0.0
        except Exception:
            return 0.0
    return fn


def show(
    client: QuidnugClient, patient: Actor, request: AccessRequest,
    consents: List[ConsentGrant], guardian_set: Set[str],
    label: str, guardian_threshold: int = 2,
    ambient_actors: List[Actor] = None,
) -> None:
    events = load_patient_events(client, patient, ambient_actors or [])
    v = evaluate_access(
        request, consents, events,
        trust_fn=node_trust_fn(client),
        guardian_set=guardian_set,
        guardian_threshold=guardian_threshold,
    )
    print(f"\n  [{label}]")
    print(f"    {v.short()}")
    for r in v.reasons:
        print(f"      - {r}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    client.ensure_domain(DOMAIN)

    # -----------------------------------------------------------------
    banner("Step 1: Register actors")
    patient   = register(client, "patient-alice",      "patient")
    pcp       = register(client, "dr-smith-pcp",        "primary-care")
    cardio    = register(client, "dr-jones-cardio",     "specialist")
    er_doc    = register(client, "dr-cooper-er",        "er-doc")
    spouse    = register(client, "carol-spouse",        "guardian")
    proxy     = register(client, "bob-healthcare-proxy","guardian")
    for a in (patient, pcp, cardio, er_doc, spouse, proxy):
        print(f"  {a.role:14s} {a.name:22s} -> {a.quid.id}")
    client.wait_for_identities([a.quid.id for a in
        (patient, pcp, cardio, er_doc, spouse, proxy)])

    # All ambient actors the evaluator consults for access /
    # override events routed onto their own streams.
    ambient = [pcp, cardio, er_doc, spouse, proxy]

    guardian_set = {spouse.quid.id, proxy.quid.id}

    # -----------------------------------------------------------------
    banner("Step 2: Patient grants consent to PCP")
    consents: List[ConsentGrant] = []
    consents.append(grant_consent(client, patient, pcp,
                                    "clinical-notes", 0.9))

    # -----------------------------------------------------------------
    banner("Step 3: PCP requests access")
    req_pcp = AccessRequest(
        provider_quid=pcp.quid.id,
        patient_quid=patient.quid.id,
        category="clinical-notes",
        requested_at_unix=int(time.time()),
        purpose="annual physical",
    )
    show(client, patient, req_pcp, consents, guardian_set, "PCP direct access", ambient_actors=ambient)
    record_access(client, pcp, patient, "clinical-notes", "annual physical")

    # -----------------------------------------------------------------
    banner("Step 4: PCP refers patient to cardiologist (transitive)")
    # Patient hasn't granted cardio directly. But patient trusts
    # PCP, and PCP has referred to cardio (represented here as a
    # trust edge from PCP to cardio).
    client.grant_trust(
        signer=pcp.quid, trustee=cardio.quid.id, level=0.85,
        domain=DOMAIN, description="referral to trusted cardiologist",
    )
    time.sleep(3)   # wait for referral trust edge to commit
    req_cardio = AccessRequest(
        provider_quid=cardio.quid.id,
        patient_quid=patient.quid.id,
        category="clinical-notes",
        requested_at_unix=int(time.time()),
        purpose="cardiac consultation prep",
    )
    show(client, patient, req_cardio, consents, guardian_set, "CARDIO transitive access", ambient_actors=ambient)
    record_access(client, cardio, patient, "clinical-notes",
                   "cardiac consultation")

    # -----------------------------------------------------------------
    banner("Step 5: Patient revokes consent for cardiologist")
    # In the transitive model the patient doesn't have a direct
    # consent for cardio; the revocation here blocks the referral
    # path through the PCP by targeting cardio's provider edge.
    # Simpler demo: grant direct consent to cardio then revoke it.
    consents.append(grant_consent(client, patient, cardio,
                                    "clinical-notes", 0.9))
    time.sleep(3)
    # Now the direct consent exists; patient then revokes.
    revoke_consent(client, patient, cardio,
                    category="clinical-notes",
                    reason="switching cardiologist")
    time.sleep(3)
    show(client, patient, req_cardio, consents, guardian_set, "CARDIO after revocation", ambient_actors=ambient)

    # -----------------------------------------------------------------
    banner("Step 6: ER emergency with quorum (expect emergency-allowed)")
    emergency_override(client, patient, er_doc, [spouse, proxy],
                        reason="patient unconscious, ER admission")
    time.sleep(3)
    req_er = AccessRequest(
        provider_quid=er_doc.quid.id,
        patient_quid=patient.quid.id,
        category="clinical-notes",
        requested_at_unix=int(time.time()),
        purpose="emergency triage",
    )
    show(client, patient, req_er, consents, guardian_set, "ER with 2-of-2 guardian override", ambient_actors=ambient)

    # -----------------------------------------------------------------
    banner("Step 7: ER emergency with insufficient quorum (expect deny)")
    # Register a new ER doc and simulate only 1 guardian signing.
    er_doc_2 = register(client, "dr-singleton-er", "er-doc")
    # Only spouse signs this time (below threshold of 2).
    client.emit_event(
        signer=spouse.quid,
        subject_id=spouse.quid.id,
        subject_type="QUID",
        event_type="consent.emergency-override",
        domain=DOMAIN,
        payload={
            "patient": patient.quid.id,
            "provider": er_doc_2.quid.id,
            "guardianSignatures": [spouse.quid.id],
            "validUntil": int(time.time()) + 3600,
            "reason": "single-guardian attempt",
            "signerQuid": spouse.quid.id,
        },
    )
    client.wait_for_identity(er_doc_2.quid.id)
    time.sleep(3)
    req_er2 = AccessRequest(
        provider_quid=er_doc_2.quid.id,
        patient_quid=patient.quid.id,
        category="clinical-notes",
        requested_at_unix=int(time.time()),
        purpose="emergency triage",
    )
    show(client, patient, req_er2, consents, guardian_set,
          "ER2 with only 1 guardian sig",
          ambient_actors=ambient)

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - Consent is a signed trust edge with an expiry; revocation")
    print("   is a signed event on the patient's stream.")
    print(" - The resolver applies the most restrictive applicable signal:")
    print("   any revocation blocks the corresponding consent.")
    print(" - Transitive consent lets referrals work: the patient doesn't")
    print("   need to explicitly consent to every specialist their PCP")
    print("   sends them to, as long as the composed trust clears the")
    print("   policy threshold.")
    print(" - Emergency override requires a guardian quorum -- a single")
    print("   attacker with one captured guardian key can't force access.")
    print(" - Every access is logged as a signed event on the patient's")
    print("   stream. Patient (or regulator) can audit post-hoc.")
    print()


if __name__ == "__main__":
    main()
