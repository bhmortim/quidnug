"""AI content authenticity, end-to-end runnable demo.

Flow:
  1. Register actors: camera (canon-quid), photographer, editor
     at Reuters, Reuters publisher, fact-checker, consumer
     newsroom. Plus an AI image-generation model and a news
     aggregator for the AI scenario.
  2. Consumer newsroom establishes trust in the capture /
     editor / publisher parties.
  3. Scenario A: authentic news photo. Camera captures -> photog
     crops -> editor grades -> Reuters publishes -> fact-checker
     endorses. Consumer evaluates -> accept.
  4. Scenario B: same asset but the hash chain is tampered
     (editor output doesn't match the next input). -> reject.
  5. Scenario C: AI-generated image. model-v2 generates -> news
     aggregator publishes. Consumer's default policy warns;
     consumer raises policy to reject; consumer can also accept.

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
from typing import List

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from content_authenticity import (
    AuthenticityPolicy,
    MediaAssetV1,
    ORIGIN_CAPTURED,
    ORIGIN_GENERATED,
    evaluate_authenticity,
    extract_provenance,
)

from quidnug import OwnershipStake, Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "media.provenance.news"


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


def sha256_hex(s: str) -> str:
    return hashlib.sha256(s.encode("utf-8")).hexdigest()


def capture_asset(
    client: QuidnugClient, camera: Actor, photographer: Actor,
    asset_id: str, original_hash: str,
) -> None:
    """Register the asset as a TITLE owned by the photographer
    and emit a media.captured event signed by the camera."""
    client.register_title(
        signer=photographer.quid,
        asset_id=asset_id,
        owners=[OwnershipStake(photographer.quid.id, 1.0, "photographer")],
        domain=DOMAIN,
        title_type="media-asset",
    )
    client.emit_event(
        signer=camera.quid,
        subject_id=asset_id,
        subject_type="TITLE",
        event_type="media.captured",
        domain=DOMAIN,
        payload={
            "captureDevice": camera.quid.id,
            "outputHash": original_hash,
            "assetType": "photo",
            "capturedAt": int(time.time()),
            "captureParams": {"iso": 400, "aperture": "f/2.8"},
        },
    )


def generate_asset(
    client: QuidnugClient, model: Actor, asset_id: str, hash_: str,
) -> None:
    """Register an AI-generated asset."""
    client.register_title(
        signer=model.quid,
        asset_id=asset_id,
        owners=[OwnershipStake(model.quid.id, 1.0, "generator")],
        domain=DOMAIN,
        title_type="media-asset",
    )
    client.emit_event(
        signer=model.quid,
        subject_id=asset_id,
        subject_type="TITLE",
        event_type="media.generated",
        domain=DOMAIN,
        payload={
            "generator": model.quid.id,
            "prompt": "sunset over mountains",
            "seed": 42,
            "outputHash": hash_,
        },
    )


def edit(
    client: QuidnugClient, editor: Actor, asset_id: str,
    step: str, input_hash: str, output_hash: str,
    software: str = "Lightroom",
) -> None:
    client.emit_event(
        signer=editor.quid,
        subject_id=asset_id,
        subject_type="TITLE",
        event_type=step,
        domain=DOMAIN,
        payload={
            "editor": editor.quid.id,
            "inputHash": input_hash,
            "outputHash": output_hash,
            "software": software,
        },
    )


def publish(
    client: QuidnugClient, publisher: Actor, asset_id: str, story: str,
) -> None:
    client.emit_event(
        signer=publisher.quid,
        subject_id=asset_id,
        subject_type="TITLE",
        event_type="media.published",
        domain=DOMAIN,
        payload={"publisher": publisher.quid.id, "storyId": story,
                  "publishedAt": int(time.time())},
    )


def fact_check(
    client: QuidnugClient, checker: Actor, asset_id: str,
    assessment: str,
) -> None:
    client.emit_event(
        signer=checker.quid,
        subject_id=asset_id,
        subject_type="TITLE",
        event_type="media.fact-checked",
        domain=DOMAIN,
        payload={"checker": checker.quid.id, "assessment": assessment,
                  "checkedAt": int(time.time())},
    )


def load_events(client: QuidnugClient, asset_id: str) -> List[dict]:
    events, _ = client.get_stream_events(asset_id, limit=200)
    out: List[dict] = []
    for ev in events or []:
        out.append({
            "eventType": ev.event_type,
            "payload": ev.payload or {},
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


def evaluate_and_show(
    client: QuidnugClient, consumer: Actor, asset: MediaAssetV1,
    label: str, policy: AuthenticityPolicy = None,
) -> None:
    events = load_events(client, asset.asset_id)
    provenance = extract_provenance(events)
    v = evaluate_authenticity(
        consumer.quid.id, asset, provenance, node_trust_fn(client),
        policy=policy,
    )
    print(f"\n  [{label}]")
    print(f"    {v.short()}")
    print(f"    trust: capture={v.trust.capture_trust:.3f} "
          f"edit_min={v.trust.edit_trust_min:.3f} "
          f"pub={v.trust.publisher_trust:.3f} "
          f"bonus={v.trust.fact_check_bonus:.3f}")
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

    banner("Step 1: Register actors")
    canon       = register(client, "canon-5d-serial-123", "camera")
    photog      = register(client, "photog-jane-doe",     "photographer")
    editor_r    = register(client, "editor-reuters-mark", "editor")
    reuters     = register(client, "reuters",              "publisher")
    fact_check_  = register(client, "factcheck-maldita",  "fact-checker")
    ai_model    = register(client, "image-model-v2",       "generator")
    aggregator  = register(client, "news-aggregator",      "publisher")
    consumer    = register(client, "newsroom-acme",        "consumer")
    for a in (canon, photog, editor_r, reuters, fact_check_, ai_model,
              aggregator, consumer):
        print(f"  {a.role:14s} {a.name:24s} -> {a.quid.id}")

    banner("Step 2: Consumer sets up trust graph")
    # Consumer trusts canon, photog, editor, reuters, fact-checker.
    for party, level in [
        (canon, 0.95),
        (photog, 0.85),
        (editor_r, 0.9),
        (reuters, 0.95),
        (fact_check_, 0.9),
        # AI sources: weaker.
        (ai_model, 0.7),
        (aggregator, 0.6),
    ]:
        client.grant_trust(
            signer=consumer.quid, trustee=party.quid.id, level=level,
            domain=DOMAIN, description=f"trust in {party.role}",
        )
        print(f"  consumer -[{level}]-> {party.name}")

    time.sleep(1)

    # --- Scenario A: authentic news photo --------------------------------
    banner("Step 3 (A): Authentic photo -> capture / edit / publish / fact-check")
    asset_a_id = f"asset-{uuid.uuid4().hex[:8]}"
    hash_raw   = sha256_hex(f"{asset_a_id}:raw")
    hash_crop  = sha256_hex(f"{asset_a_id}:crop")
    hash_grade = sha256_hex(f"{asset_a_id}:grade")

    capture_asset(client, canon, photog, asset_a_id, hash_raw)
    edit(client, photog,   asset_a_id, "media.cropped",      hash_raw,   hash_crop)
    edit(client, editor_r, asset_a_id, "media.color-graded", hash_crop,  hash_grade)
    publish(client, reuters, asset_a_id, f"reuters-story-{uuid.uuid4().hex[:6]}")
    fact_check(client, fact_check_, asset_a_id,
               "consistent-with-contextual-evidence")
    print(f"  Asset A: captured, cropped, graded, published, fact-checked")

    time.sleep(0.5)
    asset_a = MediaAssetV1(
        asset_id=asset_a_id, asset_type="photo", origin=ORIGIN_CAPTURED,
        creator_quid=canon.quid.id, original_hash=hash_raw,
        created_at_unix=int(time.time()),
    )
    evaluate_and_show(client, consumer, asset_a, "A: authentic news photo")

    # --- Scenario B: tampered chain -------------------------------------
    banner("Step 4 (B): Tampered hash chain")
    asset_b_id = f"asset-{uuid.uuid4().hex[:8]}"
    hash_raw_b = sha256_hex(f"{asset_b_id}:raw")
    hash_crop_b = sha256_hex(f"{asset_b_id}:crop")
    tampered_input = "WRONG-HASH-00000"

    capture_asset(client, canon, photog, asset_b_id, hash_raw_b)
    # Cropped says output=hash_crop_b.
    edit(client, photog, asset_b_id, "media.cropped", hash_raw_b, hash_crop_b)
    # Next edit claims a different input than previous output.
    edit(client, editor_r, asset_b_id, "media.color-graded",
          tampered_input, sha256_hex("irrelevant"))
    publish(client, reuters, asset_b_id, "broken-story")

    time.sleep(0.5)
    asset_b = MediaAssetV1(
        asset_id=asset_b_id, asset_type="photo", origin=ORIGIN_CAPTURED,
        creator_quid=canon.quid.id, original_hash=hash_raw_b,
        created_at_unix=int(time.time()),
    )
    evaluate_and_show(client, consumer, asset_b, "B: tampered (expect reject)")

    # --- Scenario C: AI-generated image ---------------------------------
    banner("Step 5 (C): AI-generated image")
    asset_c_id = f"asset-{uuid.uuid4().hex[:8]}"
    hash_c = sha256_hex(f"{asset_c_id}:generated")
    generate_asset(client, ai_model, asset_c_id, hash_c)
    publish(client, aggregator, asset_c_id, f"aggregator-post-{uuid.uuid4().hex[:6]}")

    time.sleep(0.5)
    asset_c = MediaAssetV1(
        asset_id=asset_c_id, asset_type="photo", origin=ORIGIN_GENERATED,
        creator_quid=ai_model.quid.id, original_hash=hash_c,
        created_at_unix=int(time.time()),
    )

    # Default policy warns on AI.
    evaluate_and_show(client, consumer, asset_c,
                      "C-default: AI -> warn", AuthenticityPolicy())

    # Policy that rejects AI outright.
    evaluate_and_show(
        client, consumer, asset_c,
        "C-strict: AI -> reject",
        AuthenticityPolicy(ai_generated_verdict="reject"),
    )

    # Policy that accepts AI without fuss.
    evaluate_and_show(
        client, consumer, asset_c,
        "C-permissive: AI -> accept",
        AuthenticityPolicy(ai_generated_verdict="accept"),
    )

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - Every provenance step is a signed event on the asset's stream:")
    print("   captured/generated, cropped, color-graded, published, fact-")
    print("   checked. No C2PA cert-chain PKI single-point.")
    print(" - Trust is min of the weakest link plus optional fact-check")
    print("   bonus; a single weak editor taints the chain.")
    print(" - Hash continuity is a hard check. Any broken hash chain is a")
    print("   reject regardless of trust.")
    print(" - AI-generation is a policy knob. Different consumers can")
    print("   accept, warn, or reject without changing the underlying")
    print("   stream.")
    print()


if __name__ == "__main__":
    main()
