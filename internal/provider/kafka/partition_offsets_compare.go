package kafka

import (
	"context"
	"fmt"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var _ task.Task = (*partitionOffsetsCompareTask)(nil)

type partitionOffsetsCompareTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionOffsetsCompareTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionOffsetsCompareTask) Name() string { return "kafka.partition.offsets.compare" }

func (t *partitionOffsetsCompareTask) JSONSchema() string { return partitionOffsetsCompareSchema }

func (t *partitionOffsetsCompareTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topic, err := common.RequireString(params, "topic")
	if err != nil {
		return common.TaskFailure(err)
	}
	partition, err := RequireInt32(params, "partition")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	details, err := t.currentClient().ListTopics(ctx, topic)
	if err != nil {
		return common.TaskFailure(err)
	}

	td, ok := details[topic]
	if !ok {
		return common.TaskFailure(fmt.Errorf("topic %s not found", topic))
	}
	if td.Err != nil {
		return common.TaskFailure(td.Err)
	}

	pd, ok := td.Partitions[partition]
	if !ok {
		return common.TaskFailure(fmt.Errorf("partition %d not found in topic %s", partition, topic))
	}

	// High-water mark (leader end offset) for the partition.
	endOffsets, err := t.currentClient().ListEndOffsets(ctx, topic)
	if err != nil {
		return common.TaskFailure(err)
	}
	hw, _ := endOffsets.Lookup(topic, partition)

	replicas := make([]map[string]any, 0, len(pd.Replicas))
	maxLag := int64(0)
	lagDetected := false

	for _, brokerID := range pd.Replicas {
		offset := perReplicaEndOffset(ctx, t.currentClient(), topic, partition, brokerID)
		entry := map[string]any{
			"broker_id":      brokerID,
			"log_end_offset": offset,
			"leader":         brokerID == pd.Leader,
		}
		if brokerID != pd.Leader {
			lag := hw.Offset - offset
			if lag > 0 {
				entry["lag"] = lag
				lagDetected = true
				if lag > maxLag {
					maxLag = lag
				}
			}
		}
		replicas = append(replicas, entry)
	}

	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i]["broker_id"].(int32) < replicas[j]["broker_id"].(int32)
	})

	return common.SuccessResult(map[string]any{
		"topic":                topic,
		"partition":            partition,
		"replicas":             replicas,
		"replica_lag_detected": lagDetected,
		"max_lag":              maxLag,
	}), nil
}

// perReplicaEndOffset issues a ListOffsets request targeting a specific
// replica broker and returns that replica's log end offset.
func perReplicaEndOffset(ctx context.Context, client KafkaClient, topic string, partition, brokerID int32) int64 {
	req := kmsg.NewPtrListOffsetsRequest()
	req.ReplicaID = brokerID
	rt := kmsg.NewListOffsetsRequestTopic()
	rt.Topic = topic
	rp := kmsg.NewListOffsetsRequestTopicPartition()
	rp.Partition = partition
	rp.Timestamp = -1
	rt.Partitions = append(rt.Partitions, rp)
	req.Topics = append(req.Topics, rt)

	shards := client.RequestSharded(ctx, req)
	for _, shard := range shards {
		if shard.Err != nil {
			continue
		}
		resp, ok := shard.Resp.(*kmsg.ListOffsetsResponse)
		if !ok {
			continue
		}
		for _, t := range resp.Topics {
			for _, p := range t.Partitions {
				if p.Partition == partition {
					return p.Offset
				}
			}
		}
	}
	return -1
}

const partitionOffsetsCompareSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Partition Offsets Compare Parameters",
  "description": "Compare high-water/end offsets across replicas for replication diagnosis.",
  "properties": {
    "topic": {
      "type": "string",
      "description": "Topic name."
    },
    "partition": {
      "type": "integer",
      "description": "Partition number."
    }
  },
  "required": ["topic", "partition"]
}`
