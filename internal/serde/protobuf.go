package serde

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/Smyrcu/KafkaUI/internal/schema"
)

// ProtobufDeserializer decodes Confluent wire-format Protobuf messages.
// Wire format: [0x00][4-byte schema ID][varint message index array][protobuf payload]
type ProtobufDeserializer struct {
	schemaClient *schema.Client
	descriptors  sync.Map // schema ID (int) -> *descriptorpb.FileDescriptorProto
}

func NewProtobufDeserializer(schemaClient *schema.Client) *ProtobufDeserializer {
	return &ProtobufDeserializer{schemaClient: schemaClient}
}

func (p *ProtobufDeserializer) Name() string { return "protobuf" }

// Detect checks for the Confluent wire format. Both Avro and Protobuf start with 0x00,
// but Protobuf has a varint-encoded message index array after the schema ID.
// We rely on chain ordering: Avro is tried first (it validates via schema registry),
// so if we reach Protobuf, Avro already failed.
func (p *ProtobufDeserializer) Detect(_ string, data []byte, _ map[string]string) bool {
	return len(data) >= 6 && data[0] == 0x00
}

func (p *ProtobufDeserializer) Deserialize(_ string, data []byte) (string, error) {
	schemaID := int(binary.BigEndian.Uint32(data[1:5]))

	// Parse varint-encoded message index array after the schema ID
	payload := data[5:]
	msgIndex, bytesRead, err := parseMessageIndex(payload)
	if err != nil {
		return "", fmt.Errorf("protobuf parse message index: %w", err)
	}
	payload = payload[bytesRead:]

	fdProto, err := p.getDescriptor(schemaID)
	if err != nil {
		return "", err
	}

	// Build file descriptor and find the message type
	fd, err := protodesc.NewFile(fdProto, nil)
	if err != nil {
		return "", fmt.Errorf("protobuf build descriptor: %w", err)
	}

	msgs := fd.Messages()
	if msgs.Len() == 0 {
		return "", fmt.Errorf("protobuf schema %d has no message types", schemaID)
	}

	// Navigate to the message using the index path
	idx := 0
	if len(msgIndex) > 0 {
		idx = msgIndex[0]
	}
	if idx >= msgs.Len() {
		return "", fmt.Errorf("protobuf message index %d out of range (schema has %d messages)", idx, msgs.Len())
	}
	md := msgs.Get(idx)

	// Create dynamic message and unmarshal
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(payload, msg); err != nil {
		return "", fmt.Errorf("protobuf unmarshal: %w", err)
	}

	// Serialize to JSON
	jsonBytes, err := protojson.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("protobuf to json: %w", err)
	}
	return string(jsonBytes), nil
}

func (p *ProtobufDeserializer) getDescriptor(id int) (*descriptorpb.FileDescriptorProto, error) {
	if cached, ok := p.descriptors.Load(id); ok {
		return cached.(*descriptorpb.FileDescriptorProto), nil
	}

	schemaStr, err := p.schemaClient.GetSchemaByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	// Schema Registry returns protobuf schemas as .proto file content.
	// We need to parse it into a FileDescriptorProto.
	// The schema registry actually returns the schema as a serialized FileDescriptorProto
	// when using the protobuf serializer. Try binary first, fall back to treating as .proto text.
	fdProto := &descriptorpb.FileDescriptorProto{}
	if err := proto.Unmarshal([]byte(schemaStr), fdProto); err != nil {
		// If binary unmarshal fails, this might be a raw .proto string.
		// For now, return the error — proper .proto parsing would need a protobuf compiler.
		return nil, fmt.Errorf("protobuf parse schema %d: %w", id, err)
	}

	p.descriptors.Store(id, fdProto)
	return fdProto, nil
}

// parseMessageIndex reads the varint-encoded message index array from Confluent wire format.
// Returns the index path, number of bytes consumed, and any error.
func parseMessageIndex(data []byte) ([]int, int, error) {
	if len(data) == 0 {
		return []int{0}, 0, nil
	}

	// First varint: array length
	count, n := binary.Uvarint(data)
	if n <= 0 {
		return nil, 0, fmt.Errorf("invalid varint for message index count")
	}
	totalRead := n

	if count == 0 {
		return []int{0}, totalRead, nil
	}

	indices := make([]int, count)
	for i := uint64(0); i < count; i++ {
		if totalRead >= len(data) {
			return nil, 0, fmt.Errorf("truncated message index array")
		}
		val, n := binary.Uvarint(data[totalRead:])
		if n <= 0 {
			return nil, 0, fmt.Errorf("invalid varint for message index %d", i)
		}
		indices[i] = int(val)
		totalRead += n
	}

	return indices, totalRead, nil
}
