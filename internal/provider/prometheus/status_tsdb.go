package prometheus

import (
	"context"
	"fmt"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statusTsdbTask)(nil)

type statusTsdbTask struct {
	provider *Provider
}

func (t *statusTsdbTask) Name() string { return "prometheus.status.tsdb" }

func (t *statusTsdbTask) JSONSchema() string { return statusTsdbSchema }

func (t *statusTsdbTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	limit, err := common.OptionalInt(params, "limit", 0)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit < 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than or equal to 0"))
	}
	if limit > guardrails.MaxMetadataResults {
		limit = guardrails.MaxMetadataResults
	}

	result, err := client.TSDB(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	head := map[string]any{
		"num_series":      result.HeadStats.NumSeries,
		"num_label_pairs": result.HeadStats.NumLabelPairs,
		"chunk_count":     result.HeadStats.ChunkCount,
		"min_time":        result.HeadStats.MinTime,
		"max_time":        result.HeadStats.MaxTime,
	}

	seriesByMetric := statsToMaps(result.SeriesCountByMetricName, limit)
	labelValueCount := statsToMaps(result.LabelValueCountByLabelName, limit)
	memoryByLabel := statsToMaps(result.MemoryInBytesByLabelName, limit)
	seriesByLabelPair := statsToMaps(result.SeriesCountByLabelValuePair, limit)

	truncated := limit > 0 && (len(result.SeriesCountByMetricName) > limit ||
		len(result.LabelValueCountByLabelName) > limit ||
		len(result.MemoryInBytesByLabelName) > limit ||
		len(result.SeriesCountByLabelValuePair) > limit)

	return common.SuccessResult(map[string]any{
		"head_stats":                       head,
		"series_count_by_metric_name":      seriesByMetric,
		"label_value_count_by_label_name":  labelValueCount,
		"memory_in_bytes_by_label_name":    memoryByLabel,
		"series_count_by_label_value_pair": seriesByLabelPair,
		"truncated":                        truncated,
	}), nil
}

// statsToMaps converts a slice of v1.Stat into maps, truncating to limit when
// limit is greater than zero.
func statsToMaps(stats []v1.Stat, limit int) []map[string]any {
	if limit > 0 && len(stats) > limit {
		stats = stats[:limit]
	}
	out := make([]map[string]any, 0, len(stats))
	for _, s := range stats {
		out = append(out, map[string]any{
			"name":  s.Name,
			"value": s.Value,
		})
	}
	return out
}

const statusTsdbSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus TSDB Status Parameters",
  "description": "Get Prometheus TSDB statistics and cardinality.",
  "properties": {
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of top-N cardinality entries to return."
    }
  }
}`
