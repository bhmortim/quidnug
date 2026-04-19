package iso20022

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quidnug/quidnug/pkg/client"
)

func TestRecordMessageHappyPath(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(404)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   map[string]any{"code": "NOT_FOUND"},
			})
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&posted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true, "data": map[string]any{"id": "evt-1", "sequence": 1},
		})
	}))
	defer srv.Close()

	c, _ := client.New(srv.URL, client.WithMaxRetries(0))
	r, err := New(Options{Client: c, Domain: "bank.settle"})
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := client.GenerateQuid()

	_, err = r.RecordMessage(context.Background(), signer, "payment-1", Message{
		Family:  "pacs",
		Variant: "pacs.008.001.08",
		Role:    RoleSettlement,
		Raw:     []byte("<Document>...</Document>"),
		ParsedHeader: Header{
			MessageID: "MSG-1",
			From:      "BANKAUSXXX",
			To:        "BANKBEBBXXX",
			Amount:    1000.00,
			Currency:  "EUR",
		},
	})
	if err != nil {
		t.Fatalf("RecordMessage: %v", err)
	}

	if posted["type"] != "EVENT" {
		t.Fatalf("type: %v", posted["type"])
	}
	if posted["eventType"] != "ISO20022.pacs.pacs.008.001.08" {
		t.Fatalf("eventType: %v", posted["eventType"])
	}
	payload, _ := posted["payload"].(map[string]any)
	if payload["schema"] != "iso20022-message/v1" {
		t.Fatalf("schema: %v", payload["schema"])
	}
	if payload["role"] != "settlement" {
		t.Fatalf("role: %v", payload["role"])
	}
}

func TestRecordMessageValidation(t *testing.T) {
	c, _ := client.New("http://x")
	r, _ := New(Options{Client: c})
	cases := []struct {
		name string
		asset string
		msg Message
	}{
		{"missing asset", "", Message{Family: "pacs", ParsedHeader: Header{MessageID: "m"}}},
		{"missing family and variant", "p1", Message{ParsedHeader: Header{MessageID: "m"}}},
		{"missing header id", "p1", Message{Family: "pacs"}},
	}
	for _, c_ := range cases {
		t.Run(c_.name, func(t *testing.T) {
			if _, err := r.RecordMessage(context.Background(), nil, c_.asset, c_.msg); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestEventTypeFor(t *testing.T) {
	cases := []struct {
		m Message
		want string
	}{
		{Message{Family: "pacs", Variant: "pacs.008.001.08"}, "ISO20022.pacs.pacs.008.001.08"},
		{Message{Variant: "camt.053.001.08"},                   "ISO20022.camt.053.001.08"},
		{Message{Family: "pain"},                               "ISO20022.pain"},
		{Message{},                                              "ISO20022"},
	}
	for _, c := range cases {
		if got := eventTypeFor(c.m); got != c.want {
			t.Errorf("eventTypeFor(%+v) = %q, want %q", c.m, got, c.want)
		}
	}
}

func TestExtractHeaderMinimalGrpHdr(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pacs.008.001.08">
  <FIToFICstmrCdtTrf>
    <GrpHdr>
      <MsgId>ABC-1234-5678</MsgId>
      <CreDtTm>2024-01-15T10:30:45Z</CreDtTm>
      <NbOfTxs>1</NbOfTxs>
    </GrpHdr>
    <CdtTrfTxInf>
      <PmtId><EndToEndId>E2E-REF-42</EndToEndId></PmtId>
    </CdtTrfTxInf>
  </FIToFICstmrCdtTrf>
</Document>`
	h, err := ExtractHeader([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if h.MessageID != "ABC-1234-5678" {
		t.Errorf("MessageID: %q", h.MessageID)
	}
	if h.CreatedAt == 0 {
		t.Errorf("CreatedAt should be parsed")
	}
	if h.Extras["NbOfTxs"] != "1" {
		t.Errorf("NbOfTxs extra: %v", h.Extras["NbOfTxs"])
	}
}
