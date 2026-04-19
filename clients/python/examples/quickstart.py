"""Quidnug Python SDK — two-party trust quickstart.

Run a local Quidnug node first::

    ./bin/quidnug --config config.example.yaml

Then execute this script. It will:
  1. Generate two fresh quids (Alice, Bob).
  2. Register both identities on the node.
  3. Grant Alice → Bob trust at 0.9 in the "demo.home" domain.
  4. Query the relational-trust result and print it.

Safe to re-run: the node idempotently accepts repeat identities with
the same public key. Repeated trust grants will bump the nonce.
"""

from __future__ import annotations

from quidnug import Quid, QuidnugClient


def main() -> None:
    client = QuidnugClient("http://localhost:8080")

    # Health check first — fail fast with a readable message if the
    # node isn't listening.
    info = client.info()
    print(f"Connected to node {info.get('quidId', '?')} v{info.get('version', '?')}")

    alice = Quid.generate()
    bob = Quid.generate()
    print(f"Alice quid: {alice.id}")
    print(f"Bob   quid: {bob.id}")

    client.register_identity(alice, name="Alice", home_domain="demo.home")
    client.register_identity(bob, name="Bob", home_domain="demo.home")

    client.grant_trust(alice, trustee=bob.id, level=0.9, domain="demo.home")
    print("Alice → Bob trust edge registered.")

    tr = client.get_trust(alice.id, bob.id, domain="demo.home")
    print(f"Relational trust: {tr.trust_level:.3f} via {' → '.join(tr.path) or '(none)'}")


if __name__ == "__main__":
    main()
