package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestConsumerGroupListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupListTask{}
	assert.Equal(t, "kafka.consumer_group.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupListTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &consumerGroupListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestConsumerGroupListTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listGroupsFunc: func(ctx context.Context, states ...string) (kadm.ListedGroups, error) {
			return kadm.ListedGroups{
				"order-processor":  {Group: "order-processor", State: "Stable", ProtocolType: "consumer"},
				"payment-consumer": {Group: "payment-consumer", State: "Empty", ProtocolType: "consumer"},
			}, nil
		},
	}
	task := &consumerGroupListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	assert.False(t, data["truncated"].(bool))

	groups := data["groups"].([]ConsumerGroupSummary)
	require.Len(t, groups, 2)
	assert.Equal(t, "order-processor", groups[0].GroupID)
	assert.Equal(t, "Stable", groups[0].State)
}

func TestConsumerGroupListTaskPatternFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listGroupsFunc: func(ctx context.Context, states ...string) (kadm.ListedGroups, error) {
			return kadm.ListedGroups{
				"order-processor":  {Group: "order-processor", State: "Stable"},
				"payment-consumer": {Group: "payment-consumer", State: "Empty"},
			}, nil
		},
	}
	task := &consumerGroupListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"pattern": "order-*"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	groups := data["groups"].([]ConsumerGroupSummary)
	require.Len(t, groups, 1)
	assert.Equal(t, "order-processor", groups[0].GroupID)
}

func TestConsumerGroupListTaskLimit(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listGroupsFunc: func(ctx context.Context, states ...string) (kadm.ListedGroups, error) {
			return kadm.ListedGroups{
				"g1": {Group: "g1", State: "Stable"},
				"g2": {Group: "g2", State: "Stable"},
				"g3": {Group: "g3", State: "Stable"},
			}, nil
		},
	}
	task := &consumerGroupListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"limit": 2})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	assert.True(t, data["truncated"].(bool))
}

func TestConsumerGroupListTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listGroupsFunc: func(ctx context.Context, states ...string) (kadm.ListedGroups, error) {
			return nil, errors.New("list failed")
		},
	}
	task := &consumerGroupListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "list failed")
}
