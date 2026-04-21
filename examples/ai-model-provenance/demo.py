"""AI model provenance and supply-chain, end-to-end demo.

Flow:
  1. Register actors: dataset curator, model producer (foundation),
     model producer (fine-tune), safety evaluator, benchmark org,
     consumer enterprise.
  2. Consumer's trust graph.
  3. Curator registers a dataset; producer registers a foundation
     model and emits training + safety + benchmark events.
  4. Consumer evaluates foundation model -> accept.
  5. Fine-tune producer registers a derivative model.
  6. Consumer evaluates derivative -> accept (inherits from base).
  7. Second scenario: a model with a prohibited training dataset
     license -> reject.

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
from model_provenance import (
    DatasetV1,
    ModelPolicy,
    ModelV1,
    evaluate_model_provenance,
)

from quidnug import OwnershipStake, Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DATASET_DOMAIN = "ai.provenance.datasets"
MODEL_DOMAIN = "ai.provenance.models.foundation"
FINETUNE_DOMAIN = "ai.provenance.models.fine-tunes"


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


def register(client: QuidnugClient, name: str, role: str, domain: str) -> Actor:
    q = Quid.generate()
    try:
        client.register_identity(
            q, name=name, domain=domain, home_domain=domain,
            attributes={"role": role},
        )
    except Exception as e:
        print(f"  (register {name}: {e})")
    return Actor(name=name, role=role, quid=q)


def register_dataset(
    client: QuidnugClient, curator: Actor, dataset_id: str, license: str,
) -> DatasetV1:
    try:
        client.register_title(
            signer=curator.quid,
            asset_id=dataset_id,
            owners=[OwnershipStake(curator.quid.id, 1.0, "curator")],
            domain=DATASET_DOMAIN,
            title_type="training-dataset",
        )
    except Exception as e:
        print(f"  (register_title {dataset_id}: {e})")
    client.emit_event(
        signer=curator.quid,
        subject_id=dataset_id,
        subject_type="TITLE",
        event_type="dataset.curated",
        domain=DATASET_DOMAIN,
        payload={
            "license": license,
            "sizeBytes": 10_000_000_000,
            "contentType": "web-text",
            "hash": f"ds-hash-{uuid.uuid4().hex[:12]}",
        },
    )
    print(f"  {curator.name} registered dataset {dataset_id} (license={license})")
    return DatasetV1(
        dataset_id=dataset_id,
        owner_quid=curator.quid.id,
        license=license,
        content_type="web-text",
        size_bytes=10_000_000_000,
    )


def register_model(
    client: QuidnugClient, producer: Actor, model_id: str,
    training_datasets: List[str], base_model_id: str = "",
    domain: str = MODEL_DOMAIN,
) -> ModelV1:
    try:
        client.register_title(
            signer=producer.quid,
            asset_id=model_id,
            owners=[OwnershipStake(producer.quid.id, 1.0, "producer")],
            domain=domain,
            title_type="ai-model",
        )
    except Exception as e:
        print(f"  (register_title {model_id}: {e})")

    # Training events.
    client.emit_event(
        signer=producer.quid,
        subject_id=model_id,
        subject_type="TITLE",
        event_type="training.started",
        domain=domain,
        payload={
            "trainingDataRefs": training_datasets,
            "baseModelId": base_model_id,
            "startedAt": int(time.time()),
        },
    )
    client.emit_event(
        signer=producer.quid,
        subject_id=model_id,
        subject_type="TITLE",
        event_type="training.completed",
        domain=domain,
        payload={
            "totalFLOPs": 1.2e23,
            "endedAt": int(time.time()),
        },
    )
    print(f"  {producer.name} registered model {model_id}")
    if base_model_id:
        print(f"    derivative of {base_model_id}")
    return ModelV1(
        model_id=model_id,
        producer_quid=producer.quid.id,
        architecture="decoder-transformer",
        parameters=7_000_000_000,
        weights_hash=f"hash-{uuid.uuid4().hex[:12]}",
        license="Apache-2.0",
        base_model_id=base_model_id,
        training_dataset_ids=training_datasets,
        released_at_unix=int(time.time()),
    )


def attest_safety(
    client: QuidnugClient, evaluator: Actor, model_id: str, rating: str,
    domain: str = MODEL_DOMAIN,
) -> None:
    client.emit_event(
        signer=evaluator.quid,
        subject_id=model_id,
        subject_type="TITLE",
        event_type="safety.evaluated",
        domain=domain,
        payload={
            "evaluatorQuid": evaluator.quid.id,
            "overallRating": rating,
            "evaluationHash": f"eval-{uuid.uuid4().hex[:12]}",
            "evaluatedAt": int(time.time()),
        },
    )
    print(f"  {evaluator.name} safety-evaluated {model_id}: {rating}")


def report_benchmark(
    client: QuidnugClient, org: Actor, model_id: str, benchmark_id: str,
    score: float, domain: str = MODEL_DOMAIN,
) -> None:
    client.emit_event(
        signer=org.quid,
        subject_id=model_id,
        subject_type="TITLE",
        event_type="benchmark.reported",
        domain=domain,
        payload={
            "benchmarkOrg": org.quid.id,
            "benchmarkId": benchmark_id,
            "score": score,
            "verified": True,
            "reportedAt": int(time.time()),
        },
    )
    print(f"  {org.name} benchmark {benchmark_id}: {score} on {model_id}")


def load_events(client: QuidnugClient, model_id: str) -> List[dict]:
    events, _ = client.get_stream_events(model_id, limit=200)
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
        # Try the model's home domain, then dataset domain.
        for dom in (MODEL_DOMAIN, FINETUNE_DOMAIN, DATASET_DOMAIN):
            try:
                r = client.get_trust(obs, target, domain=dom, max_depth=5)
                if r and r.trust_level > 0:
                    return r.trust_level
            except Exception:
                pass
        return 0.0
    return fn


def evaluate_and_show(
    client: QuidnugClient, consumer: Actor, model: ModelV1,
    datasets: List[DatasetV1], label: str, policy: ModelPolicy = None,
) -> None:
    events = load_events(client, model.model_id)
    v = evaluate_model_provenance(
        consumer.quid.id, model, datasets, events,
        node_trust_fn(client), policy,
    )
    print(f"\n  [{label}]")
    print(f"    {v.short()}")
    bd = v.breakdown
    print(f"    producer_trust={bd.producer_trust:.3f} "
          f"base_trust={bd.base_model_trust:.3f}  "
          f"safety_ok={bd.safety_evals_acceptable}/{bd.safety_evals_failing} "
          f"benches={bd.benchmarks_trusted}")
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
    curator     = register(client, "common-crawl-fdn",   "curator",   DATASET_DOMAIN)
    curator_bad = register(client, "scraper-anonymous",  "curator",   DATASET_DOMAIN)
    producer    = register(client, "acme-ai",             "producer", MODEL_DOMAIN)
    ft_producer = register(client, "acme-finetune-ai",    "ft-producer", FINETUNE_DOMAIN)
    evaluator   = register(client, "safety-mlcommons",    "safety-evaluator", MODEL_DOMAIN)
    benchmarker = register(client, "helm-benchmark",      "benchmark-org",    MODEL_DOMAIN)
    consumer    = register(client, "enterprise-consumer", "consumer",         MODEL_DOMAIN)
    for a in [curator, curator_bad, producer, ft_producer, evaluator, benchmarker, consumer]:
        print(f"  {a.role:18s} {a.name:22s} -> {a.quid.id}")

    banner("Step 2: Consumer trust graph")
    for party, level in [
        (producer, 0.9), (ft_producer, 0.8),
        (evaluator, 0.9), (benchmarker, 0.85),
        (curator, 0.8),
        (curator_bad, 0.1),   # untrusted
    ]:
        # Use the appropriate domain per actor.
        dom = DATASET_DOMAIN if party in (curator, curator_bad) else MODEL_DOMAIN
        client.grant_trust(
            signer=consumer.quid, trustee=party.quid.id, level=level,
            domain=dom, description=f"trust in {party.role}",
        )
        print(f"  consumer -[{level}]-> {party.name}")

    time.sleep(1)

    banner("Step 3: Curator registers a legitimate dataset")
    ds_clean_id = f"ds-cc-{uuid.uuid4().hex[:6]}"
    ds_clean = register_dataset(client, curator, ds_clean_id, "CC0")

    banner("Step 4: Producer registers the foundation model")
    base_model_id = f"model-acme-7b-{uuid.uuid4().hex[:6]}"
    base_model = register_model(
        client, producer, base_model_id, [ds_clean_id],
    )
    attest_safety(client, evaluator, base_model_id, "acceptable")
    report_benchmark(client, benchmarker, base_model_id, "MMLU", 0.78)

    time.sleep(0.5)

    banner("Step 5: Consumer evaluates the foundation model")
    evaluate_and_show(client, consumer, base_model, [ds_clean],
                      "FOUNDATION (expect accept)")

    banner("Step 6: Fine-tune producer registers a derivative")
    ft_model_id = f"ft-acme-{uuid.uuid4().hex[:6]}"
    # Grant trust in the base model quid so the derivative check passes.
    # (In a fuller deployment the producer's trust would chain here.)
    client.grant_trust(
        signer=consumer.quid, trustee=base_model_id, level=0.85,
        domain=FINETUNE_DOMAIN, description="trust in base model",
    )
    ft_model = register_model(
        client, ft_producer, ft_model_id, [ds_clean_id],
        base_model_id=base_model_id,
        domain=FINETUNE_DOMAIN,
    )
    attest_safety(client, evaluator, ft_model_id, "acceptable",
                   domain=FINETUNE_DOMAIN)

    time.sleep(0.5)
    evaluate_and_show(client, consumer, ft_model, [ds_clean],
                      "DERIVATIVE (expect accept)")

    banner("Step 7: Bad-actor scenario  (prohibited dataset license)")
    ds_pirate_id = f"ds-pirate-{uuid.uuid4().hex[:6]}"
    ds_pirate = register_dataset(client, curator_bad, ds_pirate_id, "shadow-library")
    bad_model_id = f"model-shady-{uuid.uuid4().hex[:6]}"
    bad_model = register_model(
        client, producer, bad_model_id,
        [ds_clean_id, ds_pirate_id],
    )
    attest_safety(client, evaluator, bad_model_id, "acceptable")
    time.sleep(0.5)
    evaluate_and_show(client, consumer, bad_model, [ds_clean, ds_pirate],
                      "BAD-DATASET (expect reject)")

    banner("Step 8: Missing safety scenario")
    naked_model_id = f"model-naked-{uuid.uuid4().hex[:6]}"
    naked_model = register_model(client, producer, naked_model_id, [ds_clean_id])
    # No safety.evaluated event.
    time.sleep(0.5)
    evaluate_and_show(client, consumer, naked_model, [ds_clean],
                      "NO-SAFETY (expect warn)")

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - A model is a TITLE owned by its producer; datasets are")
    print("   their own TITLEs owned by their curators. The model's")
    print("   event stream links to the datasets by ID.")
    print(" - A consumer verifier policy-checks dataset licenses against")
    print("   a locally-configured prohibited list. Producers can't hide")
    print("   a pirate dataset -- the reference is on-chain.")
    print(" - Safety evaluators and benchmark orgs are independent parties")
    print("   emitting their own signed events on the model's stream.")
    print("   Any consumer can choose whose evaluations to trust.")
    print(" - Derivative models inherit a trust dependency on the base.")
    print("   A fine-tune of a weak foundation fails verification even if")
    print("   its own producer is trusted.")
    print()


if __name__ == "__main__":
    main()
