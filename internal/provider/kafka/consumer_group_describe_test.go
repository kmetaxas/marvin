package kafka

import (
	"context"
	"errors"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func consumerAssignment(topics ...kmsg.ConsumerMemberAssignmentTopic) kadm.GroupMemberAssignment {
	a := kadm.GroupMemberAssignment{}
	*(*any)(unsafe.Pointer(&a)) = &kmsg.ConsumerMemberAssignment{Topics: topics}
	return a
}

func TestConsumerGroupDescribeTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupDescribeTask{}
	assert.Equal(t, "kafka.consumer_group.describe", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupDescribeTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")
}

func TestConsumerGroupDescribeTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return kadm.DescribedGroups{
				"order-processor": {
					Group:        "order-processor",
					State:        "Stable",
					Protocol:     "range",
					ProtocolType: "consumer",
					Coordinator:  kadm.BrokerDetail{NodeID: 1, Host: "kafka-1", Port: 9092},
					Members: []kadm.DescribedGroupMember{
						{
							MemberID:   "member-1",
							ClientID:   "consumer-1",
							ClientHost: "/10.0.0.1",
							Assigned:   consumerAssignment(kmsg.ConsumerMemberAssignmentTopic{Topic: "orders", Partitions: []int32{0, 1, 2}}),
						},
					},
				},
			}, nil
		},
	}
	task := &consumerGroupDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	assert.Equal(t, "Stable", data["state"])
	assert.Equal(t, "range", data["protocol"])
	assert.Equal(t, 1, data["member_count"])

	coordinator := data["coordinator"].(BrokerInfo)
	assert.Equal(t, int32(1), coordinator.ID)

	members := data["members"].([]map[string]any)
	require.Len(t, members, 1)
	assert.Equal(t, "member-1", members[0]["member_id"])
	assignments := members[0]["assignment"].([]map[string]any)
	require.Len(t, assignments, 1)
	assert.Equal(t, "orders", assignments[0]["topic"])
}

func TestConsumerGroupDescribeTaskNotFound(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return kadm.DescribedGroups{}, nil
		},
	}
	task := &consumerGroupDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}

func TestConsumerGroupDescribeTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &consumerGroupDescribeTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "g"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
