#!/usr/bin/env python3
"""
tally.py — tally engine reference client.

Walks the contest domain, verifies every vote's ballot proof,
rejects invalid signatures + double-votes, and emits a signed
CONTEST_TALLY_PRELIMINARY event on the tally domain.

Can run from the authority (for the official preliminary tally)
or by any observer (any laptop can produce an identical count
from the same primary data — that's the whole point of universal
verifiability).

Usage:
  # Official tally by the authority:
  python tally.py run --contest governor \
      --authority-key authority.pem \
      --output-json governor-tally.json

  # Independent recount by anyone (no signing):
  python tally.py run --contest governor --unofficial

  # Generate a signed proof of the tally for later audit:
  python tally.py proof --contest governor \
      --tally-json governor-tally.json \
      --authority-key authority.pem
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
from collections import Counter
from dataclasses import asdict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from common.config import load_config
from common.crypto import (
    ecdsa_sign_ieee1363, load_ecdsa_keypair, load_rsa_public_key,
    verify_blind_signature,
)
from common.http_client import HTTPClient
from common.types import EventTx


def run_tally(
    client: HTTPClient,
    election_id: str,
    contest: str,
    authority_rsa_pub,
) -> dict:
    """Run a full tally for one contest. Returns a dict
    describing the results + every decision made.

    This is the core verifiable-tally algorithm. Every observer
    who runs this with the same inputs will get the same output.
    """
    contest_domain = f"{election_id}.contests.{contest}"

    # Fetch all quids active in the contest domain.
    quids = client.discover_quids_in_domain(
        domain=contest_domain, sort="first-seen", limit=1_000_000,
    )

    tally = Counter()
    accepted_tokens: set[str] = set()
    rejected: list[dict] = []
    accepted_count = 0
    double_vote_count = 0
    invalid_signature_count = 0
    malformed_count = 0

    for q in quids:
        events = client.get_stream(
            q.get("quidId"), event_type="TRUST", limit=1000,
        )
        for ev in events:
            # Only consider TRUST transactions in the contest
            # domain with ballot proofs (defensive; discovery
            # should already filter).
            if ev.get("trustDomain") != contest_domain:
                continue

            bp = ev.get("ballotProof", {})
            token_hex = bp.get("ballotToken", "")
            signature_hex = bp.get("blindSignature", "")
            candidate = ev.get("trustee", "")
            tx_id = ev.get("id", "")

            # Reject malformed: missing ballot proof fields.
            if not token_hex or not signature_hex or not candidate:
                malformed_count += 1
                rejected.append({
                    "txId": tx_id, "reason": "missing ballot proof fields"
                })
                continue

            # Verify blind signature.
            try:
                token_bytes = bytes.fromhex(token_hex)
                sig_bytes = bytes.fromhex(signature_hex)
            except ValueError:
                malformed_count += 1
                rejected.append({"txId": tx_id, "reason": "hex decode failed"})
                continue

            if not verify_blind_signature(
                sig_bytes, token_bytes, authority_rsa_pub
            ):
                invalid_signature_count += 1
                rejected.append({
                    "txId": tx_id, "reason": "ballot signature invalid"
                })
                continue

            # Reject double vote (same ballot token used twice).
            if token_hex in accepted_tokens:
                double_vote_count += 1
                rejected.append({
                    "txId": tx_id, "reason": "duplicate ballot token"
                })
                continue
            accepted_tokens.add(token_hex)

            # Valid vote. Count it.
            tally[candidate] += 1
            accepted_count += 1

    return {
        "electionId":         election_id,
        "contest":            contest,
        "contestDomain":      contest_domain,
        "generatedAt":        int(time.time()),
        "totalVotes":         accepted_count,
        "byCandidate":        dict(tally.most_common()),
        "rejected":           {
            "total":              len(rejected),
            "invalidSignature":   invalid_signature_count,
            "doubleVote":         double_vote_count,
            "malformed":          malformed_count,
            "details":            rejected[:50],  # first 50 for sanity
        },
        "rejectedTotal":      len(rejected),
    }


def tally_merkle_root(tally: dict) -> str:
    """Build a deterministic merkle-ish root over the tally for
    integrity checking. Not the full Quidnug block-Merkle tree,
    just a hash commitment over the sorted tally dict."""
    canonical = json.dumps(
        {k: tally[k] for k in sorted(tally)}, separators=(",", ":")
    ).encode("utf-8")
    return hashlib.sha256(canonical).hexdigest()


def cmd_run(args):
    cfg = load_config(args.config)
    client = HTTPClient(cfg.node_url)
    rsa_pub = load_rsa_public_key(cfg.authority_rsa_pubkey_path)

    print(f"running tally for {args.contest} in {cfg.election_id}…")
    result = run_tally(client, cfg.election_id, args.contest, rsa_pub)

    # Summarize.
    print()
    print(f"=== {args.contest} tally ===")
    print(f"total valid votes: {result['totalVotes']}")
    for candidate, count in result["byCandidate"].items():
        print(f"  {candidate}: {count}")
    print()
    print("rejected:")
    print(f"  total: {result['rejectedTotal']}")
    print(f"  invalid signatures: {result['rejected']['invalidSignature']}")
    print(f"  double-votes: {result['rejected']['doubleVote']}")
    print(f"  malformed: {result['rejected']['malformed']}")
    print()

    root = tally_merkle_root(result["byCandidate"])
    result["tallyRoot"] = root
    print(f"tally integrity root: {root[:32]}…")

    # Persist to JSON.
    if args.output_json:
        with open(args.output_json, "w") as f:
            json.dump(result, f, indent=2)
        print(f"wrote {args.output_json}")

    if args.unofficial:
        print()
        print("UNOFFICIAL: this is an independent recount; run `tally.py proof`")
        print("with the authority key to produce a signed official tally.")
        return

    # Official: publish CONTEST_TALLY_PRELIMINARY event.
    if not args.authority_key:
        print("missing --authority-key; pass it or use --unofficial")
        sys.exit(2)

    authority = load_ecdsa_keypair(args.authority_key)
    tally_domain = f"{cfg.election_id}.tally"
    event = EventTx(
        subject_id=authority.quid_id,
        subject_type="QUID",
        sequence=int(time.time()),
        event_type="CONTEST_TALLY_PRELIMINARY",
        domain=tally_domain,
        payload={
            "electionId":      cfg.election_id,
            "contest":         args.contest,
            "totalVotes":      result["totalVotes"],
            "byCandidate":     result["byCandidate"],
            "rejectedTotal":   result["rejectedTotal"],
            "tallyRoot":       root,
            "generatedAt":     result["generatedAt"],
        },
    )
    event.public_key_hex = authority.public_key_hex
    event.signature_hex = ecdsa_sign_ieee1363(
        authority.private_key, event.canonical_bytes()
    ).hex()
    resp = client.submit_event(event.to_signable_dict() | {
        "signature": event.signature_hex,
    })
    print(f"CONTEST_TALLY_PRELIMINARY published — tx {resp.get('transaction_id')}")


def cmd_proof(args):
    """Wrap a previously-computed tally in a signed attestation
    event. Useful when the tally was run unofficially and the
    authority wants to sign the result after review."""
    cfg = load_config(args.config)
    client = HTTPClient(cfg.node_url)
    authority = load_ecdsa_keypair(args.authority_key)

    with open(args.tally_json) as f:
        result = json.load(f)

    tally_domain = f"{cfg.election_id}.tally"
    event = EventTx(
        subject_id=authority.quid_id,
        subject_type="QUID",
        sequence=int(time.time()),
        event_type="CONTEST_TALLY_OFFICIAL",
        domain=tally_domain,
        payload={
            "electionId":      cfg.election_id,
            "contest":         result["contest"],
            "totalVotes":      result["totalVotes"],
            "byCandidate":     result["byCandidate"],
            "tallyRoot":       result["tallyRoot"],
            "attestedAt":      int(time.time()),
            "methodology":     "ref-impl tally.py, verify_blind_signature per QDP-0021",
        },
    )
    event.public_key_hex = authority.public_key_hex
    event.signature_hex = ecdsa_sign_ieee1363(
        authority.private_key, event.canonical_bytes()
    ).hex()
    resp = client.submit_event(event.to_signable_dict() | {
        "signature": event.signature_hex,
    })
    print(f"CONTEST_TALLY_OFFICIAL published — tx {resp.get('transaction_id')}")


def main():
    parser = argparse.ArgumentParser(description="tally engine reference client")
    parser.add_argument("--config", help="path to elections config YAML")
    subs = parser.add_subparsers(dest="command", required=True)

    p_run = subs.add_parser("run")
    p_run.add_argument("--contest", required=True)
    p_run.add_argument("--authority-key", help="authority's ECDSA key for signing")
    p_run.add_argument("--output-json", help="write full result JSON here")
    p_run.add_argument("--unofficial", action="store_true",
                       help="compute but don't publish; observer-style")
    p_run.set_defaults(func=cmd_run)

    p_proof = subs.add_parser("proof")
    p_proof.add_argument("--tally-json", required=True,
                          help="tally result from a prior `run`")
    p_proof.add_argument("--authority-key", required=True)
    p_proof.set_defaults(func=cmd_proof)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
