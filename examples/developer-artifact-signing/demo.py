"""Developer artifact signing, end-to-end runnable demo.

Flow:
  1. Register actors: maintainers alice/bob/carol, a consumer,
     and a security researcher.
  2. Consumer establishes direct trust in alice.
  3. Alice publishes "webapp-js@1.0.0":
       - register_title establishes ownership
       - emit `release.published` event with the artifact hash
  4. Consumer fetches the artifact + events, runs verify_artifact.
     Expected verdict: accept.
  5. Security researcher reports CVE-2026-0001 against v1.0.0:
     emit `release.vulnerability-reported` event.
  6. Consumer re-verifies v1.0.0. Expected verdict: warn (unpatched
     HIGH-sev CVE).
  7. Alice publishes v1.0.1 that fixes the CVE:
       - register_title for v1.0.1
       - emit `release.published` + `release.vulnerability-patched`.
  8. Consumer verifies v1.0.1 -> accept.
  9. Alice's key gets compromised. She revokes v1.0.0:
     emit `release.revoked`. Consumer re-verifies v1.0.0 -> reject.

Prerequisites:
  - Local Quidnug node at http://localhost:8080.
  - Python SDK installed.

Run:
    python demo.py
"""

from __future__ import annotations

import hashlib
import os
import sys
import time
import uuid
from dataclasses import dataclass
from typing import Dict, List, Optional

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from artifact_verify import ReleaseV1, sha256_hex, verify_artifact

from quidnug import OwnershipStake, Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "developer.signing.npm"


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


def publish_release(
    client: QuidnugClient, maintainer: Actor,
    release_id: str, package: str, version: str,
    artifact_bytes: bytes, commit_hash: str, repo: str,
) -> ReleaseV1:
    artifact_hash = sha256_hex(artifact_bytes)
    # 1. Register the TITLE (ownership).
    try:
        client.register_title(
            signer=maintainer.quid,
            asset_id=release_id,
            owners=[OwnershipStake(maintainer.quid.id, 1.0, "maintainer")],
            domain=DOMAIN,
            title_type="software-release",
        )
    except Exception as e:
        print(f"  (register_title {release_id}: {e})")
    client.wait_for_title(release_id)

    # 2. Attach metadata via event on the title stream.
    client.emit_event(
        signer=maintainer.quid,
        subject_id=release_id,
        subject_type="TITLE",
        event_type="release.published",
        domain=DOMAIN,
        payload={
            "packageName": package,
            "version": version,
            "artifactHash": artifact_hash,
            "repository": repo,
            "commitHash": commit_hash,
            "publishedAt": int(time.time()),
            "buildEnvironment": "github-actions-ubuntu-22.04",
        },
    )

    print(f"  {maintainer.name:14s} published {package}@{version}")
    print(f"                    release_id={release_id}")
    print(f"                    artifact_hash={artifact_hash[:16]}...")

    return ReleaseV1(
        release_id=release_id,
        package_name=package,
        version=version,
        maintainer_quid=maintainer.quid.id,
        artifact_hash_hex=artifact_hash,
        repository=repo,
        commit_hash=commit_hash,
        published_at_unix=int(time.time()),
    )


def report_cve(
    client: QuidnugClient, reporter: Actor, release_id: str,
    cve_id: str, severity: str, description: str,
) -> None:
    """Researcher publishes a CVE report on their OWN stream,
    with the target release_id in the payload. Consumers who
    subscribe to a researcher's feed pick it up during
    verification. (The release's title stream can only be
    written to by its owners.)"""
    client.emit_event(
        signer=reporter.quid,
        subject_id=reporter.quid.id,
        subject_type="QUID",
        event_type="release.vulnerability-reported",
        domain=DOMAIN,
        payload={
            "cveId": cve_id,
            "severity": severity,
            "reporter": reporter.quid.id,
            "targetReleaseId": release_id,
            "description": description,
            "reportedAt": int(time.time()),
        },
    )
    print(f"  {reporter.name:14s} reported {cve_id} ({severity}) against {release_id}")


def patch_cve(
    client: QuidnugClient, maintainer: Actor,
    vulnerable_release_id: str, cve_id: str, patched_in_version: str,
) -> None:
    client.emit_event(
        signer=maintainer.quid,
        subject_id=vulnerable_release_id,
        subject_type="TITLE",
        event_type="release.vulnerability-patched",
        domain=DOMAIN,
        payload={
            "cveId": cve_id,
            "patchedInVersion": patched_in_version,
            "patchedAt": int(time.time()),
        },
    )
    print(f"  {maintainer.name:14s} patched {cve_id}, fix in {patched_in_version}")


def revoke_release(
    client: QuidnugClient, maintainer: Actor, release_id: str, reason: str,
) -> None:
    client.emit_event(
        signer=maintainer.quid,
        subject_id=release_id,
        subject_type="TITLE",
        event_type="release.revoked",
        domain=DOMAIN,
        payload={
            "reason": reason,
            "revokedAt": int(time.time()),
        },
    )
    print(f"  {maintainer.name:14s} revoked {release_id}: {reason}")


def load_release_events(
    client: QuidnugClient, release_id: str,
    researchers: Optional[List[Actor]] = None,
) -> List[dict]:
    """Pull the release's event stream, merged with any CVE
    reports from trusted researchers' streams that target this
    release.

    Researchers publish CVEs on their own QUID stream because
    the protocol forbids non-owners from writing to the
    artifact's title stream. Consumers subscribe to a curated
    researcher list and cross-reference.
    """
    out: List[dict] = []

    # 1. Lifecycle events on the release's title stream.
    events, _ = client.get_stream_events(release_id, limit=200)
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
            "timestamp": ev.timestamp,
            "sequence": ev.sequence,
        })

    # 2. CVE reports from trusted researcher streams, filtered
    #    to this release.
    for r in researchers or []:
        r_events, _ = client.get_stream_events(r.quid.id, limit=200)
        for ev in r_events or []:
            if ev.event_type != "release.vulnerability-reported":
                continue
            p = ev.payload or {}
            if p.get("targetReleaseId") == release_id:
                out.append({
                    "eventType": ev.event_type,
                    "payload": p,
                    "timestamp": ev.timestamp,
                    "sequence": ev.sequence,
                })
    return out


def verify_and_show(
    client: QuidnugClient, consumer: Actor, release: ReleaseV1,
    artifact_bytes: bytes, label: str,
    trusted_researchers: Optional[List[Actor]] = None,
) -> None:
    def trust_fn(obs: str, maint: str) -> float:
        try:
            r = client.get_trust(obs, maint, domain=DOMAIN, max_depth=5)
            return r.trust_level if r else 0.0
        except Exception:
            return 0.0

    events = load_release_events(
        client, release.release_id, trusted_researchers,
    )
    v = verify_artifact(
        consumer.quid.id, release, artifact_bytes, events, trust_fn,
        min_trust=0.5,
    )
    print(f"\n  [{label}] {release.describe()}")
    print(f"    VERDICT = {v.verdict.upper():7s}  trust={v.maintainer_trust:.3f}")
    for r in v.reasons:
        print(f"      - {r}")
    if v.unpatched_vulns:
        print(f"      unpatched_vulns: {', '.join(v.unpatched_vulns)}")


def main() -> None:
    print(f"Connecting to Quidnug node at {NODE_URL}")
    client = QuidnugClient(NODE_URL)
    try:
        client.info()
    except Exception as e:
        print(f"node unreachable: {e}", file=sys.stderr)
        sys.exit(1)

    client.ensure_domain(DOMAIN)

    banner("Step 1: Register actors")
    alice     = register(client, "alice-maintainer",  "maintainer")
    bob       = register(client, "bob-maintainer",    "maintainer")
    researcher = register(client, "sec-researcher-x", "researcher")
    consumer  = register(client, "consumer-acme",    "consumer")
    for a in (alice, bob, researcher, consumer):
        print(f"  {a.role:14s} {a.name:20s} -> {a.quid.id}")
    client.wait_for_identities([a.quid.id for a in (alice, bob, researcher, consumer)])

    banner("Step 2: Consumer establishes direct trust in alice")
    client.grant_trust(
        signer=consumer.quid,
        trustee=alice.quid.id,
        level=0.9,
        domain=DOMAIN,
        description="audited alice's track record on 2024 releases",
    )
    # Consumer weakly trusts bob (indirect, different track record).
    client.grant_trust(
        signer=consumer.quid,
        trustee=bob.quid.id,
        level=0.4,
        domain=DOMAIN,
        description="bob is new, unproven",
    )
    print(f"  consumer -[0.9]-> alice  (direct trust)")
    print(f"  consumer -[0.4]-> bob    (low trust, below acceptance threshold)")

    time.sleep(1)

    banner("Step 3: Alice publishes webapp-js@1.0.0")
    tarball_v1 = b"webapp-js@1.0.0: real-looking tarball contents " + os.urandom(64)
    release_id_v1 = f"webapp-js-1.0.0-{uuid.uuid4().hex[:6]}"
    r1 = publish_release(
        client, alice, release_id_v1, "webapp-js", "1.0.0",
        tarball_v1, commit_hash="abc1234567890def", repo="github.com/acme/webapp-js",
    )

    time.sleep(1)

    banner("Step 4: Consumer verifies the v1.0.0 artifact")
    verify_and_show(client, consumer, r1, tarball_v1, "HEALTHY v1.0.0")

    banner("Step 5: Security researcher reports CVE-2026-0001")
    report_cve(
        client, researcher, release_id_v1,
        "CVE-2026-0001", "HIGH",
        "prototype pollution in deep-merge helper",
    )
    time.sleep(1)

    banner("Step 6: Consumer re-verifies v1.0.0 (expect: warn)")
    verify_and_show(client, consumer, r1, tarball_v1, "WITH UNPATCHED CVE", [researcher])

    banner("Step 7: Alice publishes v1.0.1 with the fix")
    tarball_v2 = b"webapp-js@1.0.1: patched tarball contents " + os.urandom(64)
    release_id_v2 = f"webapp-js-1.0.1-{uuid.uuid4().hex[:6]}"
    r2 = publish_release(
        client, alice, release_id_v2, "webapp-js", "1.0.1",
        tarball_v2, commit_hash="fedcba09876543210",
        repo="github.com/acme/webapp-js",
    )
    # Record the patch against v1.0.0's stream.
    patch_cve(client, alice, release_id_v1, "CVE-2026-0001", "1.0.1")
    time.sleep(1)

    banner("Step 8: Consumer verifies v1.0.1 (expect: accept)")
    verify_and_show(client, consumer, r2, tarball_v2, "PATCHED v1.0.1", [researcher])

    banner("Step 8b: v1.0.0 after patch-event posted")
    # Note: v1.0.0 still has the unpatched-high-sev tag because
    # our policy only counts a CVE as "fixed for this release"
    # when a patch event names this release's stream. A downstream
    # policy could instead resolve the chain via
    # `patchedInVersion` and mark earlier versions as superseded.
    verify_and_show(client, consumer, r1, tarball_v1, "v1.0.0 SUPERSEDED", [researcher])

    banner("Step 9: Alice's key is compromised -> she revokes v1.0.0")
    revoke_release(client, alice, release_id_v1, "key-compromise-2026-04-01")
    time.sleep(1)
    verify_and_show(client, consumer, r1, tarball_v1, "POST-REVOCATION v1.0.0", [researcher])

    banner("Step 10: Hash-mismatch sanity check")
    tampered = tarball_v2 + b"extra malicious payload"
    verify_and_show(client, consumer, r2, tampered, "TAMPERED v1.0.1")

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - A release is a TITLE (ownership) + event stream (metadata,")
    print("   lifecycle, security). Nothing else is required for full")
    print("   consumer verification.")
    print(" - Hash mismatch is a hard reject. Trust is a soft decision.")
    print(" - Revocation and vulnerability reports are just signed")
    print("   events. Every consumer sees the same stream; every")
    print("   consumer applies their own policy to it.")
    print(" - Multi-maintainer would just mean multiple OwnershipStakes")
    print("   with stake_type='maintainer'. Guardian recovery on the")
    print("   maintainer quid handles key loss with no downstream")
    print("   reconfiguration.")
    print()


if __name__ == "__main__":
    main()
