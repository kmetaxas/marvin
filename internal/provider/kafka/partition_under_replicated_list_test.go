package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionUnderReplicatedListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionUnderReplicatedListTask{}
	assert.Equal(t, "kafka.partition.under_replicated.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionUnderReplicatedListTaskFilters(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
						1: {Topic: "orders", Partition: 1, Leader: 2, Replicas: []int32{2, 3, 1}, ISR: []int32{2, 3}},
					},
				},
			}, nil
		},
	}
	task := &partitionUnderReplicatedListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	partitions := data["under_replicated_partitions"].([]PartitionInfo)
	require.Len(t, partitions, 1)
	assert.Equal(t, int32(1), partitions[0].Partition)
}

func TestPartitionUnderReplicatedListTaskNone(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
					},
				},
			}, nil
		},
	}
	task := &partitionUnderReplicatedListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
}
