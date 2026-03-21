package serde

import (
	"testing"
)

func TestProtobufDeserializer_Detect(t *testing.T) {
	d := NewProtobufDeserializer(nil)

	t.Run("valid wire format", func(t *testing.T) {
		// 0x00 + 4-byte schema ID + at least 1 byte for varint
		data := []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0xff}
		if !d.Detect("t", data, nil) {
			t.Error("should detect Confluent protobuf wire format")
		}
	})

	t.Run("too short", func(t *testing.T) {
		if d.Detect("t", []byte{0x00, 0x01, 0x02, 0x03, 0x04}, nil) {
			t.Error("should not detect payload < 6 bytes")
		}
	})

	t.Run("wrong magic byte", func(t *testing.T) {
		if d.Detect("t", []byte{0x01, 0x00, 0x00, 0x00, 0x01, 0x00}, nil) {
			t.Error("should not detect non-0x00 first byte")
		}
	})
}

func TestParseMessageIndex(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		indices, n, err := parseMessageIndex([]byte{})
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("expected 0 bytes read, got %d", n)
		}
		if len(indices) != 1 || indices[0] != 0 {
			t.Errorf("expected [0], got %v", indices)
		}
	})

	t.Run("zero count", func(t *testing.T) {
		// count=0 means use default message index 0
		indices, n, err := parseMessageIndex([]byte{0x00})
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("expected 1 byte read, got %d", n)
		}
		if len(indices) != 1 || indices[0] != 0 {
			t.Errorf("expected [0], got %v", indices)
		}
	})

	t.Run("single index", func(t *testing.T) {
		// count=1, index=2
		indices, _, err := parseMessageIndex([]byte{0x01, 0x02})
		if err != nil {
			t.Fatal(err)
		}
		if len(indices) != 1 || indices[0] != 2 {
			t.Errorf("expected [2], got %v", indices)
		}
	})
}
