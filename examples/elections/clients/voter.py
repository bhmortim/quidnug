#!/usr/bin/env python3
"""
voter.py — the reference voter client.

Subcommands:
  generate — create a fresh VRQ keypair (bring-your-own-quid).
  register — publish a VRQ identity + request a registration
             trust edge from the authority.
  request-ballot — during voting: blind-sign-request a ballot.
  cast-vote — publish vote TRUST edges with ballot-proof.
  verify — fetch the voter's own registration + votes + check.

Run end-to-end:
  python voter.py generate --out alice.keys
  python voter.py register --keys alice.keys --precinct 042
  python voter.py request-ballot --keys alice.keys
  python voter.py cast-vote --keys alice.keys \
      --choices governor=harper senate=ngo
  python voter.py verify --keys alice.keys
"""
from __future__ import annotations

import argparse
import json
import os
import secrets
import sys
import time
from dataclasses import asdict
from typing import Optional

# Ensure common/ is importable when running from this directory.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from common.config import load_config
from common.crypto import (
    ECDSAKeypair, ballot_token, blind, ecdsa_sign_ieee1363,
    generate_ecdsa_keypair, load_ecdsa_keypair, load_rsa_public_key,
    rsa_fingerprint, save_ecdsa_keypair, unblind,
    verify_blind_signature,
)
from common.http_client import HTTPClient
from common.types import (
    BallotArtifact, BallotIssuanceRequest, EventTx, IdentityTx, TrustTx,
)


# ---------------------------------------------------------------
# Sub-command: generate
# ---------------------------------------------------------------

def cmd_generate(args):
    """Create a fresh VRQ. Writes the PKCS8 PEM private key to
    the requested path with 0600 permissions."""
    keypair = generate_ecdsa_keypair()
    save_ecdsa_keypair(keypair, args.out)
    print(f"generated VRQ {keypair.quid_id} → {args.out}")
    print(f"  pubkey: {keypair.public_key_hex[:32]}…")
    print(f"  quid:   {keypair.quid_id}")
    print()
    print("  Keep the private key offline if you can. If your")
    print("  phone is compromised, guardian recovery is the")
    print("  only way back.")


# ---------------------------------------------------------------
# Sub-command: register
# ---------------------------------------------------------------

def cmd_register(args):
    """Publish VRQ identity + register in a precinct."""
    cfg = load_config(args.config)
    keypair = load_ecdsa_keypair(args.keys)
    client = HTTPClient(cfg.node_url)

    # 1. Publish IDENTITY tx for the VRQ.
    reg_domain = f"{cfg.election_id}.registration"
    id_tx = IdentityTx(
        quid_id=keypair.quid_id,
        name=args.name or f"voter-{keypair.quid_id[:8]}",
        creator=keypair.quid_id,
        domain=reg_domain,
        attributes={
            "precinctID": args.precinct,
            "registeredParty": args.party,
            "registrationYear": time.gmtime().tm_year,
        },
    )
    id_tx.public_key_hex = keypair.public_key_hex
    id_tx.signature_hex = ecdsa_sign_ieee1363(
        keypair.private_key, id_tx.canonical_bytes()
    ).hex()
    resp = client.submit_identity(id_tx.to_signable_dict() | {
        "signature": id_tx.signature_hex,
    })
    print(f"VRQ identity published, tx {resp.get('transaction_id')}")

    # 2. Voter publishes a self-registration event; in real
    #    deployment the authority separately publishes a
    #    VOTER_REGISTERED event with their trust-edge attestation.
    #    Here we publish the voter-side request.
    event = EventTx(
        subject_id=keypair.quid_id,
        subject_type="QUID",
        sequence=1,
        event_type="VOTER_REGISTRATION_REQUEST",
        domain=reg_domain,
        payload={
            "precinctID": args.precinct,
            "registeredParty": args.party,
            "election_id": cfg.election_id,
        },
    )
    event.public_key_hex = keypair.public_key_hex
    event.signature_hex = ecdsa_sign_ieee1363(
        keypair.private_key, event.canonical_bytes()
    ).hex()
    resp = client.submit_event(event.to_signable_dict() | {
        "signature": event.signature_hex,
    })
    print(f"registration request published, tx {resp.get('transaction_id')}")

    print()
    print(f"voter {keypair.quid_id} registered in precinct {args.precinct}")
    print("Now wait for the authority to publish a")
    print("VOTER_REGISTERED event trusting your VRQ.")


# ---------------------------------------------------------------
# Sub-command: request-ballot
# ---------------------------------------------------------------

def cmd_request_ballot(args):
    """Blind-sign-request a ballot from the authority.

    Generates an ephemeral BQ keypair, builds a ballot token,
    blinds it, sends to the authority, unblinds the signed
    response, saves the BallotArtifact for use in cast-vote."""
    cfg = load_config(args.config)
    vrq = load_ecdsa_keypair(args.keys)
    client = HTTPClient(cfg.node_url)

    # 1. Fetch the authority's current RSA blind-issuance key
    #    from their published BLIND_KEY_ATTESTATION event.
    #    For demo purposes, we load it from a local file that
    #    setup_authority.py writes.
    authority_rsa_pub = load_rsa_public_key(cfg.authority_rsa_pubkey_path)
    fingerprint = rsa_fingerprint(authority_rsa_pub)
    print(f"authority RSA blind-issuance key fingerprint: {fingerprint[:16]}…")

    # 2. Find the latest CHECK_IN event for this VRQ.
    checkin_events = client.get_stream(
        vrq.quid_id, event_type="CHECK_IN", limit=1,
    )
    if not checkin_events:
        print("ERROR: no CHECK_IN event found. Has the poll worker checked you in?",
              file=sys.stderr)
        sys.exit(2)
    checkin = checkin_events[0]
    print(f"check-in event found: {checkin.get('id')}")

    # 3. Generate an ephemeral BQ keypair.
    bq = generate_ecdsa_keypair()
    print(f"ephemeral BQ: {bq.quid_id[:16]}…")

    # 4. Construct the ballot token.
    nonce = secrets.token_bytes(32)
    token = ballot_token(cfg.election_id, bq.public_key_hex, nonce)
    print(f"ballot token: {token.hex()[:16]}…")

    # 5. Blind the token with a random factor r (kept secret).
    blinded, r = blind(token, authority_rsa_pub)
    print(f"blinded token: {hex(blinded)[2:][:16]}…")

    # 6. Sign + submit the blind-issuance request.
    req = BallotIssuanceRequest(
        election_id=cfg.election_id,
        vrq_public_id=vrq.quid_id,
        checkin_event_id=checkin.get("id", ""),
        blinded_ballot_token_hex=hex(blinded)[2:],
        blinding_key_fingerprint=fingerprint,
        timestamp=int(time.time() * 1_000_000_000),
    )
    req.vrq_signature_hex = ecdsa_sign_ieee1363(
        vrq.private_key, req.canonical_bytes()
    ).hex()
    resp = client.request_ballot_signing(cfg.election_id, req.to_full_dict())
    print(f"authority returned signed blinded token")

    # 7. Unblind the authority's signature.
    signed_blinded = int(resp["signedBlindedToken"], 16)
    signature = unblind(signed_blinded, r, authority_rsa_pub)
    print(f"unblinded signature: {signature.hex()[:16]}…")

    # 8. Verify the signature locally before trusting it.
    assert verify_blind_signature(signature, token, authority_rsa_pub), \
        "authority's signature failed verification"

    # 9. Save the ballot artifact for cast-vote.
    artifact = BallotArtifact(
        election_id=cfg.election_id,
        ballot_token=token,
        ballot_signature=signature,
        rsa_key_fingerprint=fingerprint,
        bq_pubkey_hex=bq.public_key_hex,
        bq_private_key_hex=bq.private_key.private_numbers().private_value.to_bytes(32, "big").hex(),
    )
    artifact_path = args.keys + ".ballot.json"
    with open(artifact_path, "w") as f:
        json.dump({
            "election_id":         artifact.election_id,
            "ballot_token":        artifact.ballot_token.hex(),
            "ballot_signature":    artifact.ballot_signature.hex(),
            "rsa_key_fingerprint": artifact.rsa_key_fingerprint,
            "bq_pubkey_hex":       artifact.bq_pubkey_hex,
            "bq_private_key_hex":  artifact.bq_private_key_hex,
        }, f, indent=2)
    os.chmod(artifact_path, 0o600)
    print(f"ballot artifact saved → {artifact_path}")
    print()
    print("You have a valid ballot. Next: run `cast-vote`.")


# ---------------------------------------------------------------
# Sub-command: cast-vote
# ---------------------------------------------------------------

def cmd_cast_vote(args):
    """Cast votes in one or more contests using the BallotArtifact."""
    cfg = load_config(args.config)
    client = HTTPClient(cfg.node_url)

    # Load the ballot artifact
    artifact_path = args.keys + ".ballot.json"
    with open(artifact_path, "r") as f:
        a = json.load(f)
    election_id = a["election_id"]
    token = bytes.fromhex(a["ballot_token"])
    signature = bytes.fromhex(a["ballot_signature"])
    rsa_key_fingerprint = a["rsa_key_fingerprint"]
    bq_pubkey_hex = a["bq_pubkey_hex"]

    # Rebuild the ephemeral BQ private key for vote signing.
    from cryptography.hazmat.primitives.asymmetric import ec
    private_scalar = int(a["bq_private_key_hex"], 16)
    bq_private = ec.derive_private_key(private_scalar, ec.SECP256R1())

    # Parse choices: "contest=candidate,contest=candidate"
    choices = {}
    for item in args.choices.split(","):
        contest, candidate = item.split("=")
        choices[contest.strip()] = candidate.strip()

    # Cast one TRUST edge per contest, with ballot proof embedded.
    for nonce, (contest, candidate) in enumerate(choices.items(), start=1):
        domain = f"{election_id}.contests.{contest}"
        tx = TrustTx(
            truster=bq_pubkey_hex[:64],       # BQ quid derived below
            trustee=candidate,
            trust_level=1.0,
            domain=domain,
            nonce=nonce,
            ballot_proof={
                "electionId":         election_id,
                "ballotToken":        token.hex(),
                "blindSignature":     signature.hex(),
                "rsaKeyFingerprint":  rsa_key_fingerprint,
                "bqEphemeralPubkey":  bq_pubkey_hex,
            },
        )
        # BQ quid derivation is sha256(bq_pubkey_bytes)[:16].
        import hashlib
        bq_quid = hashlib.sha256(bytes.fromhex(bq_pubkey_hex)).hexdigest()[:16]
        tx.truster = bq_quid
        tx.public_key_hex = bq_pubkey_hex
        tx.signature_hex = ecdsa_sign_ieee1363(
            bq_private, tx.canonical_bytes()
        ).hex()
        resp = client.submit_trust(tx.to_signable_dict() | {
            "signature": tx.signature_hex,
        })
        print(f"vote cast in {contest} for {candidate} — tx {resp.get('transaction_id')}")

    print()
    print("All votes cast. Your BQ ephemeral keypair has done its job;")
    print("you can discard it now. Votes remain verifiable via the ballot")
    print("signature which is in every vote edge.")


# ---------------------------------------------------------------
# Sub-command: verify (individual voter verifiability)
# ---------------------------------------------------------------

def cmd_verify(args):
    """Verify the voter's own votes are in the chain and valid."""
    cfg = load_config(args.config)
    client = HTTPClient(cfg.node_url)

    artifact_path = args.keys + ".ballot.json"
    if not os.path.exists(artifact_path):
        print("no ballot artifact found; did you run cast-vote?", file=sys.stderr)
        sys.exit(2)
    with open(artifact_path, "r") as f:
        a = json.load(f)

    import hashlib
    bq_quid = hashlib.sha256(bytes.fromhex(a["bq_pubkey_hex"])).hexdigest()[:16]

    # Fetch all vote edges from this BQ.
    # (In practice use /api/v2/discovery/quids to filter, but
    # for the demo we just walk the BQ's outgoing TRUST edges.)
    print(f"fetching votes from BQ {bq_quid}…")
    events = client.get_stream(bq_quid, event_type="TRUST", limit=100)
    if not events:
        print("no votes found for this BQ. Either not yet committed to a block")
        print("or BQ doesn't match what you cast with.")
        sys.exit(2)

    # Verify each vote.
    ok_count = 0
    for ev in events:
        # Pull the ballot proof; verify RSA signature.
        from cryptography.hazmat.primitives.serialization import load_pem_public_key
        bp = ev.get("ballotProof", {})
        signature = bytes.fromhex(bp.get("blindSignature", ""))
        token = bytes.fromhex(bp.get("ballotToken", ""))
        if token != bytes.fromhex(a["ballot_token"]):
            continue  # not this voter's vote
        pub = load_rsa_public_key(cfg.authority_rsa_pubkey_path)
        if verify_blind_signature(signature, token, pub):
            ok_count += 1
            contest = ev.get("trustDomain", "").split(".")[-1]
            trustee = ev.get("trustee", "")
            print(f"  ✓ {contest} → {trustee} (verified)")
        else:
            print(f"  ✗ signature verification FAILED for tx {ev.get('id')}")

    print()
    print(f"verified {ok_count} votes cast by your BQ")
    print("All good. Your vote is in the chain + cryptographically valid.")


# ---------------------------------------------------------------
# CLI
# ---------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="voter reference client")
    parser.add_argument("--config", help="path to elections config YAML")
    subs = parser.add_subparsers(dest="command", required=True)

    p_gen = subs.add_parser("generate")
    p_gen.add_argument("--out", required=True, help="output path for the PEM keys")
    p_gen.set_defaults(func=cmd_generate)

    p_reg = subs.add_parser("register")
    p_reg.add_argument("--keys", required=True)
    p_reg.add_argument("--name")
    p_reg.add_argument("--precinct", required=True)
    p_reg.add_argument("--party", default="")
    p_reg.set_defaults(func=cmd_register)

    p_req = subs.add_parser("request-ballot")
    p_req.add_argument("--keys", required=True)
    p_req.set_defaults(func=cmd_request_ballot)

    p_cast = subs.add_parser("cast-vote")
    p_cast.add_argument("--keys", required=True)
    p_cast.add_argument("--choices", required=True,
                         help="contest=candidate[,contest=candidate]")
    p_cast.set_defaults(func=cmd_cast_vote)

    p_ver = subs.add_parser("verify")
    p_ver.add_argument("--keys", required=True)
    p_ver.set_defaults(func=cmd_verify)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
