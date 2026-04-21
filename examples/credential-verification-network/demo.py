"""Credential verification network — end-to-end runnable demo.

Flow:
  1. Actors: accreditor, two universities, one student, two
     employers (US + APAC).
  2. Accreditor issues trust edges to universities.
  3. Universities issue credentials (degrees) to students as
     EVENT transactions on the student's credential stream.
  4. Employer walks the trust graph: employer → accreditor →
     university → credential. Accepts if transitive trust
     exceeds their policy threshold.
  5. Demonstrate revocation: university publishes a
     revocation event; re-verify → reject.
  6. Demonstrate observer-relative verdicts: APAC employer
     with a different accreditation graph reaches a
     different decision on the same credential.

Prerequisites:
  - Local Quidnug node at http://localhost:8080.
  - Python SDK installed: `pip install -e clients/python`.

Run:
    python demo.py
"""

from __future__ import annotations

import os
import sys
import time
import uuid
from dataclasses import dataclass
from typing import Dict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from credential_verify import CredentialV1, verify_credential

from quidnug import Quid, QuidnugClient


NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "credentials.education"


@dataclass
class Actor:
    name: str
    kind: str  # "accreditor" | "university" | "student" | "employer"
    quid: Quid


def banner(msg: str) -> None:
    print()
    print("=" * 72)
    print(f"  {msg}")
    print("=" * 72)


def _register(client: QuidnugClient, name: str, kind: str) -> Actor:
    q = Quid.generate()
    try:
        client.register_identity(
            q, name=name, domain=DOMAIN, home_domain=DOMAIN,
            attributes={"kind": kind},
        )
    except Exception as e:
        print(f"  (register_identity {name}: {e})")
    return Actor(name=name, kind=kind, quid=q)


def bootstrap(client: QuidnugClient) -> Dict[str, Actor]:
    banner("Step 1: Register actors")
    actors: Dict[str, Actor] = {}
    for name, kind in [
        ("sacscoc", "accreditor"),          # US accreditor
        ("higher-ed-apac", "accreditor"),   # APAC accreditor
        ("stanford-uni", "university"),
        ("apac-tech-u", "university"),
        ("alice-student", "student"),
        ("us-employer", "employer"),
        ("apac-employer", "employer"),
    ]:
        a = _register(client, name, kind)
        actors[name] = a
        print(f"  {kind:12s} {name:18s} -> {a.quid.id}")
    # Wait for every identity to reach the committed registry
    # before follow-on trust / event transactions reference them.
    client.wait_for_identities([a.quid.id for a in actors.values()])
    return actors


def establish_accreditation(client: QuidnugClient, actors: Dict[str, Actor]) -> None:
    banner("Step 2: Accreditors vouch for universities")
    edges = [
        ("sacscoc",        "stanford-uni", 0.95, "accredited US university"),
        ("higher-ed-apac", "apac-tech-u",  0.95, "accredited APAC university"),
    ]
    for truster, trustee, level, desc in edges:
        client.grant_trust(
            signer=actors[truster].quid,
            trustee=actors[trustee].quid.id,
            level=level,
            domain=DOMAIN,
            description=desc,
        )
        print(f"  {truster:18s} -[{level}]-> {trustee}")


def employers_trust_accreditors(client: QuidnugClient, actors: Dict[str, Actor]) -> None:
    banner("Step 3: Employers declare trust in specific accreditors")
    # US employer trusts US accreditor + (weakly) APAC accreditor.
    # APAC employer trusts APAC accreditor + US accreditor.
    edges = [
        ("us-employer",   "sacscoc",        0.9,  "primary US standards body"),
        ("us-employer",   "higher-ed-apac", 0.3,  "unfamiliar; cross-border caution"),
        ("apac-employer", "higher-ed-apac", 0.9,  "local standards body"),
        ("apac-employer", "sacscoc",        0.7,  "reputable cross-border"),
    ]
    for truster, trustee, level, desc in edges:
        client.grant_trust(
            signer=actors[truster].quid,
            trustee=actors[trustee].quid.id,
            level=level,
            domain=DOMAIN,
            description=desc,
        )
        print(f"  {truster:14s} -[{level}]-> {trustee:18s} ({desc})")


def issue_credential(
    client: QuidnugClient, actors: Dict[str, Actor],
    university_name: str, credential_id: str, credential_type: str, grade: str,
) -> CredentialV1:
    """University issues a credential to a student as a TITLE
    jointly owned (student primary + university issuer stake),
    plus a `credential.issued` event on the title's stream.

    The protocol requires events on a TITLE stream to come from
    an owner; joint ownership lets both the student (hold) and
    the university (issue/revoke) sign events on the same
    credential.
    """
    from quidnug import OwnershipStake
    uni = actors[university_name]
    student = actors["alice-student"]
    now = int(time.time())

    # Create the title with joint ownership. The student holds
    # the credential (1.0 weight for presentation); the issuer
    # retains an issuer stake for revocation authority.
    client.register_title(
        signer=uni.quid,
        asset_id=credential_id,
        owners=[
            OwnershipStake(student.quid.id, 0.99, "holder"),
            OwnershipStake(uni.quid.id,     0.01, "issuer"),
        ],
        domain=DOMAIN,
        title_type="academic-credential",
    )
    # Wait for the title to commit before emitting events on it.
    client.wait_for_title(credential_id)

    payload = {
        "credentialId": credential_id,
        "credentialType": credential_type,
        "grade": grade,
        "issuedAt": now,
        "subjectQuid": student.quid.id,
        "issuerQuid": uni.quid.id,
    }
    client.emit_event(
        signer=uni.quid,
        subject_id=credential_id,
        subject_type="TITLE",
        event_type="credential.issued",
        domain=DOMAIN,
        payload=payload,
    )
    print(
        f"  {uni.name:14s} issued {credential_type} ({grade}) to "
        f"{student.name} [id={credential_id}]"
    )
    return CredentialV1(
        credential_id=credential_id,
        issuer_quid=uni.quid.id,
        subject_quid=student.quid.id,
        credential_type=credential_type,
        grade=grade,
        issued_at_unix=now,
    )


def revoke_credential(
    client: QuidnugClient, actors: Dict[str, Actor],
    university_name: str, credential_id: str, reason: str,
) -> None:
    """University publishes a revocation event on the credential's
    title stream."""
    uni = actors[university_name]
    payload = {
        "credentialId": credential_id,
        "reason": reason,
        "revokedAt": int(time.time()),
        "issuerQuid": uni.quid.id,
    }
    client.emit_event(
        signer=uni.quid,
        subject_id=credential_id,
        subject_type="TITLE",
        event_type="credential.revoked",
        domain=DOMAIN,
        payload=payload,
    )


def load_revocations_for(
    client: QuidnugClient, credential_ids: list,
) -> Dict[str, str]:
    """Scan each credential title's stream for revocation events.
    Returns a dict mapping credential_id -> reason."""
    out: Dict[str, str] = {}
    for cid in credential_ids:
        events, _ = client.get_stream_events(cid, limit=100)
        for ev in events or []:
            if ev.event_type == "credential.revoked":
                p = ev.payload or {}
                out[p.get("credentialId", cid)] = p.get("reason", "revoked")
    return out


def verify_via_node(
    client: QuidnugClient, observer: Actor, credential: CredentialV1,
    revocations: Dict[str, str], label: str,
) -> None:
    banner(f"Step 5 ({label}): {observer.name} verifies {credential.credential_id}")

    def trust_path(obs: str, issuer: str) -> float:
        try:
            r = client.get_trust(obs, issuer, domain=DOMAIN, max_depth=5)
            return r.trust_level if r else 0.0
        except Exception:
            return 0.0

    def rev_fn(cid: str):
        return revocations.get(cid)

    verdict = verify_credential(
        observer.quid.id, credential, trust_path, rev_fn,
        min_accept_score=0.6,
    )
    print(f"\n  {credential.describe()}")
    print()
    print(f"  VERDICT: {verdict.verdict.upper():15s}  "
          f"trust path score = {verdict.trust_path_score:.3f}")
    for r in verdict.reasons:
        print(f"    - {r}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    client.ensure_domain(DOMAIN)

    actors = bootstrap(client)
    establish_accreditation(client, actors)
    employers_trust_accreditors(client, actors)

    time.sleep(1)

    issued_ids: list = []

    banner("Step 4: Stanford issues Alice's degree")
    cred_id = f"cred-{uuid.uuid4().hex[:8]}"
    credential = issue_credential(
        client, actors, "stanford-uni", cred_id,
        "degree.bachelors.cs", "3.8",
    )
    issued_ids.append(cred_id)

    time.sleep(3)   # wait for title + issue event to commit

    # Fresh load of revocation events on each known title stream.
    revocations = load_revocations_for(client, issued_ids)

    # US employer verifies (should accept via sacscoc chain).
    verify_via_node(client, actors["us-employer"], credential, revocations, "US EMPLOYER")
    # APAC employer verifies (should also accept via sacscoc@0.7 * stanford@0.95 = 0.665).
    verify_via_node(client, actors["apac-employer"], credential, revocations, "APAC EMPLOYER")

    # Revoke + re-verify.
    banner("Step 6: Stanford revokes Alice's degree")
    revoke_credential(client, actors, "stanford-uni", cred_id, "academic-integrity-violation")
    time.sleep(3)
    revocations = load_revocations_for(client, issued_ids)
    verify_via_node(client, actors["us-employer"], credential, revocations, "US EMPLOYER POST-REVOCATION")

    # Demonstrate cross-jurisdiction gap: APAC university
    # issues a credential; US employer doesn't recognize the
    # APAC accreditor strongly.
    banner("Step 7: Cross-jurisdiction — APAC-only credential verified by US employer")
    apac_cred_id = f"cred-{uuid.uuid4().hex[:8]}"
    apac_cred = issue_credential(
        client, actors, "apac-tech-u", apac_cred_id,
        "degree.bachelors.eng", "first-class",
    )
    issued_ids.append(apac_cred_id)
    time.sleep(3)
    revocations = load_revocations_for(client, issued_ids)
    # US employer's chain: us-employer -[0.3]-> higher-ed-apac -[0.95]-> apac-tech-u
    # Composed ~ 0.285, below 0.6 threshold → indeterminate.
    verify_via_node(client, actors["us-employer"], apac_cred, revocations, "US EMPLOYER (CROSS-JURISDICTION)")
    verify_via_node(client, actors["apac-employer"], apac_cred, revocations, "APAC EMPLOYER (LOCAL)")

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - Transitive trust composes: employer never heard of Stanford,")
    print("   but trusted the accreditor who trusts Stanford.")
    print(" - Revocation is just another signed event; the verifier")
    print("   consults the issuer's stream at decision time.")
    print(" - Cross-jurisdiction verdicts differ by observer without")
    print("   any central authority deciding — each employer sets their own")
    print("   policy and trust graph.\n")


if __name__ == "__main__":
    main()
