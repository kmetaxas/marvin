package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionReplicaDistributionTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionReplicaDistributionTask{}
	assert.Equal(t, "kafka.partition.replica_distribution.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionReplicaDistributionTaskAggregates(t *testing.T) {
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
	task := &partitionReplicaDistributionTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	byBroker := data["by_broker"].([]map[string]any)
	require.Len(t, byBroker, 3)
	assert.Equal(t, int32(1), byBroker[0]["broker_id"])
	assert.Equal(t, 2, byBroker[0]["replica_count"])
	assert.Equal(t, int32(2), byBroker[1]["broker_id"])
	assert.Equal(t, 2, byBroker[1]["replica_count"])
	assert.Equal(t, int32(3), byBroker[2]["broker_id"])
	assert.Equal(t, 2, byBroker[2]["replica_count"])
}

func TestPartitionReplicaDistributionTaskEmpty(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{}, nil
		},
	}
	task := &partitionReplicaDistributionTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	byBroker := data["by_broker"].([]map[string]any)
	assert.Empty(t, byBroker)
}
