package iso20022

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/quidnug/quidnug/pkg/client"
)

// Role classifies the message within a payment flow. Distinct event
// types per role let consumers filter "all settlement messages from
// bank X" without loading every status report.
type Role string

const (
	RoleInitiation  Role = "initiation"   // pain.001, pain.008
	RoleSettlement  Role = "settlement"   // pacs.008, pacs.009
	RoleStatus      Role = "status"       // pacs.002
	RoleStatement   Role = "statement"    // camt.053, camt.054
	RoleRejection   Role = "rejection"    // admi.002
	RoleInquiry     Role = "inquiry"      // camt.027
	RoleGeneric     Role = "generic"
)

// Header is the minimal set of ISO 20022 header fields this package
// extracts for event payloads. Populate the fields your business
// logic cares about; unknown fields flow through via Extras.
type Header struct {
	// MessageID — globally unique <MsgId> / <GrpHdr><MsgId>.
	MessageID string `json:"messageId"`
	// From and To are BIC / LEI codes for the originating and
	// destination institutions.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Amount + Currency (ISO 4217) for payment-family messages.
	Amount   float64 `json:"amount,omitempty"`
	Currency string  `json:"currency,omitempty"`
	// CreatedAt is the <CreDtTm> in Unix seconds.
	CreatedAt int64 `json:"createdAt,omitempty"`
	// Reference is an end-to-end business reference (<EndToEndId>).
	Reference string `json:"reference,omitempty"`
	// Extras carry any additional parsed fields.
	Extras map[string]any `json:"extras,omitempty"`
}

// Message wraps a signed ISO 20022 XML payload with the minimal
// metadata the mapping layer needs.
type Message struct {
	// Family is the ISO 20022 family ("pacs", "pain", "camt", "admi").
	Family string
	// Variant is the dotted variant ID ("pacs.008.001.08").
	Variant string
	// Role classifies the message in a payment flow.
	Role Role
	// Raw is the verbatim XML bytes; stored verbatim in the event
	// payload so downstream auditors can re-verify signatures.
	Raw []byte
	// ParsedHeader is the minimal header the integrator has already
	// extracted. Leaving it zero means "don't include in payload".
	ParsedHeader Header
}

// Options configures the Recorder.
type Options struct {
	Client           *client.Client
	Domain           string
	// IncludeRawXML controls whether the raw message bytes are
	// inlined in the event payload. Default true. Disable for very
	// large messages that should be pinned to IPFS instead.
	IncludeRawXML bool
	// RawFieldName is the payload key to use for the XML string.
	// Defaults to "rawXml".
	RawFieldName string
}

// Recorder posts ISO 20022 events to a Quidnug node.
type Recorder struct {
	client        *client.Client
	domain        string
	includeRawXML bool
	rawField      string
}

// New constructs a Recorder.
func New(opts Options) (*Recorder, error) {
	if opts.Client == nil {
		return nil, errors.New("Client is required")
	}
	if opts.Domain == "" {
		opts.Domain = "iso20022.default"
	}
	if opts.RawFieldName == "" {
		opts.RawFieldName = "rawXml"
	}
	return &Recorder{
		client:        opts.Client,
		domain:        opts.Domain,
		includeRawXML: opts.IncludeRawXML || true, // default on
		rawField:      opts.RawFieldName,
	}, nil
}

// RecordMessage submits the message as a Quidnug EVENT on the given
// Title. Returns the server's receipt.
//
// The event type is derived from Family + Variant:
//
//	ISO20022.pacs.pacs.008.001.08
func (r *Recorder) RecordMessage(
	ctx context.Context,
	signer *client.Quid,
	assetTitleID string,
	msg Message,
) (map[string]any, error) {
	if err := validate(assetTitleID, msg); err != nil {
		return nil, err
	}
	payload := map[string]any{
		"schema":  "iso20022-message/v1",
		"family":  msg.Family,
		"variant": msg.Variant,
		"role":    string(roleOrDefault(msg.Role)),
		"header":  msg.ParsedHeader,
	}
	if r.includeRawXML && len(msg.Raw) > 0 {
		payload[r.rawField] = string(msg.Raw)
	}
	eventType := eventTypeFor(msg)
	return r.client.EmitEvent(ctx, signer, client.EventParams{
		SubjectID:   assetTitleID,
		SubjectType: "TITLE",
		EventType:   eventType,
		Domain:      r.domain,
		Payload:     payload,
	})
}

func eventTypeFor(m Message) string {
	base := "ISO20022"
	if m.Family != "" && m.Variant != "" {
		return fmt.Sprintf("%s.%s.%s", base, m.Family, m.Variant)
	}
	if m.Variant != "" {
		return fmt.Sprintf("%s.%s", base, m.Variant)
	}
	if m.Family != "" {
		return fmt.Sprintf("%s.%s", base, m.Family)
	}
	return base
}

func roleOrDefault(r Role) Role {
	if r == "" {
		return RoleGeneric
	}
	return r
}

func validate(assetTitleID string, m Message) error {
	if assetTitleID == "" {
		return errors.New("assetTitleID is required")
	}
	if m.Variant == "" && m.Family == "" {
		return errors.New("at least one of Family or Variant is required")
	}
	if m.ParsedHeader.MessageID == "" {
		return errors.New("ParsedHeader.MessageID is required")
	}
	return nil
}

// --- Lightweight XML extraction helper ------------------------------------

// ExtractHeader does a minimal XPath-ish parse of common ISO 20022
// header elements. Returns a populated [Header] with Extras carrying
// any remaining <GrpHdr> child elements.
//
// This is NOT a full ISO 20022 parser — it inspects the first <GrpHdr>
// it finds and pulls out MessageID / CreatedAt / end-to-end reference.
// Business integrations should use an XSD-validated parser for full
// field extraction; this helper is here for simple recording flows.
func ExtractHeader(raw []byte) (Header, error) {
	var h Header
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	inGrpHdr := false
	depth := 0
	var currentElem string
	extras := map[string]any{}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch v := tok.(type) {
		case xml.StartElement:
			depth++
			name := v.Name.Local
			if name == "GrpHdr" || name == "MsgHdr" {
				inGrpHdr = true
			}
			currentElem = name
		case xml.EndElement:
			depth--
			if v.Name.Local == "GrpHdr" || v.Name.Local == "MsgHdr" {
				inGrpHdr = false
			}
		case xml.CharData:
			if !inGrpHdr {
				continue
			}
			text := strings.TrimSpace(string(v))
			if text == "" {
				continue
			}
			switch currentElem {
			case "MsgId":
				h.MessageID = text
			case "CreDtTm":
				if t, err := time.Parse(time.RFC3339, text); err == nil {
					h.CreatedAt = t.Unix()
				}
			case "EndToEndId":
				h.Reference = text
			default:
				extras[currentElem] = text
			}
		}
	}
	if len(extras) > 0 {
		h.Extras = extras
	}
	return h, nil
}
