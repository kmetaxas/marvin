package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestTopicListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &topicListTask{}
	assert.Equal(t, "kafka.topic.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTopicListTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &topicListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestTopicListTaskFiltersInternal(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders":             {Topic: "orders", IsInternal: false, Partitions: map[int32]kadm.PartitionDetail{0: {}, 1: {}}},
				"__consumer_offsets": {Topic: "__consumer_offsets", IsInternal: true, Partitions: map[int32]kadm.PartitionDetail{0: {}}},
			}, nil
		},
	}
	task := &topicListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	topics := data["topics"].([]TopicSummary)
	require.Len(t, topics, 1)
	assert.Equal(t, "orders", topics[0].Name)
	assert.Equal(t, 2, topics[0].Partitions)
}

func TestTopicListTaskIncludeInternal(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders":             {Topic: "orders", IsInternal: false, Partitions: map[int32]kadm.PartitionDetail{0: {}}},
				"__consumer_offsets": {Topic: "__consumer_offsets", IsInternal: true, Partitions: map[int32]kadm.PartitionDetail{0: {}}},
			}, nil
		},
	}
	task := &topicListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"include_internal": true})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}

func TestTopicListTaskPatternFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders":   {Topic: "orders", Partitions: map[int32]kadm.PartitionDetail{0: {}}},
				"payments": {Topic: "payments", Partitions: map[int32]kadm.PartitionDetail{0: {}}},
			}, nil
		},
	}
	task := &topicListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"pattern": "order*"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	topics := data["topics"].([]TopicSummary)
	assert.Equal(t, "orders", topics[0].Name)
}

func TestTopicListTaskLimitTruncation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"a": {Topic: "a", Partitions: map[int32]kadm.PartitionDetail{0: {}}},
				"b": {Topic: "b", Partitions: map[int32]kadm.PartitionDetail{0: {}}},
				"c": {Topic: "c", Partitions: map[int32]kadm.PartitionDetail{0: {}}},
			}, nil
		},
	}
	task := &topicListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 2})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	assert.True(t, data["truncated"].(bool))
}
