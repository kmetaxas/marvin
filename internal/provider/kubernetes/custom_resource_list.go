package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*customResourceListTask)(nil)

type customResourceListTask struct{ provider *Provider }

func (t *customResourceListTask) Name() string { return "kubernetes.custom_resource.list" }

func (t *customResourceListTask) JSONSchema() string { return customResourceListSchema }

func (t *customResourceListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	apiGroup, err := common.RequireString(params, "api_group")
	if err != nil {
		return common.TaskFailure(err)
	}
	version, err := common.RequireString(params, "version")
	if err != nil {
		return common.TaskFailure(err)
	}
	resource, err := common.RequireString(params, "resource")
	if err != nil {
		return common.TaskFailure(err)
	}
	namespace, err := common.OptionalString(params, "namespace", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.dynamicClient == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	list, err := client.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  version,
		Resource: resource,
	}).Namespace(namespace).List(ctx, metav1.ListOptions{Limit: int64(limit)})
	if err != nil {
		return common.TaskFailure(err)
	}

	items := make([]map[string]any, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, item.Object)
	}

	return common.SuccessResult(map[string]any{"items": items, "count": len(items)}), nil
}

const customResourceListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Custom Resource List Parameters",
  "description": "Parameters for listing arbitrary Kubernetes custom resources using the dynamic API client.",
  "properties": {
    "api_group": {
      "type": "string",
      "description": "API group of the custom resource, such as cert-manager.io."
    },
    "version": {
      "type": "string",
      "description": "API version of the custom resource, such as v1."
    },
    "resource": {
      "type": "string",
      "description": "Plural resource name, such as certificates."
    },
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit or leave empty to list across all namespaces."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of custom resources to return.",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  },
  "required": ["api_group", "version", "resource"]
}`
