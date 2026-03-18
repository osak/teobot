package util

import (
	"bytes"
	"encoding/json"
)

// PlainJSONMarshal serializes payload into JSON with as minimal escaping as possible.
func PlainJSONMarshal(payload any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
