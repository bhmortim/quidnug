"""Emit an event onto a quid's stream and read it back.

Events are append-only records under a ``subject_id`` (a quid or
title). This example emits a LOGIN event and then fetches the stream
contents.
"""

from __future__ import annotations

from quidnug import Quid, QuidnugClient


def main() -> None:
    client = QuidnugClient("http://localhost:8080")

    alice = Quid.generate()
    client.register_identity(alice, name="Alice")

    # Inline payload — for larger payloads pin to IPFS and use payload_cid.
    receipt = client.emit_event(
        alice,
        subject_id=alice.id,
        subject_type="QUID",
        event_type="LOGIN",
        payload={"ip": "198.51.100.7", "ua": "Mozilla/5.0"},
    )
    print(f"Event submitted: tx_id={receipt.get('id')} sequence={receipt.get('sequence')}")

    events, pagination = client.get_stream_events(alice.id, limit=10)
    print(f"Stream has {len(events)} event(s):")
    for e in events:
        print(f"  [{e.sequence}] {e.event_type} @ {e.timestamp}: {e.payload}")


if __name__ == "__main__":
    main()
