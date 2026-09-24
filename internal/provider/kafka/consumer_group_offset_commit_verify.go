package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*consumerGroupOffsetCommitVerifyTask)(nil)

type consumerGroupOffsetCommitVerifyTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *consumerGroupOffsetCommitVerifyTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *consumerGroupOffsetCommitVerifyTask) Name() string {
	return "kafka.consumer_group.offset_commit.verify"
}

func (t *consumerGroupOffsetCommitVerifyTask) JSONSchema() string {
	return consumerGroupOffsetCommitVerifySchema
}

func (t *consumerGroupOffsetCommitVerifyTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	groupID, err := common.RequireString(params, "group_id")
	if err != nil {
		return common.TaskFailure(err)
	}
	timeoutMs, err := OptionalInt32(params, "timeout_ms", 10000)
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	responses := t.currentClient().FindGroupCoordinators(ctx, groupID)
	resp, ok := responses[groupID]
	if !ok {
		return common.TaskFailure(fmt.Errorf("no coordinator response for group %s", groupID))
	}
	if resp.Err != nil {
		return common.TaskFailure(resp.Err)
	}

	coordinator := BrokerInfo{ID: resp.NodeID, Host: resp.Host, Port: resp.Port}

	// Read-only OffsetFetch to verify connectivity and permissions. This never
	// commits offsets.
	_, err = t.currentClient().FetchOffsets(ctx, groupID)
	if err != nil {
		return common.SuccessResult(map[string]any{
			"group_id":              groupID,
			"coordinator_reachable": true,
			"coordinator":           coordinator,
			"protocol_check":        "passed",
			"auth_check":            "failed",
			"would_succeed":         false,
			"elapsed_ms":            time.Since(start).Milliseconds(),
		}), nil
	}

	return common.SuccessResult(map[string]any{
		"group_id":              groupID,
		"coordinator_reachable": true,
		"coordinator":           coordinator,
		"protocol_check":        "passed",
		"auth_check":            "passed",
		"would_succeed":         true,
		"elapsed_ms":            time.Since(start).Milliseconds(),
	}), nil
}

const consumerGroupOffsetCommitVerifySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Consumer Group Offset Commit Verify Parameters",
  "description": "Perform a non-mutating protocol path check up to coordinator discovery/metadata/auth, but never commit.",
  "properties": {
    "group_id": {
      "type": "string"
    },
    "timeout_ms": {
      "type": "integer",
      "minimum": 1000,
      "maximum": 60000,
      "default": 10000,
      "description": "Timeout for coordinator discovery and connectivity test."
    }
  },
  "required": ["group_id"]
}`
