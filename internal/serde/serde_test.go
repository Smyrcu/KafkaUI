package serde

import (
	"testing"
)

func TestStringDeserializer_ValidUTF8(t *testing.T) {
	d := &StringDeserializer{}
	result, err := d.Deserialize("test", []byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestStringDeserializer_InvalidUTF8(t *testing.T) {
	d := &StringDeserializer{}
	result, _ := d.Deserialize("test", []byte{0xff, 0xfe, 0x68, 0x69})
	if result == "" {
		t.Error("expected non-empty result for invalid UTF-8")
	}
}

func TestStringDeserializer_AlwaysDetects(t *testing.T) {
	d := &StringDeserializer{}
	if !d.Detect("any", []byte{}, nil) {
		t.Error("string deserializer should always detect")
	}
}

func TestJSONDeserializer_PrettyPrints(t *testing.T) {
	d := &JSONDeserializer{}
	input := []byte(`{"name":"test","value":42}`)
	if !d.Detect("t", input, nil) {
		t.Fatal("should detect valid JSON")
	}
	result, err := d.Deserialize("t", input)
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\n  \"name\": \"test\",\n  \"value\": 42\n}"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestJSONDeserializer_RejectsInvalidJSON(t *testing.T) {
	d := &JSONDeserializer{}
	if d.Detect("t", []byte("not json"), nil) {
		t.Error("should not detect invalid JSON")
	}
}

func TestChain_EmptyData(t *testing.T) {
	c := NewChain(&JSONDeserializer{}, &StringDeserializer{})
	if result := c.Deserialize("t", nil, nil); result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestChain_JSONFirst(t *testing.T) {
	c := NewChain(&JSONDeserializer{}, &StringDeserializer{})
	result := c.Deserialize("t", []byte(`{"a":1}`), nil)
	if result != "{\n  \"a\": 1\n}" {
		t.Errorf("expected pretty JSON, got %q", result)
	}
}

func TestChain_FallbackToString(t *testing.T) {
	c := NewChain(&JSONDeserializer{}, &StringDeserializer{})
	result := c.Deserialize("t", []byte("plain text"), nil)
	if result != "plain text" {
		t.Errorf("expected 'plain text', got %q", result)
	}
}

func TestChain_UltimateFallback(t *testing.T) {
	// Chain with no deserializers falls back to raw string
	c := NewChain()
	result := c.Deserialize("t", []byte("raw"), nil)
	if result != "raw" {
		t.Errorf("expected 'raw', got %q", result)
	}
}
