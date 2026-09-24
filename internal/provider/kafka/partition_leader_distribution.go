package kafka

import (
	"context"
	"fmt"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*partitionLeaderDistributionTask)(nil)

type partitionLeaderDistributionTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *partitionLeaderDistributionTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *partitionLeaderDistributionTask) Name() string {
	return "kafka.partition.leader_distribution.get"
}

func (t *partitionLeaderDistributionTask) JSONSchema() string {
	return partitionLeaderDistributionSchema
}

func (t *partitionLeaderDistributionTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	leaderCounts := make(map[int32]int)
	total := 0
	for _, td := range details.Sorted() {
		for _, pd := range td.Partitions.Sorted() {
			if pd.Leader < 0 {
				continue
			}
			leaderCounts[pd.Leader]++
			total++
		}
	}

	brokerIDs := make([]int32, 0, len(leaderCounts))
	for id := range leaderCounts {
		brokerIDs = append(brokerIDs, id)
	}
	sort.Slice(brokerIDs, func(i, j int) bool { return brokerIDs[i] < brokerIDs[j] })

	distribution := make([]map[string]any, 0, len(brokerIDs))
	for _, id := range brokerIDs {
		count := leaderCounts[id]
		percentage := 0.0
		if total > 0 {
			percentage = float64(count) * 100.0 / float64(total)
		}
		distribution = append(distribution, map[string]any{
			"broker_id":    id,
			"leader_count": count,
			"percentage":   percentage,
		})
	}

	imbalance := false
	if len(brokerIDs) > 1 && total > 0 {
		expected := float64(total) / float64(len(brokerIDs))
		for _, id := range brokerIDs {
			diff := float64(leaderCounts[id]) - expected
			if diff < 0 {
				diff = -diff
			}
			if diff/expected > 0.2 {
				imbalance = true
				break
			}
		}
	}

	return common.SuccessResult(map[string]any{
		"distribution":       distribution,
		"total_partitions":   total,
		"imbalance_detected": imbalance,
	}), nil
}

const partitionLeaderDistributionSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Leader Distribution Get Parameters",
  "description": "Get leader distribution across brokers.",
  "properties": {
    "topics": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Filter to specific topics. Omit for all."
    }
  }
}`
