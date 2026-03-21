package serde

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/linkedin/goavro/v2"

	"github.com/Smyrcu/KafkaUI/internal/schema"
)

// AvroDeserializer decodes Confluent wire-format Avro messages.
// Wire format: [0x00][4-byte schema ID big-endian][avro binary payload]
type AvroDeserializer struct {
	schemaClient *schema.Client
	codecs       sync.Map // schema ID (int) -> *goavro.Codec
}

func NewAvroDeserializer(schemaClient *schema.Client) *AvroDeserializer {
	return &AvroDeserializer{schemaClient: schemaClient}
}

func (a *AvroDeserializer) Name() string { return "avro" }

// Detect checks for the Confluent wire format magic byte and minimum payload length.
func (a *AvroDeserializer) Detect(_ string, data []byte, _ map[string]string) bool {
	return len(data) >= 5 && data[0] == 0x00
}

func (a *AvroDeserializer) Deserialize(_ string, data []byte) (string, error) {
	schemaID := int(binary.BigEndian.Uint32(data[1:5]))

	codec, err := a.getCodec(schemaID)
	if err != nil {
		return "", err
	}

	native, _, err := codec.NativeFromBinary(data[5:])
	if err != nil {
		return "", fmt.Errorf("avro decode: %w", err)
	}

	jsonBytes, err := json.Marshal(native)
	if err != nil {
		return "", fmt.Errorf("avro to json: %w", err)
	}
	return string(jsonBytes), nil
}

func (a *AvroDeserializer) getCodec(id int) (*goavro.Codec, error) {
	if cached, ok := a.codecs.Load(id); ok {
		return cached.(*goavro.Codec), nil
	}

	schemaJSON, err := a.schemaClient.GetSchemaByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	codec, err := goavro.NewCodec(schemaJSON)
	if err != nil {
		return nil, fmt.Errorf("avro codec for schema %d: %w", id, err)
	}

	a.codecs.Store(id, codec)
	return codec, nil
}
