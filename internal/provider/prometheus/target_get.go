package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*targetGetTask)(nil)

type targetGetTask struct {
	provider *Provider
}

func (t *targetGetTask) Name() string { return "prometheus.target.get" }

func (t *targetGetTask) JSONSchema() string { return targetGetSchema }

func (t *targetGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	job, err := common.OptionalString(params, "job", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	instance, err := common.OptionalString(params, "instance", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	scrapePool, err := common.OptionalString(params, "scrape_pool", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	if job == "" && instance == "" && scrapePool == "" {
		return common.TaskFailure(fmt.Errorf("at least one of job, instance, scrape_pool must be provided"))
	}

	result, err := client.Targets(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	for _, tgt := range result.Active {
		if job != "" && string(tgt.Labels["job"]) != job {
			continue
		}
		if instance != "" && string(tgt.Labels["instance"]) != instance {
			continue
		}
		if scrapePool != "" && tgt.ScrapePool != scrapePool {
			continue
		}
		return common.SuccessResult(map[string]any{
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
		}), nil
	}

	return common.TaskFailure(fmt.Errorf("target not found"))
}

const targetGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Target Get Parameters",
  "description": "Get detailed information for a single target.",
  "properties": {
    "job": {
      "type": "string",
      "description": "Job label of the target."
    },
    "instance": {
      "type": "string",
      "description": "Instance label of the target."
    },
    "scrape_pool": {
      "type": "string",
      "description": "Scrape pool name of the target."
    }
  }
}`
