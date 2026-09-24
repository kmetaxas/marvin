package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*clusterDescribeTask)(nil)

type clusterDescribeTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *clusterDescribeTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *clusterDescribeTask) Name() string { return "kafka.cluster.describe" }

func (t *clusterDescribeTask) JSONSchema() string { return clusterDescribeSchema }

func (t *clusterDescribeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	meta, err := t.currentClient().Metadata(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	// Best-effort version inference from API versions.
	version := ""
	if apiVers, err := t.currentClient().ApiVersions(ctx); err == nil {
		for _, v := range apiVers.Sorted() {
			if v.Err == nil {
				version = v.VersionGuess()
				break
			}
		}
	}

	brokers := brokersToInfo(meta.Brokers, meta.Controller)

	return common.SuccessResult(map[string]any{
		"cluster_id":    meta.Cluster,
		"brokers":       brokers,
		"controller_id": meta.Controller,
		"version":       version,
		"broker_count":  len(brokers),
	}), nil
}

const clusterDescribeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Cluster Describe Parameters",
  "description": "Describe basic cluster identity and metadata.",
  "properties": {}
}`
