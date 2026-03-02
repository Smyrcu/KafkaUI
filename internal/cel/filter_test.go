package cel

import (
	"testing"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

func TestNewFilter_ValidExpression(t *testing.T) {
	f, err := NewFilter(`key.contains("order")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil filter")
	}
}

func TestNewFilter_InvalidExpression(t *testing.T) {
	_, err := NewFilter(`invalid syntax !!!`)
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestNewFilter_NonBoolReturn(t *testing.T) {
	_, err := NewFilter(`key + "suffix"`)
	if err == nil {
		t.Fatal("expected error for non-bool return type")
	}
}

func TestFilter_MatchKey(t *testing.T) {
	f, err := NewFilter(`key == "order-123"`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{Key: "order-123", Value: "{}"}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match")
	}

	msg.Key = "other"
	ok, err = f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if ok {
		t.Error("expected no match")
	}
}

func TestFilter_MatchJSONValue(t *testing.T) {
	f, err := NewFilter(`value.status == "ERROR" && value.code > 400`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{
		Key:   "test",
		Value: `{"status":"ERROR","code":500}`,
	}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match")
	}
}

func TestFilter_MatchStringValue(t *testing.T) {
	f, err := NewFilter(`value.contains("error")`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{Key: "test", Value: "some error occurred"}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match for string value")
	}
}

func TestFilter_MatchHeaders(t *testing.T) {
	f, err := NewFilter(`headers.source == "payments"`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{
		Key:     "test",
		Value:   "{}",
		Headers: map[string]string{"source": "payments"},
	}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match")
	}
}

func TestFilter_MatchPartitionOffset(t *testing.T) {
	f, err := NewFilter(`partition == 3 && offset > 100`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{
		Partition: 3,
		Offset:    150,
		Key:       "test",
		Value:     "{}",
	}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match")
	}
}

func TestFilter_MatchTimestamp(t *testing.T) {
	f, err := NewFilter(`timestamp > timestamp("2026-03-01T00:00:00Z")`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{
		Timestamp: time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC),
		Key:       "test",
		Value:     "{}",
	}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match")
	}
}

func TestFilter_NilHeaders(t *testing.T) {
	f, err := NewFilter(`key == "test"`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{Key: "test", Value: "{}", Headers: nil}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match even with nil headers")
	}
}

func TestFilter_ComplexExpression(t *testing.T) {
	f, err := NewFilter(`value.amount > 1000 && headers.source == "payments" && key.contains("order")`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	msg := kafka.MessageRecord{
		Key:     "order-456",
		Value:   `{"amount":2500,"currency":"USD"}`,
		Headers: map[string]string{"source": "payments"},
	}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !ok {
		t.Error("expected match")
	}
}

func TestFilter_MissingKeyReturnsNoMatch(t *testing.T) {
	f, err := NewFilter(`value.action == "login"`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// Message without "action" field — should return false, not error
	msg := kafka.MessageRecord{Key: "test", Value: `{"status":"OK"}`}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("expected no error for missing key, got: %v", err)
	}
	if ok {
		t.Error("expected no match for missing key")
	}
}

func TestFilter_NonJSONValueWithFieldAccess(t *testing.T) {
	f, err := NewFilter(`value.action == "login"`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// Plain text value — should return false, not error
	msg := kafka.MessageRecord{Key: "test", Value: "plain text"}
	ok, err := f.Match(msg)
	if err != nil {
		t.Fatalf("expected no error for non-JSON value, got: %v", err)
	}
	if ok {
		t.Error("expected no match for non-JSON value")
	}
}
