#!/usr/bin/env python3
"""
poll_worker.py — the reference poll-worker client.

Subcommands:
  check-in — look up a voter by VRQ quid, confirm registration,
             and publish a CHECK_IN event on the precinct's
             poll-book domain.
  roll — list registered voters for a precinct.
  close-polls — publish POLLS_CLOSED event for a precinct.

This is what runs on the precinct device handed to voters as
they arrive. In production it would be an iPad/Surface app
with a dedicated secure-enclave-stored key; here it's a CLI.
"""
from __future__ import annotations

import argparse
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from common.config import load_config
from common.crypto import ecdsa_sign_ieee1363, load_ecdsa_keypair
from common.http_client import HTTPClient
from common.types import EventTx


def cmd_check_in(args):
    """Check a voter in at this precinct."""
    cfg = load_config(args.config)
    precinct_key = load_ecdsa_keypair(args.precinct_key)
    client = HTTPClient(cfg.node_url)

    reg_domain = f"{cfg.election_id}.registration"
    pollbook_domain = f"{cfg.election_id}.poll-book.{args.precinct}"

    # 1. Verify voter is registered.
    identity = client.get_identity(args.vrq)
    if not identity:
        print(f"ERROR: no identity for VRQ {args.vrq}", file=sys.stderr)
        sys.exit(2)

    # 2. Check the voter's registration attestation from the
    #    authority. In production, the authority signs a
    #    VOTER_REGISTERED event for each voter; we look for
    #    that on their stream.
    reg_events = client.get_stream(
        args.vrq, event_type="VOTER_REGISTERED", limit=1,
    )
    if not reg_events:
        print(f"ERROR: VRQ {args.vrq} not registered by authority",
              file=sys.stderr)
        sys.exit(2)
    reg = reg_events[0]
    reg_precinct = reg.get("payload", {}).get("precinctID")
    if reg_precinct != args.precinct:
        print(f"ERROR: VRQ {args.vrq} registered in precinct {reg_precinct},",
              f"not {args.precinct}", file=sys.stderr)
        sys.exit(3)

    # 3. Confirm voter hasn't already been checked in.
    existing = client.get_stream(
        args.vrq, event_type="CHECK_IN", limit=1,
    )
    if existing:
        print(f"ERROR: VRQ {args.vrq} already checked in earlier at",
              f"{time.strftime('%H:%M:%S', time.gmtime(existing[0].get('timestamp', 0) / 1e9))}",
              file=sys.stderr)
        sys.exit(4)

    # 4. Publish CHECK_IN event on the precinct's poll-book domain.
    event = EventTx(
        subject_id=args.vrq,
        subject_type="QUID",
        sequence=1,
        event_type="CHECK_IN",
        domain=pollbook_domain,
        payload={
            "electionId":    cfg.election_id,
            "vrqId":         args.vrq,
            "precinctID":    args.precinct,
            "pollWorkerQuid": precinct_key.quid_id,
            "checkinTime":   int(time.time()),
        },
    )
    event.public_key_hex = precinct_key.public_key_hex
    event.signature_hex = ecdsa_sign_ieee1363(
        precinct_key.private_key, event.canonical_bytes()
    ).hex()
    resp = client.submit_event(event.to_signable_dict() | {
        "signature": event.signature_hex,
    })
    print(f"checked in {args.vrq} — tx {resp.get('transaction_id')}")
    print(f"  poll-book domain: {pollbook_domain}")
    print(f"  poll worker: {precinct_key.quid_id[:16]}…")
    print()
    print(f"voter may now request a ballot at the voting booth.")


def cmd_roll(args):
    """List all registered voters at this precinct."""
    cfg = load_config(args.config)
    client = HTTPClient(cfg.node_url)

    reg_domain = f"{cfg.election_id}.registration"
    quids = client.discover_quids_in_domain(
        domain=reg_domain, sort="first-seen", limit=1000,
    )

    # Filter to this precinct. In production the poll-book
    # domain would be its own child (poll-book.<precinct>);
    # for the demo we filter client-side.
    matching = []
    for q in quids:
        events = client.get_stream(
            q.get("quidId"), event_type="VOTER_REGISTERED", limit=1,
        )
        if events and events[0].get("payload", {}).get("precinctID") == args.precinct:
            matching.append(q)

    print(f"registered voters in precinct {args.precinct}: {len(matching)}")
    for q in matching[:args.limit]:
        ev = client.get_stream(q["quidId"], event_type="VOTER_REGISTERED", limit=1)[0]
        party = ev.get("payload", {}).get("registeredParty", "—")
        print(f"  {q['quidId']}  party={party}  registered={ev.get('timestamp')}")


def cmd_close_polls(args):
    """End-of-day: publish POLLS_CLOSED event for this precinct."""
    cfg = load_config(args.config)
    precinct_key = load_ecdsa_keypair(args.precinct_key)
    client = HTTPClient(cfg.node_url)

    pollbook_domain = f"{cfg.election_id}.poll-book.{args.precinct}"

    # Count check-ins at this precinct.
    # In production use the per-domain quid index; for demo,
    # we count the CHECK_IN events on this precinct's poll-book
    # domain.
    checkins = client.get_stream(
        args.precinct, event_type="CHECK_IN", limit=10000,
    )
    checkin_count = len(checkins)

    event = EventTx(
        subject_id=args.precinct,
        subject_type="QUID",
        sequence=checkin_count + 1,
        event_type="POLLS_CLOSED",
        domain=pollbook_domain,
        payload={
            "electionId":    cfg.election_id,
            "precinctID":    args.precinct,
            "closingTime":   int(time.time()),
            "checkinCount":  checkin_count,
            "paperBallotCount": args.paper_count,
            "pollWorkerQuid": precinct_key.quid_id,
        },
    )
    event.public_key_hex = precinct_key.public_key_hex
    event.signature_hex = ecdsa_sign_ieee1363(
        precinct_key.private_key, event.canonical_bytes()
    ).hex()
    resp = client.submit_event(event.to_signable_dict() | {
        "signature": event.signature_hex,
    })
    print(f"POLLS_CLOSED for precinct {args.precinct} — tx {resp.get('transaction_id')}")
    print(f"  checkins: {checkin_count}")
    print(f"  paper ballots deposited: {args.paper_count}")
    if checkin_count != args.paper_count:
        print(f"  ⚠ MISMATCH: {abs(checkin_count - args.paper_count)} ballot(s) unaccounted for")


def main():
    parser = argparse.ArgumentParser(description="poll-worker reference client")
    parser.add_argument("--config", help="path to elections config YAML")
    subs = parser.add_subparsers(dest="command", required=True)

    p_ci = subs.add_parser("check-in")
    p_ci.add_argument("--vrq", required=True, help="voter's VRQ quid ID")
    p_ci.add_argument("--precinct", required=True)
    p_ci.add_argument("--precinct-key", required=True,
                      help="precinct device's private key path")
    p_ci.set_defaults(func=cmd_check_in)

    p_roll = subs.add_parser("roll")
    p_roll.add_argument("--precinct", required=True)
    p_roll.add_argument("--limit", type=int, default=100)
    p_roll.set_defaults(func=cmd_roll)

    p_cp = subs.add_parser("close-polls")
    p_cp.add_argument("--precinct", required=True)
    p_cp.add_argument("--precinct-key", required=True)
    p_cp.add_argument("--paper-count", type=int, required=True,
                       help="number of paper ballots in the ballot box")
    p_cp.set_defaults(func=cmd_close_polls)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
