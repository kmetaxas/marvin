package kafka

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"

	"github.com/marvin-agent/marvin/internal/task"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var _ task.Task = (*clusterFeaturesGetTask)(nil)

type clusterFeaturesGetTask struct {
	client   KafkaClient
	provider *Provider
}
func (t *clusterFeaturesGetTask) currentClient() KafkaClient {
	if t.provider != nil {
		return t.provider.CurrentClient()
	}
	return t.client
}


func (t *clusterFeaturesGetTask) Name() string { return "kafka.cluster.features.get" }

func (t *clusterFeaturesGetTask) JSONSchema() string { return clusterFeaturesGetSchema }

func (t *clusterFeaturesGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	if t.currentClient() == nil {
		return common.TaskFailure(fmt.Errorf("kafka client is not configured"))
	}

	// Feature metadata (KIP-584) is exposed via the ApiVersions response.
	// Issue the low-level request directly so we can read the supported and
	// finalized feature lists, and fall back gracefully on brokers that do
	// not support the feature fields.
	req := kmsg.NewPtrApiVersionsRequest()
	resp, err := t.currentClient().Request(ctx, req)
	if err != nil {
		return common.TaskFailure(err)
	}

	avResp, ok := resp.(*kmsg.ApiVersionsResponse)
	if !ok {
		return common.TaskFailure(fmt.Errorf("unexpected response type %T", resp))
	}
	if codeErr := kerr.ErrorForCode(avResp.ErrorCode); codeErr != nil {
		return common.TaskFailure(codeErr)
	}

	features := make([]map[string]any, 0)

	// Prefer cluster-wide finalized features when present.
	if avResp.FinalizedFeaturesEpoch >= 0 {
		for _, f := range avResp.FinalizedFeatures {
			features = append(features, map[string]any{
				"name":                    f.Name,
				"min_version_level":       f.MinVersionLevel,
				"max_version_level":       f.MaxVersionLevel,
				"finalized_version_level": f.MaxVersionLevel,
			})
		}
	} else {
		// Fall back to per-broker supported features.
		for _, f := range avResp.SupportedFeatures {
			features = append(features, map[string]any{
				"name":              f.Name,
				"min_version_level": f.MinVersion,
				"max_version_level": f.MaxVersion,
			})
		}
	}

	return common.SuccessResult(map[string]any{
		"features": features,
		"count":    len(features),
	}), nil
}

const clusterFeaturesGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Cluster Features Get Parameters",
  "description": "Retrieve Kafka feature/version metadata such as finalized features.",
  "properties": {}
}`
