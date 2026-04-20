#!/usr/bin/env python3
"""
observer.py — observer reference client.

Subcommands:
  watch — stream events for the election in real time. Prints
          a running summary of check-ins, ballots issued, votes
          cast, and any flagged events.
  attest — publish a signed procedural attestation event
           ("polls opened at 07:00", "all ballot boxes sealed
           at 19:05").
  flag — publish an observer flag for a suspicious event.
  recount — run an independent tally for a given contest from
            primary data (see tally.py for full spec; this is
            a convenience wrapper).
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import time
from collections import Counter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from common.config import load_config
from common.crypto import (
    ecdsa_sign_ieee1363, load_ecdsa_keypair, load_rsa_public_key,
    verify_blind_signature,
)
from common.http_client import HTTPClient
from common.types import EventTx


def cmd_watch(args):
    """Stream events for the election. Polls the per-domain
    quid index every 5 seconds, summarizing what's new."""
    cfg = load_config(args.config)
    client = HTTPClient(cfg.node_url)

    seen_events: set[str] = set()
    counters = Counter()

    print(f"watching {cfg.election_id}…")
    print(f"{'time':10}  {'type':24}  {'domain':40}  {'info'}")
    print("-" * 110)

    while True:
        try:
            # Fetch recent events from the primary election
            # domains.
            domains = [
                f"{cfg.election_id}.registration",
                f"{cfg.election_id}.ballot-issuance",
                f"{cfg.election_id}.audit",
            ]
            for domain in domains:
                quids = client.discover_quids_in_domain(
                    domain=domain, sort="last-seen", limit=50,
                )
                for q in quids:
                    events = client.get_stream(
                        q.get("quidId"), limit=10,
                    )
                    for ev in events:
                        eid = ev.get("id", "")
                        if eid in seen_events:
                            continue
                        seen_events.add(eid)
                        etype = ev.get("eventType", "?")
                        counters[etype] += 1
                        tstr = time.strftime(
                            "%H:%M:%S",
                            time.gmtime(ev.get("timestamp", 0) / 1e9),
                        )
                        # Extract short info from payload
                        payload = ev.get("payload", {})
                        info = ""
                        if etype == "CHECK_IN":
                            info = f"precinct={payload.get('precinctID', '?')}"
                        elif etype == "BALLOT_ISSUED":
                            info = f"vrq={payload.get('vrqPublicId', '?')[:8]}…"
                        elif etype == "VOTER_REGISTERED":
                            info = f"party={payload.get('registeredParty', '?')}"
                        elif etype == "POLLS_OPENED":
                            info = f"precinct={payload.get('precinctID', '?')}"
                        elif etype == "POLLS_CLOSED":
                            info = (
                                f"precinct={payload.get('precinctID', '?')} "
                                f"checkins={payload.get('checkinCount', 0)}"
                            )
                        print(f"{tstr:10}  {etype:24}  {ev.get('trustDomain', '')[:40]:40}  {info}")

            time.sleep(5)
        except KeyboardInterrupt:
            print()
            print("summary:")
            for etype, count in counters.most_common():
                print(f"  {etype}: {count}")
            break


def cmd_attest(args):
    """Publish a procedural attestation event signed by the
    observer's key."""
    cfg = load_config(args.config)
    observer = load_ecdsa_keypair(args.observer_key)
    client = HTTPClient(cfg.node_url)

    domain = f"{cfg.election_id}.audit"
    event = EventTx(
        subject_id=args.subject or observer.quid_id,
        subject_type="QUID",
        sequence=args.sequence,
        event_type="OBSERVER_ATTESTATION",
        domain=domain,
        payload={
            "electionId":     cfg.election_id,
            "observerQuid":   observer.quid_id,
            "claim":          args.claim,
            "context":        args.context or "",
            "observedAt":     int(time.time()),
        },
    )
    event.public_key_hex = observer.public_key_hex
    event.signature_hex = ecdsa_sign_ieee1363(
        observer.private_key, event.canonical_bytes()
    ).hex()
    resp = client.submit_event(event.to_signable_dict() | {
        "signature": event.signature_hex,
    })
    print(f"attestation published — tx {resp.get('transaction_id')}")
    print(f"  claim: {args.claim}")
    print(f"  observer: {observer.quid_id}")


def cmd_flag(args):
    """Flag a suspicious event for manual review."""
    cfg = load_config(args.config)
    observer = load_ecdsa_keypair(args.observer_key)
    client = HTTPClient(cfg.node_url)

    domain = f"{cfg.election_id}.audit"
    event = EventTx(
        subject_id=args.flagged_tx_id,
        subject_type="QUID",
        sequence=1,
        event_type="OBSERVER_FLAG",
        domain=domain,
        payload={
            "electionId":      cfg.election_id,
            "observerQuid":    observer.quid_id,
            "flaggedTxId":     args.flagged_tx_id,
            "reason":          args.reason,
            "severity":        args.severity,
            "flaggedAt":       int(time.time()),
        },
    )
    event.public_key_hex = observer.public_key_hex
    event.signature_hex = ecdsa_sign_ieee1363(
        observer.private_key, event.canonical_bytes()
    ).hex()
    resp = client.submit_event(event.to_signable_dict() | {
        "signature": event.signature_hex,
    })
    print(f"flag published — tx {resp.get('transaction_id')}")
    print(f"  target: {args.flagged_tx_id}")
    print(f"  reason: {args.reason}")
    print(f"  severity: {args.severity}")


def cmd_recount(args):
    """Run an independent tally (wraps tally.py logic)."""
    cfg = load_config(args.config)
    client = HTTPClient(cfg.node_url)

    rsa_pub = load_rsa_public_key(cfg.authority_rsa_pubkey_path)
    contest_domain = f"{cfg.election_id}.contests.{args.contest}"

    # Fetch all vote edges in the contest domain.
    quids = client.discover_quids_in_domain(
        domain=contest_domain, sort="first-seen", limit=100000,
    )

    ok_votes = 0
    invalid_votes = 0
    double_votes = 0
    tally = Counter()
    seen_tokens: set[str] = set()

    print(f"independent recount for {args.contest}…")
    for q in quids:
        events = client.get_stream(
            q.get("quidId"), event_type="TRUST", limit=100,
        )
        for ev in events:
            bp = ev.get("ballotProof", {})
            token_hex = bp.get("ballotToken", "")
            signature_hex = bp.get("blindSignature", "")
            if not token_hex or not signature_hex:
                invalid_votes += 1
                continue

            token = bytes.fromhex(token_hex)
            signature = bytes.fromhex(signature_hex)

            if not verify_blind_signature(signature, token, rsa_pub):
                invalid_votes += 1
                continue

            if token_hex in seen_tokens:
                double_votes += 1
                continue
            seen_tokens.add(token_hex)

            ok_votes += 1
            trustee = ev.get("trustee", "")
            tally[trustee] += 1

    print(f"  valid votes: {ok_votes}")
    print(f"  invalid signatures: {invalid_votes}")
    print(f"  double-votes rejected: {double_votes}")
    print()
    print(f"  totals for {args.contest}:")
    for candidate, count in tally.most_common():
        print(f"    {candidate}: {count}")


def main():
    parser = argparse.ArgumentParser(description="observer reference client")
    parser.add_argument("--config", help="path to elections config YAML")
    subs = parser.add_subparsers(dest="command", required=True)

    p_watch = subs.add_parser("watch")
    p_watch.set_defaults(func=cmd_watch)

    p_attest = subs.add_parser("attest")
    p_attest.add_argument("--observer-key", required=True)
    p_attest.add_argument("--claim", required=True)
    p_attest.add_argument("--context", help="free-form context notes")
    p_attest.add_argument("--subject", help="subject quid (default: observer's own)")
    p_attest.add_argument("--sequence", type=int, default=1)
    p_attest.set_defaults(func=cmd_attest)

    p_flag = subs.add_parser("flag")
    p_flag.add_argument("--observer-key", required=True)
    p_flag.add_argument("--flagged-tx-id", required=True)
    p_flag.add_argument("--reason", required=True)
    p_flag.add_argument("--severity", default="medium",
                         choices=["low", "medium", "high", "critical"])
    p_flag.set_defaults(func=cmd_flag)

    p_rc = subs.add_parser("recount")
    p_rc.add_argument("--contest", required=True)
    p_rc.set_defaults(func=cmd_recount)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
