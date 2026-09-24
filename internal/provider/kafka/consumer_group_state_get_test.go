package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestConsumerGroupStateGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupStateGetTask{}
	assert.Equal(t, "kafka.consumer_group.state.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupStateGetTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupStateGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")
}

func TestConsumerGroupStateGetTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return kadm.DescribedGroups{
				"order-processor": {
					Group:    "order-processor",
					State:    "Stable",
					Protocol: "range",
					Members: []kadm.DescribedGroupMember{
						{MemberID: "m1"},
						{MemberID: "m2"},
						{MemberID: "m3"},
					},
				},
			}, nil
		},
	}
	task := &consumerGroupStateGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	assert.Equal(t, "Stable", data["state"])
	assert.Equal(t, "range", data["protocol"])
	assert.Equal(t, 3, data["member_count"])
	assert.NotEmpty(t, data["timestamp"])
}

func TestConsumerGroupStateGetTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &consumerGroupStateGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "g"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
