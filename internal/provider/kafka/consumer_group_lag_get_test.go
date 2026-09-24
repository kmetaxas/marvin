package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestConsumerGroupLagGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupLagGetTask{}
	assert.Equal(t, "kafka.consumer_group.lag.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupLagGetTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupLagGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")

	res, err = task.Execute(context.Background(), map[string]any{"group_id": "g", "aggregate": "bogus"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "aggregate")
}

func testGroupLag() kadm.DescribedGroupLags {
	return kadm.DescribedGroupLags{
		"order-processor": {
			Group: "order-processor",
			Lag: kadm.GroupLag{
				"orders": {
					0: {Topic: "orders", Partition: 0, Commit: kadm.Offset{At: 100}, End: kadm.ListedOffset{Offset: 200}, Lag: 100},
					1: {Topic: "orders", Partition: 1, Commit: kadm.Offset{At: 50}, End: kadm.ListedOffset{Offset: 60}, Lag: 10},
				},
				"payments": {
					0: {Topic: "payments", Partition: 0, Commit: kadm.Offset{At: 0}, End: kadm.ListedOffset{Offset: 5}, Lag: 5},
				},
			},
		},
	}
}

func TestConsumerGroupLagGetTaskNone(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		lagFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error) {
			return testGroupLag(), nil
		},
	}
	task := &consumerGroupLagGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 3, data["count"])
	lag := data["lag"].([]map[string]any)
	require.Len(t, lag, 3)
	assert.Equal(t, "orders", lag[0]["topic"])
	assert.Equal(t, int64(100), lag[0]["lag"])
}

func TestConsumerGroupLagGetTaskTopic(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		lagFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error) {
			return testGroupLag(), nil
		},
	}
	task := &consumerGroupLagGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor", "aggregate": "topic"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	byTopic := data["lag_by_topic"].([]map[string]any)
	require.Len(t, byTopic, 2)

	orders := byTopic[0]
	assert.Equal(t, "orders", orders["topic"])
	assert.Equal(t, int64(110), orders["total_lag"])
	assert.Equal(t, 2, orders["partition_count"])
	assert.Equal(t, int64(100), orders["max_lag"])
}

func TestConsumerGroupLagGetTaskGroup(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		lagFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error) {
			return testGroupLag(), nil
		},
	}
	task := &consumerGroupLagGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor", "aggregate": "group"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	assert.Equal(t, int64(115), data["total_lag"])
	assert.Equal(t, 2, data["topic_count"])
	assert.Equal(t, int64(100), data["max_lag"])
}

func TestConsumerGroupLagGetTaskTopicFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		lagFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error) {
			return testGroupLag(), nil
		},
	}
	task := &consumerGroupLagGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor", "topics": []string{"orders"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}

func TestConsumerGroupLagGetTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		lagFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroupLags, error) {
			return nil, errors.New("lag failed")
		},
	}
	task := &consumerGroupLagGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "g"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "lag failed")
}
