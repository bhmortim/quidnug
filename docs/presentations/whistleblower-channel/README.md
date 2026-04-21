# The Whistleblower Channel — Presentation

75-slide PowerPoint deck covering why current whistleblower infrastructure solves submission and encryption but not credibility, and how selective disclosure credentials, stable pseudonyms with accumulating reputation, and trust-graph vouching compose into an architecture where an anonymous disclosure carries verifiable institutional context.

Companion to the blog post at [`blogs/2026-04-28-whistleblower-channel.md`](../../../blogs/2026-04-28-whistleblower-channel.md).

## Rebuild

```bash
python build_charts.py   # regenerates PNG charts into assets/
python build_deck.py     # regenerates whistleblower-channel.pptx
```

Requires `python-pptx`, `matplotlib`, and `Pillow`. Charts write to `assets/`; the final deck writes to `whistleblower-channel.pptx` in this directory.

## Sources cited

- Chaum (1985): *Security without identification*, Communications of the ACM
- Chaum (1988): *Blind signatures for untraceable payments*, CRYPTO
- Camenisch, Lysyanskaya (2001, 2003): CL signatures
- Davidson, Faz-Hernandez, Sullivan, Wood (2024): RFC 9576 Privacy Pass
- Singer (2007); Robinson (2011): journalism sourcing literature
- Donaldson (2020): whistleblowers post-Snowden
- Radsch (2019): CPJ research on journalist safety
- ACLU (2023): Espionage Act prosecutions analysis
- Donath (1999): identity in virtual communities
- Axelrod (1984): *The Evolution of Cooperation*
- Freedom of the Press Foundation: SecureDrop
- Hermes Center: GlobaLeaks

## Quidnug architecture touchpoints

- QDP-0021 RSA-FDH-3072 blind signatures for anonymous credential issuance
- QDP-0023 DNS-anchored identity attestation for institutional attesters
- QDP-0024 MLS-based private communications for source-journalist channel
- QDP-0002 guardian recovery (mentioned, but retirement model preferred for whistleblower use)
