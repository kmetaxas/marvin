package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestConsumerGroupMembersListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &consumerGroupMembersListTask{}
	assert.Equal(t, "kafka.consumer_group.members.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestConsumerGroupMembersListTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &consumerGroupMembersListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: group_id")
}

func TestConsumerGroupMembersListTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeGroupsFunc: func(ctx context.Context, groups ...string) (kadm.DescribedGroups, error) {
			return kadm.DescribedGroups{
				"order-processor": {
					Group: "order-processor",
					Members: []kadm.DescribedGroupMember{
						{
							MemberID:   "member-1",
							ClientID:   "consumer-1",
							ClientHost: "/10.0.0.1",
							Assigned:   consumerAssignment(kmsg.ConsumerMemberAssignmentTopic{Topic: "orders", Partitions: []int32{0, 1}}),
						},
						{
							MemberID:   "member-2",
							ClientID:   "consumer-2",
							ClientHost: "/10.0.0.2",
							Assigned:   consumerAssignment(kmsg.ConsumerMemberAssignmentTopic{Topic: "orders", Partitions: []int32{2, 3}}),
						},
					},
				},
			}, nil
		},
	}
	task := &consumerGroupMembersListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"group_id": "order-processor"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "order-processor", data["group_id"])
	assert.Equal(t, 2, data["member_count"])

	members := data["members"].([]map[string]any)
	require.Len(t, members, 2)
	assert.Equal(t, "member-1", members[0]["member_id"])
	assert.Equal(t, "consumer-1", members[0]["client_id"])
	assert.Equal(t, "/10.0.0.1", members[0]["client_host"])
}
