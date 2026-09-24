package kafka

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupAssignmentGetTask)(nil)

type consumerGroupAssignmentGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupAssignmentGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupAssignmentGetTask) Name() string { return "kafka.consumer_group.assignment.get" }

func (t *consumerGroupAssignmentGetTask) JSONSchema() string { return consumerGroupAssignmentGetSchema }

func (t *consumerGroupAssignmentGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	groupID, err := common.RequireString(params, "group_id")
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	described, err := t.currentClient().DescribeGroups(ctx, groupID)
	if err != nil {
		return common.TaskFailure(err)
	}

	group, err := described.On(groupID, nil)
	if err != nil {
		return common.TaskFailure(err)
	}
	if group.Err != nil {
		return common.TaskFailure(group.Err)
	}

	members := make([]map[string]any, 0, len(group.Members))
	counts := make([]int, 0, len(group.Members))
	for _, m := range group.Members {
		assignments, total := memberAssignments(m)
		topics := make([]string, 0, len(assignments))
		for _, a := range assignments {
			topics = append(topics, a["topic"].(string))
		}
		sort.Strings(topics)
		members = append(members, map[string]any{
			"member_id":           m.MemberID,
			"client_id":           m.ClientID,
			"assigned_partitions": total,
			"topics":              topics,
		})
		counts = append(counts, total)
	}

	imbalance := false
	stddev := 0.0
	if len(counts) > 1 {
		mean := 0.0
		for _, c := range counts {
			mean += float64(c)
		}
		mean /= float64(len(counts))

		variance := 0.0
		for _, c := range counts {
			d := float64(c) - mean
			variance += d * d
		}
		variance /= float64(len(counts))
		stddev = math.Sqrt(variance)

		min, max := counts[0], counts[0]
		for _, c := range counts {
			if c < min {
				min = c
			}
			if c > max {
				max = c
			}
		}
		imbalance = max-min > 1
	}

	return common.SuccessResult(map[string]any{
		"group_id":           group.Group,
		"members":            members,
		"imbalance_detected": imbalance,
		"stddev_partitions":  stddev,
	}), nil
}

const consumerGroupAssignmentGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group Assignment Get Parameters",
  "description": "Return assignment distribution among members.",
  "properties": {
    "group_id": {
      "type": "string"
    }
  },
  "required": ["group_id"]
}`
