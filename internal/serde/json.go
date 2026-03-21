package serde

import (
	"bytes"
	"encoding/json"
)

// JSONDeserializer pretty-prints valid JSON data.
type JSONDeserializer struct{}

func (j *JSONDeserializer) Name() string { return "json" }

func (j *JSONDeserializer) Detect(_ string, data []byte, _ map[string]string) bool {
	return json.Valid(data)
}

func (j *JSONDeserializer) Deserialize(_ string, data []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return string(data), nil
	}
	return buf.String(), nil
}
