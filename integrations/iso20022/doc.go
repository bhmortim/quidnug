// Package iso20022 bridges ISO 20022 financial messages (pain, pacs,
// camt families) into Quidnug event streams on organization / account /
// payment-instruction Titles, giving per-observer trust queries and
// tamper-evident audit over cross-bank settlement chains.
//
// The design:
//
//   - Each ISO 20022 subject (an organization, account, or payment
//     instruction) is represented as a Quidnug Title.
//   - Each signed ISO 20022 message becomes a single EVENT on the
//     subject's Title, with event_type = "ISO20022.<family>.<variant>"
//     (e.g. "ISO20022.pacs.008").
//   - The Title is signed by the message originator's quid; the
//     receiving bank queries relational trust before accepting.
//
// Mapping examples:
//
//   pain.001 (customer credit transfer init)  → EVENT on Payment Title
//   pacs.008 (FI-to-FI customer credit)       → EVENT on Payment Title
//   pacs.002 (payment status report)          → EVENT on Payment Title
//   camt.053 (bank-to-customer statement)     → EVENT stream on Account Title
//   admi.002 (message rejection)              → EVENT on related Title
//
// This package is deliberately minimal and focuses on the mapping
// layer. It does NOT perform ISO 20022 schema validation — pair it
// with an XSD validator (xerces-c, libxml2+schema) upstream. It also
// does NOT perform XAdES / CAdES signature verification; use an
// external library.
//
// # Usage
//
//	r, _ := iso20022.New(iso20022.Options{Client: c, Domain: "bank.settlement"})
//	_, err := r.RecordMessage(ctx, bankQuid, "payment-title-id", iso20022.Message{
//	    Family:  "pacs",
//	    Variant: "pacs.008.001.08",
//	    Role:    iso20022.RoleSettlement,
//	    Raw:     rawXMLBytes,
//	    ParsedHeader: iso20022.Header{
//	        MessageID: "MSG-2024-001",
//	        From:      "BANKAUSXXX",
//	        To:        "BANKBEBBXXX",
//	        Amount:    1000.00,
//	        Currency:  "EUR",
//	    },
//	})
package iso20022
