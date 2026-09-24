package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestConsumerGroupAssignmentGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupAssignmentGetTask{}
	assert.Equal(t, "kafka.consumer_group.assignment.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupAssignmentGetTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupAssignmentGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")
}

func TestConsumerGroupAssignmentGetTaskBalanced(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return kadm.DescribedGroups{
				"order-processor": {
					Group: "order-processor",
					Members: []kadm.DescribedGroupMember{
						{MemberID: "member-1", ClientID: "consumer-1", Assigned: consumerAssignment(kmsg.ConsumerMemberAssignmentTopic{Topic: "orders", Partitions: []int32{0, 1}})},
						{MemberID: "member-2", ClientID: "consumer-2", Assigned: consumerAssignment(kmsg.ConsumerMemberAssignmentTopic{Topic: "orders", Partitions: []int32{2, 3}})},
					},
				},
			}, nil
		},
	}
	task := &consumerGroupAssignmentGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	assert.False(t, data["imbalance_detected"].(bool))
	assert.Equal(t, 0.0, data["stddev_partitions"].(float64))

	members := data["members"].([]map[string]any)
	require.Len(t, members, 2)
	assert.Equal(t, "member-1", members[0]["member_id"])
	assert.Equal(t, 2, members[0]["assigned_partitions"])
	topics := members[0]["topics"].([]string)
	assert.Equal(t, []string{"orders"}, topics)
}

func TestConsumerGroupAssignmentGetTaskImbalanced(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return kadm.DescribedGroups{
				"order-processor": {
					Group: "order-processor",
					Members: []kadm.DescribedGroupMember{
						{MemberID: "member-1", Assigned: consumerAssignment(kmsg.ConsumerMemberAssignmentTopic{Topic: "orders", Partitions: []int32{0, 1, 2, 3}})},
						{MemberID: "member-2", Assigned: consumerAssignment(kmsg.ConsumerMemberAssignmentTopic{Topic: "orders", Partitions: []int32{4}})},
					},
				},
			}, nil
		},
	}
	task := &consumerGroupAssignmentGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.True(t, data["imbalance_detected"].(bool))
	assert.Greater(t, data["stddev_partitions"].(float64), 0.0)
}

func TestConsumerGroupAssignmentGetTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &consumerGroupAssignmentGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "g"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
