package serde

import (
	"bytes"
	"encoding/json"
)

// JSONDeserializer pretty-prints valid JSON data.
type JSONDeserializer struct{}

func (j *JSONDeserializer) Name() string { return "json" }

func (j *JSONDeserializer) Detect(_ string, data []byte, _ map[string]string) bool {
	// Quick check: first non-whitespace byte must be { or [
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{', '[':
			return true
		default:
			return false
		}
	}
	return false
}

func (j *JSONDeserializer) Deserialize(_ string, data []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return string(data), nil
	}
	return buf.String(), nil
}
