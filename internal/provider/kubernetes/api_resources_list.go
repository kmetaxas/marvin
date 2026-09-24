package kubernetes

import (
	"context"
	"sort"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*apiResourcesListTask)(nil)

type apiResourcesListTask struct{ provider *Provider }

func (t *apiResourcesListTask) Name() string { return "kubernetes.api_resources.list" }

func (t *apiResourcesListTask) JSONSchema() string { return apiResourcesListSchema }

func (t *apiResourcesListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	apiGroupFilter, err := common.OptionalString(params, "api_group", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	namespacedFilter, err := common.OptionalBool(params, "namespaced", false)
	if err != nil {
		return common.TaskFailure(err)
	}
	hasNamespacedFilter := false
	if _, ok := params["namespaced"]; ok {
		hasNamespacedFilter = true
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	groups, resources, err := client.clientset.Discovery().ServerGroupsAndResources()
	if err != nil && len(groups) == 0 && len(resources) == 0 {
		return common.TaskFailure(err)
	}

	var items []apiGroupSummary
	for _, g := range groups {
		if apiGroupFilter != "" && g.Name != apiGroupFilter {
			continue
		}
		versions := make([]string, 0, len(g.Versions))
		for _, v := range g.Versions {
			versions = append(versions, v.GroupVersion)
		}
		preferred := ""
		if g.PreferredVersion.GroupVersion != "" {
			preferred = g.PreferredVersion.GroupVersion
		}

		var apiResources []apiResourceSummary
		for _, r := range resources {
			if r.GroupVersion == "" {
				continue
			}
			// Check if this resource list belongs to this group
			if !strings.HasPrefix(r.GroupVersion, g.Name) && g.Name != "" {
				// Core group resources have empty group name
				if !strings.Contains(r.GroupVersion, "/") && g.Name == "" {
					// core group
				} else {
					continue
				}
			}
			for _, res := range r.APIResources {
				if hasNamespacedFilter && res.Namespaced != namespacedFilter {
					continue
				}
				verbs := make([]string, len(res.Verbs))
				copy(verbs, res.Verbs)
				sort.Strings(verbs)
				apiResources = append(apiResources, apiResourceSummary{
					Name:         res.Name,
					SingularName: res.SingularName,
					Namespaced:   res.Namespaced,
					Kind:         res.Kind,
					Verbs:        verbs,
				})
			}
		}
		// If filtering by namespaced and group has no matching resources, skip it
		if hasNamespacedFilter && len(apiResources) == 0 {
			continue
		}
		items = append(items, apiGroupSummary{
			Name:             g.Name,
			Versions:         versions,
			PreferredVersion: preferred,
			Resources:        apiResources,
		})
	}

	return common.SuccessResult(map[string]any{"api_groups": items, "count": len(items)}), nil
}

const apiResourcesListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "API Resources List Parameters",
  "description": "Parameters for listing supported API resources.",
  "properties": {
    "api_group": {
      "type": "string",
      "description": "Filter to a specific API group. Omit to list all."
    },
    "namespaced": {
      "type": "boolean",
      "description": "Filter to namespaced or cluster-scoped resources. Omit for all."
    }
  }
}`

type apiGroupSummary struct {
	Name             string               `json:"name,omitempty"`
	Versions         []string             `json:"versions,omitempty"`
	PreferredVersion string               `json:"preferred_version,omitempty"`
	Resources        []apiResourceSummary `json:"resources,omitempty"`
}

type apiResourceSummary struct {
	Name         string   `json:"name,omitempty"`
	SingularName string   `json:"singular_name,omitempty"`
	Namespaced   bool     `json:"namespaced,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	Verbs        []string `json:"verbs,omitempty"`
}
