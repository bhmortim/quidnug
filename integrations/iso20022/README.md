# Quidnug × ISO 20022 bridge

`integrations/iso20022` maps ISO 20022 financial messages (pain, pacs,
camt, admi) onto Quidnug event streams — making cross-bank trust,
audit chains, and settlement-lifecycle queries first-class per-observer
trust questions.

## Why

ISO 20022 is the global XML messaging standard for financial flows:
payments (pain, pacs), cash management (camt), securities, FX, cards,
trade finance. SWIFT's MT → MX migration (completing Nov 2025)
dramatically increases the volume of signed ISO 20022 messages moving
between institutions.

Today the trust graph between banks is a bilateral tangle of
correspondent agreements. Quidnug collapses that matrix into a lookup:

```
Receiving bank: "Given my observer quid, what's my transitive trust
in the originator of this pacs.008?"
  → Quidnug relational-trust query → 0.83 via BANK_B → BANK_A
```

## What this package does

For each signed ISO 20022 message, the recorder:

1. Extracts a small header (message id, BICs, amount, currency, etc.).
2. Emits a Quidnug `EVENT` on the payment's Quidnug Title, signed by
   the bank's quid.
3. Embeds the raw XML in the event payload so auditors can re-verify
   the XAdES / CAdES signature any time.

### Event-type conventions

| Message | Family | event_type |
| --- | --- | --- |
| pain.001 | pain | `ISO20022.pain.pain.001.001.09` |
| pacs.008 | pacs | `ISO20022.pacs.pacs.008.001.08` |
| pacs.002 | pacs | `ISO20022.pacs.pacs.002.001.10` |
| camt.053 | camt | `ISO20022.camt.camt.053.001.08` |
| admi.002 | admi | `ISO20022.admi.admi.002.001.01` |

The canonical format is `ISO20022.<family>.<variant>`.

## Quickstart

```go
import (
    "context"
    "github.com/quidnug/quidnug/integrations/iso20022"
    "github.com/quidnug/quidnug/pkg/client"
)

ctx := context.Background()
c, _ := client.New("http://quidnug.local:8080")

rec, _ := iso20022.New(iso20022.Options{
    Client: c,
    Domain: "bank.settlement",
})

bankQuid, _ := client.QuidFromPrivateHex(os.Getenv("BANK_KEY_HEX"))

_, err := rec.RecordMessage(ctx, bankQuid, "payment-title-id", iso20022.Message{
    Family:  "pacs",
    Variant: "pacs.008.001.08",
    Role:    iso20022.RoleSettlement,
    Raw:     rawXML,   // the verified signed XML
    ParsedHeader: iso20022.Header{
        MessageID: "E2E-REF-2024-001",
        From:      "BANKAUSXXX",
        To:        "BANKBEBBXXX",
        Amount:    50_000.00,
        Currency:  "EUR",
        CreatedAt: time.Now().Unix(),
    },
})
```

## Minimal header extraction

For simple flows where you just need the message id and timestamp, a
lightweight header parser is provided:

```go
header, _ := iso20022.ExtractHeader(rawXML)
// header.MessageID, header.CreatedAt, header.Reference, header.Extras
```

For full business-field extraction, use a proper XSD-validated
ISO 20022 parser (e.g. `moov-io/iso20022`) and fill in `ParsedHeader`
yourself.

## Verification is the caller's job

This package does NOT verify XAdES / CAdES signatures on incoming
messages. Pair it with an XML signature verification step before
calling `RecordMessage` — a recorded bundle should always be one that
has already been validated:

```go
// 1. Verify the XML signature
if err := xades.Verify(rawXML, bankCert); err != nil {
    return fmt.Errorf("signature verify: %w", err)
}

// 2. Then record onto Quidnug
rec.RecordMessage(ctx, ...)
```

## Query patterns

Once messages are on-stream, any Quidnug-aware app can ask:

```go
// "Show me every settlement message on this payment."
events, _, _ := c.GetStreamEvents(ctx, paymentTitleID, "bank.settlement", 100, 0)

// "Score the originator's transitive trust from my perspective."
tr, _ := c.GetTrust(ctx, myBankQuid, originatorQuid, "bank.settlement", 5)

// "Reject anything with trust < 0.7."
if tr.TrustLevel < 0.7 {
    return ErrUnknownCorrespondent
}
```

## Example

See [`examples/cross_border_payment.go`](examples/cross_border_payment.go)
for a full `pain.001 → pacs.008 → pacs.002` flow recorded end-to-end.

## Schema mappings

The package is intentionally light on schema specifics — pair it with
whichever ISO 20022 library your institution already uses. The role
enum is the main editorial choice:

| Role | Typical messages |
| --- | --- |
| `initiation` | pain.001, pain.008 (customer-initiated) |
| `settlement` | pacs.008, pacs.009 (FI-to-FI credit transfer) |
| `status` | pacs.002 (status report) |
| `statement` | camt.053, camt.054 (account / transaction statements) |
| `rejection` | admi.002 (message rejection) |
| `inquiry` | camt.027 (investigation) |
| `generic` | anything not mapped above |

## Status

**Working Go package** with 4 passing tests. Companion packages for
other languages will arrive as their Quidnug SDK reaches v2.x parity.

## License

Apache-2.0.
