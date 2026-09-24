package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionLeaderDistributionTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionLeaderDistributionTask{}
	assert.Equal(t, "kafka.partition.leader_distribution.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionLeaderDistributionTaskAggregates(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
						1: {Topic: "orders", Partition: 1, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
						2: {Topic: "orders", Partition: 2, Leader: 2, Replicas: []int32{2, 3, 1}, ISR: []int32{2, 3, 1}},
					},
				},
			}, nil
		},
	}
	task := &partitionLeaderDistributionTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 3, data["total_partitions"])
	distribution := data["distribution"].([]map[string]any)
	require.Len(t, distribution, 2)
	assert.Equal(t, int32(1), distribution[0]["broker_id"])
	assert.Equal(t, 2, distribution[0]["leader_count"])
	assert.Equal(t, int32(2), distribution[1]["broker_id"])
	assert.Equal(t, 1, distribution[1]["leader_count"])
}

func TestPartitionLeaderDistributionTaskEmpty(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{}, nil
		},
	}
	task := &partitionLeaderDistributionTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["total_partitions"])
	assert.False(t, data["imbalance_detected"].(bool))
}
