package serde

import (
	"strings"
	"unicode/utf8"
)

// StringDeserializer is the fallback deserializer that always succeeds.
// It returns the data as a UTF-8 string, replacing invalid bytes.
type StringDeserializer struct{}

func (s *StringDeserializer) Name() string                                        { return "string" }
func (s *StringDeserializer) Detect(_ string, _ []byte, _ map[string]string) bool { return true }

func (s *StringDeserializer) Deserialize(_ string, data []byte) (string, error) {
	if utf8.Valid(data) {
		return string(data), nil
	}
	return strings.ToValidUTF8(string(data), "\uFFFD"), nil
}
