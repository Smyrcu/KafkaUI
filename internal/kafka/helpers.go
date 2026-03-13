package kafka

import (
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

// kafkaErrMsg builds a human-readable error string from a Kafka error code
// and an optional error message pointer.
func kafkaErrMsg(code int16, msg *string) string {
	m := ""
	if msg != nil {
		m = *msg
	}
	return fmt.Sprintf("error code %d: %s", code, m)
}

// recordHeaders converts a kgo.Record's headers into a map.
// Returns nil when the record carries no headers.
func recordHeaders(rec *kgo.Record) map[string]string {
	if len(rec.Headers) == 0 {
		return nil
	}
	h := make(map[string]string, len(rec.Headers))
	for _, kh := range rec.Headers {
		h[kh.Key] = string(kh.Value)
	}
	return h
}
