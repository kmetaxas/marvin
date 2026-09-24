package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionOfflineListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionOfflineListTask{}
	assert.Equal(t, "kafka.partition.offline.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionOfflineListTaskFiltersLeaderMinusOne(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
						5: {Topic: "orders", Partition: 5, Leader: -1, Replicas: []int32{3, 1, 2}, ISR: []int32{}},
					},
				},
			}, nil
		},
	}
	task := &partitionOfflineListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	offline := data["offline_partitions"].([]PartitionInfo)
	require.Len(t, offline, 1)
	assert.Equal(t, int32(5), offline[0].Partition)
	assert.True(t, offline[0].Offline)
}

func TestPartitionOfflineListTaskNone(t *testing.T) {
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
	task := &partitionOfflineListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
}
