package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*clusterMetadataGetTask)(nil)

type clusterMetadataGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *clusterMetadataGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *clusterMetadataGetTask) Name() string { return "kafka.cluster.metadata.get" }

func (t *clusterMetadataGetTask) JSONSchema() string { return clusterMetadataGetSchema }

func (t *clusterMetadataGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	includeInternal, err := common.OptionalBool(params, "include_internal", false)
	if err != nil {
		return common.TaskFailure(err)
	}
	maxPartitions, err := OptionalInt32(params, "max_partitions", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	guardrails := t.currentClient().Guardrails()
	guardrails.ApplyDefaults()

	meta, err := t.currentClient().Metadata(ctx, topics...)
	if err != nil {
		return common.TaskFailure(err)
	}

	brokers := brokersToInfo(meta.Brokers, meta.Controller)

	topicList := make([]map[string]any, 0)
	partitionCount := 0
	truncated := false

	for _, td := range meta.Topics.Sorted() {
		if td.IsInternal && !includeInternal {
			continue
		}
		if len(topicList) >= guardrails.MaxTopicsPerRequest {
			truncated = true
			break
		}

		partitions := make([]PartitionInfo, 0)
		for _, pd := range td.Partitions.Sorted() {
			if maxPartitions > 0 && int32(len(partitions)) >= maxPartitions {
				truncated = true
				break
			}
			if len(partitions) >= guardrails.MaxPartitionsPerRequest {
				truncated = true
				break
			}
			offline := len(pd.OfflineReplicas) > 0
			partitions = append(partitions, PartitionInfo{
				Topic:       pd.Topic,
				Partition:   pd.Partition,
				Leader:      pd.Leader,
				Replicas:    pd.Replicas,
				ISR:         pd.ISR,
				LeaderEpoch: pd.LeaderEpoch,
				Offline:     offline,
			})
		}
		partitionCount += len(partitions)

		topicList = append(topicList, map[string]any{
			"topic":      td.Topic,
			"internal":   td.IsInternal,
			"partitions": partitions,
		})
	}

	return common.SuccessResult(map[string]any{
		"cluster_id":      meta.Cluster,
		"brokers":         brokers,
		"topics":          topicList,
		"topic_count":     len(topicList),
		"partition_count": partitionCount,
		"truncated":       truncated,
	}), nil
}

const clusterMetadataGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Cluster Metadata Get Parameters",
  "description": "Return raw-ish cluster metadata snapshot.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Specific topics to include. Omit for all."
    },
    "include_internal": {
      "type": "boolean",
      "default": false,
      "description": "Include internal topics (__consumer_offsets, etc.)."
    },
    "max_partitions": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10000,
      "description": "Maximum partitions to return per topic."
    }
  }
}`
