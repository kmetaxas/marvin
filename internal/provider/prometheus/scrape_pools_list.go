package prometheus

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*scrapePoolsListTask)(nil)

type scrapePoolsListTask struct {
	provider *Provider
}

func (t *scrapePoolsListTask) Name() string { return "prometheus.scrape_pools.list" }

func (t *scrapePoolsListTask) JSONSchema() string { return scrapePoolsListSchema }

func (t *scrapePoolsListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil {
		return common.TaskFailure(fmt.Errorf("prometheus client is not configured"))
	}

	includeCounts, err := common.OptionalBool(params, "include_counts", true)
	if err != nil {
		return common.TaskFailure(err)
	}

	result, err := client.Targets(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	type poolCounts struct {
		total     int
		healthy   int
		unhealthy int
		dropped   int
	}

	pools := make(map[string]*poolCounts)
	for _, tgt := range result.Active {
		p := pools[tgt.ScrapePool]
		if p == nil {
			p = &poolCounts{}
			pools[tgt.ScrapePool] = p
		}
		p.total++
		if tgt.Health == "up" {
			p.healthy++
		} else {
			p.unhealthy++
		}
	}
	for _, tgt := range result.Dropped {
		pool := tgt.DiscoveredLabels["scrape_pool"]
		if pool == "" {
			pool = tgt.DiscoveredLabels["job"]
		}
		p := pools[pool]
		if p == nil {
			p = &poolCounts{}
			pools[pool] = p
		}
		p.total++
		p.dropped++
	}

	items := make([]map[string]any, 0, len(pools))
	for name, c := range pools {
		item := map[string]any{"name": name}
		if includeCounts {
			item["total"] = c.total
			item["healthy"] = c.healthy
			item["unhealthy"] = c.unhealthy
			item["dropped"] = c.dropped
		}
		items = append(items, item)
	}

	return common.SuccessResult(map[string]any{
		"pools": items,
		"count": len(items),
	}), nil
}

const scrapePoolsListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Prometheus Scrape Pools List Parameters",
  "description": "List scrape pools with target counts.",
  "properties": {
    "include_counts": {
      "type": "boolean",
      "default": true,
      "description": "Whether to include healthy/unhealthy/dropped counts."
    }
  }
}`
