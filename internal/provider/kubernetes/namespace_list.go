package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*namespaceListTask)(nil)

type namespaceListTask struct{ provider *Provider }

func (t *namespaceListTask) Name() string { return "kubernetes.namespace.list" }

func (t *namespaceListTask) JSONSchema() string { return namespaceListSchema }

func (t *namespaceListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	labelSelector, err := common.OptionalString(params, "label_selector", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	fieldSelector, err := common.OptionalString(params, "field_selector", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
		Limit:         int64(limit),
	}

	list, err := client.clientset.CoreV1().Namespaces().List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []namespaceSummary
	for _, ns := range list.Items {
		items = append(items, flattenNamespace(ns))
	}

	return common.SuccessResult(map[string]any{"namespaces": items, "count": len(items)}), nil
}

func flattenNamespace(ns corev1.Namespace) namespaceSummary {
	return namespaceSummary{
		Name:   ns.Name,
		Status: string(ns.Status.Phase),
		Age:    formatAge(ns.CreationTimestamp.Time),
		Labels: ns.Labels,
	}
}

const namespaceListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Namespace List Parameters",
  "description": "Parameters for listing namespaces.",
  "properties": {
    "label_selector": {
      "type": "string",
      "description": "Label selector expression."
    },
    "field_selector": {
      "type": "string",
      "description": "Field selector expression."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of items to return.",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  }
}`

type namespaceSummary struct {
	Name   string            `json:"name,omitempty"`
	Status string            `json:"status,omitempty"`
	Age    string            `json:"age,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}
