package kafka

import (
	"context"
	"fmt"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionReplicaDistributionTask)(nil)

type partitionReplicaDistributionTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionReplicaDistributionTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionReplicaDistributionTask) Name() string {
	return "kafka.partition.replica_distribution.get"
}

func (t *partitionReplicaDistributionTask) JSONSchema() string {
	return partitionReplicaDistributionSchema
}

func (t *partitionReplicaDistributionTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	topics, err := optionalStringSlice(params, "topics")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	details, err := t.currentClient().ListTopics(ctx, topics...)
	if err != nil {
		return common.TaskFailure(err)
	}

	replicaCounts := make(map[int32]int)
	for _, td := range details.Sorted() {
		for _, pd := range td.Partitions.Sorted() {
			for _, replica := range pd.Replicas {
				replicaCounts[replica]++
			}
		}
	}

	brokerIDs := make([]int32, 0, len(replicaCounts))
	for id := range replicaCounts {
		brokerIDs = append(brokerIDs, id)
	}
	sort.Slice(brokerIDs, func(i, j int) bool { return brokerIDs[i] < brokerIDs[j] })

	byBroker := make([]map[string]any, 0, len(brokerIDs))
	for _, id := range brokerIDs {
		byBroker = append(byBroker, map[string]any{
			"broker_id":     id,
			"replica_count": replicaCounts[id],
		})
	}

	return common.SuccessResult(map[string]any{
		"by_broker": byBroker,
		"by_rack":   []map[string]any{},
	}), nil
}

const partitionReplicaDistributionSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Replica Distribution Get Parameters",
  "description": "Get replica distribution across brokers/racks.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific topics. Omit for all."
    }
  }
}`
