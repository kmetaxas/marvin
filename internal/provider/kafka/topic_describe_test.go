package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestTopicDescribeTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &topicDescribeTask{}
	assert.Equal(t, "kafka.topic.describe", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTopicDescribeTaskMissingTopic(t *testing.T) {
	t.Parallel()
	task := &topicDescribeTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "topic")
}

func TestTopicDescribeTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic:      "orders",
					IsInternal: false,
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
						1: {Topic: "orders", Partition: 1, Leader: 2, Replicas: []int32{2, 3, 1}, ISR: []int32{2, 3, 1}},
					},
				},
			}, nil
		},
	}
	task := &topicDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "orders", data["topic"])
	assert.Equal(t, 2, data["partition_count"])
	assert.Equal(t, 3, data["replication_factor"])
	partitions := data["partitions"].([]PartitionInfo)
	require.Len(t, partitions, 2)
	assert.Equal(t, int32(1), partitions[0].Leader)
}

func TestTopicDescribeTaskNotFound(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{}, nil
		},
	}
	task := &topicDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not found")
}
