"""Decentralized credit reputation, end-to-end demo.

Flow:
  1. Register actors: subject, two lenders (mainstream +
     alt-lender), a utility attester, a fraudulent lender who
     filed a false default, and a dispute arbiter.
  2. Mainstream lender records an auto-loan origination + 12
     on-time payments + loan closed.
  3. Utility records 24 months of on-time utility payments.
  4. A fraudulent lender records a false default.
  5. Subject files a dispute against the fraudulent default.
  6. Lender-B (a prospective mortgage lender) evaluates the
     subject with its own trust graph -> approve.
  7. A credit-denier lender with different trust graph reaches
     a different verdict on the same stream.

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
from credit_evaluate import (
    EVENT_DISPUTE,
    EVENT_LOAN_ORIGINATED,
    EVENT_LOAN_CLOSED,
    EVENT_PAYMENT_ON_TIME,
    EVENT_DEFAULT,
    EVENT_UTILITY_ON_TIME,
    LenderPolicy,
    evaluate_borrower,
)

from quidnug import Quid, QuidnugClient

NODE_URL = os.environ.get("QUIDNUG_NODE", "http://localhost:8080")
DOMAIN = "credit.reports"


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


def emit_credit_event(
    client: QuidnugClient, attester: Actor, subject: Actor,
    event_type: str, category: str, loan_id: str = "",
) -> None:
    client.emit_event(
        signer=attester.quid,
        subject_id=subject.quid.id,
        subject_type="QUID",
        event_type=event_type,
        domain=DOMAIN,
        payload={
            "attester": attester.quid.id,
            "subject": subject.quid.id,
            "category": category,
            "loanId": loan_id,
            "amountBand": "10k-30k",
            "timestamp": int(time.time()),
        },
    )


def file_dispute(
    client: QuidnugClient, subject: Actor,
    disputed_event_type: str, disputed_attester: Actor,
    loan_id: str, reason: str,
) -> None:
    client.emit_event(
        signer=subject.quid,
        subject_id=subject.quid.id,
        subject_type="QUID",
        event_type=EVENT_DISPUTE,
        domain=DOMAIN,
        payload={
            "disputesEventType": disputed_event_type,
            "disputesAttester": disputed_attester.quid.id,
            "disputesLoanId": loan_id,
            "reason": reason,
            "filedAt": int(time.time()),
        },
    )


def load_events(client: QuidnugClient, subject: Actor) -> List[dict]:
    events, _ = client.get_stream_events(subject.quid.id, limit=500)
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
    client: QuidnugClient, lender: Actor, subject: Actor,
    label: str, policy: LenderPolicy = None,
) -> None:
    events = load_events(client, subject)
    v = evaluate_borrower(
        lender.quid.id, subject.quid.id, events,
        node_trust_fn(client), policy,
    )
    print(f"\n  [{label}] lender={lender.name}")
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

    # -----------------------------------------------------------------
    banner("Step 1: Register actors")
    subject   = register(client, "alice-chen",             "subject")
    lender_a  = register(client, "mainstream-auto-lender", "lender")
    lender_b  = register(client, "prospective-mortgage-lender", "lender")
    alt_bank  = register(client, "credit-union-community",  "lender")
    utility   = register(client, "con-edison-utility",     "alt-data")
    fraud     = register(client, "fraudulent-lender-ghost", "attester")
    for a in (subject, lender_a, lender_b, alt_bank, utility, fraud):
        print(f"  {a.role:12s} {a.name:28s} -> {a.quid.id}")

    # -----------------------------------------------------------------
    banner("Step 2: Lender trust graphs")
    # lender-b's trust graph: trusts mainstream lender, alt bank,
    # utility, but not the fraudster.
    for party, level in [(lender_a, 0.9), (alt_bank, 0.7), (utility, 0.75),
                          (fraud, 0.15)]:
        client.grant_trust(
            signer=lender_b.quid, trustee=party.quid.id, level=level,
            domain=DOMAIN, description=f"{lender_b.name} trust in {party.name}",
        )
    # alt-bank trusts the utility more heavily than lender-b does.
    for party, level in [(utility, 0.9), (lender_a, 0.8)]:
        client.grant_trust(
            signer=alt_bank.quid, trustee=party.quid.id, level=level,
            domain=DOMAIN,
        )
    print(f"  lender-b  -> mainstream=0.9, alt-bank=0.7, utility=0.75, fraud=0.15")
    print(f"  alt-bank  -> utility=0.9, mainstream=0.8")

    time.sleep(1)

    # -----------------------------------------------------------------
    banner("Step 3: Mainstream lender records clean auto loan")
    loan_id = f"loan-{uuid.uuid4().hex[:6]}"
    emit_credit_event(client, lender_a, subject,
                       EVENT_LOAN_ORIGINATED, "auto-loan", loan_id)
    for _ in range(12):
        emit_credit_event(client, lender_a, subject,
                           EVENT_PAYMENT_ON_TIME, "auto-loan", loan_id)
    emit_credit_event(client, lender_a, subject,
                       EVENT_LOAN_CLOSED, "auto-loan", loan_id)
    print(f"  mainstream-auto-lender recorded 1 origination + 12 on-time + close")

    # -----------------------------------------------------------------
    banner("Step 4: Utility records 24 months of on-time payments")
    for _ in range(24):
        emit_credit_event(client, utility, subject,
                           EVENT_UTILITY_ON_TIME, "utility",
                           loan_id=f"util-{uuid.uuid4().hex[:4]}")
    print(f"  utility recorded 24 on-time alt-data events")

    # -----------------------------------------------------------------
    banner("Step 5: Fraudulent lender records a bogus default")
    fake_loan = f"ghost-{uuid.uuid4().hex[:6]}"
    emit_credit_event(client, fraud, subject,
                       EVENT_DEFAULT, "auto-loan", fake_loan)
    print(f"  {fraud.name} recorded a false DEFAULT (loan never existed)")

    # -----------------------------------------------------------------
    banner("Step 6: Subject disputes the fraudulent default")
    file_dispute(client, subject, EVENT_DEFAULT, fraud, fake_loan,
                  "not my loan; alleged account never opened")
    print(f"  subject filed dispute against {fraud.name}")

    time.sleep(1)

    # -----------------------------------------------------------------
    banner("Step 7: Lender-B evaluates subject for mortgage")
    policy_mortgage = LenderPolicy(
        approve_threshold=0.6, decline_threshold=0.3,
        relevant_categories=None,   # consider all
    )
    evaluate_and_show(client, lender_b, subject,
                      "MORTGAGE APPLICATION", policy_mortgage)

    # -----------------------------------------------------------------
    banner("Step 8: Alt bank also evaluates (different trust graph)")
    evaluate_and_show(client, alt_bank, subject,
                      "ALT BANK APPLICATION", policy_mortgage)

    # -----------------------------------------------------------------
    banner("Step 9: Credit-denier lender (doesn't trust anyone much)")
    # lender-a directly trusts lender-a at 1.0 by construction;
    # but a fresh lender with an empty trust graph reaches a
    # different verdict.
    fresh = register(client, "fresh-lender-no-edges", "lender")
    # Fresh lender trusts only the fraudster for some reason.
    client.grant_trust(
        signer=fresh.quid, trustee=fraud.quid.id, level=0.9,
        domain=DOMAIN,
    )
    evaluate_and_show(client, fresh, subject,
                      "FRESH LENDER (trusts only fraud)", policy_mortgage)

    banner("Demo complete")
    print()
    print("Insights:")
    print(" - The subject's event stream is the canonical record. There is")
    print("   no universal credit score.")
    print(" - Each lender applies its own trust graph and policy. The SAME")
    print("   history produces different verdicts across lenders.")
    print(" - A false default from an untrusted attester is filtered out")
    print("   automatically; a false default from a (mistakenly) trusted")
    print("   attester is neutralized by the subject's on-chain dispute.")
    print(" - Alternative data (utility payments) lets a thin-file subject")
    print("   build reputation without needing a credit card first.")
    print(" - The subject owns the stream. They can selectively disclose")
    print("   it to a specific lender by granting them read access via a")
    print("   group-key or by publishing selective events publicly.")
    print()


if __name__ == "__main__":
    main()
