package webauthn

import "encoding/json"

// jsonUnmarshal is a thin alias so the webauthn.go file can stay free
// of the encoding/json import — making it easier to swap in a
// canonical-JSON decoder later without touching every call site.
func jsonUnmarshal(raw []byte, v any) error {
	return json.Unmarshal(raw, v)
}
