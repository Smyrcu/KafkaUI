package kafka

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	Name       string            `json:"name"`
	Partitions int32             `json:"partitions"`
	Replicas   int16             `json:"replicas"`
	Configs    map[string]string `json:"configs,omitempty"`
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

type ConsumerGroupInfo struct {
	Name          string `json:"name"`
	State         string `json:"state"`
	Members       int    `json:"members"`
	TopicCount    int    `json:"topics"`
	CoordinatorID int32  `json:"coordinatorId"`
}

type ConsumerGroupDetail struct {
	Name          string                     `json:"name"`
	State         string                     `json:"state"`
	CoordinatorID int32                      `json:"coordinatorId"`
	Members       []ConsumerGroupMember      `json:"members"`
	Offsets       []ConsumerGroupTopicOffset `json:"offsets"`
}

type ConsumerGroupMember struct {
	ID       string   `json:"id"`
	ClientID string   `json:"clientId"`
	Host     string   `json:"host"`
	Topics   []string `json:"topics"`
}

type ConsumerGroupTopicOffset struct {
	Topic      string                         `json:"topic"`
	Partitions []ConsumerGroupPartitionOffset `json:"partitions"`
	TotalLag   int64                          `json:"totalLag"`
}

type ConsumerGroupPartitionOffset struct {
	Partition     int32 `json:"partition"`
	CurrentOffset int64 `json:"currentOffset"`
	EndOffset     int64 `json:"endOffset"`
	Lag           int64 `json:"lag"`
}

type ResetOffsetsRequest struct {
	Topic   string `json:"topic"`
	ResetTo string `json:"resetTo"`
}

func NewClient(cfg config.ClusterConfig) (*Client, error) {
	opts, err := BuildBaseOpts(cfg)
	if err != nil {
		return nil, err
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
	var configs map[string]*string
	if len(req.Configs) > 0 {
		configs = make(map[string]*string, len(req.Configs))
		for k, v := range req.Configs {
			v := v
			configs[k] = &v
		}
	}
	resp, err := c.admin.CreateTopics(ctx, int32(req.Partitions), req.Replicas, configs, req.Name)
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

// BuildBaseOpts constructs the common kgo.Opt slice (seed brokers, SASL, TLS)
// from a ClusterConfig. Both the main client constructor and any ad-hoc
// consumers (e.g. live tail, message browse) should use this to avoid
// duplicating auth/TLS wiring.
func BuildBaseOpts(cfg config.ClusterConfig) ([]kgo.Opt, error) {
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
	return opts, nil
}

func (c *Client) newConsumerOpts() ([]kgo.Opt, error) {
	return BuildBaseOpts(c.config)
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

	// For "latest" (-1), fetch high watermarks and start limit messages before the end.
	if req.Offset == -1 {
		endOffsets, err := c.admin.ListEndOffsets(ctx, topic)
		if err != nil {
			return nil, fmt.Errorf("listing end offsets: %w", err)
		}
		startOffsets, err := c.admin.ListStartOffsets(ctx, topic)
		if err != nil {
			return nil, fmt.Errorf("listing start offsets: %w", err)
		}
		endOffsets.Each(func(eo kadm.ListedOffset) {
			if req.Partition != nil && eo.Partition != *req.Partition {
				return
			}
			start := eo.Offset - int64(req.Limit)
			// Don't go before the partition's start offset.
			startOffsets.Each(func(so kadm.ListedOffset) {
				if so.Partition == eo.Partition && start < so.Offset {
					start = so.Offset
				}
			})
			if start < 0 {
				start = 0
			}
			topicOffsets[eo.Partition] = kgo.NewOffset().At(start)
		})
	} else {
		for _, p := range t.Partitions.Sorted() {
			if req.Partition != nil && p.Partition != *req.Partition {
				continue
			}
			switch req.Offset {
			case -2:
				topicOffsets[p.Partition] = kgo.NewOffset().AtStart()
			default:
				topicOffsets[p.Partition] = kgo.NewOffset().At(req.Offset)
			}
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
		pollCtx, pollCancel := context.WithTimeout(ctx, 2*time.Second)
		fetches := consumer.PollFetches(pollCtx)
		pollCancel()
		if ctx.Err() != nil {
			break
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			// Ignore poll timeout errors — they just mean no new data yet.
			if ctx.Err() != nil || errors.Is(errs[0].Err, context.DeadlineExceeded) {
				break
			}
			return records, fmt.Errorf("consuming: %v", errs[0].Err)
		}
		countBefore := len(records)
		fetches.EachRecord(func(r *kgo.Record) {
			if len(records) >= req.Limit {
				return
			}
			records = append(records, MessageRecord{
				Partition: r.Partition,
				Offset:    r.Offset,
				Timestamp: r.Timestamp,
				Key:       string(r.Key),
				Value:     string(r.Value),
				Headers:   recordHeaders(r),
			})
		})
		// No new messages — topic exhausted, stop waiting.
		if len(records) == countBefore {
			break
		}
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

	return &MessageRecord{
		Partition: r.Partition,
		Offset:    r.Offset,
		Timestamp: r.Timestamp,
		Key:       string(r.Key),
		Value:     string(r.Value),
		Headers:   recordHeaders(r),
	}, nil
}

func (c *Client) ConsumerGroups(ctx context.Context) ([]ConsumerGroupInfo, error) {
	listed, err := c.admin.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing consumer groups: %w", err)
	}

	groupNames := listed.Groups()
	if len(groupNames) == 0 {
		return []ConsumerGroupInfo{}, nil
	}

	described, err := c.admin.DescribeGroups(ctx, groupNames...)
	if err != nil {
		return nil, fmt.Errorf("describing consumer groups: %w", err)
	}

	result := make([]ConsumerGroupInfo, 0, len(described.Sorted()))
	for _, g := range described.Sorted() {
		assignedTopics := g.AssignedPartitions()
		result = append(result, ConsumerGroupInfo{
			Name:          g.Group,
			State:         g.State,
			Members:       len(g.Members),
			TopicCount:    len(assignedTopics),
			CoordinatorID: g.Coordinator.NodeID,
		})
	}

	return result, nil
}

func (c *Client) ConsumerGroupDetails(ctx context.Context, name string) (*ConsumerGroupDetail, error) {
	described, err := c.admin.DescribeGroups(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("describing consumer group: %w", err)
	}

	sorted := described.Sorted()
	if len(sorted) == 0 {
		return nil, fmt.Errorf("consumer group %q not found", name)
	}
	g := sorted[0]

	members := make([]ConsumerGroupMember, 0, len(g.Members))
	for _, m := range g.Members {
		var memberTopics []string
		if ca, ok := m.Assigned.AsConsumer(); ok {
			for _, t := range ca.Topics {
				memberTopics = append(memberTopics, t.Topic)
			}
		}
		members = append(members, ConsumerGroupMember{
			ID:       m.MemberID,
			ClientID: m.ClientID,
			Host:     m.ClientHost,
			Topics:   memberTopics,
		})
	}

	offsetResp, err := c.admin.FetchOffsets(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("fetching offsets for group %q: %w", name, err)
	}

	// Collect all topics that have committed offsets.
	topicSet := make(map[string]struct{})
	offsetResp.Each(func(o kadm.OffsetResponse) {
		topicSet[o.Topic] = struct{}{}
	})

	topicNames := make([]string, 0, len(topicSet))
	for t := range topicSet {
		topicNames = append(topicNames, t)
	}

	var endOffsets kadm.ListedOffsets
	if len(topicNames) > 0 {
		endOffsets, err = c.admin.ListEndOffsets(ctx, topicNames...)
		if err != nil {
			return nil, fmt.Errorf("listing end offsets: %w", err)
		}
	}

	// Build per-topic offset information.
	type partInfo struct {
		partition     int32
		currentOffset int64
		endOffset     int64
		lag           int64
	}
	topicPartitions := make(map[string][]partInfo)

	offsetResp.Each(func(o kadm.OffsetResponse) {
		end := int64(0)
		if lo, ok := endOffsets.Lookup(o.Topic, o.Partition); ok {
			end = lo.Offset
		}
		lag := end - o.Offset.At
		if lag < 0 {
			lag = 0
		}
		topicPartitions[o.Topic] = append(topicPartitions[o.Topic], partInfo{
			partition:     o.Partition,
			currentOffset: o.Offset.At,
			endOffset:     end,
			lag:           lag,
		})
	})

	offsets := make([]ConsumerGroupTopicOffset, 0, len(topicPartitions))
	for topic, parts := range topicPartitions {
		sort.Slice(parts, func(i, j int) bool {
			return parts[i].partition < parts[j].partition
		})
		var totalLag int64
		partitions := make([]ConsumerGroupPartitionOffset, 0, len(parts))
		for _, p := range parts {
			totalLag += p.lag
			partitions = append(partitions, ConsumerGroupPartitionOffset{
				Partition:     p.partition,
				CurrentOffset: p.currentOffset,
				EndOffset:     p.endOffset,
				Lag:           p.lag,
			})
		}
		offsets = append(offsets, ConsumerGroupTopicOffset{
			Topic:      topic,
			Partitions: partitions,
			TotalLag:   totalLag,
		})
	}

	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i].Topic < offsets[j].Topic
	})

	return &ConsumerGroupDetail{
		Name:          g.Group,
		State:         g.State,
		CoordinatorID: g.Coordinator.NodeID,
		Members:       members,
		Offsets:       offsets,
	}, nil
}

func (c *Client) ResetConsumerGroupOffsets(ctx context.Context, group string, req ResetOffsetsRequest) error {
	// Verify the group is Empty — active consumers will overwrite committed offsets.
	described, err := c.admin.DescribeGroups(ctx, group)
	if err != nil {
		return fmt.Errorf("describing group %q: %w", group, err)
	}
	sorted := described.Sorted()
	if len(sorted) == 0 {
		return fmt.Errorf("consumer group %q not found", group)
	}
	state := sorted[0].State
	if state != "Empty" {
		return fmt.Errorf("consumer group %q is in state %q — must be Empty to reset offsets (stop all consumers first)", group, state)
	}

	var targetOffsets kadm.ListedOffsets

	switch req.ResetTo {
	case "earliest":
		targetOffsets, err = c.admin.ListStartOffsets(ctx, req.Topic)
	case "latest":
		targetOffsets, err = c.admin.ListEndOffsets(ctx, req.Topic)
	default:
		return fmt.Errorf("unsupported resetTo value: %q (must be \"earliest\" or \"latest\")", req.ResetTo)
	}
	if err != nil {
		return fmt.Errorf("listing %s offsets for topic %q: %w", req.ResetTo, req.Topic, err)
	}

	offsets := targetOffsets.Offsets()

	_, err = c.admin.CommitOffsets(ctx, group, offsets)
	if err != nil {
		return fmt.Errorf("committing offsets for group %q: %w", group, err)
	}

	return nil
}
