package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var _ task.Task = (*clusterApiVersionsGetTask)(nil)

type clusterApiVersionsGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *clusterApiVersionsGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *clusterApiVersionsGetTask) Name() string { return "kafka.cluster.api_versions.get" }

func (t *clusterApiVersionsGetTask) JSONSchema() string { return clusterApiVersionsGetSchema }

func (t *clusterApiVersionsGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	brokerID, err := OptionalInt32(params, "broker_id", -1)
	if err != nil {
		return common.TaskFailure(err)
	}
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	apiVers, err := t.currentClient().ApiVersions(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	apiVersions := make([]map[string]any, 0)
	for _, v := range apiVers.Sorted() {
		if brokerID >= 0 && v.NodeID != brokerID {
			continue
		}
		if v.Err != nil {
			continue
		}
		apis := make([]map[string]any, 0)
		v.EachKeySorted(func(key, min, max int16) {
			apis = append(apis, map[string]any{
				"key":         key,
				"name":        kmsg.NameForKey(key),
				"min_version": min,
				"max_version": max,
			})
		})
		apiVersions = append(apiVersions, map[string]any{
			"broker_id": v.NodeID,
			"apis":      apis,
		})
	}

	return common.SuccessResult(map[string]any{
		"api_versions": apiVersions,
		"count":        len(apiVersions),
	}), nil
}

const clusterApiVersionsGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Cluster API Versions Get Parameters",
  "description": "Query supported Kafka protocol API versions from brokers.",
  "properties": {
    "broker_id": {
      "type": "integer",
      "description": "Specific broker to query. Omit for all."
    }
  }
}`
