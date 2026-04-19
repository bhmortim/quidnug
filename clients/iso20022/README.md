# Quidnug × ISO 20022 bridge (scaffold)

ISO 20022 is the global XML/ISO-standard messaging format for
financial messages (payments, securities, FX, cards, trade finance).
This bridge is a scaffold for mapping ISO 20022 messages onto
Quidnug titles + event streams — giving per-observer trust queries
and an audit-grade event log for cross-bank settlement, KYC/AML
flags, and payment instruction chains.

## Status

**Scaffold.** Defines the mapping layer contract only. Production
bridges will add full pacs/pain/camt schema handling, signature
validation via CAdES/XAdES, and SWIFT / TCH / FedNow rails-specific
adapters.

## The Mapping

| ISO 20022 concept | Quidnug concept |
| --- | --- |
| Payment instruction (`pain.001`) | EVENT on a Payment Title |
| Credit transfer (`pacs.008`) | EVENT on the Payment Title, signed by settlement bank |
| Payment status report (`pacs.002`) | EVENT on the Payment Title, signed by beneficiary bank |
| Cash management statement (`camt.053`) | EVENT stream on an Account Title |
| Organization (BIC + LEI) | Quidnug Title (asset_type="organization") |
| Account | Quidnug Title (asset_type="account") |
| Trust relation "bank A accepts bank B's KYC" | Trust edge A→B in domain `iso20022.kyc` |

Every signed ISO 20022 message in a flow becomes one immutable
Quidnug event. Relational trust lets a receiving bank ask "do I
transitively trust the originating institution's KYC chain at
≥ 0.8?" before accepting.

## Event types

| ISO 20022 | Quidnug event_type |
| --- | --- |
| pain.001.001.xx | `ISO20022.pain.001` |
| pacs.008.001.xx | `ISO20022.pacs.008` |
| pacs.002.001.xx | `ISO20022.pacs.002` |
| camt.053.001.xx | `ISO20022.camt.053` |
| admi.002.001.xx | `ISO20022.admi.002` |

## Roadmap

1. XSD schema bundle for pain/pacs/camt message families.
2. Go package `integrations/iso20022/` with:
   - `Parse([]byte) (Message, error)` backed by an XSD-validated parser.
   - `RecordPaymentInstruction(ctx, signer, titleID, msg)`.
   - `RecordSettlement(ctx, signer, titleID, msg)`.
3. CAdES / XAdES signature verification before recording.
4. Python bindings via the existing `quidnug` package, shipping as
   an extra dependency set.

## Why

The ISO 20022 migration (SWIFT MT → MX by Nov 2025) is creating
enormous new volumes of structured financial messages, but the trust
graph between parties is still a tangle of bilateral correspondent
agreements. Quidnug lets each receiving bank compute a transitive
trust score on the originating chain — collapsing the bilateral
matrix into a lookup.

## License

Apache-2.0.
