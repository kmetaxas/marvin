package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestStorageTopicSizesTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &storageTopicSizesTask{}
	assert.Equal(t, "kafka.storage.topic_sizes.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestStorageTopicSizesTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &storageTopicSizesTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestStorageTopicSizesTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}, 1: {Size: 200}},
					}},
				},
				2: {
					"/var/lib/kafka/data-1": {Broker: 2, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 400}},
						"users":  {0: {Size: 50}},
					}},
				},
			}, nil
		},
	}
	task := &storageTopicSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])

	topicSizes := data["topic_sizes"].([]map[string]any)
	require.Len(t, topicSizes, 2)

	// Sorted by topic name: "orders" then "users".
	orders := topicSizes[0]
	assert.Equal(t, "orders", orders["topic"])
	assert.Equal(t, int64(700), orders["total_size_bytes"])
	assert.Equal(t, 3, orders["partition_count"])

	byBroker := orders["by_broker"].([]map[string]any)
	require.Len(t, byBroker, 2)
	assert.Equal(t, int32(1), byBroker[0]["broker_id"])
	assert.Equal(t, int64(300), byBroker[0]["size_bytes"])
	assert.Equal(t, int32(2), byBroker[1]["broker_id"])
	assert.Equal(t, int64(400), byBroker[1]["size_bytes"])

	users := topicSizes[1]
	assert.Equal(t, "users", users["topic"])
	assert.Equal(t, int64(50), users["total_size_bytes"])
	assert.Equal(t, 1, users["partition_count"])
}

func TestStorageTopicSizesTaskTopicFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}},
						"users":  {0: {Size: 200}},
					}},
				},
			}, nil
		},
	}
	task := &storageTopicSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topics": []any{"orders"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	topicSizes := data["topic_sizes"].([]map[string]any)
	require.Len(t, topicSizes, 1)
	assert.Equal(t, "orders", topicSizes[0]["topic"])
}

func TestStorageTopicSizesTaskBrokerFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}},
					}},
				},
				2: {
					"/var/lib/kafka/data-1": {Broker: 2, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 400}},
					}},
				},
			}, nil
		},
	}
	task := &storageTopicSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_ids": []any{2}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	topicSizes := data["topic_sizes"].([]map[string]any)
	require.Len(t, topicSizes, 1)
	assert.Equal(t, int64(400), topicSizes[0]["total_size_bytes"])
	byBroker := topicSizes[0]["by_broker"].([]map[string]any)
	require.Len(t, byBroker, 1)
	assert.Equal(t, int32(2), byBroker[0]["broker_id"])
}

func TestStorageTopicSizesTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &storageTopicSizesTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
