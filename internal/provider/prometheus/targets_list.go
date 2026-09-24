package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*targetsListTask)(nil)

type targetsListTask struct {
	provider *Provider
}

func (t *targetsListTask) Name() string { return "prometheus.targets.list" }

func (t *targetsListTask) JSONSchema() string { return targetsListSchema }

func (t *targetsListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	guardrails := client.Guardrails()
	guardrails.ApplyDefaults()

	state, err := common.OptionalString(params, "state", "any")
	if err != nil {
		return common.TaskFailure(err)
	}
	switch state {
	case "active", "dropped", "any":
	default:
		return common.TaskFailure(fmt.Errorf("parameter state must be one of active, dropped, any"))
	}

	job, err := common.OptionalString(params, "job", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	instance, err := common.OptionalString(params, "instance", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	limit, err := common.OptionalInt(params, "limit", guardrails.MaxReturnedSeries)
	if err != nil {
		return common.TaskFailure(err)
	}
	if limit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter limit must be greater than 0"))
	}
	if limit > guardrails.MaxReturnedSeries {
		limit = guardrails.MaxReturnedSeries
	}

	result, err := client.Targets(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	targets := make([]map[string]any, 0, len(result.Active)+len(result.Dropped))

	if state == "active" || state == "any" {
		for _, tgt := range result.Active {
			if job != "" && string(tgt.Labels["job"]) != job {
				continue
			}
			if instance != "" && string(tgt.Labels["instance"]) != instance {
				continue
			}
			targets = append(targets, map[string]any{
				"scrape_url":         tgt.ScrapeURL,
				"job":                string(tgt.Labels["job"]),
				"instance":           string(tgt.Labels["instance"]),
				"health":             string(tgt.Health),
				"last_scrape":        formatTime(tgt.LastScrape),
				"scrape_duration_ms": tgt.LastScrapeDuration * 1000,
				"last_error":         tgt.LastError,
				"discovered_labels":  tgt.DiscoveredLabels,
				"labels":             labelSetToMap(tgt.Labels),
				"scrape_pool":        tgt.ScrapePool,
			})
		}
	}

	if state == "dropped" || state == "any" {
		for _, tgt := range result.Dropped {
			if job != "" && tgt.DiscoveredLabels["job"] != job {
				continue
			}
			if instance != "" && tgt.DiscoveredLabels["instance"] != instance {
				continue
			}
			targets = append(targets, map[string]any{
				"scrape_url":         "",
				"job":                tgt.DiscoveredLabels["job"],
				"instance":           tgt.DiscoveredLabels["instance"],
				"health":             "dropped",
				"last_scrape":        "",
				"scrape_duration_ms": float64(0),
				"last_error":         "",
				"discovered_labels":  tgt.DiscoveredLabels,
				"labels":             map[string]string{},
			})
		}
	}

	truncated := false
	if len(targets) > limit {
		targets = targets[:limit]
		truncated = true
	}

	return common.SuccessResult(map[string]any{
		"targets":   targets,
		"count":     len(targets),
		"truncated": truncated,
	}), nil
}

const targetsListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Targets List Parameters",
  "description": "List scrape targets and their health.",
  "properties": {
    "state": {
      "type": "string",
      "enum": ["active", "dropped", "any"],
      "default": "any",
      "description": "Filter targets by state."
    },
    "job": {
      "type": "string",
      "description": "Filter by job label."
    },
    "instance": {
      "type": "string",
      "description": "Filter by instance label."
    },
    "limit": {
      "type": "integer",
      "minimum": 1,
      "description": "Maximum number of targets to return."
    }
  }
}`
