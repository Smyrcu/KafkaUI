package kafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

type Client struct {
	raw    *kgo.Client
	admin  *kadm.Client
	name   string
	config config.ClusterConfig
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

type MessageRecord struct {
	Partition int32             `json:"partition"`
	Offset    int64             `json:"offset"`
	Timestamp time.Time         `json:"timestamp"`
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type ConsumeRequest struct {
	Partition *int32     // nil = all partitions
	Offset    int64      // -1 = latest, -2 = earliest
	Timestamp *time.Time // if set, overrides Offset
	Limit     int
}

type ProduceRequest struct {
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Partition *int32            `json:"partition,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

func NewClient(cfg config.ClusterConfig) (*Client, error) {
	seeds := strings.Split(cfg.BootstrapServers, ",")
	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
	}

	if cfg.SASL.Mechanism != "" {
		saslOpt, err := BuildSASLOpt(cfg.SASL)
		if err != nil {
			return nil, fmt.Errorf("configuring SASL: %w", err)
		}
		opts = append(opts, saslOpt)
	}

	if cfg.TLS.Enabled {
		tlsOpt, err := BuildTLSOpt(cfg.TLS)
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
		raw:    raw,
		admin:  kadm.NewClient(raw),
		name:   cfg.Name,
		config: cfg,
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

func (c *Client) Config() config.ClusterConfig {
	return c.config
}

func (c *Client) newConsumerOpts() ([]kgo.Opt, error) {
	seeds := strings.Split(c.config.BootstrapServers, ",")
	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
	}
	if c.config.SASL.Mechanism != "" {
		saslOpt, err := BuildSASLOpt(c.config.SASL)
		if err != nil {
			return nil, fmt.Errorf("configuring SASL for consumer: %w", err)
		}
		opts = append(opts, saslOpt)
	}
	if c.config.TLS.Enabled {
		tlsOpt, err := BuildTLSOpt(c.config.TLS)
		if err != nil {
			return nil, fmt.Errorf("configuring TLS for consumer: %w", err)
		}
		opts = append(opts, tlsOpt)
	}
	return opts, nil
}

func (c *Client) ConsumeMessages(ctx context.Context, topic string, req ConsumeRequest) ([]MessageRecord, error) {
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 500 {
		req.Limit = 500
	}

	topics, err := c.admin.ListTopics(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("listing topic: %w", err)
	}
	t, ok := topics[topic]
	if !ok {
		return nil, fmt.Errorf("topic %q not found", topic)
	}

	topicOffsets := make(map[int32]kgo.Offset)
	for _, p := range t.Partitions.Sorted() {
		if req.Partition != nil && p.Partition != *req.Partition {
			continue
		}
		switch req.Offset {
		case -1:
			topicOffsets[p.Partition] = kgo.NewOffset().AtEnd()
		case -2:
			topicOffsets[p.Partition] = kgo.NewOffset().AtStart()
		default:
			topicOffsets[p.Partition] = kgo.NewOffset().At(req.Offset)
		}
	}

	if req.Timestamp != nil {
		ts := req.Timestamp.UnixMilli()
		listedOffsets, err := c.admin.ListOffsetsAfterMilli(ctx, ts, topic)
		if err != nil {
			return nil, fmt.Errorf("listing offsets for timestamp: %w", err)
		}
		topicOffsets = make(map[int32]kgo.Offset)
		listedOffsets.Each(func(lo kadm.ListedOffset) {
			if req.Partition != nil && lo.Partition != *req.Partition {
				return
			}
			topicOffsets[lo.Partition] = kgo.NewOffset().At(lo.Offset)
		})
	}

	if len(topicOffsets) == 0 {
		return []MessageRecord{}, nil
	}

	offsets := map[string]map[int32]kgo.Offset{topic: topicOffsets}

	opts, err := c.newConsumerOpts()
	if err != nil {
		return nil, err
	}
	opts = append(opts, kgo.ConsumePartitions(offsets))

	consumer, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating consumer: %w", err)
	}
	defer consumer.Close()

	var records []MessageRecord
	for len(records) < req.Limit {
		fetches := consumer.PollFetches(ctx)
		if ctx.Err() != nil {
			break
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			if ctx.Err() != nil {
				break
			}
			return records, fmt.Errorf("consuming: %v", errs[0].Err)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			if len(records) >= req.Limit {
				return
			}
			headers := make(map[string]string)
			for _, h := range r.Headers {
				headers[h.Key] = string(h.Value)
			}
			records = append(records, MessageRecord{
				Partition: r.Partition,
				Offset:    r.Offset,
				Timestamp: r.Timestamp,
				Key:       string(r.Key),
				Value:     string(r.Value),
				Headers:   headers,
			})
		})
	}

	return records, nil
}

func (c *Client) ProduceMessage(ctx context.Context, topic string, req ProduceRequest) (*MessageRecord, error) {
	r := &kgo.Record{
		Topic: topic,
		Key:   []byte(req.Key),
		Value: []byte(req.Value),
	}
	if req.Partition != nil {
		r.Partition = *req.Partition
	} else {
		r.Partition = -1
	}
	for k, v := range req.Headers {
		r.Headers = append(r.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}

	var produceErr error
	c.raw.Produce(ctx, r, func(r *kgo.Record, err error) {
		produceErr = err
	})
	if err := c.raw.Flush(ctx); err != nil {
		return nil, fmt.Errorf("flushing produce: %w", err)
	}
	if produceErr != nil {
		return nil, fmt.Errorf("producing message: %w", produceErr)
	}

	headers := make(map[string]string)
	for _, h := range r.Headers {
		headers[h.Key] = string(h.Value)
	}
	return &MessageRecord{
		Partition: r.Partition,
		Offset:    r.Offset,
		Timestamp: r.Timestamp,
		Key:       string(r.Key),
		Value:     string(r.Value),
		Headers:   headers,
	}, nil
}
