package serde

// Deserializer converts raw Kafka message bytes to a display string.
type Deserializer interface {
	Name() string
	Detect(topic string, data []byte, headers map[string]string) bool
	Deserialize(topic string, data []byte) (string, error)
}

// Chain tries deserializers in order, returning the first successful result.
type Chain struct {
	deserializers []Deserializer
}

func NewChain(ds ...Deserializer) *Chain {
	return &Chain{deserializers: ds}
}

// Deserialize attempts each deserializer in order. On the first successful
// Detect+Deserialize, it returns the result. Falls back to raw string.
func (c *Chain) Deserialize(topic string, data []byte, headers map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	for _, d := range c.deserializers {
		if d.Detect(topic, data, headers) {
			if result, err := d.Deserialize(topic, data); err == nil {
				return result
			}
		}
	}
	return string(data)
}
