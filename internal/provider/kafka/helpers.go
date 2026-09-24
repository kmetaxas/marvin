package kafka

import (
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// RequireInt32 extracts a required int32 parameter from the params map.
func RequireInt32(params map[string]any, key string) (int32, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}
	switch value := v.(type) {
	case int:
		return int32(value), nil
	case int32:
		return value, nil
	case int64:
		return int32(value), nil
	case float64:
		if value != float64(int(value)) {
			return 0, fmt.Errorf("parameter %s must be an integer", key)
		}
		return int32(value), nil
	default:
		return 0, fmt.Errorf("parameter %s must be an integer", key)
	}
}

// OptionalInt32 extracts an optional int32 parameter from the params map.
func OptionalInt32(params map[string]any, key string, defaultValue int32) (int32, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	switch value := v.(type) {
	case int:
		return int32(value), nil
	case int32:
		return value, nil
	case int64:
		return int32(value), nil
	case float64:
		if value != float64(int(value)) {
			return 0, fmt.Errorf("parameter %s must be an integer", key)
		}
		return int32(value), nil
	default:
		return 0, fmt.Errorf("parameter %s must be an integer", key)
	}
}

// OptionalInt32Slice extracts a []int32 from []any containing numbers.
func OptionalInt32Slice(params map[string]any, key string) ([]int32, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch value := v.(type) {
	case []int32:
		return value, nil
	case []int:
		out := make([]int32, len(value))
		for i, n := range value {
			out[i] = int32(n)
		}
		return out, nil
	case []any:
		out := make([]int32, len(value))
		for i, item := range value {
			switch n := item.(type) {
			case int:
				out[i] = int32(n)
			case int32:
				out[i] = n
			case int64:
				out[i] = int32(n)
			case float64:
				if n != float64(int(n)) {
					return nil, fmt.Errorf("parameter %s contains non-integer values", key)
				}
				out[i] = int32(n)
			default:
				return nil, fmt.Errorf("parameter %s must be an array of integers", key)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("parameter %s must be an array of integers", key)
	}
}

// NormalizeLimitKafka wraps common.NormalizeLimit with Kafka-specific max (1000).
func NormalizeLimitKafka(params map[string]any) (int, error) {
	limit, err := common.OptionalInt(params, "limit", 100)
	if err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, fmt.Errorf("parameter limit must be greater than 0")
	}
	if limit > 1000 {
		limit = 1000
	}
	return limit, nil
}

// brokersToInfo converts kadm broker details into the shared BrokerInfo shape,
// marking the controller broker when its ID matches.
func brokersToInfo(brokers kadm.BrokerDetails, controller int32) []BrokerInfo {
	out := make([]BrokerInfo, 0, len(brokers))
	for _, b := range brokers {
		info := BrokerInfo{
			ID:   b.NodeID,
			Host: b.Host,
			Port: b.Port,
		}
		if b.Rack != nil {
			info.Rack = *b.Rack
		}
		if controller >= 0 && b.NodeID == controller {
			info.IsController = true
		}
		out = append(out, info)
	}
	return out
}

// configToEntry converts a kadm config into the shared ConfigEntry shape,
// redacting sensitive values.
func configToEntry(c kadm.Config) ConfigEntry {
	e := ConfigEntry{
		Name:      c.Key,
		Value:     c.MaybeValue(),
		Source:    c.Source.String(),
		Sensitive: c.Sensitive,
		Default:   c.Source == kmsg.ConfigSourceDefaultConfig,
	}
	if c.Sensitive {
		e.Value = "[REDACTED]"
	}
	return e
}

// brokerDetailToInfo converts a kadm broker detail into the shared BrokerInfo
// shape.
func brokerDetailToInfo(b kadm.BrokerDetail) BrokerInfo {
	info := BrokerInfo{
		ID:   b.NodeID,
		Host: b.Host,
		Port: b.Port,
	}
	if b.Rack != nil {
		info.Rack = *b.Rack
	}
	return info
}

// memberAssignments extracts the topic/partition assignments for a single
// consumer group member. It returns a slice of maps, each with a "topic" and
// "partitions" key, and the total number of assigned partitions.
func memberAssignments(m kadm.DescribedGroupMember) ([]map[string]any, int) {
	assignments := make([]map[string]any, 0)
	total := 0
	if c, ok := m.Assigned.AsConsumer(); ok {
		for _, t := range c.Topics {
			assignments = append(assignments, map[string]any{
				"topic":      t.Topic,
				"partitions": t.Partitions,
			})
			total += len(t.Partitions)
		}
	}
	return assignments, total
}

// partitionToInfo converts a kadm partition detail into the shared
// PartitionInfo shape, marking offline partitions (no leader).
func partitionToInfo(p kadm.PartitionDetail) PartitionInfo {
	info := PartitionInfo{
		Topic:       p.Topic,
		Partition:   p.Partition,
		Leader:      p.Leader,
		Replicas:    p.Replicas,
		ISR:         p.ISR,
		LeaderEpoch: p.LeaderEpoch,
	}
	if p.Leader < 0 {
		info.Offline = true
	}
	return info
}

// optionalStringSlice extracts an optional []string parameter from the params
// map, accepting either []string or []any of strings.
func optionalStringSlice(params map[string]any, key string) ([]string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch value := v.(type) {
	case []string:
		return value, nil
	case []any:
		out := make([]string, len(value))
		for i, item := range value {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %s must be an array of strings", key)
			}
			out[i] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("parameter %s must be an array of strings", key)
	}
}
