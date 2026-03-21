package serde

import (
	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/schema"
)

// BuildChain creates a deserializer chain based on the cluster's SerDe config.
// When default is "auto" (or empty), the chain tries Avro → Protobuf → JSON → String.
// When a specific format is set, only that deserializer (+ String fallback) is used.
func BuildChain(cfg config.SerDeConfig, schemaClient *schema.Client) *Chain {
	switch cfg.Default {
	case "json":
		return NewChain(&JSONDeserializer{}, &StringDeserializer{})
	case "string":
		return NewChain(&StringDeserializer{})
	case "avro":
		if schemaClient != nil {
			return NewChain(NewAvroDeserializer(schemaClient), &StringDeserializer{})
		}
		return NewChain(&JSONDeserializer{}, &StringDeserializer{})
	case "protobuf":
		if schemaClient != nil {
			return NewChain(NewProtobufDeserializer(schemaClient), &StringDeserializer{})
		}
		return NewChain(&JSONDeserializer{}, &StringDeserializer{})
	default: // "auto" or empty
		ds := []Deserializer{}
		if schemaClient != nil {
			ds = append(ds, NewAvroDeserializer(schemaClient), NewProtobufDeserializer(schemaClient))
		}
		ds = append(ds, &JSONDeserializer{}, &StringDeserializer{})
		return NewChain(ds...)
	}
}
