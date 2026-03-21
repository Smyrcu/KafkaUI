package serde

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkedin/goavro/v2"

	"github.com/Smyrcu/KafkaUI/internal/schema"
)

const testAvroSchema = `{
	"type": "record",
	"name": "TestRecord",
	"fields": [
		{"name": "name", "type": "string"},
		{"name": "age", "type": "int"}
	]
}`

func buildAvroPayload(t *testing.T, schemaID int, schemaJSON string, record map[string]any) []byte {
	t.Helper()
	codec, err := goavro.NewCodec(schemaJSON)
	if err != nil {
		t.Fatalf("failed to create codec: %v", err)
	}
	avroBytes, err := codec.BinaryFromNative(nil, record)
	if err != nil {
		t.Fatalf("failed to encode avro: %v", err)
	}
	// Confluent wire format: 0x00 + 4-byte schema ID + avro bytes
	buf := make([]byte, 5+len(avroBytes))
	buf[0] = 0x00
	binary.BigEndian.PutUint32(buf[1:5], uint32(schemaID))
	copy(buf[5:], avroBytes)
	return buf
}

func mockSchemaRegistry(t *testing.T, schemas map[int]string) *schema.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id int
		if _, err := fmt.Sscanf(r.URL.Path, "/schemas/ids/%d", &id); err != nil {
			http.NotFound(w, r)
			return
		}
		s, ok := schemas[id]
		if !ok {
			http.Error(w, `{"error_code":40403,"message":"Schema not found"}`, 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"schema": s})
	}))
	t.Cleanup(srv.Close)
	return schema.NewClient(srv.URL)
}

func TestAvroDeserializer_Detect(t *testing.T) {
	d := NewAvroDeserializer(nil)

	t.Run("valid magic byte", func(t *testing.T) {
		data := []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0xff}
		if !d.Detect("t", data, nil) {
			t.Error("should detect Confluent wire format")
		}
	})

	t.Run("too short", func(t *testing.T) {
		if d.Detect("t", []byte{0x00, 0x01}, nil) {
			t.Error("should not detect payload < 5 bytes")
		}
	})

	t.Run("wrong magic byte", func(t *testing.T) {
		if d.Detect("t", []byte{0x01, 0x00, 0x00, 0x00, 0x01}, nil) {
			t.Error("should not detect non-0x00 first byte")
		}
	})
}

func TestAvroDeserializer_Deserialize(t *testing.T) {
	client := mockSchemaRegistry(t, map[int]string{1: testAvroSchema})
	d := NewAvroDeserializer(client)

	payload := buildAvroPayload(t, 1, testAvroSchema, map[string]any{
		"name": "Alice",
		"age":  30,
	})

	result, err := d.Deserialize("test-topic", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(result), &decoded); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if decoded["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", decoded["name"])
	}
}

func TestAvroDeserializer_UnknownSchema(t *testing.T) {
	client := mockSchemaRegistry(t, map[int]string{})
	d := NewAvroDeserializer(client)

	payload := []byte{0x00, 0x00, 0x00, 0x00, 0x63} // schema ID 99, no data
	_, err := d.Deserialize("t", payload)
	if err == nil {
		t.Error("expected error for unknown schema ID")
	}
}
