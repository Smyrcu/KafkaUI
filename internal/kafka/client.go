package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

type Client struct {
	raw   *kgo.Client
	admin *kadm.Client
	name  string
}

type BrokerInfo struct {
	ID   int32  `json:"id"`
	Host string `json:"host"`
	Port int32  `json:"port"`
	Rack string `json:"rack,omitempty"`
}

type TopicInfo struct {
	Name       string            `json:"name"`
	Partitions int               `json:"partitions"`
	Replicas   int               `json:"replicas"`
	Internal   bool              `json:"internal"`
	Configs    map[string]string `json:"configs,omitempty"`
}

type TopicDetail struct {
	Name       string            `json:"name"`
	Partitions []PartitionInfo   `json:"partitions"`
	Configs    map[string]string `json:"configs"`
	Internal   bool              `json:"internal"`
}

type PartitionInfo struct {
	ID       int32   `json:"id"`
	Leader   int32   `json:"leader"`
	Replicas []int32 `json:"replicas"`
	ISR      []int32 `json:"isr"`
}

type CreateTopicRequest struct {
	Name       string `json:"name"`
	Partitions int32  `json:"partitions"`
	Replicas   int16  `json:"replicas"`
}

func NewClient(cfg config.ClusterConfig) (*Client, error) {
	seeds := strings.Split(cfg.BootstrapServers, ",")
	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
	}

	if cfg.SASL.Mechanism != "" {
		saslOpt, err := buildSASLOpt(cfg.SASL)
		if err != nil {
			return nil, fmt.Errorf("configuring SASL: %w", err)
		}
		opts = append(opts, saslOpt)
	}

	if cfg.TLS.Enabled {
		tlsOpt, err := buildTLSOpt(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("configuring TLS: %w", err)
		}
		opts = append(opts, tlsOpt)
	}

	raw, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating kafka client: %w", err)
	}

	return &Client{
		raw:   raw,
		admin: kadm.NewClient(raw),
		name:  cfg.Name,
	}, nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Close() {
	c.raw.Close()
}

func (c *Client) Brokers(ctx context.Context) ([]BrokerInfo, error) {
	meta, err := c.admin.BrokerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching broker metadata: %w", err)
	}

	brokers := make([]BrokerInfo, 0, len(meta.Brokers))
	for _, b := range meta.Brokers {
		rack := ""
		if b.Rack != nil {
			rack = *b.Rack
		}
		brokers = append(brokers, BrokerInfo{
			ID:   b.NodeID,
			Host: b.Host,
			Port: int32(b.Port),
			Rack: rack,
		})
	}
	return brokers, nil
}

func (c *Client) Topics(ctx context.Context) ([]TopicInfo, error) {
	topics, err := c.admin.ListTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}

	result := make([]TopicInfo, 0, len(topics))
	for _, t := range topics.Sorted() {
		replicas := 0
		if len(t.Partitions) > 0 {
			replicas = len(t.Partitions[0].Replicas)
		}
		result = append(result, TopicInfo{
			Name:       t.Topic,
			Partitions: len(t.Partitions),
			Replicas:   replicas,
			Internal:   t.IsInternal,
		})
	}
	return result, nil
}

func (c *Client) TopicDetails(ctx context.Context, name string) (*TopicDetail, error) {
	topics, err := c.admin.ListTopics(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("describing topic: %w", err)
	}

	t, ok := topics[name]
	if !ok {
		return nil, fmt.Errorf("topic %q not found", name)
	}

	partitions := make([]PartitionInfo, 0, len(t.Partitions))
	for _, p := range t.Partitions.Sorted() {
		partitions = append(partitions, PartitionInfo{
			ID:       p.Partition,
			Leader:   p.Leader,
			Replicas: p.Replicas,
			ISR:      p.ISR,
		})
	}

	configs := make(map[string]string)
	rc, err := c.admin.DescribeTopicConfigs(ctx, name)
	if err == nil {
		for _, r := range rc {
			for _, cv := range r.Configs {
				if cv.Value != nil {
					configs[cv.Key] = *cv.Value
				}
			}
		}
	}

	return &TopicDetail{
		Name:       t.Topic,
		Partitions: partitions,
		Configs:    configs,
		Internal:   t.IsInternal,
	}, nil
}

func (c *Client) CreateTopic(ctx context.Context, req CreateTopicRequest) error {
	resp, err := c.admin.CreateTopics(ctx, int32(req.Partitions), req.Replicas, nil, req.Name)
	if err != nil {
		return fmt.Errorf("creating topic: %w", err)
	}
	for _, t := range resp.Sorted() {
		if t.Err != nil {
			return fmt.Errorf("creating topic %q: %w", t.Topic, t.Err)
		}
	}
	return nil
}

func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	resp, err := c.admin.DeleteTopics(ctx, name)
	if err != nil {
		return fmt.Errorf("deleting topic: %w", err)
	}
	for _, t := range resp.Sorted() {
		if t.Err != nil {
			return fmt.Errorf("deleting topic %q: %w", t.Topic, t.Err)
		}
	}
	return nil
}
