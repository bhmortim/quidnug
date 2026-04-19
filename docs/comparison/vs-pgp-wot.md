# Quidnug vs. PGP Web of Trust

PGP's Web of Trust (WoT) is the original decentralized
trust-graph design, dating to 1991. It never reached mass
adoption outside a small crypto / security community. Quidnug
can be thought of as a modern rebuild of the WoT concept with
everything PGP got right and (we think) the things it got wrong.

## What PGP WoT got right

- **Decentralization.** No central authority issues identity or
  signs introductions.
- **Transitive trust.** Keys signed by keys you trust are
  transitively trusted.
- **Signed assertions.** Each trust edge is cryptographic.

## What PGP WoT got wrong

- **Binary trust on key *signatures*, not on *relationships*.** A
  PGP key signature means "I verified this key belongs to this
  person." Quidnug trust edges are richer: "I trust this party at
  level L for purpose P."
- **No domain scoping.** A PGP signature is a signature, full stop.
  Quidnug scopes trust by **domain** — `contractors.home` trust is
  separate from `elections.state.nyc` trust, even between the same
  parties.
- **No replay protection.** Revoking trust in PGP means publishing
  a revocation certificate; replay attacks on stale trust edges
  are not protocol-guarded.
- **No recovery.** Lost PGP key = lost identity. Quidnug's
  guardian-based recovery (QDP-0002) provides M-of-N key rotation
  with time-lock vetoes.
- **No programmable attributes.** Quidnug identity records have
  typed attributes (DUNS, LEI, role), queryable per-domain.
- **Keyserver as SPOF.** PGP relies on keyservers (SKS, Hagrid)
  which have famously-bad behavior (SKS flooding, Hagrid
  federation). Quidnug identity is gossiped between nodes via
  QDP-0003 / QDP-0005, bounded-work per recipient.
- **Developer experience.** PGP CLI and library ergonomics are
  notoriously difficult. Quidnug ships 7 idiomatic SDKs with
  typed APIs.

## What Quidnug adds

- **Numeric trust levels** in `[0, 1]` with multiplicative
  transitive composition. A 0.8 × 0.9 chain yields 0.72, not
  "chained trust bit set."
- **Best-path computation.** Multiple parallel chains → automatic
  selection of the highest-scoring path.
- **Domain scoping.** Trust in one domain doesn't leak to another.
- **Nonce-protected replay defense.** QDP-0001.
- **M-of-N guardian recovery with time-lock veto.** QDP-0002.
- **Cross-domain gossip with compact Merkle proofs.** QDP-0010.
- **Title semantics.** Not just identity trust, but signed
  ownership claims (a first-class concept PGP never had).
- **Event streams** per quid/title, signed and Merkle-rooted.

## What Quidnug doesn't do that PGP does

- **Encrypt messages.** PGP is an email / file encryption
  protocol; Quidnug is a trust-graph protocol. Combining them:
  use Quidnug to establish trust in a party's X25519 encryption
  key, then use that key with any standard AEAD.
- **Sign arbitrary files.** Quidnug signs protocol messages
  (transactions). For signing files or commits, pair with
  Sigstore or write a small adapter that records
  `sha256(file)` as an EVENT on a Quidnug title.

## Migration

There's no automatic PGP → Quidnug migration — the signature
algorithms are different (RSA / DSA / Ed25519 vs. ECDSA P-256)
and the semantics don't map cleanly. A practical hybrid:

1. Establish Quidnug identities for your PGP-signing principals.
2. Use Quidnug's trust graph for relationship modeling.
3. Use your existing PGP keys for email / commit signing until
   Quidnug adds equivalent (QDP-0012+).

## When to use which

### Use PGP when

- You need to encrypt email (there's no substitute).
- You need to sign git commits today (Sigstore + gitsign is
  displacing this, but PGP still works everywhere).
- You need Debian package signing (APT requires it).

### Use Quidnug when

- You need trust-graph queries your app can actually consume
  programmatically.
- You need recovery.
- You need domain-scoped trust.
- You need typed identity attributes.
- You need an audit log of trust changes.

## License

Apache-2.0.
