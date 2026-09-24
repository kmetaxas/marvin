package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionISRDescribeTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionISRDescribeTask{}
	assert.Equal(t, "kafka.partition.isr.describe", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionISRDescribeTaskMissingTopic(t *testing.T) {
	t.Parallel()
	task := &partitionISRDescribeTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "topic")
}

func TestPartitionISRDescribeTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, LeaderEpoch: 42, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2}, OfflineReplicas: []int32{3}},
					},
				},
			}, nil
		},
	}
	task := &partitionISRDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	partitions := data["partitions"].([]map[string]any)
	require.Len(t, partitions, 1)
	assert.Equal(t, int32(0), partitions[0]["partition"])
	assert.Equal(t, []int32{1, 2}, partitions[0]["isr"])
	assert.Equal(t, []int32{3}, partitions[0]["offline_replicas"])
	assert.Equal(t, int32(42), partitions[0]["leader_epoch"])
}

func TestPartitionISRDescribeTaskPartitionFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
						1: {Topic: "orders", Partition: 1, Leader: 2, Replicas: []int32{2, 3, 1}, ISR: []int32{2, 3, 1}},
					},
				},
			}, nil
		},
	}
	task := &partitionISRDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "partitions": []any{1}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	partitions := data["partitions"].([]map[string]any)
	assert.Equal(t, int32(1), partitions[0]["partition"])
}
